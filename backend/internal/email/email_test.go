package email

import (
	"net/mail"
	"testing"
)

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
	// And the encoded form still has to parse back to the right mailbox —
	// checked with net/mail rather than a local helper, since RFC 5322 is
	// exactly what a receiving MTA will apply to this header.
	addr, err := mail.ParseAddress(got)
	if err != nil {
		t.Fatalf("mail.ParseAddress(%q) = %v", got, err)
	}
	if addr.Address != "noreply@openimg.io" {
		t.Errorf("parsed address = %q, want noreply@openimg.io", addr.Address)
	}
	if addr.Name != "图床" {
		t.Errorf("parsed display name = %q, want 图床（RFC 2047 应能解回）", addr.Name)
	}
}
