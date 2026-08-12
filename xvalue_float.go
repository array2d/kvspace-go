package kvspace

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func fmtFloat(v float64, bitSize int) string {
	s := strconv.FormatFloat(v, 'f', -1, bitSize)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// ── Float32 ──────────────────────────────────────────────────────────────

type Float32 struct{ data []float32 }

func NewFloat32(v ...float32) Float32 { return Float32{data: v} }

func (v Float32) Kind() string    { return KindFloat32 }

func (v Float32) IsPtr() bool	{ return false }
func (v Float32) String() string       { return fmtArray(len(v.data), func(i int) string { return fmtFloat(float64(v.data[i]), 32) }) }
func (v Float32) ValueString() string  { return fmtFloat(float64(v.At(0)), 32) }
func (v Float32) CodeString() string   { return KindFloat32 + ":" + v.ValueString() }
func (v Float32) ByteLen() int32  { return int32(len(v.data) * 4) }
func (v Float32) ArrayLen() int32 { return int32(len(v.data)) }
func (v Float32) Encode() []byte {
	raw := make([]byte, len(v.data)*4)
	for i, val := range v.data {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(val))
	}
	return TLVEncode(KindFloat32, raw, v.ArrayLen())
}
func (v Float32) At(idx int) float32 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Float32.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeFloat32(xvaluebody []byte) Float32 {
	data := make([]float32, len(xvaluebody)/4)
	for i := range data {
		data[i] = math.Float32frombits(binary.LittleEndian.Uint32(xvaluebody[i*4:]))
	}
	return Float32{data: data}
}

// ── Float64 ──────────────────────────────────────────────────────────────

type Float64 struct{ data []float64 }

func NewFloat64(v ...float64) Float64 { return Float64{data: v} }

func (v Float64) Kind() string    { return KindFloat64 }

func (v Float64) IsPtr() bool	{ return false }
func (v Float64) String() string       { return fmtArray(len(v.data), func(i int) string { return fmtFloat(v.data[i], 64) }) }
func (v Float64) ValueString() string  { return fmtFloat(v.At(0), 64) }
func (v Float64) CodeString() string   { return KindFloat64 + ":" + v.ValueString() }
func (v Float64) ByteLen() int32  { return int32(len(v.data) * 8) }
func (v Float64) ArrayLen() int32 { return int32(len(v.data)) }
func (v Float64) Encode() []byte {
	raw := make([]byte, len(v.data)*8)
	for i, val := range v.data {
		binary.LittleEndian.PutUint64(raw[i*8:], math.Float64bits(val))
	}
	return TLVEncode(KindFloat64, raw, v.ArrayLen())
}
func (v Float64) At(idx int) float64 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Float64.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeFloat64(xvaluebody []byte) Float64 {
	data := make([]float64, len(xvaluebody)/8)
	for i := range data {
		data[i] = math.Float64frombits(binary.LittleEndian.Uint64(xvaluebody[i*8:]))
	}
	return Float64{data: data}
}
