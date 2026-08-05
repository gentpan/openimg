package main

import "testing"

// The RP ID used to be the literal "openimg.io", which silently broke WebAuthn
// on every other deployment: a browser refuses a credential scoped to a domain
// that is not a registrable suffix of the page's own origin.
func TestRPIDFor(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://openimg.io", "openimg.io"},
		{"https://openimg.io/", "openimg.io"},
		{"https://img.example.com", "img.example.com"},
		{"http://localhost:8080", "localhost"},   // port dropped: RP IDs are domains
		{"http://127.0.0.1:8080", "127.0.0.1"},
		{"https://sub.domain.co.uk:443", "sub.domain.co.uk"},
		{"", "localhost"},          // unparseable falls back rather than panicking
		{"not a url", "localhost"},
	}
	for _, c := range cases {
		if got := rpIDFor(c.in); got != c.want {
			t.Errorf("rpIDFor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
