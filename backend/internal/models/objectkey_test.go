package models

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestObjectKeyShape(t *testing.T) {
	when := time.Date(2026, 8, 4, 23, 59, 0, 0, time.UTC)
	key := ObjectKeyFor(when, "jpg")

	re := regexp.MustCompile(`^\d{4}/\d{2}/\d{2}/[0-9a-zA-Z]{12}\.jpg$`)
	if !re.MatchString(key) {
		t.Fatalf("key = %q, want 年/月/日/12位ID.扩展名", key)
	}
	if !strings.HasPrefix(key, "2026/08/04/") {
		t.Errorf("key = %q, want the UTC upload date as prefix", key)
	}
}

// The date must be the UTC day, not the server's local one — otherwise the
// same upload lands in a different folder depending on where the box is.
func TestObjectKeyUsesUTCDate(t *testing.T) {
	// 00:30 on the 5th in UTC+8 is still the 4th in UTC.
	zone := time.FixedZone("UTC+8", 8*3600)
	local := time.Date(2026, 8, 5, 0, 30, 0, 0, zone)
	if got := ObjectKeyFor(local, "png"); !strings.HasPrefix(got, "2026/08/04/") {
		t.Errorf("key = %q, want the 2026/08/04 UTC date", got)
	}
}

func TestNewKeyIDAlphabetAndUniqueness(t *testing.T) {
	ok := regexp.MustCompile(`^[0-9a-zA-Z]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 20000; i++ {
		id := NewKeyID()
		if !ok.MatchString(id) {
			t.Fatalf("id %q outside the 0-9a-zA-Z alphabet", id)
		}
		if strings.ContainsAny(id, "-_") {
			t.Fatalf("id %q contains - or _", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q within 20k draws", id)
		}
		seen[id] = true
	}
}

// Rejection sampling should leave the alphabet roughly flat. A modulo-only
// implementation biases the first 8 symbols by ~25%, which this catches.
func TestNewKeyIDIsNotBiased(t *testing.T) {
	counts := map[rune]int{}
	const draws = 40000
	for i := 0; i < draws; i++ {
		for _, r := range NewKeyID() {
			counts[r]++
		}
	}
	if len(counts) != len(keyAlphabet) {
		t.Fatalf("saw %d distinct symbols, want %d", len(counts), len(keyAlphabet))
	}
	expected := float64(draws*KeyIDLength) / float64(len(keyAlphabet))
	for r, n := range counts {
		if d := float64(n) / expected; d < 0.9 || d > 1.1 {
			t.Errorf("symbol %q appeared %.2f× its expected share", r, d)
		}
	}
}

// The whole migration story rests on this: keys written under the old
// content-addressed scheme must still resolve to the exact same derivative
// objects, because those objects are already sitting in the bucket.
func TestVariantKeyBackwardCompatible(t *testing.T) {
	const sha = "f190868f4767dcf4c716948218ad261d4bf654171231fd5425160aad97bbbd36"
	legacy := "img/" + sha[0:2] + "/" + sha[2:4] + "/" + sha + ".jpg"

	want := map[string]string{
		VariantWebP:            "img/f1/90/" + sha + ".webp",
		VariantAVIF:            "img/f1/90/" + sha + ".avif",
		VariantW200:            "img/f1/90/" + sha + "_w200.webp",
		VariantW1200:           "img/f1/90/" + sha + "_w1200.webp",
		OriginalVariant("jpg"): "img/f1/90/" + sha + ".orig.jpg",
	}
	for variant, expect := range want {
		if got := VariantKey(legacy, variant); got != expect {
			t.Errorf("VariantKey(legacy, %q) = %q, want %q", variant, got, expect)
		}
	}
}

func TestVariantKeyNewLayout(t *testing.T) {
	key := "2026/08/04/aB3dEf7hJ9kL.jpg"
	want := map[string]string{
		VariantWebP:             "2026/08/04/aB3dEf7hJ9kL.webp",
		VariantW600:             "2026/08/04/aB3dEf7hJ9kL_w600.webp",
		OriginalVariant("heic"): "2026/08/04/aB3dEf7hJ9kL.orig.heic",
	}
	for variant, expect := range want {
		if got := VariantKey(key, variant); got != expect {
			t.Errorf("VariantKey(new, %q) = %q, want %q", variant, got, expect)
		}
	}
	if got := BaseKey(key); got != "2026/08/04/aB3dEf7hJ9kL" {
		t.Errorf("BaseKey = %q", got)
	}
}
