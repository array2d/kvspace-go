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

func (v XValue) IsNone() bool { return v.kind == "" }
func (v XValue) Kind() string { return v.kind }

// None returns the explicit none XValue.
func None() XValue { return XValue{kind: KindNone} }

// RawBytes 返回底层原始字节（任意 kind）。不拷贝，调用方不得修改。
func (v XValue) RawBytes() []byte { return v.raw }

// Plain 返回纯值表示（无 kind 前缀），如 "42", "hello"。
func (v XValue) Plain() string { return plainRepr(v) }

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

// ── rwir / rwfunc 专用构造函数 ──────────────────────────────────────────
//
// 设计：计数存于 XValue 固定字段，运行时零判断零解析。
//
//   rwir:  arraylength = (numReads<<16) | numWrites   raw = sig bytes
//   rwfunc: arraylength = numInsts                     raw = [2B LE:reads][2B LE:writes][sig...]

// Rwir 构造 kind="rwir" 的 XValue。arraylength 打包读写参数量，raw 存签名。
func Rwir(numReads, numWrites int32, sig []byte) XValue {
	raw := make([]byte, len(sig))
	copy(raw, sig)
	return XValue{kind: KindRwir, arraylength: (numReads << 16) | (numWrites & 0xFFFF), raw: raw}
}

// RwirReads 返回 rwir 读参数量。零判断，直读 arraylength。
func (v XValue) RwirReads() int32 { return v.arraylength >> 16 }

// RwirWrites 返回 rwir 写参数量。零判断，直读 arraylength。
func (v XValue) RwirWrites() int32 { return v.arraylength & 0xFFFF }

// RwirSig 返回 rwir 签名正文。raw 即 sig。
func (v XValue) RwirSig() string { return string(v.raw) }

const rwfuncRawHeader = 4 // 2B reads + 2B writes

// Rwfunc 构造 kind="rwfunc" 的 XValue。arraylength=指令数，raw 头部存读写参数量+签名。
func Rwfunc(numInsts, numReads, numWrites int32, sig []byte) XValue {
	raw := make([]byte, rwfuncRawHeader+len(sig))
	binary.LittleEndian.PutUint16(raw[0:2], uint16(numReads))
	binary.LittleEndian.PutUint16(raw[2:4], uint16(numWrites))
	copy(raw[4:], sig)
	return XValue{kind: KindRwfunc, arraylength: numInsts, raw: raw}
}

// RwfuncInsts 返回 rwfunc 指令数。零判断，直读 arraylength。
func (v XValue) RwfuncInsts() int32 { return v.arraylength }

// RwfuncReads 返回 rwfunc 读参数量。固定偏移，零判断。
func (v XValue) RwfuncReads() int32 { return int32(binary.LittleEndian.Uint16(v.raw[0:2])) }

// RwfuncWrites 返回 rwfunc 写参数量。固定偏移，零判断。
func (v XValue) RwfuncWrites() int32 { return int32(binary.LittleEndian.Uint16(v.raw[2:4])) }

// RwfuncSig 返回 rwfunc 签名正文（跳过 4B 头）。
func (v XValue) RwfuncSig() string {
	if len(v.raw) >= rwfuncRawHeader { return string(v.raw[4:]) }
	return string(v.raw)
}

// ArrayLen 返回数组长度。单值返回 1，未初始化返回 0。
func (v XValue) ArrayLen() int32 {
	if v.kind == "" { return 0 }
	if v.arraylength < 0 {
		 panic("XValue.ArrayLen: negative arraylength")
	}
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
	case "":
		return KindNone
	case KindRwir:
		return v.RwirSig() + ":" + v.kind
	case KindRwfunc:
		return v.RwfuncSig() + ":" + v.kind
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
	case "":
		return KindNone
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool", "time":
		if int32(len(v.raw)) < kindBytes(v.kind) {
			return string(v.raw)
		}
		return numRepr(v)
	case "string":
		return v.Str()
	case KindRwir:
		return v.RwirSig()
	case KindRwfunc:
		return v.RwfuncSig()
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
	if v.IsNone() { return nil }
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
	if v.IsNone() { return 0 }
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
