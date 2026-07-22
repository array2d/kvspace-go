// Package redis 提供 kvspace.KVSpace 的 Redis 实现。
package redis

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/array2d/kvspace-go"
)

func init() { kvspace.Register("redis", ConnPool) }

var bg = context.Background()

const linkSentinel = "->"

func Conn(dsn string) kvspace.KVSpace      { return ConnPool(dsn, 16) }
func ConnPool(dsn string, poolSize int) kvspace.KVSpace {
	if poolSize < 16 { poolSize = 16 }
	return &redisImpl{
		rdb: goredis.NewClient(&goredis.Options{
			Addr: dsn, PoolSize: poolSize,
			MinIdleConns: min(poolSize/4, 8),
			PoolTimeout: 10*time.Second, ReadTimeout: 3*time.Second, WriteTimeout: 3*time.Second,
		}),
		links: make(map[string]linkEntry),
	}
}

type linkEntry struct{ checked bool; target string }

type redisImpl struct {
	rdb    *goredis.Client
	linkMu sync.RWMutex
	links  map[string]linkEntry
}

// ── KVSpace interface ──────────────────────────────────────────────────────

func (r *redisImpl) Get(keys []string) []kvspace.XValue {
	result := make([]kvspace.XValue, len(keys))
	pipe := r.rdb.Pipeline()
	cmds := make([]*goredis.StringCmd, len(keys))
	for i, k := range keys {
		cmds[i] = pipe.Get(bg, kvspace.ResolveCore(k, r.checkLink))
	}
	pipe.Exec(bg)
	for i, cmd := range cmds {
		raw, err := cmd.Bytes()
		if err == nil { result[i] = kvspace.DecodeXValue(raw) }
	}
	return result
}

func (r *redisImpl) Set(pairs []kvspace.KVPair) error {
	if len(pairs) == 0 { return nil }
	pipe := r.rdb.Pipeline()
	for _, p := range pairs {
		if strings.HasSuffix(p.Key, kvspace.PathSep) {
			return fmt.Errorf("kvspace: key must not end with %s: %s", kvspace.PathSep, p.Key)
		}
		resolved := kvspace.ResolveCore(p.Key, r.checkLink)
		pipeIndex(pipe, resolved)
		pipe.Set(bg, resolved, kvspace.EncodeXValue(p.Val), 0)
	}
	_, err := pipe.Exec(bg)
	return err
}

func (r *redisImpl) List(prefix string) []string {
	resolved := kvspace.ResolveCore(prefix, r.checkLink)
	members, _ := r.rdb.SMembers(bg, dirKey(resolved)).Result()
	return members
}

func (r *redisImpl) Del(keys ...string) error {
	resolved := make([]string, len(keys))
	for i, k := range keys {
		resolved[i] = kvspace.ResolveParent(k, r.checkLink)
	}
	err := r.rdb.Del(bg, resolved...).Err()
	r.linkMu.Lock()
	for _, k := range resolved { r.links[k] = linkEntry{checked: true, target: ""} }
	r.linkMu.Unlock()
	for _, k := range resolved { r.delIndex(k) }
	return err
}

func (r *redisImpl) DelTree(prefix string) error {
	resolved := kvspace.ResolveParent(prefix, r.checkLink)
	if r.checkLink(resolved) != "" { return r.Unlink(resolved) }
	r.delRecursive(resolved)
	r.delIndex(resolved)
	return nil
}

func (r *redisImpl) Watch(key string, timeout time.Duration) kvspace.XValue {
	vals, err := r.rdb.BLPop(bg, timeout, kvspace.ResolveCore(key, r.checkLink)).Result()
	if err != nil || len(vals) < 2 { return kvspace.XValue{} }
	return kvspace.DecodeXValue([]byte(vals[1]))
}

func (r *redisImpl) Notify(key string, val kvspace.XValue) error {
	return r.rdb.LPush(bg, kvspace.ResolveCore(key, r.checkLink), kvspace.EncodeXValue(val)).Err()
}

// ── mount ─────────────────────────────────────────────────────────────────

func (r *redisImpl) Mount(target, linkpath string) error { return r.Link(target, linkpath) }
func (r *redisImpl) Overlay(target, rPath, wPath string) error {
	return fmt.Errorf("overlay not implemented")
}
func (r *redisImpl) UnMount(linkpath string) error { return r.Unlink(linkpath) }

func (r *redisImpl) Clear() error   { return r.rdb.FlushDB(bg).Err() }
func (r *redisImpl) DisConn() error { return r.rdb.Close() }

// ── soft links (internal, used by Mount/UnMount) ──────────────────────────

func (r *redisImpl) Link(target, linkpath string) error {
	if err := r.rdb.Set(bg, linkpath, []byte(linkSentinel+target), 0).Err(); err != nil {
		return err
	}
	r.addIndex(linkpath)
	r.linkMu.Lock()
	r.links[linkpath] = linkEntry{checked: true, target: target}
	r.linkMu.Unlock()
	return nil
}

func (r *redisImpl) Unlink(linkpath string) error {
	if err := r.rdb.Del(bg, linkpath).Err(); err != nil { return err }
	r.delIndex(linkpath)
	r.linkMu.Lock()
	r.links[linkpath] = linkEntry{checked: true, target: ""}
	r.linkMu.Unlock()
	return nil
}

func (r *redisImpl) checkLink(path string) string {
	r.linkMu.RLock()
	e := r.links[path]
	r.linkMu.RUnlock()
	if e.checked { return e.target }
	var target string
	raw, _ := r.rdb.Get(bg, path).Bytes()
	if len(raw) >= 2 && raw[0] == '-' && raw[1] == '>' { target = string(raw[2:]) }
	r.linkMu.Lock()
	r.links[path] = linkEntry{checked: true, target: target}
	r.linkMu.Unlock()
	return target
}

// dirKey 返回目录索引键：parent 为 / 时返回 /，否则返回 parent/
func dirKey(parent string) string {
	if parent == kvspace.PathSep { return kvspace.PathSep }
	return parent + kvspace.DirIndexSuf
}

// ── index maintenance ─────────────────────────────────────────────────────

func (r *redisImpl) delRecursive(prefix string) {
	children, _ := r.rdb.SMembers(bg, dirKey(prefix)).Result()
	for _, c := range children { r.delRecursive(kvspace.JoinPath(prefix, c)) }
	r.rdb.Del(bg, prefix, dirKey(prefix))
}

func (r *redisImpl) addIndex(key string) {
	prefix := ""
	for _, p := range strings.Split(key, kvspace.PathSep)[1:] {
		if p == "" { break }
		parent := prefix
		if parent == "" { parent = kvspace.PathSep }
		r.rdb.SAdd(bg, dirKey(parent), p)
		prefix += kvspace.PathSep + p
	}
}

func (r *redisImpl) delIndex(key string) {
	for key != "" && key != kvspace.PathSep {
		if n, _ := r.rdb.SCard(bg, dirKey(key)).Result(); n > 0 { return }
		slash := strings.LastIndexByte(key, '/')
		if slash < 0 { return }
		parent := key[:slash]
		idxParent := parent
		if idxParent == "" { idxParent = kvspace.PathSep }
		r.rdb.SRem(bg, dirKey(idxParent), key[slash+1:])
		if parent == "" { return }
		if exists, _ := r.rdb.Exists(bg, parent).Result(); exists > 0 { return }
		key = parent
	}
}

func pipeIndex(pipe goredis.Pipeliner, key string) {
	prefix := ""
	for _, p := range strings.Split(key, kvspace.PathSep)[1:] {
		if p == "" { break }
		parent := prefix
		if parent == "" { parent = kvspace.PathSep }
		pipe.SAdd(bg, dirKey(parent), p)
		prefix += kvspace.PathSep + p
	}
}

// Go 1.21+ builtin, keep for compat
func min(a, b int) int { if a < b { return a }; return b }

