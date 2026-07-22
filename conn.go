package kvspace

import (
	"fmt"
	"sort"
	"strings"
)

// Factory 是 KVSpace 实现的构造函数类型；addr 为去除 scheme 后的地址。
type Factory func(addr string) KVSpace

var registry = map[string]Factory{}

// Register 注册一个 scheme 对应的 KVSpace 实现。在实现包的 init() 中调用。
func Register(scheme string, f Factory) { registry[scheme] = f }

// Conn 用默认连接池（16）创建 KVSpace。
func Conn(dsn string) KVSpace {
	scheme, addr := "redis", dsn
	if i := strings.Index(dsn, "://"); i >= 0 {
		scheme, addr = dsn[:i], dsn[i+3:]
	}
	f, ok := registry[scheme]
	if !ok {
		names := make([]string, 0, len(registry))
		for k := range registry {
			names = append(names, k)
		}
		sort.Strings(names)
		panic(fmt.Sprintf("kvspace: unknown scheme %q in dsn %q; registered: %v", scheme, dsn, names))
	}
	return f(addr)
}
