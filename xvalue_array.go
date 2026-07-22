package kvspace

// ── 1D 数组 ──────────────────────────────────────────────────────────────
//
// 数组性由 arraylength > 1 判定。所有元素同类型，raw 连续存储。
// raw 格式：kindSize(kind) * arraylength 字节，按 LE 偏移访问。

// Array 构造定长类型 1D 数组。元素必须同为一种基础类型且有固定尺寸。
// 变长类型（如 string）返回零值。
func Array(elems []XValue) XValue {
	n := int32(len(elems))
	if n == 0 { return XValue{} }
	k := elems[0].Kind()
	sz := kindSize(k)
	if sz <= 0 { return XValue{} }
	raw := make([]byte, n*sz)
	for i, e := range elems {
		copy(raw[i*int(sz):], e.RawBytes())
	}
	return XValue{kind: k, arraylength: n, raw: raw}
}

// IsArray 判断是否为数组（arraylength > 1）。
func (v XValue) IsArray() bool { return v.ArrayLen() > 1 }

// Len 返回数组元素个数。
func (v XValue) Len() int { return int(v.ArrayLen()) }

// Index 用指针偏移返回第 i 个元素。kind 必须为基础定长类型。
func (v XValue) Index(i int) XValue {
	n := int(v.ArrayLen())
	if i < 0 || i >= n { return XValue{} }
	sz := kindSize(v.kind)
	if sz <= 0 { return XValue{} }
	off := i * int(sz)
	if off+int(sz) > len(v.raw) { return XValue{} }
	raw := make([]byte, sz)
	copy(raw, v.raw[off:off+int(sz)])
	return XValue{kind: v.kind, arraylength: 1, raw: raw}
}

func kindSize(k string) int32 {
	switch k {
	case KindBool, "int8", KindUint8: return 1
	case "int16", KindUint16: return 2
	case KindInt32, KindUint32, KindFloat32: return 4
	case KindInt64, KindUint64, KindFloat64, "float", "int": return 8
	default: return 0
	}
}
