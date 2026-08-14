package kvspace

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// ── extindex helpers ──────────────────────────────────────────────────────

func ReadPrefixExt(kv KVSpace, prefix string) string {
	v := GetOne(kv, prefix)
	if ei, ok := v.(ExtIndex); ok {
		return ei.ExtPath()
	}
	return ""
}

func StripExtChildren(kv KVSpace, prefix string, children []string) []string {
	extTarget := ReadPrefixExt(kv, prefix)
	if extTarget == "" {
		return children
	}
	extChildren := kv.List(extTarget, false, true)
	return children[:len(children)-len(extChildren)]
}

// FprintList 打印 prefix 的直接子项。
// showExt=false 时，先打印自己的 children，再以 =exttarget/ 标记缩进打印 ext 子项。
func FprintList(w io.Writer, kv KVSpace, prefix string, showExt, showKind bool) {
	children := kv.List(prefix, true, true)
	if !showExt {
		children = StripExtChildren(kv, prefix, children)
	}
	for _, c := range children {
		v := GetAt(kv, prefix, c)
		childDir := JoinPath(prefix, c) + DirIndexSuf
		hasDir := len(kv.List(childDir, false, true)) > 0
		if !hasDir {
			dirV := GetAt(kv, prefix, c+DirIndexSuf)
			hasDir = !IsNone(dirV)
		}

		key := c
		if hasDir {
			key += "/"
			v = None{} // 目录不展示内部 index 细节
		}
		if IsNone(v) {
			fmt.Fprintf(w, "%s\n", key)
		} else if showKind {
			fmt.Fprintf(w, "%s\t%s\t%s\n", key, v.Kind(), Plain(v))
		} else {
			fmt.Fprintf(w, "%s\t%s\n", key, Plain(v))
		}
	}
	if !showExt {
		if ext := ReadPrefixExt(kv, prefix); ext != "" {
			fmt.Fprintln(w, ExtIndexHead+ext)
			for _, c := range kv.List(ext, false, true) {
				fmt.Fprintln(w, "  "+c)
			}
		}
	}
}

// ── array2d ──────────────────────────────────────────────────────────────

// FprintArray2D 按 list 风格打印，但将 [s0,s1] 二维 key 折叠为单行：
//
//	[0,-2~1]	v-2	v-1	v0	v1
//
// 非 [s0,s1] 的条目按 list 格式显示。
func FprintArray2D(w io.Writer, kv KVSpace, prefix string, showExt, showKind bool) {
	children := kv.List(prefix, true, true)
	if !showExt {
		children = StripExtChildren(kv, prefix, children)
	}

	type slot struct {
		s0, s1 int
		name   string
		val    XValue
	}
	twoD := map[int][]slot{}
	var regular []string

	for _, c := range children {
		var s0, s1 int
		if n, _ := fmt.Sscanf(c, "[%d,%d]", &s0, &s1); n == 2 {
			v := GetAt(kv, prefix, c)
			twoD[s0] = append(twoD[s0], slot{s0, s1, c, v})
		} else {
			regular = append(regular, c)
		}
	}

	// 打印折叠后的二维行（按 s0 升序）
	s0keys := make([]int, 0, len(twoD))
	for k := range twoD {
		s0keys = append(s0keys, k)
	}
	sort.Ints(s0keys)

	for _, s0 := range s0keys {
		slots := twoD[s0]
		sort.Slice(slots, func(i, j int) bool {
			a, b := slots[i].s1, slots[j].s1
			if a < 0 && b < 0 {
				return a > b
			}
			if a < 0 {
				return true
			}
			if b < 0 {
				return false
			}
			if a == 0 {
				return true
			}
			if b == 0 {
				return false
			}
			return a < b
		})
		minS1, maxS1 := slots[0].s1, slots[0].s1
		for _, s := range slots {
			if s.s1 < minS1 {
				minS1 = s.s1
			}
			if s.s1 > maxS1 {
				maxS1 = s.s1
			}
		}

		fmt.Fprintf(w, "[%d,%d~%d]", s0, minS1, maxS1)
		for _, s := range slots {
			if showKind {
				fmt.Fprintf(w, "\t%s:%s", s.val.Kind(), Plain(s.val))
			} else {
				fmt.Fprintf(w, "\t%s", Plain(s.val))
			}
		}
		fmt.Fprintln(w)
	}

	// 非二维条目按 list 格式打印
	for _, c := range regular {
		v := GetAt(kv, prefix, c)
		childDir := JoinPath(prefix, c) + DirIndexSuf
		hasDir := len(kv.List(childDir, false, true)) > 0
		if !hasDir {
			dirV := GetAt(kv, prefix, c+DirIndexSuf)
			hasDir = !IsNone(dirV)
		}
		key := c
		if hasDir {
			key += "/"
		}
		if IsNone(v) {
			fmt.Fprintf(w, "%s\n", key)
		} else if showKind {
			fmt.Fprintf(w, "%s\t%s\t%s\n", key, v.Kind(), Plain(v))
		} else {
			fmt.Fprintf(w, "%s\t%s\n", key, Plain(v))
		}
	}

	if !showExt {
		if ext := ReadPrefixExt(kv, prefix); ext != "" {
			fmt.Fprintln(w, ExtIndexHead+ext)
		}
	}
}

// ── common helpers ────────────────────────────────────────────────────────

func GetAt(kv KVSpace, dir, name string) XValue {
	return kv.Get(dir, []string{name}, true)[0]
}

func GetAtRaw(kv KVSpace, dir, name string) XValue {
	return kv.Get(dir, []string{name}, false)[0]
}

// ── tree ─────────────────────────────────────────────────────────────────

func FprintTree(w io.Writer, kv KVSpace, prefix, indent string, showExt, showKind bool) {
	children := kv.List(prefix, true, true)
	if !showExt {
		children = StripExtChildren(kv, prefix, children)
	}

	type entry struct {
		is2D     bool
		name     string
		val      XValue
		childDir string
		s0       int
		slots    []struct {
			s1  int
			val XValue
		}
	}

	// group [s0,s1] by s0, regular as-is
	type slot struct {
		s0, s1 int
		val    XValue
	}
	twoD := map[int][]slot{}
	var ordered []entry

	for _, c := range children {
		var s0, s1 int
		if n, _ := fmt.Sscanf(c, "[%d,%d]", &s0, &s1); n == 2 {
			v := GetAt(kv, prefix, c)
			twoD[s0] = append(twoD[s0], slot{s0, s1, v})
		} else {
			ordered = append(ordered, entry{name: c})
		}
	}

	// insert 2D rows before first regular entry (same as array2d order)
	s0keys := make([]int, 0, len(twoD))
	for k := range twoD {
		s0keys = append(s0keys, k)
	}
	sort.Ints(s0keys)

	twoDEntries := make([]entry, 0, len(s0keys))
	for _, s0 := range s0keys {
		slots := twoD[s0]
		sort.Slice(slots, func(i, j int) bool {
			a, b := slots[i].s1, slots[j].s1
			if a < 0 && b < 0 {
				return a > b
			}
			if a < 0 {
				return true
			}
			if b < 0 {
				return false
			}
			if a == 0 {
				return true
			}
			if b == 0 {
				return false
			}
			return a < b
		})
		conv := make([]struct {
			s1  int
			val XValue
		}, len(slots))
		for i, s := range slots {
			conv[i] = struct {
				s1  int
				val XValue
			}{s.s1, s.val}
		}
		twoDEntries = append(twoDEntries, entry{is2D: true, s0: s0, slots: conv})
	}

	// prepend 2D entries
	ordered = append(twoDEntries, ordered...)

	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].is2D != ordered[j].is2D {
			return ordered[i].is2D
		}
		a, b := ordered[i].name, ordered[j].name
		// 同名时非目录在前（"main" 在 "main/" 之前）
		as, bs := strings.TrimSuffix(a, DirIndexSuf), strings.TrimSuffix(b, DirIndexSuf)
		if as == bs {
			return !strings.HasSuffix(a, DirIndexSuf)
		}
		return as < bs
	})

	// fill val/childDir for regular entries
	for i := range ordered {
		if ordered[i].is2D {
			continue
		}
		c := ordered[i].name
		ordered[i].val = GetAt(kv, prefix, c)
		childDir := JoinPath(prefix, strings.TrimSuffix(c, DirIndexSuf)) + DirIndexSuf
		if len(kv.List(childDir, false, true)) > 0 {
			ordered[i].childDir = childDir
		} else if dirV := GetAt(kv, prefix, strings.TrimSuffix(c, DirIndexSuf)+DirIndexSuf); !IsNone(dirV) {
			ordered[i].childDir = childDir
		}
	}

	for i, e := range ordered {
		last := i == len(ordered)-1
		branch := "├── "
		nextIndent := indent + "│   "
		if last {
			branch = "└── "
			nextIndent = indent + "    "
		}

		if e.is2D {
			slots := e.slots
			minS1, maxS1 := slots[0].s1, slots[0].s1
			for _, s := range slots {
				if s.s1 < minS1 {
					minS1 = s.s1
				}
				if s.s1 > maxS1 {
					maxS1 = s.s1
				}
			}
			fmt.Fprintf(w, "%s%s[%d,%d~%d]", indent, branch, e.s0, minS1, maxS1)
			for _, s := range slots {
				if showKind {
					fmt.Fprintf(w, "\t%s:%s", s.val.Kind(), Plain(s.val))
				} else {
					fmt.Fprintf(w, "\t%s", Plain(s.val))
				}
			}
			fmt.Fprintln(w)
		} else {
			key := e.name
			if e.childDir != "" && strings.HasSuffix(e.name, DirIndexSuf) {
				fmt.Fprintf(w, "%s%s%s\n", indent, branch, key)
				FprintTree(w, kv, e.childDir, nextIndent, showExt, showKind)
			} else {
				if IsNone(e.val) {
					fmt.Fprintf(w, "%s%s%s\n", indent, branch, key)
				} else if showKind {
					fmt.Fprintf(w, "%s%s%s\t%s\t%s\n", indent, branch, key, e.val.Kind(), Plain(e.val))
				} else {
					fmt.Fprintf(w, "%s%s%s\t%s\n", indent, branch, key, Plain(e.val))
				}
			}
		}
	}

	// ext 标记
	if !showExt {
		if ext := ReadPrefixExt(kv, prefix); ext != "" {
			lastExt := len(ordered) == 0
			if lastExt {
				fmt.Fprintf(w, "%s└── %s\n", indent, ExtIndexHead+ext)
			} else {
				fmt.Fprintf(w, "%s└── %s\n", indent, ExtIndexHead+ext)
			}
		}
	}
}

func JoinPath(parent, child string) string {
	if parent == PathSep {
		return PathSep + child
	}
	if strings.HasSuffix(parent, PathSep) || strings.HasSuffix(parent, DictSep) {
		return parent + child
	}
	return parent + PathSep + child
}

func SepPath(path string) (prefix, last string) {
	if path == PathSep {
		return PathSep, ""
	}
	i := strings.LastIndexByte(path, PathSep[0])
	if i < 0 {
		return "", path
	}
	if i == 0 {
		return PathSep, path[1:]
	}
	return path[:i], path[i+1:]
}

// SplitDictParent 检查 path 末段是否包含 DictSep，
// 若 DictSep 前的 key 是 struct 目录，返回 dictDir 和 member。
func SplitDictParent(kv KVSpace, path string) (dictDir, member string, ok bool) {
	parent, last := SepPath(path)
	dot := strings.LastIndex(last, DictSep)
	if dot < 0 {
		return "", "", false
	}
	member = last[dot+1:]
	if member == "" {
		return "", "", false
	}
	if parent != PathSep {
		parent += DirIndexSuf
	}
	if dot == 0 {
		return "", "", false
	}
	dictDir = parent + last[:dot+1]
	v := GetOne(kv, dictDir)
	if v.Kind() == KindDict {
		return dictDir, member, true
	}
	return "", "", false
}

// SplitArrayParent 将 path 末段的分离数组索引 <i> / <i,j> 拆分为 (arrayBase, index)。
// 纯语法解析（对标 SepPath），不校验 arrayBase 的 kind。
func SplitArrayParent(path string) (arrayBase, index string, ok bool) {
	parent, last := SepPath(path)
	lt := strings.LastIndex(last, "<")
	if lt <= 0 || !strings.HasSuffix(last, ">") {
		return "", "", false
	}
	index = last[lt+1 : len(last)-1]
	if index == "" || strings.ContainsAny(index, "<>") {
		return "", "", false
	}
	if parent != PathSep {
		parent += DirIndexSuf
	}
	arrayBase = parent + last[:lt]
	return arrayBase, index, true
}

// MkIndexRecursive 递归创建目录，已存在的目录跳过。
func MkIndexRecursive(kv KVSpace, path string) {
	if !strings.HasSuffix(path, DirIndexSuf) {
		panic("MkIndex: path must end with " + DirIndexSuf)
	}
	for i := 1; i < len(path); {
		j := strings.IndexByte(path[i:], '/')
		if j < 0 {
			break
		}
		i += j + 1
		dir := path[:i]
		p, n := SepPath(dir[:len(dir)-1])
		if p != PathSep {
			p += DirIndexSuf
		}
		if !dirExists(kv, p, n) {
			kv.Set([]KVPair{{dir, NewIndex(nil)}})
		}
	}
}

func dirExists(kv KVSpace, parentDir, name string) bool {
	for _, m := range kv.List(parentDir, false, true) {
		if m == name || m == name+DirIndexSuf {
			return true
		}
	}
	return false
}

// ValidatePtr 检查 Ptr 的 kind/arraylen 与目标值的匹配。
// 目标不存在则跳过校验；kind 为空则跳过 kind 校验。
func ValidatePtr(kv KVSpace, target, ptrKind string, ptrArrayLen int32) error {
	v := GetOne(kv, target)
	if IsNone(v) {
		return nil
	}
	if ptrKind != "" && v.Kind() != ptrKind {
		return fmt.Errorf("%w: ptr kind mismatch: target %s is %s, ptr expects %s", ErrLinkTypeMismatch, target, v.Kind(), ptrKind)
	}
	if ptrArrayLen > 0 && v.ArrayLen() != ptrArrayLen {
		return fmt.Errorf("%w: ptr arraylen mismatch: target %s has %d, ptr expects %d", ErrInvalidValue, target, v.ArrayLen(), ptrArrayLen)
	}
	return nil
}

// GetOne 读取单个 key 的便捷方法。
func GetOne(kv KVSpace, key string) XValue {
	p, l := SepPath(key)
	if p != PathSep {
		p += DirIndexSuf
	}
	return kv.Get(p, []string{l}, true)[0]
}

// WatchValue 通用指数回退等待：轮询 GetOne(key) 直到 == targetValue。
// 先自旋（无 sleep），随后轮询间隔按指数回退，封顶 tickDuration。
// 生产者只需 Set(key, targetValue)；无通知队列，跨进程/节点/后端通用。
func WatchValue(kv KVSpace, key string, targetValue XValue, tickDuration time.Duration) XValue {
	const spinCount = 100
	cur := time.Duration(0)
	for i := 0; ; i++ {
		if v := GetOne(kv, key); equalXValue(v, targetValue) {
			return v
		}
		if i < spinCount {
			continue
		}
		if cur == 0 {
			cur = time.Microsecond
		} else if cur < tickDuration {
			cur *= 2
			if cur > tickDuration {
				cur = tickDuration
			}
		}
		time.Sleep(cur)
	}
}

func equalXValue(a, b XValue) bool {
	if a.Kind() != b.Kind() {
		return false
	}
	return bytes.Equal(BodyBytes(a), BodyBytes(b))
}

// Walk 递归遍历 prefix 下的树。prefix 须以 / 结尾。
func Walk(kv KVSpace, prefix string, fn func(path string, v XValue)) {
	if prefix != PathSep {
		clean := prefix[:len(prefix)-1]
		p, l := SepPath(clean)
		if p == "" {
			p = PathSep
		} else if p != PathSep {
			p += DirIndexSuf
		}
		vals := kv.Get(p, []string{l}, true)
		if len(vals) > 0 && !IsNone(vals[0]) {
			fn(clean, vals[0])
		}
	}
	for _, c := range kv.List(prefix, false, true) {
		Walk(kv, JoinPath(prefix, c)+DirIndexSuf, fn)
	}
}
