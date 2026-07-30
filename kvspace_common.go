package kvspace

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// ── value parser ─────────────────────────────────────────────────────────

func ParseValue(raw string) (XValue, error) {
	idx := strings.Index(raw, ":")
	if idx < 0 {
		return String(raw), nil
	}
	kind, repr := raw[:idx], raw[idx+1:]
	switch kind {
	case "int":
		i, err := strconv.ParseInt(repr, 10, 64)
		if err != nil {
			return XValue{}, fmt.Errorf("invalid int: %q", repr)
		}
		return Int64(i), nil
	case "float":
		f, err := strconv.ParseFloat(repr, 64)
		if err != nil {
			return XValue{}, fmt.Errorf("invalid float: %q", repr)
		}
		return Float64(f), nil
	case "bool":
		switch repr {
		case "true":
			return Bool(true), nil
		case "false":
			return Bool(false), nil
		default:
			return XValue{}, fmt.Errorf("invalid bool: %q", repr)
		}
	case "string":
		return String(repr), nil
	case "nil":
		return XValue{}, nil
	default:
		return Raw(kind, []byte(repr)), nil
	}
}

// ── extindex helpers ──────────────────────────────────────────────────────

func ReadPrefixExt(kv KVSpace, prefix string) string {
	v := GetOne(kv, prefix)
	_, extpath := DecodeExtIndex(v)
	return extpath
}

func ListDirExt(kv KVSpace, prefix string) []string {
	if ext := ReadPrefixExt(kv, prefix); ext != "" {
		return []string{ExtIndexHead + ext}
	}
	return nil
}

func StripExtChildren(kv KVSpace, prefix string, children []string) []string {
	extTarget := ReadPrefixExt(kv, prefix)
	if extTarget == "" {
		return children
	}
	extChildren := kv.List(extTarget, false)
	return children[:len(children)-len(extChildren)]
}

// FprintList 打印 prefix 的直接子项。
// showExt=false 时，先打印自己的 children，再以 =exttarget/ 标记缩进打印 ext 子项。
func FprintList(w io.Writer, kv KVSpace, prefix string, showExt, showKind bool) {
	children := kv.List(prefix, true)
	if !showExt {
		children = StripExtChildren(kv, prefix, children)
	}
	for _, c := range children {
		v := GetAt(kv, prefix, c)
		childDir := JoinPath(prefix, c) + DirIndexSuf
		hasDir := len(kv.List(childDir, false)) > 0
		if !hasDir {
			dirV := GetAt(kv, prefix, c+DirIndexSuf)
			hasDir = !dirV.IsNone()
		}

		key := c
		if hasDir {
			key += "/"
		}
		if v.IsNone() {
			fmt.Fprintf(w, "%s\n", key)
		} else if showKind {
			fmt.Fprintf(w, "%s\t%s\t%s\n", key, v.Kind(), v.Plain())
		} else {
			fmt.Fprintf(w, "%s\t%s\n", key, v.Plain())
		}
	}
	if !showExt {
		if ext := ReadPrefixExt(kv, prefix); ext != "" {
			fmt.Fprintln(w, ExtIndexHead+ext)
			for _, c := range kv.List(ext, false) {
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
	children := kv.List(prefix, true)
	if !showExt {
		children = StripExtChildren(kv, prefix, children)
	}

	type slot struct{ s0, s1 int; name string; val XValue }
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
	for k := range twoD { s0keys = append(s0keys, k) }
	sort.Ints(s0keys)

	for _, s0 := range s0keys {
		slots := twoD[s0]
		sort.Slice(slots, func(i, j int) bool { return slots[i].s1 < slots[j].s1 })
		minS1, maxS1 := slots[0].s1, slots[len(slots)-1].s1

		fmt.Fprintf(w, "[%d,%d~%d]", s0, minS1, maxS1)
		for _, s := range slots {
			if showKind {
				fmt.Fprintf(w, "\t%s:%s", s.val.Kind(), s.val.Plain())
			} else {
				fmt.Fprintf(w, "\t%s", s.val.Plain())
			}
		}
		fmt.Fprintln(w)
	}

	// 非二维条目按 list 格式打印
	for _, c := range regular {
		v := GetAt(kv, prefix, c)
		childDir := JoinPath(prefix, c) + DirIndexSuf
		hasDir := len(kv.List(childDir, false)) > 0
		if !hasDir {
			dirV := GetAt(kv, prefix, c+DirIndexSuf)
			hasDir = !dirV.IsNone()
		}
		key := c
		if hasDir { key += "/" }
		if v.IsNone() {
			fmt.Fprintf(w, "%s\n", key)
		} else if showKind {
			fmt.Fprintf(w, "%s\t%s\t%s\n", key, v.Kind(), v.Plain())
		} else {
			fmt.Fprintf(w, "%s\t%s\n", key, v.Plain())
		}
	}

	if !showExt {
		if ext := ReadPrefixExt(kv, prefix); ext != "" {
			fmt.Fprintln(w, ExtIndexHead+ext)
		}
	}
}

// ── tree helpers ──────────────────────────────────────────────────────────

func GetAt(kv KVSpace, dir, name string) XValue {
	return kv.Get(dir, []string{name})[0]
}

func SplitSlots(kv KVSpace, prefix string, children []string) (slots, nonslots []string) {
	for _, c := range children {
		if strings.HasPrefix(c, "[") && strings.HasSuffix(c, "]") {
			childDir := JoinPath(prefix, c) + DirIndexSuf
			if len(kv.List(childDir, false)) > 0 {
				nonslots = append(nonslots, c)
			} else {
				slots = append(slots, c)
			}
		} else {
			nonslots = append(nonslots, c)
		}
	}
	return
}

// ── 2D slot collapse ──────────────────────────────────────────────────────

// collapse2D 将 [s0,s1] 按 s0 聚合为 [s0]，返回新 children 和合成值。
// kvlang 指令序列：[s0,0]=opcode, [s0,s1<0]=读参(左), [s0,s1>0]=写参(右) → 汇编式行。
func collapse2D(kv KVSpace, prefix string, children []string) ([]string, map[string]string) {
	type key struct{ s0, s1 int }
	is2D := map[key]bool{}
	for _, c := range children {
		var s0, s1 int
		if n, _ := fmt.Sscanf(c, "[%d,%d]", &s0, &s1); n == 2 {
			// 只聚合叶子 [s0,s1]：有 childs 的目录保留原样
			if len(kv.List(JoinPath(prefix, c)+DirIndexSuf, false)) == 0 {
				is2D[key{s0, s1}] = true
			}
		}
	}
	if len(is2D) == 0 {
		return children, nil
	}

	s0set, minS1, maxS1 := map[int]bool{}, 0, 0
	first := true
	for k := range is2D {
		s0set[k.s0] = true
		if first {
			minS1, maxS1, first = k.s1, k.s1, false
		} else {
			if k.s1 < minS1 { minS1 = k.s1 }
			if k.s1 > maxS1 { maxS1 = k.s1 }
		}
	}

	aggVals := map[string]string{}
	for s0 := range s0set {
		var parts []string
		for s1 := minS1; s1 <= maxS1; s1++ {
			if is2D[key{s0, s1}] {
				v := GetAt(kv, prefix, fmt.Sprintf("[%d,%d]", s0, s1))
				parts = append(parts, v.String())
			}
		}
		aggVals[fmt.Sprintf("[%d]", s0)] = strings.Join(parts, "\t")
	}

	seen := map[int]bool{}
	var out []string
	for _, c := range children {
		var s0, s1 int
		if n, _ := fmt.Sscanf(c, "[%d,%d]", &s0, &s1); n == 2 && is2D[key{s0, s1}] {
			if !seen[s0] {
				seen[s0] = true
				out = append(out, fmt.Sprintf("[%d]", s0))
			}
		} else {
			out = append(out, c)
		}
	}
	return out, aggVals
}

// ── tree print ────────────────────────────────────────────────────────────

func FprintChildren(w io.Writer, kv KVSpace, prefix, indent string, showExt, showKind bool) {
	children := kv.List(prefix, true)
	if !showExt {
		for _, e := range ListDirExt(kv, prefix) {
			fmt.Fprintf(w, "%s%s\n", indent, e)
		}
		children = StripExtChildren(kv, prefix, children)
	}

	children, aggVals := collapse2D(kv, prefix, children)

	slots, nonslots := SplitSlots(kv, prefix, children)
	// 先打印 [x,y] 二维 slot table
	if len(slots) > 0 {
		fprintSlotTable(w, kv, prefix, indent, slots, aggVals)
	}

	// 构建非 slot 条目，目录与文件分离
	type item struct {
		name     string
		val      XValue
		childDir string
	}
	var items []item
	for _, c := range nonslots {
		v := GetAt(kv, prefix, c)
		childDir := JoinPath(prefix, c) + DirIndexSuf
		hasDir := len(kv.List(childDir, false)) > 0
		if !hasDir {
			dirV := GetAt(kv, prefix, c+DirIndexSuf)
			hasDir = !dirV.IsNone()
		}
		if hasDir {
			if !v.IsNone() {
				items = append(items, item{c + DirIndexSuf, XValue{}, childDir})
				items = append(items, item{c, v, ""})
			} else {
				items = append(items, item{c + DirIndexSuf, XValue{}, childDir})
			}
		} else {
			items = append(items, item{c, v, ""})
		}
	}

	for i, it := range items {
		last := i == len(items)-1
		branch := "├── "
		if last { branch = "└── " }
		if !it.val.IsNone() {
			if showKind {
				fmt.Fprintf(w, "%s%s%s\t%s\t%s\n", indent, branch, it.name, it.val.Kind(), it.val.Plain())
			} else {
				fmt.Fprintf(w, "%s%s%s\t%s\n", indent, branch, it.name, it.val.Plain())
			}
		} else {
			fmt.Fprintf(w, "%s%s%s\n", indent, branch, it.name)
		}
		next := indent + "│   "
		if last { next = indent + "    " }
		if it.childDir != "" {
			FprintChildren(w, kv, it.childDir, next, showExt, showKind)
		}
	}
}

func fprintSlotTable(w io.Writer, kv KVSpace, prefix, indent string, slots []string, aggVals map[string]string) {
	for i, s := range slots {
		branch := "├── "
		if i == len(slots)-1 { branch = "└── " }
		fmt.Fprintf(w, "%s%s%s\t%s\n", indent, branch, s, slotVal(kv, prefix, s, aggVals))
	}
}

func slotVal(kv KVSpace, prefix, name string, aggVals map[string]string) string {
	if v, ok := aggVals[name]; ok { return v }
	v := GetAt(kv, prefix, name)
	if v.IsNone() { return "(nil)" }
	return v.String()
}

func FprintTree(w io.Writer, kv KVSpace, prefix, indent string, showExt, showKind bool) {
	FprintChildren(w, kv, prefix, indent, showExt, showKind)
}

// JoinPath 连接父路径与子名，父路径已含尾 / 时不重复插入。
func JoinPath(parent, child string) string {
	if parent == PathSep {
		return PathSep + child
	}
	if strings.HasSuffix(parent, PathSep) {
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

// MkIndexRecursive 递归创建目录，已存在的目录跳过。
func MkIndexRecursive(kv KVSpace, path string) {
	if !strings.HasSuffix(path, DirIndexSuf) {
		panic("MkIndex: path must end with " + DirIndexSuf)
	}
	for i := 1; i < len(path); {
		j := strings.IndexByte(path[i:], '/')
		if j < 0 { break }
		i += j + 1
		dir := path[:i]
		p, n := SepPath(dir[:len(dir)-1])
		if p != PathSep { p += DirIndexSuf }
		if !dirExists(kv, p, n) {
			kv.Set([]KVPair{{dir, Raw(KindIndex, nil)}})
		}
	}
}

func dirExists(kv KVSpace, parentDir, name string) bool {
	for _, m := range kv.List(parentDir, false) {
		if m == name { return true }
	}
	return false
}

// GetOne 读取单个 key 的便捷方法。
func GetOne(kv KVSpace, key string) XValue {
	p, l := SepPath(key)
	if p != PathSep { p += DirIndexSuf }
	return kv.Get(p, []string{l})[0]
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
		vals := kv.Get(p, []string{l})
		if len(vals) > 0 && !vals[0].IsNone() {
			fn(clean, vals[0])
		}
	}
	for _, c := range kv.List(prefix, false) {
		Walk(kv, JoinPath(prefix, c)+DirIndexSuf, fn)
	}
}
