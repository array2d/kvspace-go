package kvspace

import (
	"encoding/binary"
	"strconv"
	"strings"
)

// XValue 是 kvspace 中存储的类型化值。
//   - 零值（IsNil()==true）表示"不存在"或"未初始化"。
//   - 一旦由构造函数创建，字段不可修改（逻辑不可变）。
//   - raw 字节由 XValue 自身 owned，不与外部缓冲区共享。
type XValue struct {
	kind string // vtype name
	arraylength int32
	raw  []byte // 类型化原始字节
}

// ── 判断 ─────────────────────────────────────────────────────────────────

func (v XValue) IsNil() bool  { return v.kind == "" || v.kind == KindNull }
func (v XValue) Kind() string { return v.kind }

// Null returns the explicit null XValue.
func Null() XValue { return XValue{kind: KindNull} }

// RawBytes 返回底层原始字节（任意 kind）。不拷贝，调用方不得修改。
func (v XValue) RawBytes() []byte { return v.raw }

// Raw 构造任意 vtype 的 XValue（用于第三方 vtype 扩展，如 "tensor"、"rwir"）。
// raw 会被复制，调用方可安全复用原缓冲区。
// arraylength 默认=1（单值）。
func Raw(kind string, raw []byte) XValue {
	c := make([]byte, len(raw))
	copy(c, raw)
	return XValue{kind: kind, arraylength: 1, raw: c}
}

// RawN 构造 arraylength=N 的 XValue（用于数组类型的 raw 值）。
func RawN(kind string, raw []byte, n int32) XValue {
	c := make([]byte, len(raw))
	copy(c, raw)
	return XValue{kind: kind, arraylength: n, raw: c}
}

// ArrayLen 返回数组长度。单值返回 1，未初始化返回 0。
func (v XValue) ArrayLen() int32 {
	if v.kind == "" { return 0 }
	if v.arraylength <= 0 { return 1 }
	return v.arraylength
}

// ── Stringer ─────────────────────────────────────────────────────────────

// String 实现 fmt.Stringer。
// 引用（raw 长度与 kind 预期不匹配）："name:kind"
// 值（raw 匹配）："kind:value"
// 数组："kind:[elem0, elem1, ...]"
func (v XValue) String() string {
	n := v.ArrayLen()
	if n > 1 {
		parts := make([]string, n)
		for i := int32(0); i < n; i++ {
			parts[i] = plainRepr(v.Index(int(i)))
		}
		return v.kind + ":[" + strings.Join(parts, ", ") + "]"
	}
	if v.kind == KindLinkIndex {
		return "→" + string(v.raw)
	}
	return valueRepr(v)
}

// valueRepr 返回带 kind 标签的完整显示字符串。引用: name:kind，值: kind:value。
func valueRepr(v XValue) string {
	switch v.kind {
	case "", KindNull:
		return KindNull
	case "rwir":
		return plainRepr(v) + ":" + v.kind
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool", "time":
		if int32(len(v.raw)) < kindBytes(v.kind) {
			return plainRepr(v) + ":" + v.kind
		}
		return v.kind + ":" + plainRepr(v)
	case "string":
		return v.kind + ":" + plainRepr(v)
	case KindLinkIndex:
		return "→" + string(v.raw)
	default:
		if len(v.raw) > 0 {
			return plainRepr(v) + ":" + v.kind
		}
		return v.kind
	}
}

// plainRepr 返回纯值表示（无 kind 前缀），供数组元素和 valueRepr 使用。
func plainRepr(v XValue) string {
	switch v.kind {
	case "", KindNull:
		return KindNull
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool", "time":
		if int32(len(v.raw)) < kindBytes(v.kind) {
			return string(v.raw)
		}
		return numRepr(v)
	case "string":
		return v.Str()
	case "rwir":
		return string(v.raw)
	default:
		return string(v.raw)
	}
}

func kindBytes(k string) int32 {
	switch k {
	case KindBool, "int8", KindUint8: return 1
	case "int16", KindUint16: return 2
	case KindInt32, KindUint32, KindFloat32: return 4
	case KindInt64, KindUint64, KindFloat64, "time": return 8
	default: return 0
	}
}

func numRepr(v XValue) string {
	switch v.kind {
	case "int8", "int16", "int32", "int64":
		return strconv.FormatInt(v.Int64(), 10)
	case "uint8", "uint16", "uint32", "uint64":
		return strconv.FormatUint(v.Uint64(), 10)
	case "float32":
		return strconv.FormatFloat(float64(v.Float32()), 'f', -1, 32)
	case "float64":
		return strconv.FormatFloat(v.Float64(), 'f', -1, 64)
	case "bool":
		return strconv.FormatBool(v.Bool())
	case "time":
		return strconv.FormatInt(v.TimeNs(), 10)
	}
	return string(v.raw)
}

// ── TLV 编解码 ───────────────────────────────────────────────────────────
//
// 格式：[1B kind_len][N B kind_name][4B arraylength LE][4B raw_len LE][M B raw_value]
// IsNil() 编码为 nil（零字节）。
// arraylength 默认=1（单值），>1 表示数组。

func EncodeXValue(v XValue) []byte {
	if v.IsNil() { return nil }
	al := v.arraylength
	if al <= 0 { al = 1 }
	buf := make([]byte, 1+len(v.kind)+4+4+len(v.raw))
	buf[0] = byte(len(v.kind))
	copy(buf[1:], v.kind)
	binary.LittleEndian.PutUint32(buf[1+len(v.kind):], uint32(al))
	binary.LittleEndian.PutUint32(buf[1+len(v.kind)+4:], uint32(len(v.raw)))
	copy(buf[1+len(v.kind)+8:], v.raw)
	return buf
}

func DecodeXValue(data []byte) XValue {
	if len(data) == 0 { return XValue{} }
	kindLen := int(data[0])
	if len(data) < 1+kindLen+4+4 { return XValue{} }
	kind := string(data[1 : 1+kindLen])
	if !isValidKind(kind) { return XValue{} }
	al := int32(binary.LittleEndian.Uint32(data[1+kindLen : 1+kindLen+4]))
	rawLen := binary.LittleEndian.Uint32(data[1+kindLen+4 : 1+kindLen+8])
	start := 1 + kindLen + 8
	if len(data) < start+int(rawLen) { return XValue{} }
	raw := make([]byte, rawLen)
	copy(raw, data[start:start+int(rawLen)])
	return XValue{kind: kind, arraylength: al, raw: raw}
}

func EncodedXSize(v XValue) int {
	if v.IsNil() { return 0 }
	return 1 + len(v.kind) + 4 + 4 + len(v.raw)
}

func isValidKind(s string) bool {
	if len(s) == 0 || len(s) > 127 { return false }
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}
