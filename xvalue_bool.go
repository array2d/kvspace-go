package kvspace

import (
	"fmt"
	"strconv"
)

// ── Bool ─────────────────────────────────────────────────────────────────

type Bool struct{ data []bool }

func NewBool(v ...bool) Bool { return Bool{data: v} }

func (v Bool) Kind() string    { return KindBool }

func (v Bool) IsPtr() bool	{ return false }
func (v Bool) String() string  { return fmtArray(len(v.data), func(i int) string { return strconv.FormatBool(v.data[i]) }) }
func (v Bool) ByteLen() int32  { return int32(len(v.data)) }
func (v Bool) ArrayLen() int32 { return int32(len(v.data)) }
func (v Bool) Encode() []byte {
	raw := make([]byte, len(v.data))
	for i, b := range v.data {
		if b {
			raw[i] = 1
		}
	}
	return TLVEncode(KindBool, raw, v.ArrayLen())
}
func (v Bool) At(idx int) bool {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Bool.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeBool(xvaluebody []byte) Bool {
	data := make([]bool, len(xvaluebody))
	for i, b := range xvaluebody {
		data[i] = b != 0
	}
	return Bool{data: data}
}
