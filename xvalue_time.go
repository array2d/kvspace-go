package kvspace

import (
	"encoding/binary"
	"fmt"
)

// ── Time ─────────────────────────────────────────────────────────────────

type Time struct{ xvaluebody []byte }

func NewTime(v ...int64) Time {
	raw := make([]byte, len(v)*8)
	for i, val := range v {
		binary.LittleEndian.PutUint64(raw[i*8:], uint64(val))
	}
	return Time{xvaluebody: raw}
}

func (v Time) Kind() string    { return "time" }
func (v Time) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Time) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 8 }
func (v Time) Encode() []byte  { return TLVEncode("time", v.xvaluebody, v.ArrayLen()) }
func (v Time) At(idx int) int64 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Time.At: index %d out of range [0,%d)", idx, n))
	}
	return int64(binary.LittleEndian.Uint64(v.xvaluebody[idx*8:]))
}

func DecodeTime(xvaluebody []byte) Time { return Time{xvaluebody: xvaluebody} }
