package kvspace

import (
	"encoding/binary"
	"fmt"
	"strconv"
)

// ── Int8 ─────────────────────────────────────────────────────────────────

type Int8 struct{ data []int8 }

func NewInt8(v ...int8) Int8 { return Int8{data: v} }

func (v Int8) Kind() string    { return KindInt8 }

func (v Int8) IsPtr() bool	{ return false }
func (v Int8) String() string       { return fmtArray(len(v.data), func(i int) string { return strconv.FormatInt(int64(v.data[i]), 10) }) }
func (v Int8) ValueString() string  { return strconv.FormatInt(int64(v.At(0)), 10) }
func (v Int8) CodeString() string   { return KindInt8 + ":" + v.ValueString() }
func (v Int8) ByteLen() int32  { return int32(len(v.data)) }
func (v Int8) ArrayLen() int32 { return int32(len(v.data)) }
func (v Int8) Encode() []byte {
	raw := make([]byte, len(v.data))
	for i, val := range v.data {
		raw[i] = byte(val)
	}
	return TLVEncode(KindInt8, raw, v.ArrayLen())
}
func (v Int8) At(idx int) int8 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Int8.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeInt8(xvaluebody []byte) Int8 {
	data := make([]int8, len(xvaluebody))
	for i, b := range xvaluebody {
		data[i] = int8(b)
	}
	return Int8{data: data}
}

// ── Int16 ────────────────────────────────────────────────────────────────

type Int16 struct{ data []int16 }

func NewInt16(v ...int16) Int16 { return Int16{data: v} }

func (v Int16) Kind() string    { return KindInt16 }

func (v Int16) IsPtr() bool	{ return false }
func (v Int16) String() string       { return fmtArray(len(v.data), func(i int) string { return strconv.FormatInt(int64(v.data[i]), 10) }) }
func (v Int16) ValueString() string  { return strconv.FormatInt(int64(v.At(0)), 10) }
func (v Int16) CodeString() string   { return KindInt16 + ":" + v.ValueString() }
func (v Int16) ByteLen() int32  { return int32(len(v.data) * 2) }
func (v Int16) ArrayLen() int32 { return int32(len(v.data)) }
func (v Int16) Encode() []byte {
	raw := make([]byte, len(v.data)*2)
	for i, val := range v.data {
		binary.LittleEndian.PutUint16(raw[i*2:], uint16(val))
	}
	return TLVEncode(KindInt16, raw, v.ArrayLen())
}
func (v Int16) At(idx int) int16 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Int16.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeInt16(xvaluebody []byte) Int16 {
	data := make([]int16, len(xvaluebody)/2)
	for i := range data {
		data[i] = int16(binary.LittleEndian.Uint16(xvaluebody[i*2:]))
	}
	return Int16{data: data}
}

// ── Int32 ────────────────────────────────────────────────────────────────

type Int32 struct{ data []int32 }

func NewInt32(v ...int32) Int32 { return Int32{data: v} }

func (v Int32) Kind() string    { return KindInt32 }

func (v Int32) IsPtr() bool	{ return false }
func (v Int32) String() string       { return fmtArray(len(v.data), func(i int) string { return strconv.FormatInt(int64(v.data[i]), 10) }) }
func (v Int32) ValueString() string  { return strconv.FormatInt(int64(v.At(0)), 10) }
func (v Int32) CodeString() string   { return KindInt32 + ":" + v.ValueString() }
func (v Int32) ByteLen() int32  { return int32(len(v.data) * 4) }
func (v Int32) ArrayLen() int32 { return int32(len(v.data)) }
func (v Int32) Encode() []byte {
	raw := make([]byte, len(v.data)*4)
	for i, val := range v.data {
		binary.LittleEndian.PutUint32(raw[i*4:], uint32(val))
	}
	return TLVEncode(KindInt32, raw, v.ArrayLen())
}
func (v Int32) At(idx int) int32 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Int32.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeInt32(xvaluebody []byte) Int32 {
	data := make([]int32, len(xvaluebody)/4)
	for i := range data {
		data[i] = int32(binary.LittleEndian.Uint32(xvaluebody[i*4:]))
	}
	return Int32{data: data}
}

// ── Int64 ────────────────────────────────────────────────────────────────

type Int64 struct{ data []int64 }

func NewInt64(v ...int64) Int64 { return Int64{data: v} }

func (v Int64) Kind() string    { return KindInt64 }

func (v Int64) IsPtr() bool	{ return false }
func (v Int64) String() string       { return fmtArray(len(v.data), func(i int) string { return strconv.FormatInt(v.data[i], 10) }) }
func (v Int64) ValueString() string  { return strconv.FormatInt(v.At(0), 10) }
func (v Int64) CodeString() string   { return KindInt64 + ":" + v.ValueString() }
func (v Int64) ByteLen() int32  { return int32(len(v.data) * 8) }
func (v Int64) ArrayLen() int32 { return int32(len(v.data)) }
func (v Int64) Encode() []byte {
	raw := make([]byte, len(v.data)*8)
	for i, val := range v.data {
		binary.LittleEndian.PutUint64(raw[i*8:], uint64(val))
	}
	return TLVEncode(KindInt64, raw, v.ArrayLen())
}
func (v Int64) At(idx int) int64 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Int64.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeInt64(xvaluebody []byte) Int64 {
	data := make([]int64, len(xvaluebody)/8)
	for i := range data {
		data[i] = int64(binary.LittleEndian.Uint64(xvaluebody[i*8:]))
	}
	return Int64{data: data}
}
