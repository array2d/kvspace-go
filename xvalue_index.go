package kvspace

import "strings"

// ── index  ───────────────────────────────────────────────────────────

func NewIndexValue(target []string) XValue {
	return Raw(KindIndex, []byte(strings.Join(target, IndexValueSep)))
}
func DecodeIndex(v XValue) []string {
	if v.Kind() != KindIndex { return nil }
	return strings.Split(string(v.RawBytes()), IndexValueSep)
}
// --link
// NewLinkValue 返回 link XValue：kind="link", raw=target 路径。
func NewLinkValue(target string) XValue {
	return Raw(KindLinkIndex, []byte(target))
}
// DecodeLink 从 link XValue 提取目标路径。非 link kind 返回空串。
func DecodeLink(v XValue) string {
	if v.Kind() != KindLinkIndex { return "" }
	return string(v.RawBytes())
}

// NewExtIndexValue 返回 extindex XValue：kind="extindex"。
// bytes 格式：=extpath\nchild1\nchild2...
func NewExtIndexValue(childs []string, extpath string) XValue {
	values := append([]string{ExtIndexHead + extpath}, childs...)
	return Raw(KindExtIndex, []byte(strings.Join(values, IndexValueSep)))
}

// DecodeExtIndex 从 extindex XValue 提取子成员列表和扩展路径。
// 非 extindex kind 返回 nil, ""。
func DecodeExtIndex(v XValue) (childs []string, extpath string) {
	if v.Kind() != KindExtIndex { return nil, "" }
	parts := strings.SplitN(string(v.RawBytes()), IndexValueSep, 2)
	if len(parts) == 0 { return nil, "" }
	extpath = strings.TrimPrefix(parts[0], ExtIndexHead)
	if len(parts) > 1 {
		childs = strings.Split(parts[1], IndexValueSep)
	}
	return
}

// HasExtRef 判定 XValue 是否持有 extindex 引用（link 或 extindex）。
func HasExtRef(v XValue) bool { return v.Kind() == KindLinkIndex || v.Kind() == KindExtIndex }

// ── link ───────────────────────────────────────────────────────────
