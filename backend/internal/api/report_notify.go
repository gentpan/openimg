package api

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/gentpan/openimg/backend/internal/storage"
)

// notifyAdminsOfReport mails the administrators when a report arrives.
//
// A badge in the admin panel only works for someone who is already looking at
// it. Abuse reports are the one thing on this site with a clock attached — the
// gap between "someone reported it" and "an admin saw it" is time the content
// stays up — so they get pushed rather than waiting to be pulled.
//
// Fire-and-forget: a mail provider being slow or down must never turn a
// successful report submission into an error for the reporter.
func (s *Server) notifyAdminsOfReport(rep *models.Report, img *models.Image) {
	if s.Email == nil || !s.Email.Configured() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var admins []models.User
		if err := s.DB.Where("role = ? AND status = ?", models.RoleAdmin, models.UserActive).
			Find(&admins).Error; err != nil || len(admins) == 0 {
			log.Printf("report: no admin to notify: %v", err)
			return
		}

		var profile *models.StorageProfile
		var p models.StorageProfile
		if s.DB.First(&p, "id = ?", img.ProfileID).Error == nil {
			profile = &p
		}
		imageURL := storage.URLFor(profile, img.ObjectKey, s.PublicBaseURL)
		adminURL := storage.JoinURL(s.PublicBaseURL, "admin")

		label := models.ReportCategoryLabel(rep.Category)
		subject := fmt.Sprintf("[Openimg] 新举报：%s", label)
		body := reportEmailHTML(label, rep, img, imageURL, adminURL)

		for _, a := range admins {
			if err := s.Email.Send(ctx, a.Email, subject, body); err != nil {
				log.Printf("report: notifying %s failed: %v", a.Email, err)
			}
		}
	}()
}

// Report categories differ in how fast they need a human. Porn, violence and
// fraud can carry legal exposure and are flagged red; the rest read as amber.
// The point is a colour an admin can triage from the notification list without
// opening anything.
func categoryTone(key string) (bg, fg, border string) {
	switch key {
	case "porn", "violence", "fraud":
		return "#fef2f2", "#b91c1c", "#fecaca"
	default:
		return "#fffbeb", "#b45309", "#fde68a"
	}
}

// reportEmailHTML renders the admin notification.
//
// Structured as a full document with a centred card rather than the bare
// <div> it used to be: mail clients give a loose <div> no width and no
// background, so it arrived as unstyled left-aligned text while every other
// message from us looked like a product. Layout is tables and inline styles
// because Gmail strips <style> blocks and ignores flex.
//
// The image is linked, never embedded — an admin's mail client should not
// auto-render content that was just reported as possibly illegal.
func reportEmailHTML(label string, rep *models.Report, img *models.Image, imageURL, adminURL string) string {
	esc := html.EscapeString
	reason := esc(rep.Reason)
	if reason == "" {
		reason = `<span style="color:#9ca3af">（举报人未填写补充说明）</span>`
	}
	contact := esc(rep.Contact)
	if contact == "" {
		contact = `<span style="color:#9ca3af">未留</span>`
	}
	reporter := "匿名访客"
	if rep.ReporterID != nil {
		reporter = "已登录用户"
	}
	dims := ""
	if img.Width > 0 && img.Height > 0 {
		dims = fmt.Sprintf("%d × %d · ", img.Width, img.Height)
	}
	bg, fg, border := categoryTone(rep.Category)

	return strings.Join([]string{
		`<!DOCTYPE html>`,
		`<html><body style="margin:0;padding:0;background:#f5f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif">`,
		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="padding:32px 16px">`,
		`<tr><td align="center">`,
		`<table role="presentation" width="520" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;padding:32px;max-width:100%;text-align:left">`,
		`<tr><td>`,

		`<h1 style="margin:0 0 4px;color:#111827;font-size:20px">收到新的举报</h1>`,
		`<p style="margin:0 0 20px;color:#6b7280;font-size:13px">` + rep.CreatedAt.Format("2006-01-02 15:04:05") + `</p>`,

		// Category badge — the one thing worth seeing before reading anything.
		`<div style="display:inline-block;background:` + bg + `;border:1px solid ` + border +
			`;border-radius:6px;padding:6px 12px;color:` + fg + `;font-size:14px;font-weight:600;margin-bottom:20px">` +
			esc(label) + `</div>`,

		// The reason is why the mail exists, so it gets the most weight.
		`<div style="background:#f8f9fa;border-left:3px solid #90FF3A;border-radius:0 6px 6px 0;padding:12px 14px;margin-bottom:20px">`,
		`<div style="color:#6b7280;font-size:11px;margin-bottom:4px">举报理由</div>`,
		`<div style="color:#111827;font-size:14px;line-height:1.6">` + reason + `</div>`,
		`</div>`,

		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="font-size:13px;border-collapse:collapse">`,
		mailRow("被举报图片", esc(img.OrigName)),
		mailRow("规格", dims+strings.ToUpper(img.Ext)),
		mailRow("举报人", reporter),
		mailRow("联系方式", contact),
		`</table>`,

		`<div style="background:#f8f9fa;border-radius:6px;padding:12px 14px;margin:20px 0">`,
		`<div style="color:#6b7280;font-size:11px;margin-bottom:6px">图片直链 · 请自行判断是否打开</div>`,
		`<a href="` + esc(imageURL) + `" style="color:#337400;font-size:12px;word-break:break-all;text-decoration:none">` + esc(imageURL) + `</a>`,
		`</div>`,

		`<a href="` + esc(adminURL) + `" style="display:inline-block;background:#90FF3A;color:#0A1B02;` +
			`text-decoration:none;padding:11px 20px;border-radius:8px;font-size:14px;font-weight:500">前往后台处理</a>`,

		`</td></tr>`,
		`</table>`,
		`<p style="margin:16px 0 0;color:#a3a3a3;font-size:12px">Openimg · 举报通知</p>`,
		`</td></tr>`,
		`</table>`,
		`</body></html>`,
	}, "")
}

func mailRow(k, v string) string {
	return `<tr>` +
		`<td style="padding:7px 14px 7px 0;color:#6b7280;white-space:nowrap;vertical-align:top;border-bottom:1px solid #f0f1f3">` + k + `</td>` +
		`<td style="padding:7px 0;color:#111827;vertical-align:top;border-bottom:1px solid #f0f1f3">` + v + `</td>` +
		`</tr>`
}
