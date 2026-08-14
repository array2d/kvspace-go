package kvspace

import (
	"encoding/binary"
	"fmt"
	"time"
)

// ── Duration ─────────────────────────────────────────────────────────────

type Duration struct{ data []int64 }

func NewDuration(v ...int64) Duration { return Duration{data: v} }

func (v Duration) Kind() string { return "duration" }

func (v Duration) IsPtr() bool	{ return false }

func (v Duration) ValueString() string { return time.Duration(v.At(0)).String() }
func (v Duration) CodeString() string  { return "duration:" + v.ValueString() }

func (v Duration) String() string {
	if len(v.data) > 1 {
		return fmtArray(len(v.data), func(i int) string { return time.Duration(v.data[i]).String() })
	}
	return time.Duration(v.data[0]).String()
}

func (v Duration) ByteLen() int32  { return int32(len(v.data) * 8) }
func (v Duration) ArrayLen() int32 { return int32(len(v.data)) }
func (v Duration) Encode() []byte {
	raw := make([]byte, len(v.data)*8)
	for i, val := range v.data {
		binary.LittleEndian.PutUint64(raw[i*8:], uint64(val))
	}
	return TLVEncode("duration", raw, v.ArrayLen())
}
func (v Duration) At(idx int) int64 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Duration.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeDuration(xvaluebody []byte) Duration {
	data := make([]int64, len(xvaluebody)/8)
	for i := range data {
		data[i] = int64(binary.LittleEndian.Uint64(xvaluebody[i*8:]))
	}
	return Duration{data: data}
}
