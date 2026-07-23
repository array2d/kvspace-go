package redis

import (
	"testing"

	kvspace "github.com/array2d/kvspace-go"
)

// ── ResolveIndexChain 单元测试（纯函数，无 Redis）───────────────────────────

func testGetSet(members map[string][]string) func(string) []string {
	return func(dk string) []string { return members[dk] }
}

func TestResolveIndexChain_NoExt(t *testing.T) {
	got := kvspace.ResolveIndexChain("/a/", testGetSet(map[string][]string{"/a/": {"x", "y"}}))
	if len(got) != 1 || got[0] != "/a/" {
		t.Errorf("got %v", got)
	}
}

func TestResolveIndexChain_SingleLink(t *testing.T) {
	sets := map[string][]string{
		"/link/":   {".ext=/target/"},
		"/target/": {"x", "y"},
	}
	got := kvspace.ResolveIndexChain("/link/", testGetSet(sets))
	if len(got) != 2 || got[0] != "/link/" || got[1] != "/target/" {
		t.Errorf("got %v", got)
	}
}

func TestResolveIndexChain_Chain(t *testing.T) {
	sets := map[string][]string{
		"/a/": {".ext=/b/"},
		"/b/": {".ext=/c/"},
		"/c/": {"x"},
	}
	got := kvspace.ResolveIndexChain("/a/", testGetSet(sets))
	if len(got) != 3 {
		t.Errorf("got %v", got)
	}
}

func TestResolveIndexChain_Cycle(t *testing.T) {
	sets := map[string][]string{
		"/a/": {".ext=/b/"},
		"/b/": {".ext=/a/"},
	}
	got := kvspace.ResolveIndexChain("/a/", testGetSet(sets))
	if len(got) > 40 {
		t.Errorf("cycle not stopped: len=%d", len(got))
	}
}

// ── EncodeExtEntry / DecodeExtEntry ─────────────────────────────────────────

func TestExtEntryRoundTrip(t *testing.T) {
	enc := kvspace.EncodeExtEntry("/target")
	if enc != ".ext=/target/" {
		t.Errorf("encode: %q", enc)
	}
	dec := kvspace.DecodeExtEntry(enc)
	if dec != "/target/" {
		t.Errorf("decode: %q", dec)
	}
}

func TestDecodeExtEntry_NonExt(t *testing.T) {
	if s := kvspace.DecodeExtEntry("normal_key"); s != "" {
		t.Errorf("expected empty, got %q", s)
	}
}

// ── delIndex 目录索引不变量测试（需本机 Redis）─────────────────────────────

func testKV(t *testing.T) kvspace.KVSpace {
	t.Helper()
	kv := Conn("127.0.0.1:6379")
	kv.Clear()
	return kv
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want { return true }
	}
	return false
}

func kvp(k string, v kvspace.XValue) kvspace.KVPair { return kvspace.KVPair{Key: k, Val: v} }
func i64(v int64) kvspace.XValue                  { return kvspace.Int64(v) }

func TestDelIndex_SiblingSurvives(t *testing.T) {
	kv := testKV(t)
	defer kv.DelTree("/t13a")
	kv.Set([]kvspace.KVPair{kvp("/t13a/b", i64(1))})
	kv.Set([]kvspace.KVPair{kvp("/t13a/c", i64(2))})
	kv.Del("/t13a/b")
	children := kv.List("/t13a")
	if !contains(children, "c") || contains(children, "b") {
		t.Errorf("List(/t13a) = %v, want [c]", children)
	}
	root := kv.List("/")
	if !contains(root, "t13a") {
		t.Errorf("兄弟 /t13a/c 仍存活，但 t13a 被误清出根索引")
	}
}

func TestDelIndex_CascadeCleansGhost(t *testing.T) {
	kv := testKV(t)
	kv.Set([]kvspace.KVPair{kvp("/t13b/x/y", i64(1))})
	kv.Del("/t13b/x/y")
	root := kv.List("/")
	if contains(root, "t13b") {
		t.Errorf("空目录链未级联清理，根索引残留幽灵 t13b")
	}
}

func TestDelIndex_ParentValueKeeps(t *testing.T) {
	kv := testKV(t)
	defer kv.DelTree("/t13c")
	kv.Set([]kvspace.KVPair{kvp("/t13c", i64(5))})
	kv.Set([]kvspace.KVPair{kvp("/t13c/k", i64(1))})
	kv.Del("/t13c/k")
	root := kv.List("/")
	if !contains(root, "t13c") {
		t.Errorf("/t13c 自身有值，不应被清出根索引")
	}
}

func TestDelIndex_DirWithChildrenKept(t *testing.T) {
	kv := testKV(t)
	defer kv.DelTree("/t13d")
	kv.Set([]kvspace.KVPair{kvp("/t13d", i64(1))})
	kv.Set([]kvspace.KVPair{kvp("/t13d/k", i64(2))})
	kv.Del("/t13d")
	children := kv.List("/t13d")
	if !contains(children, "k") {
		t.Errorf("List(/t13d) = %v, want 含 k", children)
	}
	root := kv.List("/")
	if !contains(root, "t13d") {
		t.Errorf("/t13d/. 非空，t13d 应保留于根索引")
	}
}

// ── Link / ExtIndex 集成测试 ───────────────────────────────────────────────

func TestLink_GetTransparent(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.UnLink("/t14lnk"); kv.DelTree("/t14tgt") }()
	kv.Set([]kvspace.KVPair{kvp("/t14tgt/x", i64(42))})
	kv.Link("/t14tgt", "/t14lnk")
	v := kv.Get("/t14lnk", []string{"x"})[0]
	if v.Int64() != 42 {
		t.Errorf("link Get: got %d, want 42", v.Int64())
	}
}

func TestLink_ListTransparent(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.UnLink("/t14lnk2"); kv.DelTree("/t14tgt2") }()
	kv.Set([]kvspace.KVPair{kvp("/t14tgt2/a", i64(1))})
	kv.Set([]kvspace.KVPair{kvp("/t14tgt2/b", i64(2))})
	kv.Link("/t14tgt2", "/t14lnk2")
	children := kv.List("/t14lnk2")
	if !contains(children, "a") || !contains(children, "b") {
		t.Errorf("link List: %v, want [a b]", children)
	}
}

func TestLink_SetTransparent(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.UnLink("/t14lnk3"); kv.DelTree("/t14tgt3") }()
	kv.Set([]kvspace.KVPair{kvp("/t14tgt3/x", i64(1))})
	kv.Link("/t14tgt3", "/t14lnk3")
	kv.Set([]kvspace.KVPair{kvp("/t14lnk3/y", i64(99))})
	v := kv.Get("/t14tgt3", []string{"y"})[0]
	if v.Int64() != 99 {
		t.Errorf("link Set should write through: got %d, want 99", v.Int64())
	}
}

func TestLink_DelLinkNotTarget(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.Del("/t14lnk4"); kv.DelTree("/t14tgt4") }()
	kv.Set([]kvspace.KVPair{kvp("/t14tgt4", i64(1))})
	kv.Link("/t14tgt4", "/t14lnk4")
	kv.Del("/t14lnk4")
	v := kv.Get("/", []string{"t14tgt4"})[0]
	if v.Int64() != 1 {
		t.Errorf("target 应存活: v=%d", v.Int64())
	}
}

func TestLink_DelThroughAncestorLink(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.UnLink("/t14al"); kv.DelTree("/t14real") }()
	kv.Set([]kvspace.KVPair{kvp("/t14real", i64(9))})
	kv.Set([]kvspace.KVPair{kvp("/t14real/x", i64(7))})
	kv.Link("/t14real", "/t14al")
	kv.Del("/t14al/x")
	_, lst := kvspace.SepPath("/t14real/x")
	v := kv.Get("/t14real", []string{lst})[0]
	if !v.IsNil() {
		t.Errorf("祖先链接穿透删除应删除 /t14real/x")
	}
}

func TestLink_DelTreeLinkUnlinkOnly(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.UnLink("/t14lnk5"); kv.DelTree("/t14tgt5") }()
	kv.Set([]kvspace.KVPair{kvp("/t14tgt5/a", i64(1))})
	kv.Link("/t14tgt5", "/t14lnk5")
	kv.DelTree("/t14lnk5")
	v := kv.Get("/t14tgt5", []string{"a"})[0]
	if v.Int64() != 1 {
		t.Errorf("target 子树应存活: v=%d", v.Int64())
	}
}

func TestExtIndex_ReadFallthrough(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.UnLink("/t14merge"); kv.DelTree("/t14base") }()
	kv.Set([]kvspace.KVPair{kvp("/t14base/a", i64(1))})
	kv.Set([]kvspace.KVPair{kvp("/t14base/b", i64(2))})
	kv.ExtIndex("/t14merge", "/t14base")
	v := kv.Get("/t14merge", []string{"b"})[0]
	if v.Int64() != 2 {
		t.Errorf("extindex read fallthrough: got %d, want 2", v.Int64())
	}
}

func TestExtIndex_WriteUpper(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.UnLink("/t14merge2"); kv.DelTree("/t14base2") }()
	kv.Set([]kvspace.KVPair{kvp("/t14base2/a", i64(1))})
	kv.ExtIndex("/t14merge2", "/t14base2")
	kv.Set([]kvspace.KVPair{kvp("/t14merge2/a", i64(99))})
	v := kv.Get("/t14merge2", []string{"a"})[0]
	if v.Int64() != 99 {
		t.Errorf("extindex write upper: got %d, want 99", v.Int64())
	}
	v2 := kv.Get("/t14base2", []string{"a"})[0]
	if v2.Int64() != 1 {
		t.Errorf("lower should be unchanged: got %d, want 1", v2.Int64())
	}
}

func TestExtIndex_ListMerge(t *testing.T) {
	kv := testKV(t)
	defer func() { kv.UnLink("/t14merge3"); kv.DelTree("/t14base3") }()
	kv.Set([]kvspace.KVPair{kvp("/t14base3/a", i64(1))})
	kv.Set([]kvspace.KVPair{kvp("/t14base3/b", i64(2))})
	kv.ExtIndex("/t14merge3", "/t14base3")
	kv.Set([]kvspace.KVPair{kvp("/t14merge3/c", i64(3))})
	children := kv.List("/t14merge3")
	if !contains(children, "a") || !contains(children, "b") || !contains(children, "c") {
		t.Errorf("extindex List: %v, want [a b c]", children)
	}
}
