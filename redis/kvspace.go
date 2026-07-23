package redis

import (
	"fmt"
	"strings"
	"time"

	"github.com/array2d/kvspace-go"
	goredis "github.com/redis/go-redis/v9"
)

// ── Get ───────────────────────────────────────────────────────────────────────

func (r *redisImpl) Get(prefix string, keys []string) []kvspace.XValue {
	ctx := bg
	prefix, err := r.resolvePath(ctx, prefix)
	if err != nil {
		return nil
	}

	// 检查 prefix 是否为 extindex，合并读取时需要回退到目标
	var extT string
	if isDir(prefix) {
		extT, _ = r.extTarget(ctx, prefix)
	}

	results := make([]kvspace.XValue, len(keys))
	for i, k := range keys {
		full := kvspace.JoinPath(prefix, k)

		if isDir(full) {
			results[i] = r.getDir(ctx, full)
			continue
		}

		// 文件值：先查本地
		data, err := r.rdb.Get(ctx, full).Bytes()
		if err == nil {
			results[i] = kvspace.DecodeXValue(data)
			continue
		}
		if err != goredis.Nil {
			continue
		}

		// extindex 回退：查目标
		if extT != "" {
			data, err := r.rdb.Get(ctx, kvspace.JoinPath(extT, k)).Bytes()
			if err == nil {
				results[i] = kvspace.DecodeXValue(data)
				continue
			}
		}
		results[i] = kvspace.Null()
	}
	return results
}

// getDir 从 Set 重建目录索引 XValue。
func (r *redisImpl) getDir(ctx context.Context, dir string) kvspace.XValue {
	members, err := r.rdb.SMembers(ctx, dir).Result()
	if err != nil || len(members) == 0 {
		return kvspace.Null()
	}

	var extT string
	var nodes []string
	for _, m := range members {
		if t := kvspace.DecodeExtEntry(m); t != "" {
			extT = t
		} else if !strings.HasPrefix(m, kvspace.ReservedPrefix) {
			nodes = append(nodes, m)
		}
	}

	if extT != "" {
		raw := append([]byte(extT), sepNodes(nodes)...)
		return kvspace.Raw(kvspace.KindExtIndex, raw)
	}
	return kvspace.Raw(kvspace.KindIndex, []byte(strings.Join(nodes, kvspace.PathSep)))
}

// sepNodes 将节点名编码为 ["/node1/node2"...]。
func sepNodes(nodes []string) []byte {
	if len(nodes) == 0 {
		return nil
	}
	var b []byte
	for _, n := range nodes {
		b = append(b, kvspace.PathSep...)
		b = append(b, n...)
	}
	return b
}

// ── Set ───────────────────────────────────────────────────────────────────────

func (r *redisImpl) Set(pairs []kvspace.KVPair) error {
	ctx := bg
	pipe := r.rdb.Pipeline()

	for _, p := range pairs {
		key, val := p.Key, p.Val
		resolved, err := r.resolvePath(ctx, key)
		if err != nil {
			return err
		}

		if isDir(resolved) {
			if err := r.setDir(ctx, pipe, resolved, val); err != nil {
				return err
			}
		} else {
			if err := r.setFile(ctx, pipe, resolved, val); err != nil {
				return err
			}
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// setFile 写入文件值，维护父目录索引。
func (r *redisImpl) setFile(ctx context.Context, pipe goredis.Pipeliner, path string, val kvspace.XValue) error {
	parent, name := kvspace.SepPath(path)
	if err := r.ensureParent(ctx, parent, name); err != nil {
		return err
	}

	// extindex 写保护：key 仅在目标存在时 panic
	if extT, _ := r.extTarget(ctx, parent); extT != "" {
		localExists, _ := r.rdb.SIsMember(ctx, parent, name).Result()
		if !localExists {
			targetExists, _ := r.rdb.SIsMember(ctx, extT, name).Result()
			if targetExists {
				panic(fmt.Sprintf("kvspace: 禁止对 extindex 只读路径执行写操作: %s", path))
			}
		}
	}

	pipe.Set(ctx, path, kvspace.EncodeXValue(val), 0)
	pipe.SAdd(ctx, parent, name)
	return nil
}

// setDir 写入目录索引，kind 必须为 index/extindex。
func (r *redisImpl) setDir(ctx context.Context, pipe goredis.Pipeliner, path string, val kvspace.XValue) error {
	parent, name := kvspace.SepPath(path)
	if err := r.ensureParent(ctx, parent, name); err != nil {
		return err
	}

	pipe.Del(ctx, path)
	switch val.Kind() {
	case kvspace.KindIndex:
		if len(val.RawBytes()) > 0 {
			for _, n := range strings.Split(string(val.RawBytes()), kvspace.PathSep) {
				if n != "" {
					pipe.SAdd(ctx, path, n)
				}
			}
		}
	case kvspace.KindExtIndex:
		if extpath := kvspace.DecodeExtIndex(val); extpath != "" {
			pipe.SAdd(ctx, path, kvspace.EncodeExtEntry(extpath))
		}
	}
	pipe.SAdd(ctx, parent, name)
	return nil
}

// ── List ──────────────────────────────────────────────────────────────────────

func (r *redisImpl) List(prefix string) []string {
	ctx := bg
	resolved, err := r.resolvePath(ctx, prefix)
	if err != nil {
		return nil
	}
	if !isDir(resolved) {
		return nil
	}

	members, err := r.rdb.SMembers(ctx, resolved).Result()
	if err != nil {
		return nil
	}

	// 合并 extindex 目标成员
	extT, _ := r.extTarget(ctx, resolved)
	var extMembers []string
	if extT != "" {
		extMembers, _ = r.rdb.SMembers(ctx, extT).Result()
	}

	localSet := make(map[string]bool, len(members))
	var result []string
	for _, m := range members {
		if strings.HasPrefix(m, kvspace.ReservedPrefix) {
			continue
		}
		localSet[m] = true
		result = append(result, m)
	}
	for _, m := range extMembers {
		if strings.HasPrefix(m, kvspace.ReservedPrefix) || localSet[m] {
			continue
		}
		result = append(result, m)
	}
	return result
}

// ── Del ───────────────────────────────────────────────────────────────────────

func (r *redisImpl) Del(keys ...string) error {
	ctx := bg
	pipe := r.rdb.Pipeline()

	for _, key := range keys {
		resolved, err := r.resolveParent(ctx, key)
		if err != nil {
			return err
		}

		if isDir(resolved) {
			parent, name := kvspace.SepPath(resolved)
			pipe.Del(ctx, resolved)
			pipe.SRem(ctx, parent, name)
		} else {
			parent, name := kvspace.SepPath(resolved)
			// extindex 写保护
			if extT, _ := r.extTarget(ctx, parent); extT != "" {
				localExists, _ := r.rdb.SIsMember(ctx, parent, name).Result()
				if !localExists {
					targetExists, _ := r.rdb.SIsMember(ctx, extT, name).Result()
					if targetExists {
						panic(fmt.Sprintf("kvspace: 禁止删除 extindex 只读路径: %s", resolved))
					}
				}
			}
			pipe.Del(ctx, resolved)
			pipe.SRem(ctx, parent, name)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// ── DelTree ───────────────────────────────────────────────────────────────────

func (r *redisImpl) DelTree(prefix string) error {
	ctx := bg

	// prefix 本身是 link → 只删 link
	data, err := r.rdb.Get(ctx, prefix).Bytes()
	if err == nil {
		v := kvspace.DecodeXValue(data)
		if v.Kind() == kvspace.KindLinkIndex {
			return r.Del(prefix)
		}
	}

	resolved, err := r.resolvePath(ctx, prefix)
	if err != nil {
		return err
	}

	keys := r.collectKeys(ctx, resolved)
	if len(keys) == 0 {
		return nil
	}

	pipe := r.rdb.Pipeline()
	for _, k := range keys {
		pipe.Del(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	parent, name := kvspace.SepPath(resolved)
	return r.rdb.SRem(ctx, parent, name).Err()
}

// ── Notify / Watch ────────────────────────────────────────────────────────────

func (r *redisImpl) Notify(key string, val kvspace.XValue) error {
	ctx := bg
	resolved, err := r.resolvePath(ctx, key)
	if err != nil {
		return err
	}
	return r.rdb.LPush(ctx, notifyPrefix+resolved, kvspace.EncodeXValue(val)).Err()
}

func (r *redisImpl) Watch(key string, timeout time.Duration) kvspace.XValue {
	ctx := bg
	resolved, err := r.resolvePath(ctx, key)
	if err != nil {
		return kvspace.Null()
	}
	results, err := r.rdb.BLPop(ctx, timeout, notifyPrefix+resolved).Result()
	if err != nil || len(results) < 2 {
		return kvspace.Null()
	}
	return kvspace.DecodeXValue([]byte(results[1]))
}

// ── Link ──────────────────────────────────────────────────────────────────────

func (r *redisImpl) Link(target, linkpath string) error {
	ctx := bg

	if isDir(target) != isDir(linkpath) {
		return fmt.Errorf("kvspace: Link target 和 linkpath 类型不一致: %s → %s", target, linkpath)
	}

	resolved, err := r.resolveParent(ctx, linkpath)
	if err != nil {
		return err
	}

	parent, name := kvspace.SepPath(resolved)
	if err := r.ensureParent(ctx, parent, name); err != nil {
		return err
	}

	pipe := r.rdb.Pipeline()
	if isDir(resolved) {
		pipe.SAdd(ctx, resolved, kvspace.EncodeExtEntry(target))
	} else {
		pipe.Set(ctx, resolved, kvspace.EncodeXValue(kvspace.NewLinkValue(target)), 0)
	}
	pipe.SAdd(ctx, parent, name)
	_, err = pipe.Exec(ctx)
	return err
}

// ── ExtIndex ──────────────────────────────────────────────────────────────────

func (r *redisImpl) ExtIndex(path, extpath string) error {
	ctx := bg
	if !isDir(path) || !isDir(extpath) {
		return fmt.Errorf("kvspace: ExtIndex path 和 extpath 必须以 / 结尾: %s, %s", path, extpath)
	}

	// extpath 不能是 extindex（不容许级联）
	if t, _ := r.extTarget(ctx, extpath); t != "" {
		return fmt.Errorf("kvspace: ExtIndex 不容许级联: %s 已是 extindex", extpath)
	}

	resolved, err := r.resolveParent(ctx, path)
	if err != nil {
		return err
	}

	parent, name := kvspace.SepPath(resolved)
	if err := r.ensureParent(ctx, parent, name); err != nil {
		return err
	}

	pipe := r.rdb.Pipeline()
	pipe.SAdd(ctx, resolved, kvspace.EncodeExtEntry(extpath))
	pipe.SAdd(ctx, parent, name)
	_, err = pipe.Exec(ctx)
	return err
}

// ── UnLink ────────────────────────────────────────────────────────────────────

func (r *redisImpl) UnLink(path string) error {
	ctx := bg
	resolved, err := r.resolveParent(ctx, path)
	if err != nil {
		return err
	}

	if isDir(resolved) {
		members, err := r.rdb.SMembers(ctx, resolved).Result()
		if err != nil {
			return err
		}
		pipe := r.rdb.Pipeline()
		for _, m := range members {
			if kvspace.DecodeExtEntry(m) != "" || strings.HasPrefix(m, kvspace.ReservedPrefix) {
				pipe.SRem(ctx, resolved, m)
			}
		}
		_, err = pipe.Exec(ctx)
		return err
	}

	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, resolved)
	parent, name := kvspace.SepPath(resolved)
	pipe.SRem(ctx, parent, name)
	_, err = pipe.Exec(ctx)
	return err
}

// ── Clear / DisConn ───────────────────────────────────────────────────────────

func (r *redisImpl) Clear() error  { return r.rdb.FlushDB(bg).Err() }
func (r *redisImpl) DisConn() error { return r.rdb.Close() }
