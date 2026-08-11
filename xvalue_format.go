package kvspace

import (
	"fmt"
	"strconv"
	"strings"
)

// ── 显示 ─────────────────────────────────────────────────────────────────

// fmtFloat 格式化浮点，整数加 .0。
func fmtFloat(v float64, bitSize int) string {
	s := strconv.FormatFloat(v, 'f', -1, bitSize)
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// fmtArray 格式化数组：[elem0, elem1, ...]；len==1 时仅返元素本身。
func fmtArray(n int, fn func(int) string) string {
	if n == 1 {
		return fn(0)
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = fn(i)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func fmtKindVal(v XValue) string {
	switch v := v.(type) {
	case Int8, Int16, Int32, Int64, Uint8, Uint16, Uint32, Uint64, Float32, Float64, Bool, Time:
		return v.Kind() + ":" + fmtPlain(v)
	case Char:
		return KindString + ":" + string(v.xvaluebody)
	case None:
		return KindNone
	case Rwir:
		return v.Sig() + ":" + KindRwir
	case Rwfunc:
		return fmt.Sprintf("r%d/w%d", v.NumReads(), v.NumWrites()) + ":" + KindRwfunc
	case Ptr:
		return fmt.Sprintf("→%s:%s", v.Target(), v.Kind())
	case LinkIndex:
		return "→" + v.Target()
	default:
		if len(v.Encode()) > 0 {
			return fmtPlain(v) + ":" + v.Kind()
		}
		return v.Kind()
	}
}

func fmtPlain(v XValue) string {
	switch v := v.(type) {
	case None:
		return KindNone
	case Int8:
		return strconv.FormatInt(int64(v.At(0)), 10)
	case Int16:
		return strconv.FormatInt(int64(v.At(0)), 10)
	case Int32:
		return strconv.FormatInt(int64(v.At(0)), 10)
	case Int64:
		return strconv.FormatInt(v.At(0), 10)
	case Uint8:
		return strconv.FormatUint(uint64(v.At(0)), 10)
	case Uint16:
		return strconv.FormatUint(uint64(v.At(0)), 10)
	case Uint32:
		return strconv.FormatUint(uint64(v.At(0)), 10)
	case Uint64:
		return strconv.FormatUint(v.At(0), 10)
	case Float32:
		return strconv.FormatFloat(float64(v.At(0)), 'f', -1, 32)
	case Float64:
		return strconv.FormatFloat(v.At(0), 'f', -1, 64)
	case Bool:
		return strconv.FormatBool(v.At(0))
	case Time:
		return strconv.FormatInt(v.At(0), 10)
	case Char:
		return string(v.xvaluebody)
	case Rwir:
		return v.Sig()
	case Rwfunc:
		return fmt.Sprintf("r%d/w%d", v.NumReads(), v.NumWrites())
	case Ptr:
		return "→" + v.Target()
	case Index:
		c := v.Children()
		if len(c) == 1 && c[0] == "" {
			return "(empty)"
		}
		return fmt.Sprintf("(%d)", len(c))
	case ExtIndex:
		c, ep := v.Children(), v.ExtPath()
		if ep != "" {
			return fmt.Sprintf("(%d) …%s", len(c), ep)
		}
		if len(c) == 0 {
			return "(empty ext)"
		}
		return fmt.Sprintf("(%d)", len(c))
	default:
		return string(v.Encode())
	}
}

// ── error ────────────────────────────────────────────────────────────────

func typeError(expected, got string) string {
	return fmt.Sprintf("TypeError: expected %s, got %s", expected, got)
}
