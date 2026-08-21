package storage

import (
	"testing"

	"github.com/gentpan/openimg/backend/internal/models"
)

// 缩略图和原图分属两个源站。这个测试盯的是路由本身:w600 必须落在缩略图域名,
// 而 webp/avif/原图必须留在图片域名。曾经有三处各自内联这条规则,按需生成那处
// 漏了缩略图分支,同一张 w600 在列表里是 cache 域名、在生成响应里是 files 域名。
func TestVariantURLFor(t *testing.T) {
	p := &models.StorageProfile{
		PublicBaseURL: "https://files.example.com",
		ThumbBaseURL:  "https://cache.example.com",
	}
	cases := []struct{ variant, key, want string }{
		{"w200", "a/b_w200.webp", "https://cache.example.com/a/b_w200.webp"},
		{"w600", "a/b_w600.webp", "https://cache.example.com/a/b_w600.webp"},
		{"w400.avif", "a/b_w400.avif", "https://cache.example.com/a/b_w400.avif"},
		{"webp", "a/b.webp", "https://files.example.com/a/b.webp"},
		{"avif", "a/b.avif", "https://files.example.com/a/b.avif"},
		{"orig-heic", "a/b.orig.heic", "https://files.example.com/a/b.orig.heic"},
	}
	for _, c := range cases {
		if got := VariantURLFor(p, c.variant, c.key, "https://site.example.com"); got != c.want {
			t.Errorf("VariantURLFor(%q) = %q, want %q", c.variant, got, c.want)
		}
	}
}

// 没配缩略图域名时,缩略图回落到图片域名——而不是回落到站点自身。
func TestVariantURLForNoThumbHost(t *testing.T) {
	p := &models.StorageProfile{PublicBaseURL: "https://files.example.com"}
	got := VariantURLFor(p, "w600", "a/b_w600.webp", "https://site.example.com")
	if want := "https://files.example.com/a/b_w600.webp"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
