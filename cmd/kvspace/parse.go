package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/array2d/kvspace-go"
)

// ParseValue 解析 CLI value 字符串为 XValue。
// 格式：
//
//	int:42  float:3.14  bool:true  string:hello  nil:  index:
//	*int:/tgt/y  *index:/tgt/  (Ptr 指针)
func ParseValue(raw string) (kvspace.XValue, error) {
	if strings.HasPrefix(raw, "*") {
		rest := raw[1:]
		colon := strings.Index(rest, ":")
		if colon >= 0 {
			return kvspace.NewPtr(rest[:colon], rest[colon+1:], 1), nil
		}
		return kvspace.NewPtr("", rest, 1), nil
	}

	idx := strings.Index(raw, ":")
	if idx < 0 {
		return kvspace.NewChar(raw), nil
	}
	kind, repr := raw[:idx], raw[idx+1:]
	switch kind {
	case "int":
		i, err := strconv.ParseInt(repr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid int: %q", repr)
		}
		return kvspace.NewInt64(i), nil
	case "float":
		f, err := strconv.ParseFloat(repr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float: %q", repr)
		}
		return kvspace.NewFloat64(f), nil
	case "bool":
		switch repr {
		case "true":
			return kvspace.NewBool(true), nil
		case "false":
			return kvspace.NewBool(false), nil
		default:
			return nil, fmt.Errorf("invalid bool: %q", repr)
		}
	case "string":
		return kvspace.NewChar(repr), nil
	case "nil":
		return kvspace.None{}, nil
	case kvspace.KindIndex:
		return kvspace.NewIndex(nil), nil
	case kvspace.KindDict:
		return kvspace.NewDictIndex(nil), nil
	default:
		return nil, fmt.Errorf("unknown kind: %q", kind)
	}
}
