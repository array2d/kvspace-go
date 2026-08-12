// Package kvspace 抽象 KV 存储。
package kvspace

import "time"

// KVPair 用于批量写入，顺序确定（非 map）。
type KVPair struct {
	Key string
	Val XValue
}

// KVSpace KV 存储接口。
//
// 使用模式：
//
//	kv.Set("/vt/0/pc", kvspace.Str("init/[0,0]"))
//	v := kv.Get("/vt/0", []string{"pc"})[0]; pc := v.Str()
//	kv.Notify("/vt/0/status", kvspace.Str("running"))
//	val, _ := kv.Watch("/vt/0/status", 5*time.Second)
//
// Watch/Notify 语义：监听单个 key 的值变化通知，不是通用消息队列。
//
//	Notify(key, val) 向等待者投递 val；不等价于 Set（不写持久值）。
//	Watch(key, timeout) 阻塞等待下一次 Notify；超时返回 (Value{}, ErrNotFound)。
//
// 软链接透明穿透：Link(target, linkpath) 后，访问 linkpath/x 透明地访问 target/x。
// 删除语义例外（POSIX rm 式）：Del/DelTree/Unlink 的最终组件作用于链接本体，
// 不穿透 target；路径中的祖先链接仍穿透（Del("/alias/x") 删 /real/x）。
type KVSpace interface {
	// ── 单点读写 ─────────────────────────────────────────────────────────
	// resolve: 是否穿透 link。true=获取 target 的值（默认），false=获取 link 本身。
	Get(prefix string, keys []string, resolve bool) []XValue
	Set(pairs []KVPair) error // 写入并维护目录索引。总是穿透 link 写入 target。

	// ── 目录操作 ─────────────────────────────────────────────────────────
	// resolve: 是否穿透 link 列出 target 的子节点。
	List(prefix string, expandExt bool, resolve bool) []string
	Del(keys ...string) error    // POSIX rm: 最终组件是 link → 删 link 本体
	DelTree(prefix string) error // 递归删除；prefix 本身是链接则只删链接

	// ── 变更通知 ─────────────────────────────────────────────────────────
	Notify(key string, val XValue) error                    // 投递一次性通知信号（穿透 link）
	Watch(key string, timeout time.Duration) XValue // 阻塞等待通知（穿透 link）

	// ── 目录创建 ─────────────────────────────────────────────────────────
	Mkindex(path string) error // 递归创建目录，类似 mkdir -p；path 须以 / 结尾

	// ── mount系统 ───────────────────────────────────────────────────────────
ExtIndex(path, extpath string) error // 创建扩展索引，path 为写层，extpath 为只读扩展
	DelExtIndex(path string) error            // 移除 extindex

	// ── 生命周期 ─────────────────────────────────────────────────────────
	// 范围警示：redis 实现 = FLUSHDB，清空所在 db 的全部键——共享 Redis 实例时会波及非 kvlang 数据。
	Clear() error
	DisConn() error
}
