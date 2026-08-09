package email

import (
	"strings"

	"github.com/gentpan/openimg/backend/internal/i18n"
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

// Each theme's mail must carry its own palette and none of the other's — the
// bug this replaces was exactly a mixed one (violet box, green digits).
func TestOTPEmailFollowsBrand(t *testing.T) {
	green := OTPEmailHTML(i18n.ZH, "green", "445207", 5)
	violet := OTPEmailHTML(i18n.ZH, "violet", "445207", 5)

	for _, hex := range []string{"#f2fbe9", "#cfe8bd", "#3a7a12"} {
		if !strings.Contains(green, hex) {
			t.Errorf("绿色主题邮件缺少配色 %s", hex)
		}
		if strings.Contains(violet, hex) {
			t.Errorf("紫色主题邮件混进了绿色配色 %s", hex)
		}
	}
	for _, hex := range []string{"#f5f2ff", "#ded3ff", "#6d3fd4"} {
		if !strings.Contains(violet, hex) {
			t.Errorf("紫色主题邮件缺少配色 %s", hex)
		}
		if strings.Contains(green, hex) {
			t.Errorf("绿色主题邮件混进了紫色配色 %s", hex)
		}
	}
}

// Unknown hints clamp to the default instead of reaching the template — the
// header is client input like any other.
func TestMailBrandClampsUnknown(t *testing.T) {
	for _, hint := range []string{"", "purple", "GREEN", "<script>", "violet; drop"} {
		if got := MailBrand(hint); got != "green" {
			t.Errorf("MailBrand(%q) = %q, 未知值应回落 green", hint, got)
		}
	}
	if MailBrand("violet") != "violet" {
		t.Error("合法的 violet 被误clamp了")
	}
}

// The body follows the caller's language, same as every API response.
func TestOTPEmailFollowsLanguage(t *testing.T) {
	zh := OTPEmailHTML(i18n.ZH, "green", "445207", 5)
	en := OTPEmailHTML(i18n.EN, "green", "445207", 5)
	if !strings.Contains(zh, "验证码") {
		t.Error("中文邮件里没有中文")
	}
	if strings.Contains(en, "验证码") {
		t.Error("英文邮件里仍有中文")
	}
	if !strings.Contains(en, "valid for 5 minutes") {
		t.Errorf("英文正文没按语言渲染")
	}
}
