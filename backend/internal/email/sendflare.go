package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// OTPEmailHTML renders the verification-code email body.
func OTPEmailHTML(code string, ttlMinutes int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html><body style="margin:0;padding:0;background:#f5f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 16px">
    <tr><td align="center">
      <table role="presentation" width="480" style="background:#ffffff;border-radius:12px;padding:40px 32px;max-width:100%%">
        <tr><td>
          <h1 style="margin:0 0 8px;color:#1a1a1a;font-size:20px">Openimg 验证码</h1>
          <p style="margin:0 0 24px;color:#6b6b6b;font-size:14px;line-height:1.5">
            请在登录页输入下方验证码完成登录。验证码 %d 分钟内有效。
          </p>
          <div style="background:#f5f0ff;border:1px solid #e0d4ff;border-radius:8px;padding:18px;text-align:center">
            <div style="font-family:'SF Mono',Monaco,monospace;font-size:32px;letter-spacing:8px;color:#7c3aed;font-weight:600">%s</div>
          </div>
          <p style="margin:24px 0 0;color:#9b9b9b;font-size:12px;line-height:1.5">
            如果不是你本人操作，请忽略此邮件。任何人（包括 Openimg 工作人员）都不会向你索要这个验证码。
          </p>
        </td></tr>
      </table>
      <p style="margin:16px 0 0;color:#a3a3a3;font-size:12px">Openimg · 免费图床</p>
    </td></tr>
  </table>
</body></html>`, ttlMinutes, code)
}
