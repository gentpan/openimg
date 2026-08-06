// Package email sends transactional mail (OTP codes, notifications).
//
// Two providers ship: Cloudflare Email Sending and Sendflare. They're behind a
// Sender interface so the deployment picks one via configuration — an image
// host that can't send a verification code can't gate uploads on a verified
// email, which is the cheapest anti-abuse control there is.
package email

import (
	"context"
	"mime"
	"regexp"
	"strings"
)

type Sender interface {
	// Send delivers one message. text is the plain-text alternative; an empty
	// string means "derive it from the HTML".
	Send(ctx context.Context, to, subject, htmlBody string) error
	// Configured reports whether credentials are present. Callers check this
	// before offering email-based flows in the UI.
	Configured() bool
	// Name identifies the provider for logs and the admin panel.
	Name() string
}

// noopSender stands in when nothing is configured, so callers never have to
// nil-check before asking Configured().
type noopSender struct{}

func (noopSender) Send(context.Context, string, string, string) error { return ErrNotConfigured }
func (noopSender) Configured() bool                                   { return false }
func (noopSender) Name() string                                       { return "none" }

// Disabled returns a Sender that reports itself unconfigured.
func Disabled() Sender { return noopSender{} }

var tagRe = regexp.MustCompile(`<[^>]*>`)

// textFromHTML produces a passable plain-text alternative. Multipart mail with
// a text part lands in the inbox measurably more often than HTML alone, and
// generating one is cheaper than asking every caller to write it twice.
func textFromHTML(html string) string {
	s := strings.ReplaceAll(html, "</p>", "\n\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = tagRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")

	lines := []string{}
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			lines = append(lines, t)
		}
	}
	return strings.Join(lines, "\n")
}

// FormatFrom builds an RFC 5322 From header with a display name, which is what
// mail clients show in the message list — a bare address leaves them showing
// the local part ("noreply"), which reads like a machine rather than the site.
//
// Non-ASCII names are RFC 2047 encoded; mime.QEncoding leaves plain ASCII
// untouched, so "Openimg" stays literal and a Chinese name still arrives
// intact.
func FormatFrom(name, addr string) string {
	name = strings.TrimSpace(name)
	addr = strings.TrimSpace(addr)
	if name == "" || addr == "" {
		return addr
	}
	// Already a full "Name <addr>" — leave whatever the operator configured.
	if strings.Contains(addr, "<") {
		return addr
	}
	return mime.QEncoding.Encode("utf-8", name) + " <" + addr + ">"
}
