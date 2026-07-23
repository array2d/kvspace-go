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
	if v := os.Getenv("KVLANG_KVSPACE"); v != "" { return v }
	return "redis://127.0.0.1:6379"
}

func main() {
	fs := flag.NewFlagSet("kvspace", flag.ExitOnError)
	dsn := fs.String("kvspace", defaultKVSpace(), "kvspace DSN (redis://host:port)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: kvspace [--kvspace dsn] <subcommand> [args]")
		fmt.Fprintln(os.Stderr, "subcommands: get set del deltree mkindex link unlink extindex list tree dump watch notify clear")
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[1:])

	sub := fs.Args()
	if len(sub) == 0 { fs.Usage(); os.Exit(1) }

	kv := kvspace.Conn(*dsn)
	defer kv.DisConn()

	switch sub[0] {
	case "get":
		if len(sub) < 2 { exitUsage("kvspace get <key1> [key2 ...]") }
		for _, k := range sub[1:] {
			pfx, lst := kvspace.SepPath(k)
			if pfx == "" {
				pfx = kvspace.PathSep
			} else if pfx != kvspace.PathSep {
				pfx += kvspace.DirIndexSuf
			}
			v := kvspace.GetOne(kv, pfx, lst)
			if v.IsNil() { fmt.Printf("%s\t(nil)\n", k) } else { fmt.Printf("%s\t%s\n", k, v) }
		}
	case "set":
		if len(sub) < 3 { exitUsage("kvspace set <key> <value>") }
		v, err := parseValue(sub[2])
		if err != nil { fatalf("%v", err) }
		if err := kv.Set([]kvspace.KVPair{{Key: sub[1], Val: v}}); err != nil { fatalf("%v", err) }
	case "del":
		if len(sub) < 2 { exitUsage("kvspace del <key1> [key2 ...]") }
		if err := kv.Del(sub[1:]...); err != nil { fatalf("%v", err) }
	case "deltree":
		if len(sub) < 2 { exitUsage("kvspace deltree <prefix>") }
		if err := kv.DelTree(sub[1]); err != nil { fatalf("%v", err) }
	case "mkindex":
		if len(sub) < 2 { exitUsage("kvspace mkindex <path>") }
		if err := kv.Mkindex(ensureDirSuf(sub[1])); err != nil { fatalf("%v", err) }
	case "link":
		if len(sub) < 3 { exitUsage("kvspace link <target> <linkpath>") }
		if err := kv.Link(sub[1], sub[2]); err != nil { fatalf("%v", err) }
	case "unlink":
		if len(sub) < 2 { exitUsage("kvspace unlink <path>") }
		if err := kv.UnLink(sub[1]); err != nil { fatalf("%v", err) }
	case "extindex":
		if len(sub) < 3 { exitUsage("kvspace extindex <path> <extpath>") }
		if err := kv.ExtIndex(sub[1], sub[2]); err != nil { fatalf("%v", err) }
	case "list":
		if len(sub) < 2 { exitUsage("kvspace list <prefix>") }
		for _, c := range kv.List(ensureDirSuf(sub[1])) { fmt.Println(c) }
	case "tree":
		if len(sub) < 2 { exitUsage("kvspace tree <prefix>") }
		p := ensureDirSuf(sub[1])
		fmt.Println(strings.TrimSuffix(p, kvspace.DirIndexSuf))
		printTree(kv, p, "")
	case "dump":
		if len(sub) < 2 { exitUsage("kvspace dump <prefix>") }
		p := ensureDirSuf(sub[1])
		kvspace.Walk(kv, p, func(path string, v kvspace.XValue) {
			short := strings.ReplaceAll(v.String(), "\n", "↵")
			if len(short) > 80 { short = short[:80] + "…" }
			fmt.Printf("%-60s %s\n", path, short)
		})
	case "watch":
		cmdWatch(kv, sub[1:])
	case "notify":
		if len(sub) < 3 { exitUsage("kvspace notify <key> <value>") }
		if err := kv.Notify(sub[1], kvspace.Str(sub[2])); err != nil { fatalf("%v", err) }
	case "clear":
		if err := kv.Clear(); err != nil { fatalf("%v", err) }
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", sub[0])
		fs.Usage(); os.Exit(1)
	}
}

func ensureDirSuf(p string) string {
	if p == kvspace.PathSep || strings.HasSuffix(p, kvspace.DirIndexSuf) {
		return p
	}
	return p + kvspace.DirIndexSuf
}

func exitUsage(msg string)    { fmt.Fprintln(os.Stderr, "usage:", msg); os.Exit(1) }
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
	fmt.Println(kv.Watch(fs.Arg(0), *timeout))
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

// ── tree ─────────────────────────────────────────────────────────────────────

func printTree(kv kvspace.KVSpace, prefix, indent string) {
	children := kv.List(prefix)
	if len(children) > 0 && isSlotTable(children) {
		printSlotTable(kv, prefix, indent, children)
		return
	}
	slots, nonslots := splitSlots(kv, prefix, children)
	if len(slots) > 0 {
		printSlotTable(kv, prefix, indent, slots)
	}
	for i, c := range nonslots {
		last := i == len(nonslots)-1
		branch := "├── "
		if last {
			branch = "└── "
		}
		v := getAt(kv, prefix, c)
		if !v.IsNil() {
			fmt.Printf("%s%s%s\t%s\n", indent, branch, c, v)
		} else {
			fmt.Printf("%s%s%s\n", indent, branch, c)
		}
		next := indent + "│   "
		if last {
			next = indent + "    "
		}
		printTree(kv, kvspace.JoinPath(prefix, c)+kvspace.DirIndexSuf, next)
	}
}

func getAt(kv kvspace.KVSpace, dir, name string) kvspace.XValue {
	return kv.Get(dir, []string{name})[0]
}

func isSlotTable(children []string) bool {
	for _, c := range children {
		if !strings.HasPrefix(c, "[") || !strings.HasSuffix(c, "]") {
			return false
		}
	}
	return len(children) > 0
}

func splitSlots(kv kvspace.KVSpace, prefix string, children []string) (slots, nonslots []string) {
	for _, c := range children {
		if strings.HasPrefix(c, "[") && strings.HasSuffix(c, "]") {
			childDir := kvspace.JoinPath(prefix, c) + kvspace.DirIndexSuf
			if len(kv.List(childDir)) > 0 {
				nonslots = append(nonslots, c)
			} else {
				slots = append(slots, c)
			}
		} else {
			nonslots = append(nonslots, c)
		}
	}
	return
}

func printSlotTable(kv kvspace.KVSpace, prefix, indent string, slots []string) {
	type slot struct{ s0, s1 int; val string }
	var rows []slot
	minS1, maxS1, maxS0 := 0, 0, 0
	for _, s := range slots {
		var s0, s1 int
		fmt.Sscanf(s, "[%d,%d]", &s0, &s1)
		v := getAt(kv, prefix, s)
		val := "(nil)"
		if !v.IsNil() {
			val = v.String()
		}
		rows = append(rows, slot{s0, s1, val})
		if s1 < minS1 {
			minS1 = s1
		}
		if s1 > maxS1 {
			maxS1 = s1
		}
		if s0 > maxS0 {
			maxS0 = s0
		}
	}

	grid := make([][]string, maxS0+1)
	for i := range grid {
		row := make([]string, maxS1-minS1+1)
		for j := range row {
			row[j] = ""
		}
		grid[i] = row
	}
	for _, r := range rows {
		grid[r.s0][r.s1-minS1] = r.val
	}

	colOrder := make([]int, 0, maxS1-minS1+1)
	for s1 := -1; s1 >= minS1; s1-- {
		colOrder = append(colOrder, s1)
	}
	for s1 := 0; s1 <= maxS1; s1++ {
		colOrder = append(colOrder, s1)
	}

	for s0 := 0; s0 <= maxS0; s0++ {
		branch := "├── "
		if s0 == maxS0 {
			branch = "└── "
		}
		fmt.Fprintf(os.Stdout, "%s%s[%d]", indent, branch, s0)
		for _, s1 := range colOrder {
			fmt.Fprintf(os.Stdout, "\t%s", grid[s0][s1-minS1])
		}
		fmt.Fprintln(os.Stdout)
	}
}
