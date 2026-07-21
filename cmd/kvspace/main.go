// kvspace — KVSpace 命令行工具。
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/array2d/kvspace-go"
	_ "github.com/array2d/kvspace-go/redis"
)

func defaultKVSpace() string {
	if v := os.Getenv("KVLANG_KVSPACE"); v != "" {
		return v
	}
	return "redis://127.0.0.1:6379"
}

func main() {
	fs := flag.NewFlagSet("kvspace", flag.ExitOnError)
	dsn := fs.String("kvspace", defaultKVSpace(), "kvspace DSN (redis://host:port)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: kvspace [--kvspace dsn] <subcommand> [args]")
		fmt.Fprintln(os.Stderr, "subcommands: get mget set mset del deltree link unlink list tree table dump watch notify clear")
		fmt.Fprintln(os.Stderr, "  clear 清空整个后端 db（redis: FLUSHDB）——共享 Redis 实例慎用")
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[1:])

	sub := fs.Args()
	if len(sub) == 0 { fs.Usage(); os.Exit(1) }

	kv := kvspace.Conn(*dsn)
	defer kv.DisConn()

	switch sub[0] {
	case "get":
		if len(sub) < 2 { exitUsage("kvspace get <key>") }
		v, err := kv.Get(sub[1])
		if err != nil { fatalf("%v", err) }
		if v.IsNil() { fmt.Printf("%s	(nil)\n", sub[1]) } else { fmt.Printf("%s	%s\n", sub[1], v) }
	case "mget":
		if len(sub) < 2 { exitUsage("kvspace mget <key1> [key2 ...]") }
		vals, err := kv.GetMany(sub[1:])
		if err != nil { fatalf("%v", err) }
		for i, v := range vals {
			if v.IsNil() { fmt.Printf("%s\t(nil)\n", sub[i+1]) } else { fmt.Printf("%s\t%s\n", sub[i+1], v) }
		}
	case "set":
		if len(sub) < 3 { exitUsage("kvspace set <key> <value>") }
		val, err := parseValue(sub[2])
		if err != nil { fatalf("%v", err) }
		if err := kv.Set(sub[1], val); err != nil { fatalf("%v", err) }
	case "del":
		if len(sub) < 2 { exitUsage("kvspace del <key1> [key2 ...]") }
		if err := kv.Del(sub[1:]...); err != nil { fatalf("%v", err) }
	case "mset":
		if len(sub) < 3 || len(sub[1:])%2 != 0 { exitUsage("kvspace mset <k1> <v1> [k2 v2 ...]") }
		pairs := make([]kvspace.KVPair, 0, len(sub[1:])/2)
		for i := 1; i < len(sub); i += 2 {
			v, err := parseValue(sub[i+1])
			if err != nil { fatalf("%v", err) }
			pairs = append(pairs, kvspace.KVPair{Key: sub[i], Val: v})
		}
		if err := kv.MSet(pairs); err != nil { fatalf("%v", err) }
	case "deltree":
		if len(sub) < 2 { exitUsage("kvspace deltree <prefix>") }
		if err := kv.DelTree(sub[1]); err != nil { fatalf("%v", err) }
	case "link":
		if len(sub) < 3 { exitUsage("kvspace link <target> <linkpath>") }
		if err := kv.Link(sub[1], sub[2]); err != nil { fatalf("%v", err) }
	case "unlink":
		if len(sub) < 2 { exitUsage("kvspace unlink <linkpath>") }
		if err := kv.Unlink(sub[1]); err != nil { fatalf("%v", err) }
	case "list":
		if len(sub) < 2 { exitUsage("kvspace list <prefix>") }
		children, err := kv.List(sub[1])
		if err != nil { fatalf("%v", err) }
		for _, c := range children {
			p := sub[1] + "/" + c
			if v, err := kv.Get(p); err == nil && !v.IsNil() {
				fmt.Printf("%s	%s\n", c, v)
			} else {
				fmt.Printf("%s	(nil)\n", c)
			}
		}
	case "tree":
		if len(sub) < 2 { exitUsage("kvspace tree <prefix>") }
		fmt.Println(sub[1])
		printTree(kv, sub[1], "")
	case "dump":
		if len(sub) < 2 { exitUsage("kvspace dump <prefix>") }
		dumpPrefix(kv, sub[1])
	case "watch":
		cmdWatch(kv, sub[1:])
	case "notify":
		if len(sub) < 3 { exitUsage("kvspace notify <key> <value>") }
		if err := kv.Notify(sub[1], kvspace.Str(sub[2])); err != nil { fatalf("%v", err) }
	case "clear":
		if err := kv.ClearAll(); err != nil { fatalf("%v", err) }
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", sub[0])
		fs.Usage()
		os.Exit(1)
	}
}

func exitUsage(msg string) { fmt.Fprintln(os.Stderr, "usage:", msg); os.Exit(1) }
func fatalf(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...); os.Exit(1) }

func cmdWatch(kv kvspace.KVSpace, args []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	timeout := fs.Duration("timeout", 0, "timeout (e.g. 5s, 1m); 0 = block forever")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: kvspace watch [--timeout duration] <key>")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() == 0 { fs.Usage(); os.Exit(1) }
	result, err := kv.Watch(fs.Arg(0), *timeout)
	if err != nil { fatalf("%v", err) }
	fmt.Println(result)
}

func printTree(kv kvspace.KVSpace, prefix, indent string) {
	children, _ := kv.List(prefix)
	if len(children) > 0 && isSlotTable(children) {
		printSlotTable(kv, prefix, indent, children)
		return
	}
	for i, c := range children {
		last := i == len(children)-1
		branch := "├── "
		if last { branch = "└── " }
		if v, err := kv.Get(prefix+"/"+c); err == nil && !v.IsNil() {
			fmt.Printf("%s%s%s\t%s\n", indent, branch, c, v)
		} else {
			fmt.Printf("%s%s%s\n", indent, branch, c)
		}
		next := indent + "│   "
		if last { next = indent + "    " }
		printTree(kv, prefix+"/"+c, next)
	}
}

func isSlotTable(children []string) bool {
	for _, c := range children {
		if !strings.HasPrefix(c, "[") || !strings.HasSuffix(c, "]") {
			return false
		}
	}
	return true
}

func printSlotTable(kv kvspace.KVSpace, prefix, indent string, slots []string) {
	// 解析 [s0,s1]，找 s1 范围
	type slot struct{ s0, s1 int; val string }
	var rows []slot
	minS1, maxS1 := 0, 0
	maxS0 := 0
	for _, s := range slots {
		var s0, s1 int
		fmt.Sscanf(s, "[%d,%d]", &s0, &s1)
		v, _ := kv.Get(prefix + "/" + s)
		val := "(nil)"
		if !v.IsNil() { val = v.String() }
		rows = append(rows, slot{s0, s1, val})
		if s1 < minS1 { minS1 = s1 }
		if s1 > maxS1 { maxS1 = s1 }
		if s0 > maxS0 { maxS0 = s0 }
	}
	// 构建 2D 数组: rows[s0][s1-minS1]
	grid := make([][]string, maxS0+1)
	for i := range grid {
		row := make([]string, maxS1-minS1+1)
		for j := range row { row[j] = "" }
		grid[i] = row
	}
	for _, r := range rows {
		grid[r.s0][r.s1-minS1] = r.val
	}
	// 列序：负 s1 降序（-1,-2,...），0，正 s1 升序（1,2,...）→ 源码顺序
	colOrder := make([]int, 0, maxS1-minS1+1)
	for s1 := -1; s1 >= minS1; s1-- {
		colOrder = append(colOrder, s1)
	}
	for s1 := 0; s1 <= maxS1; s1++ {
		if s1 >= 0 {
			colOrder = append(colOrder, s1)
		}
	}
	// 打印：每行 [s0] \t cell \t cell ...
	for s0 := 0; s0 <= maxS0; s0++ {
		rowLast := s0 == maxS0
		branch := "├── "
		if rowLast { branch = "└── " }
		fmt.Fprint(os.Stdout, indent+branch+fmt.Sprintf("[%d]", s0))
		for _, s1 := range colOrder {
			fmt.Fprint(os.Stdout, "\t"+grid[s0][s1-minS1])
		}
		fmt.Fprintln(os.Stdout)
	}
}

func dumpPrefix(kv kvspace.KVSpace, prefix string) {
	kvspace.Walk(kv, prefix, func(path string, v kvspace.XValue) {
		short := strings.ReplaceAll(v.String(), "\n", "↵")
		if len(short) > 80 { short = short[:80] + "…" }
		fmt.Printf("%-60s %s\n", path, short)
	})
}

func parseValue(raw string) (kvspace.XValue, error) {
	idx := strings.Index(raw, ":")
	if idx < 0 { return kvspace.Str(raw), nil }
	kind, repr := raw[:idx], raw[idx+1:]
	switch kind {
	case "int":
		i, err := strconv.ParseInt(repr, 10, 64)
		if err != nil { return kvspace.XValue{}, fmt.Errorf("invalid int: %q", repr) }
		return kvspace.Int64(i), nil
	case "float":
		f, err := strconv.ParseFloat(repr, 64)
		if err != nil { return kvspace.XValue{}, fmt.Errorf("invalid float: %q", repr) }
		return kvspace.Float64(f), nil
	case "bool":
		switch repr {
		case "true": return kvspace.Bool(true), nil
		case "false": return kvspace.Bool(false), nil
		default: return kvspace.XValue{}, fmt.Errorf("invalid bool: %q", repr)
		}
	case "string": return kvspace.Str(repr), nil
	case "nil": return kvspace.XValue{}, nil
	default: return kvspace.Raw(kind, []byte(repr)), nil
	}
}
