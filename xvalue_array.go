package kvspace

import (
	"encoding/binary"
)

// ── 1D 数组 ──────────────────────────────────────────────────────────────
//
// 数组性由 arraylength > 1 判定，不再有独立的 "array" kind。
// 同构定长类型（int/float/bool 等）用定长打包；变长类型（string）用 TLV per-element。
// raw 格式（TLV 变长）：[4B count LE][elem0 TLV][elem1 TLV]...
// raw 格式（定长打包）：elementSize * arraylength 字节连续存储。

// Array 构造 1D TLV 数组（变长类型如 string 使用）。
// 固定长度类型优先用 RawN + 定长 raw 直接构造。
func Array(elems []XValue) XValue {
	n := int32(len(elems))
	if n == 0 { return XValue{} }
	k := elems[0].Kind()
	// 尝试定长打包
	sz := kindSize(k)
	if sz > 0 {
		raw := make([]byte, n*sz)
		for i, e := range elems {
			copy(raw[i*int(sz):], e.RawBytes())
		}
		return XValue{kind: k, arraylength: n, raw: raw}
	}
	// 变长 TLV
	total := 4
	encoded := make([][]byte, len(elems))
	for i, e := range elems {
		encoded[i] = EncodeXValue(e)
		total += len(encoded[i])
	}
	raw := make([]byte, total)
	binary.LittleEndian.PutUint32(raw[:4], uint32(n))
	off := 4
	for _, enc := range encoded {
		copy(raw[off:], enc)
		off += len(enc)
	}
	return XValue{kind: k, arraylength: n, raw: raw}
}

// ArrayInts 构造整数 1D 数组的便捷方法。
func ArrayInts(vals []int64) XValue {
	elems := make([]XValue, len(vals))
	for i, v := range vals {
		elems[i] = Int64(v)
	}
	return Array(elems)
}

// IsArray 判断是否为数组（arraylength > 1）。
func (v XValue) IsArray() bool { return v.ArrayLen() > 1 }

// Len 返回 TLV 数组元素个数。同构数组用 ArrayLen()。
func (v XValue) Len() int {
	al := v.ArrayLen()
	if al > 1 { return int(al) }
	if len(v.raw) < 4 { return 0 }
	return int(binary.LittleEndian.Uint32(v.raw[:4]))
}

// Index 返回数组的第 i 个元素。支持 TLV 和定长两种编码。
func (v XValue) Index(i int) XValue {
	n := int(v.ArrayLen())
	if n == 0 { n = v.Len() }
	if i < 0 || i >= n { return XValue{} }
	sz := kindSize(v.kind)
	if sz > 0 {
		// 定长：直接偏移
		off := i * int(sz)
		if off+int(sz) > len(v.raw) { return XValue{} }
		raw := make([]byte, sz)
		copy(raw, v.raw[off:off+int(sz)])
		return XValue{kind: v.kind, arraylength: 1, raw: raw}
	}
	// TLV 变长
	off := 4
	for j := 0; j < i; j++ {
		_, size := decodeXValueLen(v.raw[off:])
		off += size
	}
	elem, _ := decodeXValueLen(v.raw[off:])
	return DecodeXValue(v.raw[off : off+elem])
}

func kindSize(k string) int32 {
	switch k {
	case "bool", "int8", "uint8": return 1
	case "int16", "uint16": return 2
	case "int32", "uint32", "float32": return 4
	case "int64", "uint64", "float64", "float", "int": return 8
	default: return 0
	}
}

// decodeXValueLen 返回新版 TLV 元素的总字节数和消耗的 raw 字节数。
func decodeXValueLen(data []byte) (total, consumed int) {
	if len(data) == 0 { return 0, 1 }
	kindLen := int(data[0])
	if len(data) < 1+kindLen+4+4 { return 0, len(data) }
	rawLen := int(binary.LittleEndian.Uint32(data[1+kindLen+4:]))
	consumed = 1 + kindLen + 4 + 4 + rawLen
	return consumed, consumed
}
