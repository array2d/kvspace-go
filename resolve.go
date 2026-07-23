package kvspace

// ResolveExt 从 dirKey 的 Set 中解析 extindex 引用。
// 返回 (ext_target 路径, 是否有 extindex)。ext_target 以 / 结尾。
func ResolveExt(dirKey string, getSet func(string) []string) (string, bool) {
	for _, m := range getSet(dirKey) {
		if e := DecodeExtEntry(m); e != "" {
			return e, true
		}
	}
	return "", false
}
