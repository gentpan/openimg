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

func reportEmailHTML(label string, rep *models.Report, img *models.Image, imageURL, adminURL string) string {
	esc := html.EscapeString
	reason := esc(rep.Reason)
	if reason == "" {
		reason = "（无补充说明）"
	}
	contact := esc(rep.Contact)
	if contact == "" {
		contact = "未留"
	}
	reporter := "匿名访客"
	if rep.ReporterID != nil {
		reporter = "已登录用户"
	}

	// The image is linked, not embedded: an admin's mail client should not
	// render content that was just reported as possibly illegal.
	return strings.Join([]string{
		`<div style="font-family:system-ui,-apple-system,'Segoe UI',sans-serif;max-width:520px;color:#111827">`,
		`<h2 style="font-size:16px;margin:0 0 12px">收到新的举报</h2>`,
		`<table style="width:100%;border-collapse:collapse;font-size:13px">`,
		mailRow("类型", `<strong>`+esc(label)+`</strong>`),
		mailRow("说明", reason),
		mailRow("图片", esc(img.OrigName)),
		mailRow("举报人", reporter+"，联系方式："+contact),
		mailRow("时间", rep.CreatedAt.Format("2006-01-02 15:04:05")),
		`</table>`,
		`<p style="font-size:12px;color:#6b7280;margin:14px 0 6px">图片链接（请自行判断是否打开）：</p>`,
		`<p style="font-size:12px;word-break:break-all;margin:0 0 16px"><a href="` + esc(imageURL) + `">` + esc(imageURL) + `</a></p>`,
		`<a href="` + esc(adminURL) + `" style="display:inline-block;background:#7c2ee0;color:#fff;text-decoration:none;padding:9px 16px;border-radius:8px;font-size:13px">前往后台处理</a>`,
		`<p style="font-size:11px;color:#9ca3af;margin-top:18px">Openimg 举报通知</p>`,
		`</div>`,
	}, "")
}

func mailRow(k, v string) string {
	return `<tr><td style="padding:5px 12px 5px 0;color:#6b7280;white-space:nowrap;vertical-align:top">` +
		k + `</td><td style="padding:5px 0">` + v + `</td></tr>`
}
