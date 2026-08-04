package models

import (
	"regexp"
	"testing"
)

// A code that equals a client-side route would shadow the page it names, so
// generation must never produce one and lookup must never accept one.
func TestShortCodeNeverShadowsAppRoutes(t *testing.T) {
	for _, route := range []string{"login", "admin", "space", "refer", "upload", "LOGIN", "Admin"} {
		if IsValidShortCode(route) {
			t.Errorf("IsValidShortCode(%q) = true — it would shadow a real page", route)
		}
	}
}

func TestIsValidShortCode(t *testing.T) {
	ok := []string{"aB3d", "0000", "zzzzzz", "A1b2C3"}
	bad := []string{
		"abc",     // too short
		"abcdefg", // too long
		"ab-d",    // hyphen is outside the alphabet
		"ab_d",    // underscore likewise
		"ab.d",    // a dot would make it look like a file
		"ab/d",    // path separator
		"",        //
		"日本語です",   // non-ASCII
	}
	for _, s := range ok {
		if !IsValidShortCode(s) {
			t.Errorf("IsValidShortCode(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if IsValidShortCode(s) {
			t.Errorf("IsValidShortCode(%q) = true, want false", s)
		}
	}
}

func TestNewShortCodeShapeAndReservedAvoidance(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-zA-Z]+$`)
	for length := 4; length <= 6; length++ {
		for i := 0; i < 3000; i++ {
			c := NewShortCode(length)
			if len(c) != length {
				t.Fatalf("NewShortCode(%d) = %q, wrong length", length, c)
			}
			if !re.MatchString(c) {
				t.Fatalf("code %q outside the 0-9a-zA-Z alphabet", c)
			}
			if !IsValidShortCode(c) {
				t.Fatalf("generated code %q is not accepted by IsValidShortCode", c)
			}
		}
	}
}
