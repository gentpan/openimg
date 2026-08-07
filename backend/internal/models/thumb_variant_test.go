package models

import "testing"

// The name format is load-bearing: the object key's extension is derived from
// it, and the CSV in images.variants is walked by the purge and backup jobs.
// A name that round-trips wrong here is an object that never gets deleted.
func TestThumbVariantNameRoundTrip(t *testing.T) {
	cases := []struct {
		width int
		ext   string
		name  string
		key   string
	}{
		// The historic shape: bare width, WebP implied. New uploads with
		// default settings must produce names identical to existing rows.
		{600, "webp", "w600", "base_w600.webp"},
		{600, "", "w600", "base_w600.webp"},
		{200, "webp", "w200", "base_w200.webp"},
		// Policy-configured tiers carry their format.
		{400, "avif", "w400.avif", "base_w400.avif"},
		{1000, "jpg", "w1000.jpg", "base_w1000.jpg"},
		{800, "webp", "w800", "base_w800.webp"},
	}
	for _, c := range cases {
		name := ThumbVariantName(c.width, c.ext)
		if name != c.name {
			t.Errorf("ThumbVariantName(%d, %q) = %q, want %q", c.width, c.ext, name, c.name)
		}
		w, ext := ParseThumbVariant(name)
		wantExt := c.ext
		if wantExt == "" {
			wantExt = "webp"
		}
		if w != c.width || ext != wantExt {
			t.Errorf("ParseThumbVariant(%q) = (%d, %q), want (%d, %q)", name, w, ext, c.width, wantExt)
		}
		if key := VariantKey("base.jpg", name); key != c.key {
			t.Errorf("VariantKey(base.jpg, %q) = %q, want %q", name, key, c.key)
		}
	}
}

func TestParseThumbVariantRejectsNonThumbs(t *testing.T) {
	for _, name := range []string{"webp", "avif", "orig-heic", "w", "wabc", "w0", "w600.png", "w600.exe", ""} {
		if w, _ := ParseThumbVariant(name); w != 0 {
			t.Errorf("ParseThumbVariant(%q) 认成了缩略图（宽 %d）", name, w)
		}
	}
}

// The selection rule must reproduce the old fixed preference for existing
// images while extending to configured tiers.
func TestGridThumbVariantSelection(t *testing.T) {
	cases := []struct {
		variants []string
		want     string
	}{
		{[]string{"webp", "w600", "w200"}, "w600"},            // old rule: grid tier over legacy
		{[]string{"w200"}, "w200"},                            // legacy only
		{[]string{"avif", "webp"}, ""},                        // no thumbs at all
		{[]string{"w400.avif", "w600", "w1200"}, "w400.avif"}, // configured tier is cheapest ≥2x
		{[]string{"w200.avif", "w600"}, "w600"},               // sub-2x tier upgraded when possible
		{[]string{"w1000.jpg"}, "w1000.jpg"},
		{[]string{}, ""},
	}
	for _, c := range cases {
		if got := GridThumbVariant(c.variants); got != c.want {
			t.Errorf("GridThumbVariant(%v) = %q, want %q", c.variants, got, c.want)
		}
	}
}
