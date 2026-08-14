// Package kvspace 抽象 KV 存储。
package kvspace

import "time"

// KVPair 用于批量写入，顺序确定（非 map）。
type KVPair struct {
	Key    string
	Val    XValue
}

// KVSpace KV 存储接口。
//
// 使用模式：
//
//	kv.Set([]KVPair{{"/vt/0/pc", NewCharByte([]byte("init/[0,0]")...)}})
//	v := kv.Get("/vt/0", []string{"pc"}, true)[0]
//
// Watch 语义：阻塞等待 Get(key) == targetValue。
// 先自旋（无 sleep），随后轮询间隔按指数回退，封顶 tickDuration。
// 生产者只需 Set(key, targetValue)；无通知队列，跨进程/节点/后端通用。
//
// 软链接透明穿透：Set 写入 Ptr 值（*kind:target）后，访问 /linkpath/x 透明地访问 target/x。
// 删除语义例外（POSIX rm 式）：Del/DelTree 的最终组件作用于链接本体，
// 不穿透 target；路径中的祖先链接仍穿透（Del("/alias/x") 删 /real/x）。
type KVSpace interface {
	// ── 单点读写 ─────────────────────────────────────────────────────────
	// 整存整取：Get 返回完整 XValue，Set 写完整 XValue，无部分读写。
	Get(prefix string, keys []string, resolve bool) []XValue
	Set(pairs []KVPair) error // 写入并维护目录索引。总是穿透 link 写入 target。

	// ── 目录操作 ─────────────────────────────────────────────────────────
	// resolve: 是否穿透 link 列出 target 的子节点。
	List(prefix string, expandExt bool, resolve bool) []string
	Del(keys ...string) error    // POSIX rm: 最终组件是 link → 删 link 本体
	DelTree(prefix string) error // 递归删除；prefix 本身是链接则只删链接

	// ── 变更等待 ─────────────────────────────────────────────────────────
	Watch(key string, targetValue XValue, tickDuration time.Duration) XValue // 阻塞等待 Get(key)==targetValue

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
