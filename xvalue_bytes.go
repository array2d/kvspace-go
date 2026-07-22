package kvspace

// ── 字符串 ────────────────────────────────────────────────────────────────

func Str(v string) XValue { return XValue{kind: KindString, arraylength: 1, raw: []byte(v)} }

func (v XValue) Str() string {
	if v.kind != KindString { return "" }
	return string(v.raw)
}

// ── rwir 指令槽 ────────────────────────────────────────────────────────────

func Rwir(v string) XValue { return XValue{kind: "rwir", arraylength: 1, raw: []byte(v)} }

// ── 字节数组 ──────────────────────────────────────────────────────────────

func Bytes(v []byte) XValue {
	c := make([]byte, len(v))
	copy(c, v)
	return XValue{kind: KindBytes, arraylength: 1, raw: c}
}

func (v XValue) Bytes() []byte {
	if v.kind != KindBytes { return nil }
	return v.raw
}
