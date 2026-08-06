package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The wildcards are the point. A filename with _ or % in it is ordinary
// ("draft_v2.png", "50%_off.jpg"), and without escaping the search silently
// widens: "draft_v2" would also match "draftXv2", and a lone "%" would match
// every row in the table while looking like a successful search.
func TestLikePatternEscapesWildcards(t *testing.T) {
	cases := map[string]string{
		"photo":        `%photo%`,
		"draft_v2.png": `%draft\_v2.png%`,
		"50%off":       `%50\%off%`,
		"%":            `%\%%`,
		"_":            `%\_%`,
		// Backslash has to be escaped first, or it corrupts the escapes added
		// after it: a naive replacer turns `a\b` into `a\\b` only if backslash
		// is handled before the others touch the string.
		`a\b`: `%a\\b%`,
		// A term that is already all escapes must not double-escape.
		`\%`: `%\\\%%`,
		"":   `%%`,
	}
	for in, want := range cases {
		if got := likePattern(in); got != want {
			t.Errorf("likePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every ILIKE in the package must go through likePattern and declare ESCAPE.
//
// The one that matters is bulk delete: it takes the gallery's search string and
// removes everything it matches. Unescaped, a search for "%" selects the user's
// entire library while the confirmation dialog shows a count they believe came
// from what they typed. This test is a grep because the failure mode is someone
// adding a fifth call site and reaching for the familiar "%"+kw+"%".
func TestAllILikeCallSitesEscape(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "ILIKE") {
				continue
			}
			if !strings.Contains(line, `ESCAPE '\'`) {
				t.Errorf("%s:%d 的 ILIKE 没有声明 ESCAPE：%s", f, i+1, strings.TrimSpace(line))
			}
			if strings.Contains(line, `"%"+`) || strings.Contains(line, `"%" +`) {
				t.Errorf("%s:%d 手工拼了通配符，应该用 likePattern：%s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}
