package kvspace

import "fmt"

// ── Bytes ────────────────────────────────────────────────────────────────

type Bytes struct{ xvaluebody []byte }

func NewBytes(v ...[]byte) Bytes {
	var raw []byte
	for _, b := range v {
		raw = append(raw, b...)
		raw = append(raw, 0) // null-delimited
	}
	return Bytes{xvaluebody: raw}
}

func (v Bytes) Kind() string   { return KindBytes }
func (v Bytes) ByteLen() int32 { return int32(len(v.xvaluebody)) }
func (v Bytes) ArrayLen() int32 {
	c := 0
	for _, b := range v.xvaluebody {
		if b == 0 {
			c++
		}
	}
	if c == 0 && len(v.xvaluebody) > 0 {
		return 1
	}
	return int32(c)
}
func (v Bytes) Encode() []byte { return TLVEncode(KindBytes, v.xvaluebody, v.ArrayLen()) }

func (v Bytes) At(idx int) []byte {
	start, end := 0, 0
	for i := 0; i <= idx; i++ {
		start = end
		pos := start
		for pos < len(v.xvaluebody) && v.xvaluebody[pos] != 0 {
			pos++
		}
		end = pos + 1
		if pos >= len(v.xvaluebody) {
			panic(fmt.Sprintf("Bytes.At: index %d out of range [0,%d)", idx, i))
		}
	}
	return v.xvaluebody[start : end-1]
}

func AsBytes(v XValue) []byte             { return v.(Bytes).At(0) }
func DecodeBytes(xvaluebody []byte) Bytes { return Bytes{xvaluebody: xvaluebody} }
