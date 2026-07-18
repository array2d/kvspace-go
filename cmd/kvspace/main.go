// kvspace — KVSpace 命令行工具。
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/array2d/kvlang-go"
	_ "github.com/array2d/kvlang-go/redis"
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
		fmt.Fprintln(os.Stderr, "subcommands: get mget set del list tree dump watch notify clear")
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
		fmt.Println(v)
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
	case "list":
		if len(sub) < 2 { exitUsage("kvspace list <prefix>") }
		children, err := kv.List(sub[1])
		if err != nil { fatalf("%v", err) }
		for _, c := range children { fmt.Println(c) }
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
	for i, c := range children {
		last := i == len(children)-1
		branch := "├── "
		if last { branch = "└── " }
		fmt.Printf("%s%s%s\n", indent, branch, c)
		next := indent + "│   "
		if last { next = indent + "    " }
		printTree(kv, prefix+"/"+c, next)
	}
}

func dumpPrefix(kv kvspace.KVSpace, prefix string) {
	if valV, err := kv.Get(prefix); err == nil && !valV.IsNil() {
		short := strings.ReplaceAll(valV.String(), "\n", "↵")
		if len(short) > 80 { short = short[:80] + "…" }
		fmt.Printf("%-60s %s\n", prefix, short)
	}
	children, _ := kv.List(prefix)
	for _, c := range children { dumpPrefix(kv, prefix+"/"+c) }
}

func parseValue(raw string) (kvspace.XValue, error) {
	idx := strings.Index(raw, ":")
	if idx < 0 { return kvspace.Str(raw), nil }
	kind, repr := raw[:idx], raw[idx+1:]
	switch kind {
	case "int":
		i, err := strconv.ParseInt(repr, 10, 64)
		if err != nil { return kvspace.XValue{}, fmt.Errorf("invalid int: %q", repr) }
		return kvspace.Int(i), nil
	case "float":
		f, err := strconv.ParseFloat(repr, 64)
		if err != nil { return kvspace.XValue{}, fmt.Errorf("invalid float: %q", repr) }
		return kvspace.Float(f), nil
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
