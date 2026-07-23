// Package redis 提供 kvspace.KVSpace 的 Redis 实现。
package redis

import (
	"context"
	"fmt"
	"log"
	"net"
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

// ── 日志 Hook ─────────────────────────────────────────────────────────────────

type logHook struct{}

func (h *logHook) DialHook(next goredis.DialHook) goredis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *logHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		t0 := time.Now()
		err := next(ctx, cmd)
		if redisLogLv >= 2 {
			log.Printf("[redis] %s → %v", cmd.String(), time.Since(t0).Round(time.Microsecond))
		} else {
			log.Printf("[redis] %s", cmdName(cmd))
		}
		return err
	}
}

func (h *logHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		t0 := time.Now()
		err := next(ctx, cmds)
		var sb strings.Builder
		for i, c := range cmds {
			if i > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString(cmdName(c))
		}
		if redisLogLv >= 2 {
			log.Printf("[redis] pipeline(%d) [%s] → %v", len(cmds), sb.String(), time.Since(t0).Round(time.Microsecond))
		} else {
			log.Printf("[redis] pipeline(%d) [%s]", len(cmds), sb.String())
		}
		return err
	}
}

func cmdName(c goredis.Cmder) string {
	s := c.String()
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}

// ── 实现体 ────────────────────────────────────────────────────────────────────

type redisImpl struct {
	rdb *goredis.Client
}

// ── 目录与路径工具 ────────────────────────────────────────────────────────────

func isDir(path string) bool { return strings.HasSuffix(path, kvspace.DirIndexSuf) }

func assertDir(path string) {
	if path != kvspace.PathSep && !isDir(path) {
		panic(fmt.Sprintf("kvspace: 目录必须以 / 结尾: %s", path))
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
			panic(fmt.Sprintf("kvspace: 路径解析 GET 失败: %s err=%v", cur, err))
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

func (r *redisImpl) extTarget(ctx context.Context, dir string) string {
	if !isDir(dir) {
		return ""
	}
	members, err := r.rdb.SMembers(ctx, dir).Result()
	if err == goredis.Nil {
		return ""
	}
	if err != nil {
		panic(fmt.Sprintf("kvspace: extTarget SMEMBERS 失败: %s err=%v", dir, err))
	}
	for _, m := range members {
		if t := kvspace.DecodeExtEntry(m); t != "" {
			return t
		}
	}
	return ""
}

func (r *redisImpl) ensureParent(ctx context.Context, parent, child string) {
	if parent == kvspace.PathSep {
		return
	}
	if !isDir(parent) {
		panic(fmt.Sprintf("kvspace: 父路径不是目录: %s（Set %s）", parent, child))
	}
	gp, pn := parentName(parent)
	exists, err := r.rdb.SIsMember(ctx, gp, pn).Result()
	if err != nil {
		panic(fmt.Sprintf("kvspace: 父目录检查失败: %s err=%v", parent, err))
	}
	if exists {
		return
	}

	// 检查 exttarget：父目录可能在 extIndex 的只读层
	extT := r.extTarget(ctx, gp)
	if extT != "" {
		extExists, err := r.rdb.SIsMember(ctx, extT, pn).Result()
		if err != nil {
			panic(fmt.Sprintf("kvspace: 父目录 ext 检查失败: %s err=%v", parent, err))
		}
		if extExists {
			// 在 extIndex 本地层创建影子目录，后续写操作落在本地
			if err := r.rdb.SAdd(ctx, gp, pn).Err(); err != nil {
				panic(fmt.Sprintf("kvspace: 父目录影子创建失败: %s err=%v", parent, err))
			}
			return
		}
	}
	panic(fmt.Sprintf("kvspace: 父目录不存在: %s（Set %s）", parent, child))
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
			panic(fmt.Sprintf("kvspace: SCAN 失败: pattern=%s err=%v", pattern, err))
		}
		keys = append(keys, batch...)
		if cursor == 0 {
			break
		}
	}
	return keys
}
