package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrNotConfigured is returned when a send is attempted without credentials.
var ErrNotConfigured = errors.New("email: provider not configured")

// Cloudflare Email Sending.
//
//	POST https://api.cloudflare.com/client/v4/accounts/{account_id}/email/sending/send
//
// The sending domain must be verified in the Cloudflare dashboard first, and
// the API token needs the "Email Sending" permission on that account.
type Cloudflare struct {
	AccountID string
	Token     string
	From      string
	BaseURL   string
	HTTP      *http.Client
}

func NewCloudflare(accountID, token, from string) *Cloudflare {
	return &Cloudflare{
		AccountID: accountID,
		Token:     token,
		From:      from,
		BaseURL:   "https://api.cloudflare.com/client/v4",
		HTTP:      &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Cloudflare) Name() string { return "cloudflare" }

func (c *Cloudflare) Configured() bool {
	return c != nil && c.AccountID != "" && c.Token != "" && c.From != ""
}

type cfSendReq struct {
	To      string `json:"to"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text,omitempty"`
}

// cfResponse is Cloudflare's standard envelope. A 200 with success=false is a
// real failure, so the body is checked rather than just the status code.
type cfResponse struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func (c *Cloudflare) Send(ctx context.Context, to, subject, htmlBody string) error {
	if !c.Configured() {
		return fmt.Errorf("cloudflare: %w", ErrNotConfigured)
	}
	payload, err := json.Marshal(cfSendReq{
		To:      to,
		From:    c.From,
		Subject: subject,
		HTML:    htmlBody,
		Text:    textFromHTML(htmlBody),
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/accounts/%s/email/sending/send", c.BaseURL, c.AccountID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare email: %w", err)
	}
	defer resp.Body.Close()

	// Cap the read: an error page from a proxy could be arbitrarily large.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("cloudflare email: status %d: %s", resp.StatusCode, string(body))
	}
	var out cfResponse
	if err := json.Unmarshal(body, &out); err == nil && !out.Success && len(out.Errors) > 0 {
		return fmt.Errorf("cloudflare email: %s (code %d)", out.Errors[0].Message, out.Errors[0].Code)
	}
	return nil
}
