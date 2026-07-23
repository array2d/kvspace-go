package kvspace

// ResolveIndexChain 从 dirKey 出发沿 extindex 链展开，上限 40 跳防环。
// getSet 返回 dirKey 的 Set 成员列表（用于测试注入）。
// 返回 [dirKey, ext1, ext2, ...]。
func ResolveIndexChain(dirKey string, getSet func(string) []string) []string {
	seen := map[string]bool{dirKey: true}
	chain := []string{dirKey}
	for range ExtMaxHops {
		ext := findExt(getSet(chain[len(chain)-1]))
		if ext == "" || seen[ext] {
			break
		}
		seen[ext] = true
		chain = append(chain, ext)
	}
	return chain
}

// findExt 从 Set 成员列表中找 extindex 条目，返回 ext 目标路径（含尾 /）。无则返 ""。
func findExt(members []string) string {
	for _, m := range members {
		if e := DecodeExtEntry(m); e != "" {
			return e
		}
	}
	return ""
}
