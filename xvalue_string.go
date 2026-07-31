package kvspace

import (
	"fmt"
	"unicode/utf8"
)

// ── Char ─────────────────────────────────────────────────────────────────
// 存储 UTF-8 字节序列。ArrayLen 返回 rune 数，At(idx) 返回第 idx 个 rune。

type Char struct{ xvaluebody []byte }

func NewChar(v ...string) Char {
	var raw []byte
	for _, s := range v {
		raw = append(raw, []byte(s)...)
	}
	return Char{xvaluebody: raw}
}

func (v Char) Kind() string   { return KindString }
func (v Char) ByteLen() int32 { return int32(len(v.xvaluebody)) }
func (v Char) ArrayLen() int32 {
	return int32(utf8.RuneCount(v.xvaluebody))
}
func (v Char) Encode() []byte { return TLVEncode(KindString, v.xvaluebody, v.ArrayLen()) }

func (v Char) At(idx int) rune {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Char.At: index %d out of range [0,%d)", idx, n))
	}
	pos := 0
	for i := 0; i < idx; i++ {
		_, sz := utf8.DecodeRune(v.xvaluebody[pos:])
		pos += sz
	}
	r, _ := utf8.DecodeRune(v.xvaluebody[pos:])
	return r
}

func DecodeChar(raw []byte) Char { return Char{xvaluebody: raw} }
