package kvspace

import "testing"

func TestSplitIndex(t *testing.T) {
	cases := []struct {
		path  string
		dir   string
		child string
		kind  SepKind
	}{
		{"/lib/init", "/lib/", "init", SepDir},
		{"/lib/math.Pi", "/lib/math.", "Pi", SepDict},
		{"/lib/b[0]", "/lib/", "b[0]", SepArray},
		{"/lib/b[0,1]", "/lib/", "b[0,1]", SepArray},
		{"/", "/", "", SepDir},
		{"/a.b.c", "/a.b.", "c", SepDict},
		{"/lib/sum/while0[0,0]", "/lib/sum/", "while0[0,0]", SepArray},
		{"/lib/sum/[0,0]", "/lib/sum/", "[0,0]", SepDir}, // 匿名代码坐标：末段 [ 在首位，不拆数组
	}
	for _, c := range cases {
		dir, child, kind := SplitIndex(c.path)
		if dir != c.dir || child != c.child || kind != c.kind {
			t.Errorf("SplitIndex(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.path, dir, child, kind, c.dir, c.child, c.kind)
		}
	}
}
