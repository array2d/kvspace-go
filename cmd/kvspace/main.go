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
		fmt.Fprintln(os.Stderr, "subcommands: get set del deltree mount unmount list tree dump watch notify clear")
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
		vals := kv.Get([]string{sub[1]})
		if vals[0].IsNil() { fmt.Printf("%s\t(nil)\n", sub[1]) } else { fmt.Printf("%s\t%s\n", sub[1], vals[0]) }
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
	case "mount":
		if len(sub) < 3 { exitUsage("kvspace mount <target> <linkpath>") }
		if err := kv.Mount(sub[1], sub[2]); err != nil { fatalf("%v", err) }
	case "unmount":
		if len(sub) < 2 { exitUsage("kvspace unmount <linkpath>") }
		if err := kv.UnMount(sub[1]); err != nil { fatalf("%v", err) }
	case "list":
		if len(sub) < 2 { exitUsage("kvspace list <prefix>") }
		for _, c := range kv.List(sub[1]) { fmt.Println(c) }
	case "tree":
		if len(sub) < 2 { exitUsage("kvspace tree <prefix>") }
		fmt.Println(sub[1])
		kvspace.Walk(kv, sub[1], func(path string, v kvspace.XValue) {
			short := strings.ReplaceAll(v.String(), "\n", "↵")
			if len(short) > 80 { short = short[:80] + "…" }
			fmt.Printf("%-60s %s\n", path, short)
		})
	case "dump":
		if len(sub) < 2 { exitUsage("kvspace dump <prefix>") }
		dumpPrefix(kv, sub[1])
	case "watch":
		if len(sub) < 2 { exitUsage("kvspace watch <key>") }
		fmt.Println(kv.Watch(sub[1], 0))
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

func exitUsage(msg string)    { fmt.Fprintln(os.Stderr, "usage:", msg); os.Exit(1) }
func fatalf(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...); os.Exit(1) }

func dumpPrefix(kv kvspace.KVSpace, prefix string) {
	v := kv.Get([]string{prefix})[0]
	if !v.IsNil() { fmt.Printf("%-60s %s\n", prefix, v.String()) }
	for _, c := range kv.List(prefix) { dumpPrefix(kv, kvspace.JoinPath(prefix, c)) }
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
