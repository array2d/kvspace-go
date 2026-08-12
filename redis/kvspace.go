package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/array2d/kvspace-go"
	goredis "github.com/redis/go-redis/v9"
)

// ── helpers ─────────────────────────────────────────────────────────────────────

// ── Get ───────────────────────────────────────────────────────────────────────

func (r *redisImpl) Get(prefix string, keys []string, resolve bool, arridx int32) []kvspace.XValue {
	ctx := bg
	assertDir(prefix)
	if resolve {
		prefix = r.resolvePath(ctx, prefix)
	}

	var extT string
	if data, err := r.rdb.Get(ctx, prefix).Bytes(); err == nil {
		head := kvspace.DecodeXValueHead(data)
		if head.Kind == kvspace.KindExtIndex {
			extT = kvspace.DecodeExtIndex(head.Raw).ExtPath()
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
			results[i] = decodeElem(data, arridx)
			continue
		}
		if err != goredis.Nil {
			panic(fmt.Errorf("%w: key=%s err=%v", kvspace.ErrGet, full, err))
		}

		if extT != "" {
			targetKey := kvspace.JoinPath(extT, k)
			data, err := r.rdb.Get(ctx, targetKey).Bytes()
			if err == nil {
				results[i] = decodeElem(data, arridx)
				continue
			}
			if err != goredis.Nil {
				panic(fmt.Errorf("%w: ext fallback key=%s err=%v", kvspace.ErrGet, targetKey, err))
			}
		}
		results[i] = kvspace.None{}
	}
	return results
}

func decodeElem(data []byte, arridx int32) kvspace.XValue {
	if arridx < 0 {
		return kvspace.DecodeXValueHead(data).Decode()
	}
	head := kvspace.DecodeXValueHead(data)
	if head.ArrayLen > arridx {
		return kvspace.SliceElem(head.Kind, head.Raw, arridx)
	}
	return kvspace.None{}
}

func (r *redisImpl) getDir(ctx context.Context, dir string) kvspace.XValue {
	data, err := r.rdb.Get(ctx, dir).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return kvspace.None{}
		}
		panic(fmt.Errorf("%w: getDir %s err=%v", kvspace.ErrGet, dir, err))
	}
	return kvspace.DecodeXValueHead(data).Decode()
}

// ── 目录 index 读写 ────────────────────────────────────────────────────────────

func (r *redisImpl) readDirIndex(ctx context.Context, dir string) []string {
	data, err := r.rdb.Get(ctx, dir).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return nil
		}
		panic(fmt.Errorf("%w: readDirIndex %s err=%v", kvspace.ErrGet, dir, err))
	}
	v := kvspace.DecodeXValueHead(data).Decode()
	if kvspace.IsNone(v) {
		return nil
	}
	switch v := v.(type) {
	case kvspace.Index:
		c := v.Children()
		if len(c) == 1 && c[0] == "" {
			return nil
		}
		return c
	case kvspace.DictIndex:
		c := v.Children()
		if len(c) == 1 && c[0] == "" {
			return nil
		}
		return c
	case kvspace.ExtIndex:
		return v.Children()
	default:
		panic(fmt.Sprintf("readDirIndex: unexpected kind %s", v.Kind()))
	}
}

func (r *redisImpl) addChild(ctx context.Context, pipe goredis.Pipeliner, parent, name string) {
	data, err := r.rdb.Get(ctx, parent).Bytes()
	if err != nil {
		if err == goredis.Nil {
			if strings.HasSuffix(parent, kvspace.DictSep) {
				pipe.Set(ctx, parent, kvspace.NewDictIndex([]string{name}).Encode(), 0)
			} else {
				pipe.Set(ctx, parent, kvspace.NewIndex([]string{name}).Encode(), 0)
			}
			return
		}
		panic(fmt.Errorf("%w: addChild %s err=%v", kvspace.ErrGet, parent, err))
	}
	v := kvspace.DecodeXValueHead(data).Decode()
	switch v := v.(type) {
	case kvspace.Index:
		nodes := v.Children()
		if len(nodes) == 1 && nodes[0] == "" {
			nodes = nil
		}
		for _, n := range nodes {
			if n == name {
				return
			}
		}
		nodes = append(nodes, name)
		pipe.Set(ctx, parent, kvspace.NewIndex(nodes).Encode(), 0)
	case kvspace.DictIndex:
		nodes := v.Children()
		if len(nodes) == 1 && nodes[0] == "" {
			nodes = nil
		}
		for _, n := range nodes {
			if n == name {
				return
			}
		}
		nodes = append(nodes, name)
		pipe.Set(ctx, parent, kvspace.NewDictIndex(nodes).Encode(), 0)
	case kvspace.ExtIndex:
		for _, c := range v.Children() {
			if c == name {
				return
			}
		}
		childs := append(v.Children(), name)
		pipe.Set(ctx, parent, kvspace.NewExtIndex(childs, v.ExtPath()).Encode(), 0)
	default:
		panic(fmt.Sprintf("addChild: unexpected kind %s", v.Kind()))
	}
}

func (r *redisImpl) removeChild(ctx context.Context, pipe goredis.Pipeliner, parent, name string) {
	data, err := r.rdb.Get(ctx, parent).Bytes()
	if err != nil {
		if err == goredis.Nil {
			return
		}
		panic(fmt.Errorf("%w: removeChild %s/%s err=%v", kvspace.ErrGet, parent, name, err))
	}
	v := kvspace.DecodeXValueHead(data).Decode()
	switch v := v.(type) {
	case kvspace.Index:
		nodes := v.Children()
		if len(nodes) == 1 && nodes[0] == "" {
			nodes = nil
		}
		filtered := make([]string, 0, len(nodes))
		for _, n := range nodes {
			if n != name && n != name+kvspace.DirIndexSuf {
				filtered = append(filtered, n)
			}
		}
		pipe.Set(ctx, parent, kvspace.NewIndex(filtered).Encode(), 0)
	case kvspace.DictIndex:
		nodes := v.Children()
		if len(nodes) == 1 && nodes[0] == "" {
			nodes = nil
		}
		filtered := make([]string, 0, len(nodes))
		for _, n := range nodes {
			if n != name && n != name+kvspace.DirIndexSuf {
				filtered = append(filtered, n)
			}
		}
		pipe.Set(ctx, parent, kvspace.NewDictIndex(filtered).Encode(), 0)
	case kvspace.ExtIndex:
		filtered := make([]string, 0, len(v.Children()))
		for _, c := range v.Children() {
			if c != name && c != name+kvspace.DirIndexSuf {
				filtered = append(filtered, c)
			}
		}
		pipe.Set(ctx, parent, kvspace.NewExtIndex(filtered, v.ExtPath()).Encode(), 0)
	default:
		panic(fmt.Sprintf("removeChild: unexpected kind %s", v.Kind()))
	}
}

// ── Mkindex ───────────────────────────────────────────────────────────────────

func (r *redisImpl) Mkindex(path string) error {
	ctx := bg
	if !isDir(path) {
		return fmt.Errorf("%w: Mkindex %s", kvspace.ErrDirMustEndWithSlash, path)
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
		pipe := r.rdb.Pipeline()
		r.addChild(ctx, pipe, parent, name+kvspace.DirIndexSuf)
		if _, err := pipe.Exec(ctx); err != nil {
			return fmt.Errorf("%w: Mkindex %s err=%v", kvspace.ErrPipeExec, cur, err)
		}
	}
	return nil
}

// ── Set ───────────────────────────────────────────────────────────────────────

func encodeSetVal(r *redisImpl, ctx context.Context, key string, val kvspace.XValue, arridx int32) []byte {
	if arridx < 0 {
		return val.Encode()
	}
	data, err := r.rdb.Get(ctx, key).Bytes()
	if err != nil {
		panic(fmt.Errorf("Set arridx: read existing %s: %w", key, err))
	}
	head := kvspace.DecodeXValueHead(data)
	kvspace.WriteElem(head.Kind, head.Raw, arridx, val)
	return kvspace.EncodeRaw(head.Kind, head.IsPtr, head.Raw, head.ArrayLen)
}

func (r *redisImpl) Set(pairs []kvspace.KVPair) error {
	ctx := bg
	pipe := r.rdb.Pipeline()

	type childEntry struct{ parent, name string }
	var children []childEntry

	for _, p := range pairs {
		key, val := p.Key, p.Val
		resolved := r.resolvePath(ctx, key)

		if strings.Contains(resolved, "//") {
			panic(fmt.Errorf("Set: double-slash in key %q", resolved))
		}
		switch val.(type) {
		case kvspace.Index, kvspace.ExtIndex:
			if !isDir(resolved) {
				panic(fmt.Errorf("Set: index/extindex value at non-directory key %q", resolved))
			}
		}

		if ptr, ok := val.(kvspace.Ptr); ok {
			if err := kvspace.ValidatePtr(r, ptr.Target(), ptr.Kind(), ptr.ArrayLen()); err != nil {
				return err
			}
		}

		var parent, name string
		if isDir(resolved) {
			parent, name = parentName(resolved)
			kvspace.MkIndexRecursive(r, parent)
			pipe.Set(ctx, resolved, encodeSetVal(r, ctx, resolved, val, p.Arridx), 0)
			children = append(children, childEntry{parent, name + suffixFor(resolved)})
			continue
		}

		if sd, m, ok := kvspace.SplitDictParent(r, resolved); ok {
			parent, name = sd, m
		} else {
			parent, name = parentName(resolved)
		}
		if !strings.HasSuffix(parent, kvspace.DictSep) {
			kvspace.MkIndexRecursive(r, parent)
		}

		if data, err := r.rdb.Get(ctx, parent).Bytes(); err == nil {
			head := kvspace.DecodeXValueHead(data)
			if head.Kind == kvspace.KindExtIndex {
				extT := kvspace.DecodeExtIndex(head.Raw).ExtPath()
				localNodes := r.readDirIndex(ctx, parent)
				localExists := false
				for _, n := range localNodes {
					if n == name {
						localExists = true
						break
					}
				}
				if !localExists {
					extNodes := r.readDirIndex(ctx, extT)
					for _, n := range extNodes {
						if n == name {
							panic(fmt.Errorf("%w: %s", kvspace.ErrExtWrite, resolved))
						}
					}
				}
			}
		}

		pipe.Set(ctx, resolved, encodeSetVal(r, ctx, resolved, val, p.Arridx), 0)
		children = append(children, childEntry{parent, name})
	}

	parentChildren := make(map[string][]string, len(children))
	for _, c := range children {
		parentChildren[c.parent] = append(parentChildren[c.parent], c.name)
	}
	for parent, names := range parentChildren {
		data, err := r.rdb.Get(ctx, parent).Bytes()
		if err != nil && err != goredis.Nil {
			panic(fmt.Errorf("%w: addChild %s err=%v", kvspace.ErrGet, parent, err))
		}
		var nodes []string
		var extpath string
		isExt, isDict := false, false
		if err != goredis.Nil {
			v := kvspace.DecodeXValueHead(data).Decode()
			switch v := v.(type) {
			case kvspace.Index:
				nodes = v.Children()
				if len(nodes) == 1 && nodes[0] == "" {
					nodes = nil
				}
			case kvspace.DictIndex:
				nodes = v.Children()
				if len(nodes) == 1 && nodes[0] == "" {
					nodes = nil
				}
				isDict = true
			case kvspace.ExtIndex:
				nodes = v.Children()
				extpath = v.ExtPath()
				isExt = true
			default:
				panic(fmt.Sprintf("Set parentChildren: unexpected kind %s", v.Kind()))
			}
		}
		seen := make(map[string]bool, len(nodes)+len(names))
		for _, n := range nodes {
			seen[n] = true
		}
		for _, n := range names {
			if !seen[n] {
				nodes = append(nodes, n)
				seen[n] = true
			}
		}
		if isExt {
			pipe.Set(ctx, parent, kvspace.NewExtIndex(nodes, extpath).Encode(), 0)
		} else if isDict {
			pipe.Set(ctx, parent, kvspace.NewDictIndex(nodes).Encode(), 0)
		} else {
			pipe.Set(ctx, parent, kvspace.NewIndex(nodes).Encode(), 0)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// ── List ──────────────────────────────────────────────────────────────────────

func (r *redisImpl) List(prefix string, expandExt bool, resolve bool) []string {
	ctx := bg
	assertDir(prefix)
	resolved := prefix
	if resolve {
		resolved = r.resolvePath(ctx, prefix)
	}
	if !isDir(resolved) {
		return nil
	}

	members := r.readDirIndex(ctx, resolved)

	var extMembers []string
	if expandExt {
		if data, err := r.rdb.Get(ctx, resolved).Bytes(); err == nil {
			head := kvspace.DecodeXValueHead(data)
			if head.Kind == kvspace.KindExtIndex {
				extT := kvspace.DecodeExtIndex(head.Raw).ExtPath()
				extMembers = r.readDirIndex(ctx, extT)
			}
		}
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

		if data, err := r.rdb.Get(ctx, parent).Bytes(); err == nil {
			head := kvspace.DecodeXValueHead(data)
			if head.Kind == kvspace.KindExtIndex {
				extT := kvspace.DecodeExtIndex(head.Raw).ExtPath()
				localNodes := r.readDirIndex(ctx, parent)
				localExists := false
				for _, n := range localNodes {
					if n == name {
						localExists = true
						break
					}
				}
				if !localExists {
					extNodes := r.readDirIndex(ctx, extT)
					for _, n := range extNodes {
						if n == name {
							panic(fmt.Errorf("%w: %s", kvspace.ErrExtDel, resolved))
						}
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
		head := kvspace.DecodeXValueHead(data)
		if head.IsPtr {
			return r.Del(prefix)
		}
	} else if err != goredis.Nil {
		panic(fmt.Errorf("%w: DelTree %s err=%v", kvspace.ErrGet, prefix, err))
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
	return r.rdb.LPush(ctx, notifyPrefix+resolved, val.Encode()).Err()
}

func (r *redisImpl) Watch(key string, timeout time.Duration) kvspace.XValue {
	ctx := bg
	resolved := r.resolvePath(ctx, key)
	results, err := r.rdb.BLPop(ctx, timeout, notifyPrefix+resolved).Result()
	if err != nil || len(results) < 2 {
		return kvspace.None{}
	}
	return kvspace.DecodeXValueHead([]byte(results[1])).Decode()
}

// Link removed

// ── ExtIndex ──────────────────────────────────────────────────────────────────

func (r *redisImpl) ExtIndex(path, extpath string) error {
	ctx := bg
	if !isDir(path) || !isDir(extpath) {
		return fmt.Errorf("%w: ExtIndex path=%s extpath=%s", kvspace.ErrDirMustEndWithSlash, path, extpath)
	}

	if data, err := r.rdb.Get(ctx, extpath).Bytes(); err == nil {
		head := kvspace.DecodeXValueHead(data)
		if head.Kind == kvspace.KindExtIndex {
			return fmt.Errorf("%w: %s", kvspace.ErrExtCascade, extpath)
		}
	} else if err != goredis.Nil {
		panic(fmt.Errorf("%w: ExtIndex extpath=%s err=%v", kvspace.ErrGet, extpath, err))
	}

	resolved := r.resolveParent(ctx, path)
	parent, name := parentName(resolved)
	kvspace.MkIndexRecursive(r, parent)

	pipe := r.rdb.Pipeline()
	pipe.Set(ctx, resolved, kvspace.NewExtIndex(nil, extpath).Encode(), 0)
	r.addChild(ctx, pipe, parent, name+kvspace.DirIndexSuf)
	_, err := pipe.Exec(ctx)
	return err
}

// ── UnLink ────────────────────────────────────────────────────────────────────

func (r *redisImpl) DelExtIndex(path string) error {
	ctx := bg
	resolved := r.resolveParent(ctx, path)

	linkKey := resolved
	if isDir(linkKey) {
		linkKey = linkKey[:len(linkKey)-len(kvspace.DirIndexSuf)]
	}
	data, err := r.rdb.Get(ctx, linkKey).Bytes()
	if err == nil {
		head := kvspace.DecodeXValueHead(data)
		if head.IsPtr {
			pipe := r.rdb.Pipeline()
			pipe.Del(ctx, linkKey)
			parent, name := parentName(resolved)
			r.removeChild(ctx, pipe, parent, name)
			_, err = pipe.Exec(ctx)
			return err
		}
	} else if err != goredis.Nil {
		panic(fmt.Errorf("%w: UnLink %s err=%v", kvspace.ErrGet, path, err))
	}

	pipe := r.rdb.Pipeline()
	pipe.Del(ctx, resolved)
	parent, name := parentName(resolved)
	r.removeChild(ctx, pipe, parent, name)
	_, err = pipe.Exec(ctx)
	return err
}

// ── Clear / DisConn ───────────────────────────────────────────────────────────

func (r *redisImpl) Clear() error   { return r.rdb.FlushDB(bg).Err() }
func (r *redisImpl) DisConn() error { return r.rdb.Close() }
