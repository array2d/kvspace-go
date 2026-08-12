package kvspace

import "fmt"

// ── StringByte ───────────────────────────────────────────────────────────

type StringByte struct{ data []uint8 }

func NewStringByte(v ...uint8) StringByte { return StringByte{data: v} }

func (v StringByte) Kind() string    { return KindStringByte }
func (v StringByte) IsPtr() bool     { return false }
func (v StringByte) ByteLen() int32  { return int32(len(v.data)) }
func (v StringByte) ArrayLen() int32 { return int32(len(v.data)) }
func (v StringByte) Encode() []byte {
	return TLVEncode(KindStringByte, copyBytes(v.data), v.ArrayLen())
}
func (v StringByte) String() string       { return string(v.data) }
func (v StringByte) ValueString() string  { return string(v.data) }
func (v StringByte) CodeString() string   { return KindStringByte + ":" + string(v.data) }
func (v StringByte) At(idx int) uint8 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("StringByte.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeStringByte(raw []byte) StringByte {
	data := make([]uint8, len(raw))
	copy(data, raw)
	return StringByte{data: data}
}

func copyBytes(b []uint8) []byte {
	raw := make([]byte, len(b))
	for i, v := range b {
		raw[i] = byte(v)
	}
	return raw
}
