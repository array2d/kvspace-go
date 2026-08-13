package kvspace

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// XValueHead 是 XValue 的元数据头（不含 body）。body 是 head 之后的偏移。
// XValueHead = [1B kind_len][kind][1B ref][1B arr_flag][1B ndim][ndim×4B dims][4B raw_len]
type XValueHead struct {
	Kind     string
	IsPtr    bool    // 派生：ref==1
	ArrayLen int32   // 派生：标量=1，定长=∏dims，变长=raw_len/elemSize
	Ref      int32   // 0=内联 1=软链接(*) 2=扩展句柄(@)
	ArrFlag  int32   // 0=标量 1=连续([]) 2=分离(<>)
	Ndim     int32   // 0=变长，N=定长 N 维
	Dims     []int32 // 各维长度
	BodyLen   int32   // body 字节数
}

// HeadLen 返回 XValueHead（元数据）字节数，不含 body。
func (h XValueHead) HeadLen() int32 {
	return 1 + int32(len(h.Kind)) + 1 + 1 + 1 + 4*int32(len(h.Dims)) + 4
}

// Body 从完整 XValue 字节 data 截取 body。
func (h XValueHead) Body(data []byte) []byte {
	off := int(h.HeadLen())
	if off+int(h.BodyLen) > len(data) {
		return nil
	}
	return data[off : off+int(h.BodyLen)]
}

// Decode 用 body 字节解码为 XValue。
func (h XValueHead) Decode(body []byte) XValue {
	if h.IsPtr {
		return Ptr{kind: h.Kind, target: string(body), arraylen: h.ArrayLen, isptr: true}
	}
	switch h.Kind {
	case KindInt8:
		return DecodeInt8(body)
	case KindInt16:
		return DecodeInt16(body)
	case KindInt32:
		return DecodeInt32(body)
	case KindInt64:
		return DecodeInt64(body)
	case KindUint8:
		return DecodeUint8(body)
	case KindStringByte:
		return DecodeStringByte(body)
	case KindUint16:
		return DecodeUint16(body)
	case KindUint32:
		return DecodeUint32(body)
	case KindUint64:
		return DecodeUint64(body)
	case KindFloat32:
		return DecodeFloat32(body)
	case KindFloat64:
		return DecodeFloat64(body)
	case KindBool:
		return DecodeBool(body)
	case "time":
		return DecodeTime(body)
	case "duration":
		return DecodeDuration(body)
	case KindDict:
		if len(body) == 0 {
			return Dict{}
		}
		return DecodeDictIndex(body)
	case KindIndex:
		return DecodeIndex(body)
	case KindExtIndex:
		return DecodeExtIndex(body)
	case KindRwir:
		return DecodeRwir(body)
	case KindRwfunc:
		return DecodeRwfunc(body, h.ArrayLen)
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
// XValue = XValueHead + body。
// XValueHead = [1B kind_len][kind][1B ref][1B arr_flag][1B ndim][ndim×4B dims][4B raw_len]
// body       = [raw]，offset = HeadLen()。
// ref: 0=内联 1=软链接(*) 2=扩展句柄(@)；ref=1 时 body 为目标 key 路径。None 编码为 nil。

func TLVEncode(kind string, raw []byte, arraylen int32) []byte {
	if arraylen <= 0 {
		arraylen = 1
	}
	arrFlag, dims := arrayToHeader(arraylen)
	return encodeHead(kind, 0, arrFlag, dims, raw)
}

func TLVEncodePtr(kind string, raw []byte, arraylen int32) []byte {
	if arraylen <= 0 {
		arraylen = 1
	}
	arrFlag, dims := arrayToHeader(arraylen)
	return encodeHead(kind, 1, arrFlag, dims, raw)
}

// arrayToHeader 将 arraylen 映射为 (arr_flag, dims)：<=1 标量，>1 连续一维数组。
func arrayToHeader(arraylen int32) (arrFlag int32, dims []int32) {
	if arraylen <= 1 {
		return 0, nil
	}
	return 1, []int32{arraylen}
}

func encodeHead(kind string, ref, arrFlag int32, dims []int32, raw []byte) []byte {
	ndim := int32(len(dims))
	buf := make([]byte, 1+len(kind)+1+1+1+4*len(dims)+4+len(raw))
	buf[0] = byte(len(kind))
	copy(buf[1:], kind)
	o := 1 + len(kind)
	buf[o] = byte(ref)
	buf[o+1] = byte(arrFlag)
	buf[o+2] = byte(ndim)
	for i, d := range dims {
		binary.LittleEndian.PutUint32(buf[o+3+4*i:], uint32(d))
	}
	rawLenOff := o + 3 + 4*len(dims)
	binary.LittleEndian.PutUint32(buf[rawLenOff:], uint32(len(raw)))
	copy(buf[rawLenOff+4:], raw)
	return buf
}

func DecodeXValueHead(data []byte) XValueHead {
	if len(data) == 0 {
		return XValueHead{}
	}
	kindLen := int(data[0])
	o := 1 + kindLen
	if len(data) < o+3+4 {
		return XValueHead{}
	}
	kind := string(data[1:o])
	ref := int32(data[o])
	arrFlag := int32(data[o+1])
	ndim := int32(data[o+2])
	if len(data) < o+3+4*int(ndim)+4 {
		return XValueHead{}
	}
	dims := make([]int32, ndim)
	for i := 0; i < int(ndim); i++ {
		dims[i] = int32(binary.LittleEndian.Uint32(data[o+3+4*i:]))
	}
	rawLenOff := o + 3 + 4*int(ndim)
	rawLen := int(binary.LittleEndian.Uint32(data[rawLenOff:]))
	start := rawLenOff + 4
	if len(data) < start+rawLen {
		return XValueHead{}
	}
	return XValueHead{
		Kind:     kind,
		IsPtr:    ref == 1,
		ArrayLen: headerArrayLen(arrFlag, ndim, dims, rawLen, kind),
		Ref:      ref,
		ArrFlag:  arrFlag,
		Ndim:     ndim,
		Dims:     dims,
		BodyLen:   int32(rawLen),
	}
}

// DecodeXValue 解析完整 XValue（head + body）为 XValue。
func DecodeXValue(data []byte) XValue {
	h := DecodeXValueHead(data)
	if h.Kind == "" {
		return None{}
	}
	return h.Decode(h.Body(data))
}

// BodyBytes 返回 XValue 的 body 字节。
func BodyBytes(v XValue) []byte {
	if v == nil || IsNone(v) {
		return nil
	}
	data := v.Encode()
	return DecodeXValueHead(data).Body(data)
}

// headerArrayLen 从 arr_flag/ndim/dims 推导 arraylength。
func headerArrayLen(arrFlag, ndim int32, dims []int32, rawLen int, kind string) int32 {
	switch {
	case arrFlag == 0:
		return 1
	case ndim > 0:
		n := int32(1)
		for _, d := range dims {
			n *= d
		}
		return n
	default:
		if es := ElemSize(kind); es > 0 {
			return int32(rawLen) / es
		}
		return 0
	}
}

// ── element ops ──────────────────────────────────────────────────────────

// IsByteDerived 判断 kind 是否继承 uint8（定长元素类型）。
func IsByteDerived(kind string) bool { return ElemSize(kind) > 0 }

// ElemSize 返回 kind 的单元素字节数；≤0 表示非定长类型（非 byte 派生）。
func ElemSize(kind string) int32 {
	switch kind {
	case KindInt8, KindUint8, KindStringByte, KindBool:
		return 1
	case KindInt16, KindUint16:
		return 2
	case KindInt32, KindUint32, KindFloat32:
		return 4
	case KindInt64, KindUint64, KindFloat64, "time", "duration":
		return 8
	}
	return 0
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
