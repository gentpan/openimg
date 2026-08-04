package email

import "testing"

func TestFormatFrom(t *testing.T) {
	cases := []struct{ name, addr, want string }{
		{"Openimg", "noreply@openimg.io", "Openimg <noreply@openimg.io>"},
		{"", "noreply@openimg.io", "noreply@openimg.io"},
		// An operator who already configured a full header keeps it verbatim.
		{"Openimg", "Other <a@b.com>", "Other <a@b.com>"},
	}
	for _, c := range cases {
		if got := FormatFrom(c.name, c.addr); got != c.want {
			t.Errorf("FormatFrom(%q, %q) = %q, want %q", c.name, c.addr, got, c.want)
		}
	}
	// Non-ASCII must be RFC 2047 encoded, not passed through raw.
	got := FormatFrom("图床", "noreply@openimg.io")
	if got == "图床 <noreply@openimg.io>" {
		t.Error("non-ASCII display name was not encoded")
	}
	if BareAddress(got) != "noreply@openimg.io" {
		t.Errorf("BareAddress(%q) = %q", got, BareAddress(got))
	}
}

func TestBareAddress(t *testing.T) {
	for in, want := range map[string]string{
		"Openimg <noreply@openimg.io>": "noreply@openimg.io",
		"noreply@openimg.io":           "noreply@openimg.io",
		"  spaced@x.io  ":              "spaced@x.io",
	} {
		if got := BareAddress(in); got != want {
			t.Errorf("BareAddress(%q) = %q, want %q", in, got, want)
		}
	}
}
