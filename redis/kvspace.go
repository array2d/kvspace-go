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
	var extT string
	if data, err := r.rdb.Get(ctx, prefix).Bytes(); err == nil {
		if v := kvspace.DecodeXValue(data); v.Kind() == kvspace.KindExtIndex {
			_, extT = kvspace.DecodeExtIndex(v)
		}
	}

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
	data, err := r.rdb.Get(ctx, dir).Bytes()
	if err != nil {
		return kvspace.Null()
	}
	return kvspace.DecodeXValue(data)
}

// ── 目录 index 读写 ────────────────────────────────────────────────────────────

func (r *redisImpl) readDirIndex(ctx context.Context, dir string) []string {
	data, err := r.rdb.Get(ctx, dir).Bytes()
	if err != nil {
		return nil
	}
	v := kvspace.DecodeXValue(data)
	switch v.Kind() {
	case kvspace.KindIndex:
		nodes := kvspace.DecodeIndex(v)
		if len(nodes) == 1 && nodes[0] == "" {
			return nil
		}
		return nodes
	case kvspace.KindExtIndex:
		childs, _ := kvspace.DecodeExtIndex(v)
		return childs
	}
	return nil
}

func (r *redisImpl) addChild(ctx context.Context, pipe goredis.Pipeliner, parent, name string) {
	data, err := r.rdb.Get(ctx, parent).Bytes()
	if err != nil {
		pipe.Set(ctx, parent, kvspace.EncodeXValue(kvspace.NewIndexValue([]string{name})), 0)
		return
	}
	v := kvspace.DecodeXValue(data)
	switch v.Kind() {
	case kvspace.KindIndex:
		nodes := kvspace.DecodeIndex(v)
		if len(nodes) == 1 && nodes[0] == "" { nodes = nil }
		for _, n := range nodes {
			if n == name { return }
		}
		nodes = append(nodes, name)
		pipe.Set(ctx, parent, kvspace.EncodeXValue(kvspace.NewIndexValue(nodes)), 0)
	case kvspace.KindExtIndex:
		childs, extpath := kvspace.DecodeExtIndex(v)
		for _, c := range childs {
			if c == name { return }
		}
		childs = append(childs, name)
		pipe.Set(ctx, parent, kvspace.EncodeXValue(kvspace.NewExtIndexValue(childs, extpath)), 0)
	}
}

func (r *redisImpl) removeChild(ctx context.Context, pipe goredis.Pipeliner, parent, name string) {
	data, err := r.rdb.Get(ctx, parent).Bytes()
	if err != nil {
		return
	}
	v := kvspace.DecodeXValue(data)
	switch v.Kind() {
	case kvspace.KindIndex:
		nodes := kvspace.DecodeIndex(v)
		if len(nodes) == 1 && nodes[0] == "" { nodes = nil }
		filtered := make([]string, 0, len(nodes))
		for _, n := range nodes {
			if n != name { filtered = append(filtered, n) }
		}
		pipe.Set(ctx, parent, kvspace.EncodeXValue(kvspace.NewIndexValue(filtered)), 0)
	case kvspace.KindExtIndex:
		childs, extpath := kvspace.DecodeExtIndex(v)
		filtered := make([]string, 0, len(childs))
		for _, c := range childs {
			if c != name { filtered = append(filtered, c) }
		}
		pipe.Set(ctx, parent, kvspace.EncodeXValue(kvspace.NewExtIndexValue(filtered, extpath)), 0)
	}
}

// ── Mkindex ───────────────────────────────────────────────────────────────────

func (r *redisImpl) Mkindex(path string) error {
	ctx := bg
	if !isDir(path) {
		return fmt.Errorf("kvspace: Mkindex 路径必须以 / 结尾: %s", path)
	}
	resolved := r.resolvePath(ctx, path)

	parts := strings.Split(strings.Trim(resolved, kvspace.PathSep), kvspace.PathSep)
	cur := kvspace.PathSep
	for _, p := range parts {
		cur = kvspace.JoinPath(cur, p) + kvspace.DirIndexSuf
		if r.readDirIndex(ctx, cur) != nil {
			continue
		}
		parent, name := parentName(cur)
		r.ensureParent(ctx, parent, name)
		pipe := r.rdb.Pipeline()
		r.addChild(ctx, pipe, parent, name)
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("kvspace: Mkindex 失败: %s err=%v", cur, err)
		}
	}
	return nil
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

	var extT string
	if data, err := r.rdb.Get(ctx, parent).Bytes(); err == nil {
		if v := kvspace.DecodeXValue(data); v.Kind() == kvspace.KindExtIndex {
			_, extT = kvspace.DecodeExtIndex(v)
		}
	}
	if extT != "" {
		localNodes := r.readDirIndex(ctx, parent)
		localExists := false
		for _, n := range localNodes {
			if n == name { localExists = true; break }
		}
		if !localExists {
			extNodes := r.readDirIndex(ctx, extT)
			for _, n := range extNodes {
				if n == name {
					panic(fmt.Sprintf("kvspace: 禁止对 extindex 只读路径执行写操作: %s", path))
				}
			}
		}
	}

	pipe.Set(ctx, path, kvspace.EncodeXValue(val), 0)
	r.addChild(ctx, pipe, parent, name)
}

func (r *redisImpl) setDir(ctx context.Context, pipe goredis.Pipeliner, path string, val kvspace.XValue) {
	parent, name := parentName(path)
	r.ensureParent(ctx, parent, name)

	pipe.Set(ctx, path, kvspace.EncodeXValue(val), 0)
	r.addChild(ctx, pipe, parent, name)
}

// ── List ──────────────────────────────────────────────────────────────────────

func (r *redisImpl) List(prefix string) []string {
	ctx := bg
	assertDir(prefix)
	resolved := r.resolvePath(ctx, prefix)
	if !isDir(resolved) {
		return nil
	}

	members := r.readDirIndex(ctx, resolved)

	var extT string
	if data, err := r.rdb.Get(ctx, resolved).Bytes(); err == nil {
		if v := kvspace.DecodeXValue(data); v.Kind() == kvspace.KindExtIndex {
			_, extT = kvspace.DecodeExtIndex(v)
		}
	}
	var extMembers []string
	if extT != "" {
		extMembers = r.readDirIndex(ctx, extT)
	}

	localSet := make(map[string]bool, len(members))
	var result []string
	for _, m := range members {
		localSet[m] = true
		result = append(result, m)
	}
	for _, m := range extMembers {
		if localSet[m] {
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

		var extT string
		if data, err := r.rdb.Get(ctx, parent).Bytes(); err == nil {
			if v := kvspace.DecodeXValue(data); v.Kind() == kvspace.KindExtIndex {
				_, extT = kvspace.DecodeExtIndex(v)
			}
		}
		if extT != "" {
			localNodes := r.readDirIndex(ctx, parent)
			localExists := false
			for _, n := range localNodes {
				if n == name { localExists = true; break }
			}
			if !localExists {
				extNodes := r.readDirIndex(ctx, extT)
				for _, n := range extNodes {
					if n == name {
						panic(fmt.Sprintf("kvspace: 禁止删除 extindex 只读路径: %s", resolved))
					}
				}
			}
		}

		if isDir(resolved) {
			linkKey := resolved[:len(resolved)-len(kvspace.DirIndexSuf)]
			pipe.Del(ctx, linkKey)
			pipe.Del(ctx, resolved)
		} else {
			pipe.Del(ctx, resolved)
		}
		r.removeChild(ctx, pipe, parent, name)
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

	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, resolved)
	for _, k := range keys {
		pipe.Del(ctx, k)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}

	parent, name := parentName(resolved)
	pipe = r.rdb.Pipeline()
	r.removeChild(ctx, pipe, parent, name)
	_, err = pipe.Exec(ctx)
	return err
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
	r.addChild(ctx, pipe, parent, name)
	_, err := pipe.Exec(ctx)
	return err
}

// ── ExtIndex ──────────────────────────────────────────────────────────────────

func (r *redisImpl) ExtIndex(path, extpath string) error {
	ctx := bg
	if !isDir(path) || !isDir(extpath) {
		return fmt.Errorf("kvspace: ExtIndex path 和 extpath 必须以 / 结尾: %s, %s", path, extpath)
	}

	if data, err := r.rdb.Get(ctx, extpath).Bytes(); err == nil {
		if v := kvspace.DecodeXValue(data); v.Kind() == kvspace.KindExtIndex {
			return fmt.Errorf("kvspace: ExtIndex 不容许级联: %s 已是 extindex", extpath)
		}
	}

	resolved := r.resolveParent(ctx, path)
	parent, name := parentName(resolved)
	r.ensureParent(ctx, parent, name)

	pipe := r.rdb.Pipeline()
	val := kvspace.NewExtIndexValue(nil, extpath)
	pipe.Set(ctx, resolved, kvspace.EncodeXValue(val), 0)
	r.addChild(ctx, pipe, parent, name)
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
			parent, name := parentName(resolved)
			r.removeChild(ctx, pipe, parent, name)
			_, err = pipe.Exec(ctx)
			return err
		}
	}

	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, resolved)
	parent, name := parentName(resolved)
	r.removeChild(ctx, pipe, parent, name)
	_, err = pipe.Exec(ctx)
	return err
}

// ── Clear / DisConn ───────────────────────────────────────────────────────────

func (r *redisImpl) Clear() error  { return r.rdb.FlushDB(bg).Err() }
func (r *redisImpl) DisConn() error { return r.rdb.Close() }
