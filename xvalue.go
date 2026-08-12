package kvspace

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// XValueHead 是 TLV 解析后的纯数据头。
// TLV 头部第一字节 bit7 = isptr，bit6-0 = kind_len。
type XValueHead struct {
	Kind     string
	IsPtr    bool // bit7 of header byte
	ArrayLen int32
	Raw      []byte
}

// HeadLen 返回整个 TLV 帧字节数：[1B][kind][4B al][4B raw_len][raw]
func (h XValueHead) HeadLen() int32 {
	return 1 + int32(len(h.Kind)) + 1 + 4 + 4 + int32(len(h.Raw))
}

// Decode 按 Kind 分发到对应的 Decode* 函数，返回 XValue。
func (h XValueHead) Decode() XValue {
	if h.IsPtr {
		return Ptr{kind: h.Kind, target: string(h.Raw), arraylen: h.ArrayLen, isptr: true}
	}
	switch h.Kind {
	case KindInt8:
		return DecodeInt8(h.Raw)
	case KindInt16:
		return DecodeInt16(h.Raw)
	case KindInt32:
		return DecodeInt32(h.Raw)
	case KindInt64:
		return DecodeInt64(h.Raw)
	case KindUint8:
		return DecodeUint8(h.Raw)
	case KindUint16:
		return DecodeUint16(h.Raw)
	case KindUint32:
		return DecodeUint32(h.Raw)
	case KindUint64:
		return DecodeUint64(h.Raw)
	case KindFloat32:
		return DecodeFloat32(h.Raw)
	case KindFloat64:
		return DecodeFloat64(h.Raw)
	case KindBool:
		return DecodeBool(h.Raw)
	case KindString:
		return DecodeChar(h.Raw)
	case KindBytes:
		return DecodeBytes(h.Raw)
	case "time":
		return DecodeTime(h.Raw)
	case "duration":
		return DecodeDuration(h.Raw)
	case KindDict:
		if len(h.Raw) == 0 {
			return Dict{}
		}
		return DecodeDictIndex(h.Raw)
	case KindIndex:
		return DecodeIndex(h.Raw)
	case KindExtIndex:
		return DecodeExtIndex(h.Raw)
	case KindRwir:
		return DecodeRwir(h.Raw)
	case KindRwfunc:
		return DecodeRwfunc(h.Raw, h.ArrayLen)
	}
	return None{}
}

// XValue 是 kvspace 中所有值的统一接口。
// 零值（nil）表示"不存在"。
type XValue interface {
	Kind() string
	IsPtr() bool
	Encode() []byte  // 完整 TLV 编码：[1B kind_len][N B kind][1B isptr][4B arraylen LE][4B raw_len LE][M B raw]
	ByteLen() int32  // 数据字节长度
	ArrayLen() int32 // None=0, scalar=1, array=N
	ValueString() string
	CodeString() string
}

// ── None ─────────────────────────────────────────────────────────────────

type None struct{}

func (None) Kind() string    { return "" }
func (None) IsPtr() bool      { return false }
func (None) Encode() []byte  { return nil }
func (None) ByteLen() int32  { return 0 }
func (None) ArrayLen() int32 { return 0 }
func (None) ValueString() string { return KindNone }
func (None) CodeString() string { return KindNone }

func IsNone(v XValue) bool {
	if v == nil {
		return true
	}
	_, ok := v.(None)
	return ok
}

// ── Ptr ───────────────────────────────────────────────────────────────────

type Ptr struct {
	kind     string // 目标类型
	target   string // 目标 key 路径
	arraylen int32
	isptr    bool // always true
}

func NewPtr(kind, target string, arraylen int32) Ptr {
	return Ptr{kind: kind, target: target, arraylen: arraylen, isptr: true}
}

func (v Ptr) Kind() string    { return v.kind }
func (v Ptr) IsPtr() bool      { return v.isptr }
func (v Ptr) ByteLen() int32  { return int32(len(v.target)) }
func (v Ptr) ArrayLen() int32 { return v.arraylen }
func (v Ptr) Encode() []byte  { return TLVEncodePtr(v.kind, []byte(v.target), v.arraylen) }
func (v Ptr) Target() string  { return v.target }

func (v Ptr) ValueString() string { return "→" + v.target }
func (v Ptr) CodeString() string  { return fmt.Sprintf("→%s:%s", v.Target(), v.Kind()) }

func IsPtr(v XValue) bool {
	if v == nil { return false }
	return v.IsPtr()
}

func PtrTarget(v XValue) string {
	if p, ok := v.(Ptr); ok {
		return p.target
	}
	return ""
}

// ── TLV 编解码 ───────────────────────────────────────────────────────────
//
// 格式：[1B kind_len][N B kind][1B isptr][4B arraylen LE][4B raw_len LE][M B raw]
// isptr=1 时 raw 为指针目标 key 路径，Kind 为目标类型。
// None 编码为 nil。

func TLVEncode(kind string, raw []byte, arraylen int32) []byte {
	if arraylen <= 0 {
		arraylen = 1
	}
	buf := make([]byte, 1+len(kind)+1+4+4+len(raw))
	buf[0] = byte(len(kind))
	copy(buf[1:], kind)
	buf[1+len(kind)] = 0 // isptr=0
	binary.LittleEndian.PutUint32(buf[1+len(kind)+1:], uint32(arraylen))
	binary.LittleEndian.PutUint32(buf[1+len(kind)+1+4:], uint32(len(raw)))
	copy(buf[1+len(kind)+1+8:], raw)
	return buf
}

func TLVEncodePtr(kind string, raw []byte, arraylen int32) []byte {
	buf := TLVEncode(kind, raw, arraylen)
	buf[1+len(kind)] = 1
	return buf
}

func DecodeXValueHead(data []byte) XValueHead {
	if len(data) == 0 {
		return XValueHead{}
	}
	kindLen := int(data[0])
	if len(data) < 1+kindLen+1+4+4 {
		return XValueHead{}
	}
	kind := string(data[1 : 1+kindLen])
	isPtr := data[1+kindLen] != 0
	arraylen := int32(binary.LittleEndian.Uint32(data[1+kindLen+1 : 1+kindLen+1+4]))
	rawLen := binary.LittleEndian.Uint32(data[1+kindLen+1+4 : 1+kindLen+1+8])
	start := 1 + kindLen + 1 + 8
	if len(data) < start+int(rawLen) {
		return XValueHead{}
	}
	raw := make([]byte, rawLen)
	copy(raw, data[start:start+int(rawLen)])
	return XValueHead{Kind: kind, IsPtr: isPtr, ArrayLen: arraylen, Raw: raw}
}

// ── helpers ──────────────────────────────────────────────────────────────

func Format(v XValue) string {
	if IsNone(v) {
		return KindNone
	}
	return v.CodeString()
}

func Plain(v XValue) string {
	if IsNone(v) {
		return KindNone
	}
	return v.ValueString()
}

func fmtArray(n int, fn func(int) string) string {
	if n == 1 {
		return fn(0)
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = fn(i)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
