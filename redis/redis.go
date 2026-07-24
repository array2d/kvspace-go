// Package redis 提供 kvspace.KVSpace 的 Redis 实现。
package redis

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/array2d/kvspace-go"
	goredis "github.com/redis/go-redis/v9"
)

func init() { kvspace.Register("redis", ConnPool) }

var bg = context.Background()

var redisLogLv = func() int {
	if v, _ := strconv.Atoi(os.Getenv("KVSPACE_REDIS_LOG")); v > 0 {
		return v
	}
	return 0
}()

const notifyPrefix = "__notify:"

func Conn(dsn string) kvspace.KVSpace { return ConnPool(dsn) }

func ConnPool(dsn string) kvspace.KVSpace {
	poolSize := 16
	rdb := goredis.NewClient(&goredis.Options{
		Addr: dsn, PoolSize: poolSize,
		MinIdleConns: 4,
		PoolTimeout:  10 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second,
	})
	if redisLogLv > 0 {
		rdb.AddHook(&logHook{})
	}
	return &redisImpl{rdb: rdb}
}

// ── 实现体 ────────────────────────────────────────────────────────────────────

type redisImpl struct {
	rdb *goredis.Client
}

// ── 目录与路径工具 ────────────────────────────────────────────────────────────

func isDir(path string) bool { return strings.HasSuffix(path, kvspace.DirIndexSuf) }

func assertDir(path string) {
	if path != kvspace.PathSep && !isDir(path) {
		panic(fmt.Errorf("%w: %s", kvspace.ErrDirMustEndWithSlash, path))
	}
}

func parentName(path string) (string, string) {
	if isDir(path) && path != kvspace.PathSep {
		path = path[:len(path)-len(kvspace.DirIndexSuf)]
	}
	parent, last := kvspace.SepPath(path)
	if parent != kvspace.PathSep {
		parent += kvspace.DirIndexSuf
	}
	return parent, last
}

// resolvePath 解析路径中所有 link，直接 panic 于异常。
func (r *redisImpl) resolvePath(ctx context.Context, path string) string {
	for {
		resolved, changed := r.resolveOne(ctx, path)
		if !changed {
			return resolved
		}
		path = resolved
	}
}

// resolveParent 解析父路径中的 link（末段不解析）。
func (r *redisImpl) resolveParent(ctx context.Context, path string) string {
	dirSuf := isDir(path) && path != kvspace.PathSep
	clean := path
	if dirSuf {
		clean = path[:len(path)-len(kvspace.DirIndexSuf)]
	}
	parent, last := kvspace.SepPath(clean)
	if parent == clean {
		return path
	}
	resolved := r.resolvePath(ctx, parent)
	result := kvspace.JoinPath(resolved, last)
	if dirSuf {
		result += kvspace.DirIndexSuf
	}
	return result
}

func (r *redisImpl) resolveOne(ctx context.Context, path string) (string, bool) {
	if path == kvspace.PathSep {
		return path, false
	}
	parts := strings.Split(strings.Trim(path, kvspace.PathSep), kvspace.PathSep)
	cur := kvspace.PathSep
	for i, p := range parts {
		cur = kvspace.JoinPath(cur, p)
		data, err := r.rdb.Get(ctx, cur).Bytes()
		if err == goredis.Nil {
			continue
		}
		if err != nil {
			panic(fmt.Errorf("%w: %s err=%v", kvspace.ErrResolve, cur, err))
		}
		v := kvspace.DecodeXValue(data)
		if v.Kind() == kvspace.KindLinkIndex {
			target := string(v.RawBytes())
			if i+1 < len(parts) {
				return kvspace.JoinPath(target, strings.Join(parts[i+1:], kvspace.PathSep)), true
			}
			return target, true
		}
	}
	return path, false
}

func (r *redisImpl) extIndex(ctx context.Context, dir string)(values []string, extpath string) {
	if !isDir(dir) {
		panic(kvspace.ErrDirMustEndWithSlash)
	}
	data, err := r.rdb.Get(ctx, dir).Bytes()
	if err != nil {
		panic(err)
	}
	values, extpath = kvspace.DecodeExtIndex(kvspace.DecodeXValue(data))
	return
}

func (r *redisImpl) ensureParent(ctx context.Context, parent, child string) {
	if parent == kvspace.PathSep {
		return
	}
	if !isDir(parent) {
		panic(fmt.Errorf("%w: %s", kvspace.ErrNotDir, parent))
	}
	gp, pn := parentName(parent)
	nodes := r.readDirIndex(ctx, gp)
	for _, n := range nodes {
		if n == pn {
			return
		}
	}

	// 检查 exttarget：父目录可能在 extIndex 的只读层
	var extT string
	if data, err := r.rdb.Get(ctx, gp).Bytes(); err == nil {
		if v := kvspace.DecodeXValue(data); v.Kind() == kvspace.KindExtIndex {
			_, extT = kvspace.DecodeExtIndex(v)
		}
	}
	if extT != "" {
		extNodes := r.readDirIndex(ctx, extT)
		for _, n := range extNodes {
			if n == pn {
				// 在 extIndex 本地层创建影子目录
				nodes = append(nodes, pn)
				r.rdb.Set(ctx, gp, kvspace.EncodeXValue(kvspace.NewIndexValue(nodes)), 0)
				return
			}
		}
	}
	panic(fmt.Errorf("%w: %s", kvspace.ErrParentNotFound, parent))
}

func (r *redisImpl) collectKeys(ctx context.Context, prefix string) []string {
	pattern := prefix + "*"
	var keys []string
	var cursor uint64
	for {
		var batch []string
		var err error
		batch, cursor, err = r.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			panic(fmt.Errorf("%w: pattern=%s err=%v", kvspace.ErrScan, pattern, err))
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}
	return keys
}
