package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gentpan/openimg/backend/internal/i18n"
	"html"
	"io"
	"net/http"
	"time"
)

// Sendflare client. POST https://api.sendflare.com/v1/send with Bearer auth.
type Sendflare struct {
	APIKey  string
	From    string
	BaseURL string
	HTTP    *http.Client
}

func NewSendflare(apiKey, from string) *Sendflare {
	return &Sendflare{
		APIKey:  apiKey,
		From:    from,
		BaseURL: "https://api.sendflare.com",
		HTTP:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Configured reports whether we have credentials to send.
func (s *Sendflare) Configured() bool { return s != nil && s.APIKey != "" && s.From != "" }

func (s *Sendflare) Name() string { return "sendflare" }

type sendReq struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (s *Sendflare) Send(ctx context.Context, to, subject, htmlBody string) error {
	if !s.Configured() {
		return fmt.Errorf("sendflare not configured")
	}
	body, _ := json.Marshal(sendReq{
		From:    s.From,
		To:      to,
		Subject: subject,
		Body:    htmlBody,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/v1/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("sendflare http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("sendflare %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

// Brand palettes for the OTP mail, mirroring the site's two themes.
//
// Emails stay light-background regardless: dark mode in email clients is a
// minefield (Gmail recolors, Outlook inverts), so the brand shows in the
// accents, not the canvas. Each code color is the darkened shade of its brand
// that stays readable on the tinted box — the raw brand green is a 1.27:1
// against white, which is exactly the mistake the old template made in
// reverse (violet box, green digits, neither matching the site).
type mailPalette struct {
	BoxBg, BoxBorder, Code string
}

var mailPalettes = map[string]mailPalette{
	"green":  {BoxBg: "#f1fbe9", BoxBorder: "#d0e8bd", Code: "#3f7a12"},
	"violet": {BoxBg: "#f7f2ff", BoxBorder: "#e4d3ff", Code: "#7a3fd4"},
}

// MailBrand clamps a client-supplied brand hint to a known palette. The header
// is attacker-controlled like any other; an unknown value falls back to the
// default rather than reaching the template.
func MailBrand(hint string) string {
	if _, ok := mailPalettes[hint]; ok {
		return hint
	}
	return "green"
}

// OTPEmailHTML renders the verification-code email body in the caller's
// language and theme.
func OTPEmailHTML(lang i18n.Lang, brand, code string, ttlMinutes int) string {
	pal := mailPalettes[MailBrand(brand)]
	return fmt.Sprintf(`<!DOCTYPE html>
<html><body style="margin:0;padding:0;background:#f5f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 16px">
    <tr><td align="center">
      <table role="presentation" width="480" style="background:#ffffff;border-radius:12px;padding:40px 32px;max-width:100%%">
        <tr><td>
          <h1 style="margin:0 0 8px;color:#1a1a1a;font-size:20px">%s</h1>
          <p style="margin:0 0 24px;color:#6b6b6b;font-size:14px;line-height:1.5">
            %s
          </p>
          <div style="background:%s;border:1px solid %s;border-radius:8px;padding:18px;text-align:center">
            <div style="font-family:'SF Mono',Monaco,monospace;font-size:32px;letter-spacing:8px;color:%s;font-weight:600">%s</div>
          </div>
          <p style="margin:24px 0 0;color:#9b9b9b;font-size:12px;line-height:1.5">
            %s
          </p>
        </td></tr>
      </table>
      <p style="margin:16px 0 0;color:#a3a3a3;font-size:12px">%s</p>
    </td></tr>
  </table>
</body></html>`,
		i18n.TL(lang, "email.otp.heading"),
		i18n.TL(lang, "email.otp.body", ttlMinutes),
		pal.BoxBg, pal.BoxBorder, pal.Code, code,
		i18n.TL(lang, "email.otp.warning"),
		i18n.TL(lang, "email.footer.tagline"))
}

// InactiveEmailHTML 渲染"很久没见，你的图都还在"这封信。
//
// 刻意不提删除、不设期限、不写"逾期将清理"。这不是催促,是一次唤回,顺带把
// 图床最该给人的那个承诺再说一遍——链接不会坏。图床存的东西和网盘不一样:
// 它被写进了别人的博客和帖子里,收信人自己不登录,不代表那些页面不在被人看。
// 拿删除吓唬他,吓走的是这个承诺本身。
func InactiveEmailHTML(brand, nickname string, images int, space, siteURL string) string {
	pal := mailPalettes[MailBrand(brand)]
	who := nickname
	if who == "" {
		who = "你"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html><body style="margin:0;padding:0;background:#f5f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 16px">
    <tr><td align="center">
      <table role="presentation" width="480" style="background:#ffffff;border-radius:12px;padding:40px 32px;max-width:100%%">
        <tr><td>
          <h1 style="margin:0 0 8px;color:#1a1a1a;font-size:20px">好久不见</h1>
          <p style="margin:0 0 24px;color:#6b6b6b;font-size:14px;line-height:1.6">
            %s 有半年多没来 Openimg 了。这封信只是想说一声：图都还在，链接也都还能用。
          </p>
          <div style="background:%s;border:1px solid %s;border-radius:8px;padding:18px;text-align:center">
            <div style="color:%s;font-size:28px;font-weight:600;line-height:1.2">%d 张</div>
            <div style="color:#6b6b6b;font-size:13px;margin-top:4px">共 %s</div>
          </div>
          <p style="margin:24px 0 0;color:#6b6b6b;font-size:14px;line-height:1.6">
            我们不会因为你没登录就删掉它们——那些链接可能正躺在别人的文章里。
          </p>
          <p style="margin:24px 0 0">
            <a href="%s" style="display:inline-block;background:%s;color:#ffffff;text-decoration:none;padding:10px 20px;border-radius:8px;font-size:14px">回去看看</a>
          </p>
          <p style="margin:28px 0 0;color:#9b9b9b;font-size:12px;line-height:1.5">
            这封信不需要回复。如果你不想再收到，登录后可以在设置里关掉。
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body></html>`, html.EscapeString(who), pal.BoxBg, pal.BoxBorder, pal.Code, images, html.EscapeString(space),
		html.EscapeString(siteURL), pal.Code)
}
