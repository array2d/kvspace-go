package kvspace

import "strings"

// ── Index ────────────────────────────────────────────────────────────────

type Index struct{ xvaluebody []byte }

func NewIndex(children []string) Index {
	return Index{xvaluebody: []byte(strings.Join(children, IndexValueSep))}
}

func (v Index) Kind() string    { return KindIndex }
func (v Index) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Index) ArrayLen() int32 { return 1 }
func (v Index) Encode() []byte  { return TLVEncode(KindIndex, v.xvaluebody, 1) }

func (v Index) Children() []string {
	s := string(v.xvaluebody)
	if s == "" {
		return nil
	}
	return strings.Split(s, IndexValueSep)
}

func DecodeIndex(xvaluebody []byte) Index { return Index{xvaluebody: xvaluebody} }

// ── LinkIndex ────────────────────────────────────────────────────────────

type LinkIndex struct{ xvaluebody []byte }

func NewLinkIndex(target string) LinkIndex { return LinkIndex{xvaluebody: []byte(target)} }

func (v LinkIndex) Kind() string    { return KindLinkIndex }
func (v LinkIndex) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v LinkIndex) ArrayLen() int32 { return 1 }
func (v LinkIndex) Encode() []byte  { return TLVEncode(KindLinkIndex, v.xvaluebody, 1) }

func (v LinkIndex) Target() string { return string(v.xvaluebody) }

func DecodeLinkIndex(xvaluebody []byte) LinkIndex { return LinkIndex{xvaluebody: xvaluebody} }

// ── ExtIndex ─────────────────────────────────────────────────────────────

type ExtIndex struct{ xvaluebody []byte }

func NewExtIndex(children []string, extpath string) ExtIndex {
	parts := append([]string{ExtIndexHead + extpath}, children...)
	return ExtIndex{xvaluebody: []byte(strings.Join(parts, IndexValueSep))}
}

func (v ExtIndex) Kind() string    { return KindExtIndex }
func (v ExtIndex) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v ExtIndex) ArrayLen() int32 { return 1 }
func (v ExtIndex) Encode() []byte  { return TLVEncode(KindExtIndex, v.xvaluebody, 1) }

func (v ExtIndex) Children() []string {
	_, children := decodeExtIndexRaw(v.xvaluebody)
	return children
}

func (v ExtIndex) ExtPath() string {
	extpath, _ := decodeExtIndexRaw(v.xvaluebody)
	return extpath
}

func DecodeExtIndex(xvaluebody []byte) ExtIndex { return ExtIndex{xvaluebody: xvaluebody} }

// ── helpers ──────────────────────────────────────────────────────────────

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
