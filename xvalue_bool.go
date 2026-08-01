package kvspace

import (
	"fmt"
	"strconv"
)

// ── Bool ─────────────────────────────────────────────────────────────────

type Bool struct{ xvaluebody []byte }

func NewBool(v ...bool) Bool {
	raw := make([]byte, len(v))
	for i, val := range v {
		if val {
			raw[i] = 1
		}
	}
	return Bool{xvaluebody: raw}
}

func (v Bool) Kind() string    { return KindBool }
func (v Bool) String() string  { return fmtArray(int(v.ArrayLen()), func(i int) string { return strconv.FormatBool(v.At(i)) }) }
func (v Bool) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Bool) ArrayLen() int32 { return int32(len(v.xvaluebody)) }
func (v Bool) Encode() []byte  { return TLVEncode(KindBool, v.xvaluebody, v.ArrayLen()) }
func (v Bool) At(idx int) bool {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Bool.At: index %d out of range [0,%d)", idx, n))
	}
	return v.xvaluebody[idx] != 0
}

func DecodeBool(xvaluebody []byte) Bool { return Bool{xvaluebody: xvaluebody} }
