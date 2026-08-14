package kvspace

import "fmt"

// ── CharByte ───────────────────────────────────────────────────────────

type CharByte struct{ data []uint8 }

func NewCharByte(v ...uint8) CharByte { return CharByte{data: v} }

func (v CharByte) Kind() string    { return KindCharByte }
func (v CharByte) IsPtr() bool     { return false }
func (v CharByte) ByteLen() int32  { return int32(len(v.data)) }
func (v CharByte) ArrayLen() int32 { return int32(len(v.data)) }
func (v CharByte) Encode() []byte {
	return TLVEncode(KindCharByte, copyBytes(v.data), v.ArrayLen())
}
func (v CharByte) String() string       { return string(v.data) }
func (v CharByte) ValueString() string  { return string(v.data) }
func (v CharByte) CodeString() string   { return KindCharByte + ":" + string(v.data) }
func (v CharByte) At(idx int) uint8 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("CharByte.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeCharByte(raw []byte) CharByte {
	data := make([]uint8, len(raw))
	copy(data, raw)
	return CharByte{data: data}
}

func copyBytes(b []uint8) []byte {
	raw := make([]byte, len(b))
	for i, v := range b {
		raw[i] = byte(v)
	}
	return raw
}
