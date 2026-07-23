// Package redis 提供 kvspace.KVSpace 的 Redis 实现。
package redis

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/array2d/kvspace-go"
	goredis "github.com/redis/go-redis/v9"
)

func init() { kvspace.Register("redis", ConnPool) }

var bg = context.Background()

// redisLogLv: 0=off, 1=cmd names, 2=full args+timing
var redisLogLv = func() int {
	if v, _ := strconv.Atoi(os.Getenv("KVSPACE_REDIS_LOG")); v > 0 {
		return v
	}
	return 0
}()

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

// logHook 记录每条 Redis 命令及耗时。等级由 KVSPACE_REDIS_LOG 控制。
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

type redisImpl struct {
	rdb *goredis.Client
	mu  sync.Mutex
}

// ── KVSpace interface ──────────────────────────────────────────────────────────
