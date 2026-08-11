package kvspace

import (
	"encoding/binary"
	"fmt"
)

// ── Rwir ─────────────────────────────────────────────────────────────────

type Rwir struct{ xvaluebody []byte }

func NewRwir(numReads, numWrites int32, sig string) Rwir {
	raw := make([]byte, 4+len(sig))
	binary.LittleEndian.PutUint16(raw[0:2], uint16(numReads))
	binary.LittleEndian.PutUint16(raw[2:4], uint16(numWrites))
	copy(raw[4:], sig)
	return Rwir{xvaluebody: raw}
}

func (v Rwir) Kind() string    { return KindRwir }
func (v Rwir) String() string  { return v.Sig() }
func (v Rwir) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Rwir) ArrayLen() int32 { return 1 }
func (v Rwir) Encode() []byte  { return TLVEncode(KindRwir, v.xvaluebody, 1) }
func (v Rwir) At(idx int) Rwir {
	if idx != 0 {
		panic(fmt.Sprintf("Rwir.At: index %d out of range [0,1)", idx))
	}
	return v
}

func (v Rwir) NumReads() int32  { return int32(binary.LittleEndian.Uint16(v.xvaluebody[0:2])) }
func (v Rwir) NumWrites() int32 { return int32(binary.LittleEndian.Uint16(v.xvaluebody[2:4])) }
func (v Rwir) Sig() string      { return string(v.xvaluebody[4:]) }

func DecodeRwir(xvaluebody []byte) Rwir { return Rwir{xvaluebody: xvaluebody} }

// ── Rwfunc ───────────────────────────────────────────────────────────────
//
// body 格式：[2B numReads LE][2B numWrites LE]
// 函数名在 key 中（/lib/<pkg>.<name>）。参数定义在 slot [0,-j]/[0,+j]。
// al = 指令数（ArrayLen），不含函数定义所在 slot 0。

type Rwfunc struct {
	xvaluebody []byte
	al         int32
}

func NewRwfunc(numInsts, numReads, numWrites int32) Rwfunc {
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint16(raw[0:2], uint16(numReads))
	binary.LittleEndian.PutUint16(raw[2:4], uint16(numWrites))
	return Rwfunc{xvaluebody: raw, al: numInsts}
}

func (v Rwfunc) Kind() string    { return KindRwfunc }
func (v Rwfunc) String() string  { return "" }
func (v Rwfunc) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Rwfunc) ArrayLen() int32 { return v.al }
func (v Rwfunc) Encode() []byte  { return TLVEncode(KindRwfunc, v.xvaluebody, v.al) }
func (v Rwfunc) At(idx int) Rwfunc {
	if idx < 0 || idx >= int(v.al) {
		panic(fmt.Sprintf("Rwfunc.At: index %d out of range [0,%d)", idx, v.al))
	}
	return v
}

func (v Rwfunc) NumReads() int32  { return int32(binary.LittleEndian.Uint16(v.xvaluebody[0:2])) }
func (v Rwfunc) NumWrites() int32 { return int32(binary.LittleEndian.Uint16(v.xvaluebody[2:4])) }
func (v Rwfunc) NumInsts() int32  { return v.al }

func DecodeRwfunc(xvaluebody []byte, al int32) Rwfunc { return Rwfunc{xvaluebody: xvaluebody, al: al} }
