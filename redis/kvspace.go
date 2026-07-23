package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/array2d/kvspace-go"
	goredis "github.com/redis/go-redis/v9"
)

// ── Get ───────────────────────────────────────────────────────────────────────

func (r *redisImpl) Get(prefix string, keys []string) []kvspace.XValue {
	ctx := bg
	assertDir(prefix)
	prefix = r.resolvePath(ctx, prefix)
	extT := r.extTarget(ctx, prefix)

	results := make([]kvspace.XValue, len(keys))
	for i, k := range keys {
		full := kvspace.JoinPath(prefix, k)

		if isDir(full) {
			results[i] = r.getDir(ctx, full)
			continue
		}

		data, err := r.rdb.Get(ctx, full).Bytes()
		if err == nil {
			results[i] = kvspace.DecodeXValue(data)
			continue
		}
		if err != goredis.Nil {
			panic(fmt.Sprintf("kvspace: Get 失败: key=%s err=%v", full, err))
		}

		if extT != "" {
			targetKey := kvspace.JoinPath(extT, k)
			data, err := r.rdb.Get(ctx, targetKey).Bytes()
			if err == nil {
				results[i] = kvspace.DecodeXValue(data)
				continue
			}
			if err != goredis.Nil {
				panic(fmt.Sprintf("kvspace: Get ext 回退失败: key=%s err=%v", targetKey, err))
			}
		}
		results[i] = kvspace.Null()
	}
	return results
}

func (r *redisImpl) getDir(ctx context.Context, dir string) kvspace.XValue {
	members, err := r.rdb.SMembers(ctx, dir).Result()
	if err == goredis.Nil || len(members) == 0 {
		return kvspace.Null()
	}
	if err != nil {
		panic(fmt.Sprintf("kvspace: getDir SMEMBERS 失败: %s err=%v", dir, err))
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
		resolved := r.resolvePath(ctx, key)

		if isDir(resolved) {
			r.setDir(ctx, pipe, resolved, val)
		} else {
			r.setFile(ctx, pipe, resolved, val)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (r *redisImpl) setFile(ctx context.Context, pipe goredis.Pipeliner, path string, val kvspace.XValue) {
	parent, name := parentName(path)
	r.ensureParent(ctx, parent, name)

	if extT := r.extTarget(ctx, parent); extT != "" {
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
}

func (r *redisImpl) setDir(ctx context.Context, pipe goredis.Pipeliner, path string, val kvspace.XValue) {
	parent, name := parentName(path)
	r.ensureParent(ctx, parent, name)

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
}

// ── List ──────────────────────────────────────────────────────────────────────

func (r *redisImpl) List(prefix string) []string {
	ctx := bg
	assertDir(prefix)
	resolved := r.resolvePath(ctx, prefix)
	if !isDir(resolved) {
		return nil
	}

	members, err := r.rdb.SMembers(ctx, resolved).Result()
	if err != nil {
		panic(fmt.Sprintf("kvspace: List SMEMBERS 失败: prefix=%s err=%v", resolved, err))
	}

	extT := r.extTarget(ctx, resolved)
	var extMembers []string
	if extT != "" {
		extMembers, err = r.rdb.SMembers(ctx, extT).Result()
		if err != nil {
			panic(fmt.Sprintf("kvspace: List SMEMBERS ext 失败: ext=%s err=%v", extT, err))
		}
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
		resolved := r.resolveParent(ctx, key)
		parent, name := parentName(resolved)

		if isDir(resolved) {
			linkKey := resolved[:len(resolved)-len(kvspace.DirIndexSuf)]
			pipe.Del(ctx, linkKey)
			pipe.Del(ctx, resolved)
			pipe.SRem(ctx, parent, name)
		} else {
			if extT := r.extTarget(ctx, parent); extT != "" {
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

	linkKey := prefix
	if isDir(linkKey) && linkKey != kvspace.PathSep {
		linkKey = linkKey[:len(linkKey)-len(kvspace.DirIndexSuf)]
	}
	data, err := r.rdb.Get(ctx, linkKey).Bytes()
	if err == nil {
		v := kvspace.DecodeXValue(data)
		if v.Kind() == kvspace.KindLinkIndex {
			return r.Del(prefix)
		}
	}

	resolved := r.resolvePath(ctx, prefix)
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

	parent, name := parentName(resolved)
	return r.rdb.SRem(ctx, parent, name).Err()
}

// ── Notify / Watch ────────────────────────────────────────────────────────────

func (r *redisImpl) Notify(key string, val kvspace.XValue) error {
	ctx := bg
	resolved := r.resolvePath(ctx, key)
	return r.rdb.LPush(ctx, notifyPrefix+resolved, kvspace.EncodeXValue(val)).Err()
}

func (r *redisImpl) Watch(key string, timeout time.Duration) kvspace.XValue {
	ctx := bg
	resolved := r.resolvePath(ctx, key)
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

	resolved := r.resolveParent(ctx, linkpath)

	storeKey := resolved
	if isDir(storeKey) {
		storeKey = storeKey[:len(storeKey)-len(kvspace.DirIndexSuf)]
	}

	parent, name := parentName(resolved)
	r.ensureParent(ctx, parent, name)

	pipe := r.rdb.Pipeline()
	pipe.Set(ctx, storeKey, kvspace.EncodeXValue(kvspace.NewLinkValue(target)), 0)
	pipe.SAdd(ctx, parent, name)
	_, err := pipe.Exec(ctx)
	return err
}

// ── ExtIndex ──────────────────────────────────────────────────────────────────

func (r *redisImpl) ExtIndex(path, extpath string) error {
	ctx := bg
	if !isDir(path) || !isDir(extpath) {
		return fmt.Errorf("kvspace: ExtIndex path 和 extpath 必须以 / 结尾: %s, %s", path, extpath)
	}

	if t := r.extTarget(ctx, extpath); t != "" {
		return fmt.Errorf("kvspace: ExtIndex 不容许级联: %s 已是 extindex", extpath)
	}

	resolved := r.resolveParent(ctx, path)
	parent, name := parentName(resolved)
	r.ensureParent(ctx, parent, name)

	pipe := r.rdb.Pipeline()
	pipe.SAdd(ctx, resolved, kvspace.EncodeExtEntry(extpath))
	pipe.SAdd(ctx, parent, name)
	_, err := pipe.Exec(ctx)
	return err
}

// ── UnLink ────────────────────────────────────────────────────────────────────

func (r *redisImpl) UnLink(path string) error {
	ctx := bg
	resolved := r.resolveParent(ctx, path)

	linkKey := resolved
	if isDir(linkKey) {
		linkKey = linkKey[:len(linkKey)-len(kvspace.DirIndexSuf)]
	}
	data, err := r.rdb.Get(ctx, linkKey).Bytes()
	if err == nil {
		v := kvspace.DecodeXValue(data)
		if v.Kind() == kvspace.KindLinkIndex {
			pipe := r.rdb.Pipeline()
			pipe.Del(ctx, linkKey)
			parent, name := parentName(path)
			pipe.SRem(ctx, parent, name)
			_, err = pipe.Exec(ctx)
			return err
		}
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
	parent, name := parentName(resolved)
	pipe.SRem(ctx, parent, name)
	_, err = pipe.Exec(ctx)
	return err
}

// ── Clear / DisConn ───────────────────────────────────────────────────────────

func (r *redisImpl) Clear() error  { return r.rdb.FlushDB(bg).Err() }
func (r *redisImpl) DisConn() error { return r.rdb.Close() }
