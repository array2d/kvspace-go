package kvspace

import "strings"

// ── ExtIndex ──────────────────────────────────────────────────────────────────

// NewExtIndexValue 返回 extindex XValue：kind="extindex", raw=extpath。
func NewExtIndexValue(extpath string) XValue {
	return Raw(KindExtIndex, []byte(extpath))
}

// DecodeExtIndex 从 extindex XValue 提取扩展路径。非 extindex kind 返回空串。
func DecodeExtIndex(v XValue) string {
	if v.Kind() != KindExtIndex {
		return ""
	}
	return string(v.RawBytes())
}

// ── Dir Set extindex 条目编解码 ──────────────────────────────────────────────

// EncodeExtEntry 编码 dir Set 中的 extindex 条目，如 ".ext=/target/"。
func EncodeExtEntry(extpath string) string {
	return ExtIndexTag + ExtIndexSep + extpath + DirIndexSuf
}

// DecodeExtEntry 从 dir Set 成员解码 extindex 目标路径（含尾 /）。
// 非 ext 条目返回 ""。
func DecodeExtEntry(member string) string {
	if !strings.HasPrefix(member, ExtIndexTag+ExtIndexSep) {
		return ""
	}
	return member[len(ExtIndexTag)+len(ExtIndexSep):]
}
