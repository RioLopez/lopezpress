package services

// Code-mail composition lives here, next to the notification templates in
// mail.go, so the verification mechanics file stays about codes.

import (
	"context"
	"strings"
	"time"

	"lopezpress/internal/models"
)

// sendCodeMail composes and delivers one code mail synchronously, so a broken
// SMTP server becomes a visible error instead of a silent void.
func (s *Service) sendCodeMail(ctx context.Context, email, purpose, code string, ttl time.Duration) error {
	cfg, err := s.GetSiteConfig(ctx, false)
	if err != nil {
		return err
	}
	from := cfg.SmtpUser
	sm := smtpConfig{Host: cfg.SmtpHost, Port: cfg.SmtpPort, User: cfg.SmtpUser, Pass: cfg.SmtpPass}
	if from == "" {
		from = "echo@" + sm.Host
	}
	subject, body := composeCodeMail(cfg.SiteName, purpose, code, ttl)
	sendCtx, cancel := context.WithTimeout(ctx, emailCodeSendTimeout)
	defer cancel()
	return smtpSend(sendCtx, sm, from, email, subject, body)
}

// composeCodeMail is the message template. Kept plain text, matching the rest
// of the mail stack (no HTML part, no tracking pixel).
func composeCodeMail(siteName, purpose, code string, ttl time.Duration) (string, string) {
	if strings.TrimSpace(siteName) == "" {
		siteName = "Echo"
	}
	mins := int(ttl.Minutes() + 0.5)
	if mins < 1 {
		mins = 1
	}
	what := "验证邮箱 / verify your email address"
	switch purpose {
	case models.EmailPurposeReset:
		what = "重置密码 / reset your password"
	case models.EmailPurposeChangeMail:
		what = "更换邮箱 / confirm your new email address"
	}
	subject := "[" + siteName + "] 验证码 " + code + " — " + what
	lines := []string{
		"你在 " + siteName + " 上请求了「" + what + "」。",
		"",
		"    验证码 / Code: " + code,
		"",
		"有效期 " + itoa(mins) + " 分钟。如果不是你本人操作，请忽略这封邮件。",
		"",
		"You requested to " + what + " on " + siteName + ".",
		"The code expires in " + itoa(mins) + " minutes.",
		"If this was not you, ignore this message — no change has been made.",
		"",
		"-- " + siteName,
	}
	return subject, strings.Join(lines, "\n")
}
