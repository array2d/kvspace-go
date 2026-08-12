package kvspace

import (
	"encoding/binary"
	"fmt"
	"strconv"
)

// ── Uint8 ────────────────────────────────────────────────────────────────

type Uint8 struct{ data []uint8 }

func NewUint8(v ...uint8) Uint8 { return Uint8{data: v} }

func (v Uint8) Kind() string    { return KindUint8 }

func (v Uint8) IsPtr() bool	{ return false }
func (v Uint8) String() string  { return fmtArray(len(v.data), func(i int) string { return strconv.FormatUint(uint64(v.data[i]), 10) }) }
func (v Uint8) ByteLen() int32  { return int32(len(v.data)) }
func (v Uint8) ArrayLen() int32 { return int32(len(v.data)) }
func (v Uint8) Encode() []byte {
	raw := make([]byte, len(v.data))
	for i, val := range v.data {
		raw[i] = val
	}
	return TLVEncode(KindUint8, raw, v.ArrayLen())
}
func (v Uint8) At(idx int) uint8 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Uint8.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeUint8(xvaluebody []byte) Uint8 {
	data := make([]uint8, len(xvaluebody))
	copy(data, xvaluebody)
	return Uint8{data: data}
}

// ── Uint16 ───────────────────────────────────────────────────────────────

type Uint16 struct{ data []uint16 }

func NewUint16(v ...uint16) Uint16 { return Uint16{data: v} }

func (v Uint16) Kind() string    { return KindUint16 }

func (v Uint16) IsPtr() bool	{ return false }
func (v Uint16) String() string  { return fmtArray(len(v.data), func(i int) string { return strconv.FormatUint(uint64(v.data[i]), 10) }) }
func (v Uint16) ByteLen() int32  { return int32(len(v.data) * 2) }
func (v Uint16) ArrayLen() int32 { return int32(len(v.data)) }
func (v Uint16) Encode() []byte {
	raw := make([]byte, len(v.data)*2)
	for i, val := range v.data {
		binary.LittleEndian.PutUint16(raw[i*2:], val)
	}
	return TLVEncode(KindUint16, raw, v.ArrayLen())
}
func (v Uint16) At(idx int) uint16 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Uint16.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeUint16(xvaluebody []byte) Uint16 {
	data := make([]uint16, len(xvaluebody)/2)
	for i := range data {
		data[i] = binary.LittleEndian.Uint16(xvaluebody[i*2:])
	}
	return Uint16{data: data}
}

// ── Uint32 ───────────────────────────────────────────────────────────────

type Uint32 struct{ data []uint32 }

func NewUint32(v ...uint32) Uint32 { return Uint32{data: v} }

func (v Uint32) Kind() string    { return KindUint32 }

func (v Uint32) IsPtr() bool	{ return false }
func (v Uint32) String() string  { return fmtArray(len(v.data), func(i int) string { return strconv.FormatUint(uint64(v.data[i]), 10) }) }
func (v Uint32) ByteLen() int32  { return int32(len(v.data) * 4) }
func (v Uint32) ArrayLen() int32 { return int32(len(v.data)) }
func (v Uint32) Encode() []byte {
	raw := make([]byte, len(v.data)*4)
	for i, val := range v.data {
		binary.LittleEndian.PutUint32(raw[i*4:], val)
	}
	return TLVEncode(KindUint32, raw, v.ArrayLen())
}
func (v Uint32) At(idx int) uint32 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Uint32.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeUint32(xvaluebody []byte) Uint32 {
	data := make([]uint32, len(xvaluebody)/4)
	for i := range data {
		data[i] = binary.LittleEndian.Uint32(xvaluebody[i*4:])
	}
	return Uint32{data: data}
}

// ── Uint64 ───────────────────────────────────────────────────────────────

type Uint64 struct{ data []uint64 }

func NewUint64(v ...uint64) Uint64 { return Uint64{data: v} }

func (v Uint64) Kind() string    { return KindUint64 }

func (v Uint64) IsPtr() bool	{ return false }
func (v Uint64) String() string  { return fmtArray(len(v.data), func(i int) string { return strconv.FormatUint(v.data[i], 10) }) }
func (v Uint64) ByteLen() int32  { return int32(len(v.data) * 8) }
func (v Uint64) ArrayLen() int32 { return int32(len(v.data)) }
func (v Uint64) Encode() []byte {
	raw := make([]byte, len(v.data)*8)
	for i, val := range v.data {
		binary.LittleEndian.PutUint64(raw[i*8:], val)
	}
	return TLVEncode(KindUint64, raw, v.ArrayLen())
}
func (v Uint64) At(idx int) uint64 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Uint64.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeUint64(xvaluebody []byte) Uint64 {
	data := make([]uint64, len(xvaluebody)/8)
	for i := range data {
		data[i] = binary.LittleEndian.Uint64(xvaluebody[i*8:])
	}
	return Uint64{data: data}
}
