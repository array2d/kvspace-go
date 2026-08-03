package kvspace

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ── Duration ─────────────────────────────────────────────────────────────

type Duration struct{ xvaluebody []byte }

func NewDuration(v ...int64) Duration {
	raw := make([]byte, len(v)*8)
	for i, val := range v {
		binary.LittleEndian.PutUint64(raw[i*8:], uint64(val))
	}
	return Duration{xvaluebody: raw}
}

func (v Duration) Kind() string { return "duration" }

func (v Duration) String() string {
	if v.ArrayLen() > 1 {
		return fmtArray(int(v.ArrayLen()), func(i int) string { return formatDuration(v.At(i)) })
	}
	return formatDuration(v.At(0))
}

// formatDuration 对齐 Go time.Duration.String()：0s / 1.5s / 2m3s / 1h2m3s
func formatDuration(ns int64) string {
	if ns == 0 {
		return "0s"
	}
	sign := ""
	if ns < 0 {
		sign = "-"
		ns = -ns
	}
	h := ns / 3600_000_000_000
	ns %= 3600_000_000_000
	m := ns / 60_000_000_000
	ns %= 60_000_000_000
	s := ns / 1_000_000_000
	rem := ns % 1_000_000_000

	parts := make([]string, 0, 3)
	showMin := m > 0 || h > 0
	if h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if showMin {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	// 总是带 s：0s / 1.5s / 2m0s / 1h0m0s（对齐 Go time.Duration.String）
	if rem == 0 {
		parts = append(parts, fmt.Sprintf("%ds", s))
	} else {
		frac := fmt.Sprintf("%09d", rem)
		frac = strings.TrimRight(frac, "0")
		parts = append(parts, fmt.Sprintf("%d.%ss", s, frac))
	}
	return sign + strings.Join(parts, "")
}

func (v Duration) ByteLen() int32  { return int32(len(v.xvaluebody)) }
func (v Duration) ArrayLen() int32 { return int32(len(v.xvaluebody)) / 8 }
func (v Duration) Encode() []byte  { return TLVEncode("duration", v.xvaluebody, v.ArrayLen()) }
func (v Duration) At(idx int) int64 {
	n := int(v.ArrayLen())
	if idx < 0 || idx >= n {
		panic(fmt.Sprintf("Duration.At: index %d out of range [0,%d)", idx, n))
	}
	return int64(binary.LittleEndian.Uint64(v.xvaluebody[idx*8:]))
}

func DecodeDuration(xvaluebody []byte) Duration { return Duration{xvaluebody: xvaluebody} }
