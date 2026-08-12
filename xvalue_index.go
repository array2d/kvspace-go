package kvspace

import (
	"fmt"
	"strings"
)

// ── Index ────────────────────────────────────────────────────────────────

type Index struct{ childs []string }

func NewIndex(children []string) Index { return Index{childs: children} }

func (v Index) Kind() string    { return KindIndex }

func (v Index) IsPtr() bool	{ return false }
func (v Index) String() string  { return fmt.Sprintf("(%d)", len(v.childs)) }
func (v Index) ByteLen() int32  { return int32(len([]byte(strings.Join(v.childs, IndexValueSep)))) }
func (v Index) ArrayLen() int32 { return 1 }
func (v Index) Encode() []byte  { return TLVEncode(KindIndex, []byte(strings.Join(v.childs, IndexValueSep)), 1) }

func (v Index) Children() []string { return v.childs }

func DecodeIndex(xvaluebody []byte) Index {
	s := string(xvaluebody)
	if s == "" {
		return Index{}
	}
	return Index{childs: strings.Split(s, IndexValueSep)}
}

// ── ExtIndex ─────────────────────────────────────────────────────────────

type ExtIndex struct {
	childs  []string
	extpath string
}

func NewExtIndex(children []string, extpath string) ExtIndex {
	return ExtIndex{childs: children, extpath: extpath}
}

func (v ExtIndex) Kind() string    { return KindExtIndex }

func (v ExtIndex) IsPtr() bool	{ return false }
func (v ExtIndex) String() string  { return fmt.Sprintf("(%d) …%s", len(v.childs), v.extpath) }
func (v ExtIndex) ByteLen() int32 {
	body := encodeExtIndexRaw(v.extpath, v.childs)
	return int32(len(body))
}
func (v ExtIndex) ArrayLen() int32 { return 1 }
func (v ExtIndex) Encode() []byte {
	body := encodeExtIndexRaw(v.extpath, v.childs)
	return TLVEncode(KindExtIndex, body, 1)
}

func (v ExtIndex) Children() []string { return v.childs }
func (v ExtIndex) ExtPath() string    { return v.extpath }

func DecodeExtIndex(xvaluebody []byte) ExtIndex {
	extpath, children := decodeExtIndexRaw(xvaluebody)
	return ExtIndex{childs: children, extpath: extpath}
}

// ── helpers ──────────────────────────────────────────────────────────────

func encodeExtIndexRaw(extpath string, children []string) []byte {
	parts := append([]string{ExtIndexHead + extpath}, children...)
	return []byte(strings.Join(parts, IndexValueSep))
}

func decodeExtIndexRaw(xvaluebody []byte) (extpath string, children []string) {
	s := string(xvaluebody)
	if s == "" {
		return "", nil
	}
	parts := strings.SplitN(s, IndexValueSep, 2)
	if len(parts) == 0 {
		return "", nil
	}
	extpath = strings.TrimPrefix(parts[0], ExtIndexHead)
	if len(parts) > 1 {
		children = strings.Split(parts[1], IndexValueSep)
	}
	return
}
