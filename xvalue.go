package kvspace

import "encoding/binary"

// XValueHead 是 TLV 解析后的纯数据头。
type XValueHead struct {
	Kind     string
	ArrayLen int32
	Raw      []byte
}

// HeadLen 返回整个 TLV 帧字节数：[1B][kind][4B al][4B raw_len][raw]
func (h XValueHead) HeadLen() int32 {
	return 1 + int32(len(h.Kind)) + 4 + 4 + int32(len(h.Raw))
}

// Decode 按 Kind 分发到对应的 Decode* 函数，返回 XValue。
func (h XValueHead) Decode() XValue {
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
		return Dict{}
	case KindIndex:
		return DecodeIndex(h.Raw)
	case KindLinkIndex:
		return DecodeLinkIndex(h.Raw)
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
	Encode() []byte  // 完整 TLV 编码：[1B kind_len][N B kind][4B al LE][4B raw_len LE][M B raw]
	ByteLen() int32  // 数据字节长度
	ArrayLen() int32 // None=0, scalar=1, array=N
	String() string  // 纯值，无 kind 前缀（fmt.Stringer）
}

// ── None ─────────────────────────────────────────────────────────────────

type None struct{}

func (None) Kind() string    { return "" }
func (None) Encode() []byte  { return nil }
func (None) ByteLen() int32  { return 0 }
func (None) ArrayLen() int32 { return 0 }
func (None) String() string  { return "" }

func IsNone(v XValue) bool {
	if v == nil {
		return true
	}
	_, ok := v.(None)
	return ok
}

// ── TLV 编解码 ───────────────────────────────────────────────────────────
//
// 格式：[1B kind_len][N B kind_name][4B arraylength LE][4B raw_len LE][M B raw_value]
// None 编码为 nil。

func TLVEncode(kind string, raw []byte, al int32) []byte {
	if al <= 0 {
		al = 1
	}
	buf := make([]byte, 1+len(kind)+4+4+len(raw))
	buf[0] = byte(len(kind))
	copy(buf[1:], kind)
	binary.LittleEndian.PutUint32(buf[1+len(kind):], uint32(al))
	binary.LittleEndian.PutUint32(buf[1+len(kind)+4:], uint32(len(raw)))
	copy(buf[1+len(kind)+8:], raw)
	return buf
}

func DecodeXValueHead(data []byte) XValueHead {
	if len(data) == 0 {
		return XValueHead{}
	}
	kindLen := int(data[0])
	if len(data) < 1+kindLen+4+4 {
		return XValueHead{}
	}
	kind := string(data[1 : 1+kindLen])
	al := int32(binary.LittleEndian.Uint32(data[1+kindLen : 1+kindLen+4]))
	rawLen := binary.LittleEndian.Uint32(data[1+kindLen+4 : 1+kindLen+8])
	start := 1 + kindLen + 8
	if len(data) < start+int(rawLen) {
		return XValueHead{}
	}
	raw := make([]byte, rawLen)
	copy(raw, data[start:start+int(rawLen)])
	return XValueHead{Kind: kind, ArrayLen: al, Raw: raw}
}

// ── helpers ──────────────────────────────────────────────────────────────

func Format(v XValue) string {
	if IsNone(v) {
		return KindNone
	}
	return fmtKindVal(v)
}

func Plain(v XValue) string {
	if IsNone(v) {
		return KindNone
	}
	return fmtPlain(v)
}
