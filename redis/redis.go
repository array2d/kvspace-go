// Package redis 提供 kvspace.KVSpace 的 Redis 实现。
package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/array2d/kvspace-go"
	goredis "github.com/redis/go-redis/v9"
)

func init() { kvspace.Register("redis", ConnPool) }

var bg = context.Background()

func Conn(dsn string) kvspace.KVSpace { return ConnPool(dsn) }

func ConnPool(dsn string) kvspace.KVSpace {
	poolSize := 16
	return &redisImpl{
		rdb: goredis.NewClient(&goredis.Options{
			Addr: dsn, PoolSize: poolSize,
			MinIdleConns: 4,
			PoolTimeout:  10 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second,
		}),
	}
}

type redisImpl struct {
	rdb *goredis.Client
	mu  sync.Mutex
}

// ── KVSpace interface ──────────────────────────────────────────────────────────

func (r *redisImpl) Get(prefix string, keys []string) []kvspace.XValue {
	chain, sets := r.resolveSets(dirKey(prefix))

	// 为每个 key 定位实际存储路径
	paths := make([]string, len(keys))
	for i, k := range keys {
		paths[i] = findInSets(prefix, k, chain, sets)
	}

	// pipeline GET
	pipe := r.rdb.Pipeline()
	cmds := make([]*goredis.StringCmd, len(keys))
	for i, p := range paths {
		if p != "" {
			cmds[i] = pipe.Get(bg, p)
		}
	}
	_, _ = pipe.Exec(bg)

	result := make([]kvspace.XValue, len(keys))
	for i, cmd := range cmds {
		if cmd == nil {
			continue
		}
		raw, _ := cmd.Bytes()
		result[i] = kvspace.DecodeXValue(raw)
	}
	return result
}

func (r *redisImpl) Set(pairs []kvspace.KVPair) error {
	if len(pairs) == 0 {
		return nil
	}
	pipe := r.rdb.Pipeline()
	for _, p := range pairs {
		if strings.HasSuffix(p.Key, kvspace.PathSep) {
			return fmt.Errorf("kvspace: key must not end with %s: %s", kvspace.PathSep, p.Key)
		}
		prefix, last := kvspace.SepPath(p.Key)
		dk := dirKey(prefix)
		chain := r.resolveChain(dk)

		if len(chain) > 1 && kvspace.IsLink(r.getXV(prefix)) {
			// 纯链接：写入终端层
			writeIdx := chain[len(chain)-1]
			writePath := kvspace.JoinPath(kvspace.StripDirSuf(writeIdx), last)
			pipe.SAdd(bg, writeIdx, last)
			pipe.Set(bg, writePath, kvspace.EncodeXValue(p.Val), 0)
			pipeIndex(pipe, writePath)
		} else {
			// 普通路径或 extindex：写入当前层
			pipeIndex(pipe, p.Key)
			pipe.Set(bg, p.Key, kvspace.EncodeXValue(p.Val), 0)
		}
	}
	_, err := pipe.Exec(bg)
	return err
}

func (r *redisImpl) List(prefix string) []string {
	_, sets := r.resolveSets(dirKey(prefix))
	seen := map[string]bool{}
	var result []string
	for _, members := range sets {
		for _, m := range members {
			if strings.HasPrefix(m, kvspace.ReservedPrefix) || seen[m] {
				continue
			}
			seen[m] = true
			result = append(result, m)
		}
	}
	return result
}

func (r *redisImpl) Del(keys ...string) error {
	pipe := r.rdb.Pipeline()
	for _, k := range keys {
		prefix, last := kvspace.SepPath(k)
		dk := dirKey(prefix)
		chain, sets := r.resolveSets(dk)

		if len(chain) > 1 {
			// prefix 是 extindex：在链中定位末段归属层
			layer, _ := findLayer(last, sets)
			if layer >= 0 {
				idx := chain[layer]
				targetKey := kvspace.JoinPath(kvspace.StripDirSuf(idx), last)
				pipe.Del(bg, targetKey)
				pipe.SRem(bg, idx, last)
				continue
			}
		}
		// 普通删除
		pipe.Del(bg, k)
		pipe.SRem(bg, dk, last)
	}
	_, err := pipe.Exec(bg)
	if err != nil {
		return err
	}
	for _, k := range keys {
		r.delIndex(k)
	}
	return nil
}

func (r *redisImpl) DelTree(prefix string) error {
	resolved := r.resolveTreeTarget(prefix)
	if resolved != prefix {
		// 纯链接：只删链接本体
		return r.UnLink(prefix)
	}
	r.delRecursive(resolved)
	r.delIndex(resolved)
	return nil
}

func (r *redisImpl) Watch(key string, timeout time.Duration) kvspace.XValue {
	resolved := r.resolveKey(key)
	vals, err := r.rdb.BLPop(bg, timeout, resolved).Result()
	if err != nil || len(vals) < 2 {
		return kvspace.XValue{}
	}
	return kvspace.DecodeXValue([]byte(vals[1]))
}

func (r *redisImpl) Notify(key string, val kvspace.XValue) error {
	return r.rdb.LPush(bg, r.resolveKey(key), kvspace.EncodeXValue(val)).Err()
}

// ── mount ─────────────────────────────────────────────────────────────────────
func (r *redisImpl) Link(target, linkpath string) error {
	val := kvspace.NewLinkValue(target + kvspace.DirIndexSuf)
	pipe := r.rdb.Pipeline()
	pipe.Set(bg, linkpath, kvspace.EncodeXValue(val), 0)
	pipe.SAdd(bg, dirKey(linkpath), kvspace.EncodeExtEntry(target))
	_, err := pipe.Exec(bg)
	if err != nil {
		return err
	}
	r.addIndex(linkpath)
	return nil
}

func (r *redisImpl) ExtIndex(path, extpath string) error {
	val := kvspace.NewExtIndexValue(extpath + kvspace.DirIndexSuf)
	pipe := r.rdb.Pipeline()
	pipe.Set(bg, path, kvspace.EncodeXValue(val), 0)
	pipe.SAdd(bg, dirKey(path), kvspace.EncodeExtEntry(extpath))
	_, err := pipe.Exec(bg)
	if err != nil {
		return err
	}
	r.addIndex(path)
	return nil
}

func (r *redisImpl) UnLink(path string) error {
	pipe := r.rdb.Pipeline()
	pipe.Del(bg, path)
	pipe.Del(bg, dirKey(path))
	_, err := pipe.Exec(bg)
	if err != nil {
		return err
	}
	r.delIndex(path)
	return nil
}

func (r *redisImpl) Clear() error   { return r.rdb.FlushDB(bg).Err() }
func (r *redisImpl) DisConn() error { return r.rdb.Close() }

// ── 索引链解析（内部）──────────────────────────────────────────────────────────

func (r *redisImpl) resolveChain(dirKey string) []string {
	chain := []string{dirKey}
	if ext, ok := kvspace.ResolveExt(dirKey, func(dk string) []string {
		return r.rdb.SMembers(bg, dk).Val()
	}); ok {
		chain = append(chain, ext)
	}
	return chain
}

func (r *redisImpl) resolveSets(dirKey string) ([]string, [][]string) {
	chain := r.resolveChain(dirKey)
	sets := make([][]string, len(chain))
	for i, dk := range chain {
		sets[i] = r.rdb.SMembers(bg, dk).Val()
	}
	return chain, sets
}

// resolveKey 解析单个 key：若 prefix 是纯链接则穿透到终端。
func (r *redisImpl) resolveKey(key string) string {
	prefix, last := kvspace.SepPath(key)
	dk := dirKey(prefix)
	chain := r.resolveChain(dk)
	if len(chain) > 1 && kvspace.IsLink(r.getXV(prefix)) {
		idx := chain[len(chain)-1]
		return kvspace.JoinPath(kvspace.StripDirSuf(idx), last)
	}
	return key
}

// resolveTreeTarget 返回 DelTree 的实际操作目标。纯链接返回自身（删链接本体）。
func (r *redisImpl) resolveTreeTarget(prefix string) string {
	dk := dirKey(prefix)
	chain := r.resolveChain(dk)
	if len(chain) > 1 && kvspace.IsLink(r.getXV(prefix)) {
		return prefix
	}
	return prefix
}

// ── 辅助函数 ───────────────────────────────────────────────────────────────────

// findInSets 在索引链中定位 key，返回实际存储路径。未找到返 ""。
func findInSets(prefix, key string, chain []string, sets [][]string) string {
	for i, members := range sets {
		for _, m := range members {
			if m == key {
				idx := chain[i]
				return kvspace.JoinPath(kvspace.StripDirSuf(idx), key)
			}
		}
	}
	return ""
}

// findLayer 在 sets 中定位 key 所属层级索引。未找到返 -1。
func findLayer(key string, sets [][]string) (int, int) {
	for i, members := range sets {
		for j, m := range members {
			if m == key {
				return i, j
			}
		}
	}
	return -1, -1
}

// getXV 读取 path 的 XValue，不存在返零值。
func (r *redisImpl) getXV(path string) kvspace.XValue {
	raw, _ := r.rdb.Get(bg, path).Bytes()
	return kvspace.DecodeXValue(raw)
}

// dirKey 返回目录索引键。
func dirKey(parent string) string {
	if parent == kvspace.PathSep {
		return kvspace.PathSep
	}
	return parent + kvspace.DirIndexSuf
}

// ── 索引维护 ───────────────────────────────────────────────────────────────────

func (r *redisImpl) delRecursive(prefix string) {
	children, _ := r.rdb.SMembers(bg, dirKey(prefix)).Result()
	for _, c := range children {
		if kvspace.DecodeExtEntry(c) != "" {
			continue // extindex 条目：不递归删除 ext 目标
		}
		r.delRecursive(kvspace.JoinPath(prefix, c))
	}
	r.rdb.Del(bg, prefix, dirKey(prefix))
}

func (r *redisImpl) addIndex(key string) {
	prefix := ""
	for _, p := range strings.Split(key, kvspace.PathSep)[1:] {
		if p == "" {
			break
		}
		parent := prefix
		if parent == "" {
			parent = kvspace.PathSep
		}
		r.rdb.SAdd(bg, dirKey(parent), p)
		prefix += kvspace.PathSep + p
	}
}

func (r *redisImpl) delIndex(key string) {
	for key != "" && key != kvspace.PathSep {
		if n, _ := r.rdb.SCard(bg, dirKey(key)).Result(); n > 0 {
			return
		}
		slash := strings.LastIndexByte(key, '/')
		if slash < 0 {
			return
		}
		parent := key[:slash]
		idxParent := parent
		if idxParent == "" {
			idxParent = kvspace.PathSep
		}
		r.rdb.SRem(bg, dirKey(idxParent), key[slash+1:])
		if parent == "" {
			return
		}
		if exists, _ := r.rdb.Exists(bg, parent).Result(); exists > 0 {
			return
		}
		key = parent
	}
}

func pipeIndex(pipe goredis.Pipeliner, key string) {
	prefix := ""
	for _, p := range strings.Split(key, kvspace.PathSep)[1:] {
		if p == "" {
			break
		}
		parent := prefix
		if parent == "" {
			parent = kvspace.PathSep
		}
		pipe.SAdd(bg, dirKey(parent), p)
		prefix += kvspace.PathSep + p
	}
}
