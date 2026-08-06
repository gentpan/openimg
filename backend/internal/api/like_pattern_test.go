package api

import "testing"

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
