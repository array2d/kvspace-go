package kvspace

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ── CharByte（char/utf8，UTF-8 字节串）────────────────────────────────────

type CharByte struct{ data []uint8 }

func NewCharByte(v ...uint8) CharByte { return CharByte{data: v} }

func (v CharByte) Kind() string    { return KindCharUtf8 }
func (v CharByte) IsPtr() bool     { return false }
func (v CharByte) ByteLen() int32  { return int32(len(v.data)) }
func (v CharByte) ArrayLen() int32 { return int32(len(v.data)) }
func (v CharByte) Encode() []byte {
	return TLVEncode(KindCharUtf8, copyBytes(v.data), v.ArrayLen())
}
func (v CharByte) String() string       { return string(v.data) }
func (v CharByte) ValueString() string  { return string(v.data) }
func (v CharByte) CodeString() string   { return KindCharUtf8 + ":" + string(v.data) }
func (v CharByte) At(idx int) uint8 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("CharByte.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeCharByte(raw []byte) CharByte {
	data := make([]uint8, len(raw))
	copy(data, raw)
	return CharByte{data: data}
}

func copyBytes(b []uint8) []byte {
	raw := make([]byte, len(b))
	for i, v := range b {
		raw[i] = byte(v)
	}
	return raw
}

// ── CharAscii（char/ascii，ASCII 字节串，1B×N）──────────────────────────

type CharAscii struct{ data []uint8 }

func NewCharAscii(v ...uint8) CharAscii { return CharAscii{data: v} }

func (v CharAscii) Kind() string    { return KindCharAscii }
func (v CharAscii) IsPtr() bool     { return false }
func (v CharAscii) ByteLen() int32  { return int32(len(v.data)) }
func (v CharAscii) ArrayLen() int32 { return int32(len(v.data)) }
func (v CharAscii) Encode() []byte {
	return TLVEncode(KindCharAscii, copyBytes(v.data), v.ArrayLen())
}
func (v CharAscii) String() string      { return string(v.data) }
func (v CharAscii) ValueString() string { return string(v.data) }
func (v CharAscii) CodeString() string  { return KindCharAscii + ":" + string(v.data) }
func (v CharAscii) At(idx int) uint8 {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("CharAscii.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeCharAscii(raw []byte) CharAscii {
	data := make([]uint8, len(raw))
	copy(data, raw)
	return CharAscii{data: data}
}

// ── Char32（char/utf32，码点，4B×N）────────────────────────────────────

type Char32 struct{ data []rune }

func NewChar32(v ...rune) Char32 { return Char32{data: v} }

// NewChar 根据 kind 从 Go 字符串构造字符值（char/utf32 默认，char/utf8/ascii 为字节）。
func NewChar(kind, s string) XValue {
	switch kind {
	case KindCharUtf8:
		return NewCharByte([]byte(s)...)
	case KindCharAscii:
		return NewCharAscii([]byte(s)...)
	default:
		return NewChar32([]rune(s)...)
	}
}

func (v Char32) Kind() string    { return KindChar }
func (v Char32) IsPtr() bool     { return false }
func (v Char32) ByteLen() int32  { return int32(len(v.data) * 4) }
func (v Char32) ArrayLen() int32 { return int32(len(v.data)) }
func (v Char32) Encode() []byte {
	raw := make([]byte, len(v.data)*4)
	for i, r := range v.data {
		binary.LittleEndian.PutUint32(raw[i*4:], uint32(r))
	}
	return TLVEncode(KindChar, raw, v.ArrayLen())
}
func (v Char32) String() string      { return string(v.data) }
func (v Char32) ValueString() string { return string(v.data) }
func (v Char32) CodeString() string  { return KindChar + ":" + string(v.data) }
func (v Char32) At(idx int) rune {
	if idx < 0 || idx >= len(v.data) {
		panic(fmt.Sprintf("Char32.At: index %d out of range [0,%d)", idx, len(v.data)))
	}
	return v.data[idx]
}

func DecodeChar32(raw []byte) Char32 {
	data := make([]rune, len(raw)/4)
	for i := range data {
		data[i] = rune(binary.LittleEndian.Uint32(raw[i*4:]))
	}
	return Char32{data: data}
}

// IsCharKind 判断 kind 是否为字符家族（前缀 char/）。
func IsCharKind(kind string) bool {
	return strings.HasPrefix(kind, "char/")
}
