package api

import "testing"

// 这个名字会一路走到存储层,而路径拼接对 ".." 是没有免疫力的。
func TestDisplayNameFor(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/photos/cat.png", "cat.png"},
		{"/cat.png", "cat.png"},
		{"/", "image"},
		{"", "image"},
		{"/photos/", "photos"},
		{"/..", "image"},
		{"/a/../../etc/passwd", "passwd"},
		{"/x/..%2f..", "image"},
	}
	for _, c := range cases {
		if got := displayNameFor(c.in); got != c.want {
			t.Errorf("displayNameFor(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}

	long := "/" + string(make([]byte, 300))
	if got := displayNameFor(long); len(got) > 100 {
		t.Errorf("超长名字要截断，得到 %d 个字符", len(got))
	}

	for _, in := range []string{"/a/../b", "/..%2e/x", "/foo..bar"} {
		if got := displayNameFor(in); got == ".." {
			t.Errorf("displayNameFor(%q) 漏出了 ..", in)
		}
	}
}
