package kvspace

import "strings"

// ── Link / ExtIndex ───────────────────────────────────────────────────────────

// NewLinkValue 返回 link XValue：kind="link", raw=target 路径。
func NewLinkValue(target string) XValue {
	return Raw(KindLinkIndex, []byte(target))
}

// DecodeLink 从 link XValue 提取目标路径。非 link kind 返回空串。
func DecodeLink(v XValue) string {
	if v.Kind() != KindLinkIndex { return "" }
	return string(v.RawBytes())
}

// NewExtIndexValue 返回 extindex XValue：kind="extindex", raw=extpath。
func NewExtIndexValue(extpath string) XValue {
	return Raw(KindExtIndex, []byte(extpath))
}

// DecodeExtIndex 从 extindex XValue 提取扩展路径。非 extindex kind 返回空串。
func DecodeExtIndex(v XValue) string {
	if v.Kind() != KindExtIndex { return "" }
	return string(v.RawBytes())
}

// HasExtRef 判定 XValue 是否持有 extindex 引用（link 或 extindex）。
func HasExtRef(v XValue) bool { return v.Kind() == KindLinkIndex || v.Kind() == KindExtIndex }

// IsLink 判定 XValue 是否为纯链接。
func IsLink(v XValue) bool { return v.Kind() == KindLinkIndex }

// ── Dir Set extindex 条目编解码 ──────────────────────────────────────────────

// EncodeExtEntry 编码 dir Set 中的 extindex 条目，如 "=/target/"。
func EncodeExtEntry(extpath string) string {
	if strings.HasSuffix(extpath, DirIndexSuf) {
		return ExtIndexHead + extpath
	}
	return ExtIndexHead + extpath + DirIndexSuf
}

// DecodeExtEntry 从 dir Set 成员解码 extindex 目标路径（含尾 /）。
// 非 ext 条目返回 ""。
func DecodeExtEntry(member string) string {
	if !strings.HasPrefix(member, ExtIndexHead) {
		return ""
	}
	return member[len(ExtIndexHead):]
}
