package kvspace

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ── value parser ─────────────────────────────────────────────────────────

func ParseValue(raw string) (XValue, error) {
	idx := strings.Index(raw, ":")
	if idx < 0 {
		return Str(raw), nil
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
		return Str(repr), nil
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
	extChildren := kv.List(extTarget)
	return children[:len(children)-len(extChildren)]
}

// FprintList 打印 prefix 的直接子项。
// showExt=false 时，先打印自己的 children，再以 =exttarget/ 标记缩进打印 ext 子项。
func FprintList(w io.Writer, kv KVSpace, prefix string, showExt bool) {
	children := kv.List(prefix)
	if !showExt {
		children = StripExtChildren(kv, prefix, children)
	}
	for _, c := range children {
		v := GetAt(kv, prefix, c)
		childDir := JoinPath(prefix, c) + DirIndexSuf
		hasDir := len(kv.List(childDir)) > 0
		if !hasDir {
			dirV := GetAt(kv, prefix, c+DirIndexSuf)
			hasDir = !dirV.IsNil()
		}
		if hasDir {
			fmt.Fprintf(w, "%s/\n", c)
		}
		if !v.IsNil() {
			fmt.Fprintf(w, "%s\t%s\n", c, v)
		}
	}
	if !showExt {
		if ext := ReadPrefixExt(kv, prefix); ext != "" {
			fmt.Fprintln(w, ExtIndexHead+ext)
			for _, c := range kv.List(ext) {
				fmt.Fprintln(w, "  "+c)
			}
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
			if len(kv.List(childDir)) > 0 {
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

// ── tree print ────────────────────────────────────────────────────────────

func FprintChildren(w io.Writer, kv KVSpace, prefix, indent string, showExt bool) {
	children := kv.List(prefix)
	if !showExt {
		for _, e := range ListDirExt(kv, prefix) {
			fmt.Fprintf(w, "%s%s\n", indent, e)
		}
		children = StripExtChildren(kv, prefix, children)
	}

	slots, nonslots := SplitSlots(kv, prefix, children)
	// 先打印 [x,y] 二维 slot table
	if len(slots) > 0 {
		fprintSlotTable(w, kv, prefix, indent, slots)
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
		hasDir := len(kv.List(childDir)) > 0
		if !hasDir {
			dirV := GetAt(kv, prefix, c+DirIndexSuf)
			hasDir = !dirV.IsNil()
		}
		if hasDir {
			if !v.IsNil() {
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
		if !it.val.IsNil() {
			fmt.Fprintf(w, "%s%s%s\t%s\n", indent, branch, it.name, it.val)
		} else {
			fmt.Fprintf(w, "%s%s%s\n", indent, branch, it.name)
		}
		next := indent + "│   "
		if last { next = indent + "    " }
		if it.childDir != "" {
			FprintChildren(w, kv, it.childDir, next, showExt)
		}
	}
}

func fprintSlotTable(w io.Writer, kv KVSpace, prefix, indent string, slots []string) {
	type slot struct{ s0, s1 int; val string }
	var rows []slot
	minS1, maxS1, maxS0 := 0, 0, 0
	for _, s := range slots {
		var s0, s1 int
		if n, _ := fmt.Sscanf(s, "[%d,%d]", &s0, &s1); n != 2 {
			panic("fprintSlotTable: invalid slot name " + s)
		}
		v := GetAt(kv, prefix, s)
		val := "(nil)"
		if !v.IsNil() { val = v.String() }
		rows = append(rows, slot{s0, s1, val})
		if s1 < minS1 { minS1 = s1 }
		if s1 > maxS1 { maxS1 = s1 }
		if s0 > maxS0 { maxS0 = s0 }
	}
	grid := make([][]string, maxS0+1)
	for i := range grid {
		row := make([]string, maxS1-minS1+1)
		for j := range row { row[j] = "" }
		grid[i] = row
	}
	for _, r := range rows { grid[r.s0][r.s1-minS1] = r.val }

	colOrder := make([]int, 0, maxS1-minS1+1)
	for s1 := -1; s1 >= minS1; s1-- { colOrder = append(colOrder, s1) }
	for s1 := 0; s1 <= maxS1; s1++ { colOrder = append(colOrder, s1) }

	for s0 := 0; s0 <= maxS0; s0++ {
		branch := "├── "
		if s0 == maxS0 { branch = "└── " }
		fmt.Fprintf(w, "%s%s[%d]", indent, branch, s0)
		for _, s1 := range colOrder {
			fmt.Fprintf(w, "\t%s", grid[s0][s1-minS1])
		}
		fmt.Fprintln(w)
	}
}

func FprintTree(w io.Writer, kv KVSpace, prefix, indent string, showExt bool) {
	FprintChildren(w, kv, prefix, indent, showExt)
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
	for _, m := range kv.List(parentDir) {
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
		if len(vals) > 0 && !vals[0].IsNil() {
			fn(clean, vals[0])
		}
	}
	for _, c := range kv.List(prefix) {
		Walk(kv, JoinPath(prefix, c)+DirIndexSuf, fn)
	}
}
