// kvspace — KVSpace 命令行工具。
package main

import (
	"flag"
	"fmt"
	"os"
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
			v := kvspace.GetOne(kv, k)
			if v.IsNil() { fmt.Printf("%s\t(nil)\n", k) } else { fmt.Printf("%s\t%s\n", k, v) }
		}
	case "set":
		if len(sub) < 3 { exitUsage("kvspace set <key> <value>") }
		v, err := kvspace.ParseValue(sub[2])
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
		if err := kv.Mkindex(sub[1]); err != nil { fatalf("%v", err) }
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
		for _, c := range kv.List(sub[1]) { fmt.Println(c) }
	case "tree":
		cmdTree(kv, sub[1:])
	case "dump":
		if len(sub) < 2 { exitUsage("kvspace dump <prefix>") }
		kvspace.Walk(kv, sub[1], func(path string, v kvspace.XValue) {
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

func cmdTree(kv kvspace.KVSpace, args []string) {
	fs := flag.NewFlagSet("tree", flag.ExitOnError)
	showExt := fs.Bool("showext", true, "expand extindex entries (=target/)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: kvspace tree [--showext] <prefix>")
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if fs.NArg() == 0 { fs.Usage(); os.Exit(1) }
	p := fs.Arg(0)
	root := strings.TrimSuffix(p, kvspace.DirIndexSuf)
	if root == "" { root = kvspace.PathSep }
	fmt.Println(root)
	kvspace.FprintTree(os.Stdout, kv, p, "", *showExt)
}
