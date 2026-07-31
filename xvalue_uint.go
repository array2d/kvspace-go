package kvspace

import (
	"encoding/binary"
	"fmt"
)

// ── Uint8 ────────────────────────────────────────────────────────────────

type Uint8 struct{ xvaluebody []byte }

func NewUint8(v ...uint8) Uint8 {
	raw := make([]byte, len(v))
	for i, val := range v {
		raw[i] = val
	}
	return Uint8{xvaluebody: raw}
}

func (v Uint8) Kind() string    { return KindUint8 }
func (v Uint8) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Uint8) ArrayLen() int32 { return int32(len(v.xvaluebody)) }
func (v Uint8) Encode() []byte  { return TLVEncode(KindUint8, v.xvaluebody, v.ArrayLen()) }
func (v Uint8) At(idx int) uint8 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Uint8.At: index %d out of range [0,%d)", idx, n))
	}
	return v.xvaluebody[idx]
}

func AsUint8(v XValue) uint8              { return v.(Uint8).At(0) }
func DecodeUint8(xvaluebody []byte) Uint8 { return Uint8{xvaluebody: xvaluebody} }

// ── Uint16 ───────────────────────────────────────────────────────────────

type Uint16 struct{ xvaluebody []byte }

func NewUint16(v ...uint16) Uint16 {
	raw := make([]byte, len(v)*2)
	for i, val := range v {
		binary.LittleEndian.PutUint16(raw[i*2:], val)
	}
	return Uint16{xvaluebody: raw}
}

func (v Uint16) Kind() string    { return KindUint16 }
func (v Uint16) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Uint16) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 2 }
func (v Uint16) Encode() []byte  { return TLVEncode(KindUint16, v.xvaluebody, v.ArrayLen()) }
func (v Uint16) At(idx int) uint16 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Uint16.At: index %d out of range [0,%d)", idx, n))
	}
	return binary.LittleEndian.Uint16(v.xvaluebody[idx*2:])
}

func AsUint16(v XValue) uint16              { return v.(Uint16).At(0) }
func DecodeUint16(xvaluebody []byte) Uint16 { return Uint16{xvaluebody: xvaluebody} }

// ── Uint32 ───────────────────────────────────────────────────────────────

type Uint32 struct{ xvaluebody []byte }

func NewUint32(v ...uint32) Uint32 {
	raw := make([]byte, len(v)*4)
	for i, val := range v {
		binary.LittleEndian.PutUint32(raw[i*4:], val)
	}
	return Uint32{xvaluebody: raw}
}

func (v Uint32) Kind() string    { return KindUint32 }
func (v Uint32) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Uint32) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 4 }
func (v Uint32) Encode() []byte  { return TLVEncode(KindUint32, v.xvaluebody, v.ArrayLen()) }
func (v Uint32) At(idx int) uint32 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Uint32.At: index %d out of range [0,%d)", idx, n))
	}
	return binary.LittleEndian.Uint32(v.xvaluebody[idx*4:])
}

func AsUint32(v XValue) uint32              { return v.(Uint32).At(0) }
func DecodeUint32(xvaluebody []byte) Uint32 { return Uint32{xvaluebody: xvaluebody} }

// ── Uint64 ───────────────────────────────────────────────────────────────

type Uint64 struct{ xvaluebody []byte }

func NewUint64(v ...uint64) Uint64 {
	raw := make([]byte, len(v)*8)
	for i, val := range v {
		binary.LittleEndian.PutUint64(raw[i*8:], val)
	}
	return Uint64{xvaluebody: raw}
}

func (v Uint64) Kind() string    { return KindUint64 }
func (v Uint64) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Uint64) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 8 }
func (v Uint64) Encode() []byte  { return TLVEncode(KindUint64, v.xvaluebody, v.ArrayLen()) }
func (v Uint64) At(idx int) uint64 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Uint64.At: index %d out of range [0,%d)", idx, n))
	}
	return binary.LittleEndian.Uint64(v.xvaluebody[idx*8:])
}

func AsUint64(v XValue) uint64              { return v.(Uint64).At(0) }
func DecodeUint64(xvaluebody []byte) Uint64 { return Uint64{xvaluebody: xvaluebody} }
