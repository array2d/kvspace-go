package kvspace

import "strings"

// ResolveCore 路径解析：从短到长逐 '/' 边界查链接并展开，上限 40 跳防环。
//
// lookup 返回 path 的链接目标，非链接时返回 ""。
// 调用方（memImpl / redisImpl）传入各自的 checkLink 方法。
//
// 用 strings.IndexByte 快速定位 '/' 边界（SIMD 加速），避免逐字符扫描。
func ResolveCore(path string, lookup func(string) string) string {
	for range 40 {
		found := false
		for i := 1; i < len(path); {
			j := strings.IndexByte(path[i:], '/')
			if j < 0 {
				break
			}
			i += j
			if t := lookup(path[:i]); t != "" {
				path, found = t+path[i:], true
				break
			}
			i++
		}
		if !found {
			if t := lookup(path); t != "" {
				path, found = t, true
			}
		}
		if !found {
			return path
		}
	}
	return path
}

// ResolveParent 删除语义的路径解析：祖先链接穿透，末段保留本体（POSIX rm 式）。
// Del/DelTree/Unlink 的最终组件作用于链接本身，不穿透到 target；
// 路径中的祖先链接仍正常穿透（Del("/alias/x") 删 /real/x）。
func ResolveParent(path string, lookup func(string) string) string {
	slash := strings.LastIndexByte(path, '/')
	if slash <= 0 {
		return path // 根层键或异常形态：无祖先可穿透
	}
	return ResolveCore(path[:slash], lookup) + path[slash:]
}
