package kvspace

import (
	"encoding/binary"
	"fmt"
	"strconv"
)

// ── Int8 ─────────────────────────────────────────────────────────────────

type Int8 struct{ xvaluebody []byte }

func NewInt8(v ...int8) Int8 {
	raw := make([]byte, len(v))
	for i, val := range v {
		raw[i] = byte(val)
	}
	return Int8{xvaluebody: raw}
}

func (v Int8) Kind() string    { return KindInt8 }
func (v Int8) String() string  { return fmtArray(int(v.ArrayLen()), func(i int) string { return strconv.FormatInt(int64(v.At(i)), 10) }) }
func (v Int8) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Int8) ArrayLen() int32 { return int32(len(v.xvaluebody)) }
func (v Int8) Encode() []byte  { return TLVEncode(KindInt8, v.xvaluebody, v.ArrayLen()) }
func (v Int8) At(idx int) int8 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Int8.At: index %d out of range [0,%d)", idx, n))
	}
	return int8(v.xvaluebody[idx])
}

func DecodeInt8(xvaluebody []byte) Int8 { return Int8{xvaluebody: xvaluebody} }

// ── Int16 ────────────────────────────────────────────────────────────────

type Int16 struct{ xvaluebody []byte }

func NewInt16(v ...int16) Int16 {
	raw := make([]byte, len(v)*2)
	for i, val := range v {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(val))
	}
	return Int16{xvaluebody: raw}
}

func (v Int16) Kind() string    { return KindInt16 }
func (v Int16) String() string  { return fmtArray(int(v.ArrayLen()), func(i int) string { return strconv.FormatInt(int64(v.At(i)), 10) }) }
func (v Int16) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Int16) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 2 }
func (v Int16) Encode() []byte  { return TLVEncode(KindInt16, v.xvaluebody, v.ArrayLen()) }
func (v Int16) At(idx int) int16 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Int16.At: index %d out of range [0,%d)", idx, n))
	}
	return int16(binary.LittleEndian.Uint16(v.xvaluebody[idx*2:]))
}

func DecodeInt16(xvaluebody []byte) Int16 { return Int16{xvaluebody: xvaluebody} }

// ── Int32 ────────────────────────────────────────────────────────────────

type Int32 struct{ xvaluebody []byte }

func NewInt32(v ...int32) Int32 {
	raw := make([]byte, len(v)*4)
	for i, val := range v {
		binary.LittleEndian.PutUint32(raw[i*4:], uint32(val))
	}
	return Int32{xvaluebody: raw}
}

func (v Int32) Kind() string    { return KindInt32 }
func (v Int32) String() string  { return fmtArray(int(v.ArrayLen()), func(i int) string { return strconv.FormatInt(int64(v.At(i)), 10) }) }
func (v Int32) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Int32) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 4 }
func (v Int32) Encode() []byte  { return TLVEncode(KindInt32, v.xvaluebody, v.ArrayLen()) }
func (v Int32) At(idx int) int32 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Int32.At: index %d out of range [0,%d)", idx, n))
	}
	return int32(binary.LittleEndian.Uint32(v.xvaluebody[idx*4:]))
}

func DecodeInt32(xvaluebody []byte) Int32 { return Int32{xvaluebody: xvaluebody} }

// ── Int64 ────────────────────────────────────────────────────────────────

type Int64 struct{ xvaluebody []byte }

func NewInt64(v ...int64) Int64 {
	raw := make([]byte, len(v)*8)
	for i, val := range v {
		binary.LittleEndian.PutUint64(raw[i*8:], uint64(val))
	}
	return Int64{xvaluebody: raw}
}

func (v Int64) Kind() string    { return KindInt64 }
func (v Int64) String() string  { return fmtArray(int(v.ArrayLen()), func(i int) string { return strconv.FormatInt(v.At(i), 10) }) }
func (v Int64) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Int64) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 8 }
func (v Int64) Encode() []byte  { return TLVEncode(KindInt64, v.xvaluebody, v.ArrayLen()) }
func (v Int64) At(idx int) int64 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Int64.At: index %d out of range [0,%d)", idx, n))
	}
	return int64(binary.LittleEndian.Uint64(v.xvaluebody[idx*8:]))
}

func DecodeInt64(xvaluebody []byte) Int64 { return Int64{xvaluebody: xvaluebody} }
