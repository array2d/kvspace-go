package kvspace

import (
	"encoding/binary"
	"fmt"
	"math"
)

// ── Float32 ──────────────────────────────────────────────────────────────

type Float32 struct{ xvaluebody []byte }

func NewFloat32(v ...float32) Float32 {
	raw := make([]byte, len(v)*4)
	for i, val := range v {
		binary.LittleEndian.PutUint32(raw[i*4:], math.Float32bits(val))
	}
	return Float32{xvaluebody: raw}
}

func (v Float32) Kind() string    { return KindFloat32 }
func (v Float32) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Float32) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 4 }
func (v Float32) Encode() []byte  { return TLVEncode(KindFloat32, v.xvaluebody, v.ArrayLen()) }
func (v Float32) At(idx int) float32 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Float32.At: index %d out of range [0,%d)", idx, n))
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(v.xvaluebody[idx*4:]))
}

func AsFloat32(v XValue) float32              { return v.(Float32).At(0) }
func DecodeFloat32(xvaluebody []byte) Float32 { return Float32{xvaluebody: xvaluebody} }

// ── Float64 ──────────────────────────────────────────────────────────────

type Float64 struct{ xvaluebody []byte }

func NewFloat64(v ...float64) Float64 {
	raw := make([]byte, len(v)*8)
	for i, val := range v {
		binary.LittleEndian.PutUint64(raw[i*8:], math.Float64bits(val))
	}
	return Float64{xvaluebody: raw}
}

func (v Float64) Kind() string    { return KindFloat64 }
func (v Float64) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Float64) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 8 }
func (v Float64) Encode() []byte  { return TLVEncode(KindFloat64, v.xvaluebody, v.ArrayLen()) }
func (v Float64) At(idx int) float64 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Float64.At: index %d out of range [0,%d)", idx, n))
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(v.xvaluebody[idx*8:]))
}

func AsFloat64(v XValue) float64              { return v.(Float64).At(0) }
func DecodeFloat64(xvaluebody []byte) Float64 { return Float64{xvaluebody: xvaluebody} }
