package redis

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

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
