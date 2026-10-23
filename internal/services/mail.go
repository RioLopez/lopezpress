package services

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lopezpress/internal/models"
)

// --- security notifications via SMTP ---
//
// This is deliberately a minimal, dependency-free SMTP client (standard
// library net + crypto/tls only) so a low-spec VPS does not pull in a heavy
// mail stack. Messages are plain-text only. All sends run in a detached
// goroutine and swallow errors: a broken SMTP server must never block or
// fail the primary operation (login / password change).
//
// Throttling: login-failure mails are rate-limited per recipient so a
// brute-force attacker cannot use the site to spam the user's inbox.

// MailKind is the notification type; each has its own subject template.
type MailKind string

const (
	MailLoginFailure   MailKind = "login_failure"
	MailPasswordChange MailKind = "password_change"
	MailNewLocation    MailKind = "new_location"
	// MailReviewResult notifies an author that one of their posts was
	// approved (published) or rejected (sent back for revision) by a
	// reviewer. Sent on review outcomes; not throttled (one-shot events).
	MailReviewResult MailKind = "review_result"
)

// Throttle window: at most one login-failure notification per user per 15m.
const loginFailThrottle = 15 * time.Minute

// sendMail is the actual SMTP sender. It is a method on Service so tests can
// override the behaviour without touching real sockets.
type mailFn func(ctx context.Context, to, subject, body string) error

// smtpThrottle records the last login-failure notification time per user.
// Keyed by user ID; values are monotonic send timestamps.
type mailThrottler struct {
	last map[string]time.Time
}

func newMailThrottler() *mailThrottler {
	return &mailThrottler{last: make(map[string]time.Time)}
}

// allow reports whether a login-failure mail for user id should be sent now.
func (t *mailThrottler) allow(id string) bool {
	now := time.Now()
	if last, ok := t.last[id]; ok && now.Sub(last) < loginFailThrottle {
		return false
	}
	t.last[id] = now
	// Opportunistic trim: drop entries older than the throttle window so the
	// map cannot grow unbounded on long-running instances.
	for k, ts := range t.last {
		if now.Sub(ts) >= loginFailThrottle {
			delete(t.last, k)
		}
	}
	return true
}

// smtpConfig is the effective SMTP settings for one send. An empty host means
// the feature is disabled and no mail is sent.
type smtpConfig struct {
	Host string
	Port int
	User string
	Pass string
}

func (c smtpConfig) enabled() bool { return strings.TrimSpace(c.Host) != "" }

func (c smtpConfig) addr() string {
	port := c.Port
	if port <= 0 {
		port = 587
	}
	return net.JoinHostPort(c.Host, strconv.Itoa(port))
}

// SendMail dispatches a notification of the given kind to user.Email. It is
// safe to call concurrently; it returns immediately after spawning the send
// goroutine (or synchronously returns when SMTP is not configured / the
// recipient has no address / the throttle blocks it).
//
// kind selects the subject/body. extra fields are formatted into the body.
func (s *Service) SendMail(ctx context.Context, user models.User, kind MailKind, extras map[string]string) {
	if user.Email == "" {
		return
	}

	// Load SMTP config fresh so admin edits take effect without restart.
	cfg, err := s.GetSiteConfig(ctx, false)
	if err != nil {
		return
	}
	sm := smtpConfig{Host: cfg.SmtpHost, Port: cfg.SmtpPort, User: cfg.SmtpUser, Pass: cfg.SmtpPass}
	if !sm.enabled() {
		return
	}

	subject, body := composeMail(kind, user, extras)

	// Throttle login-failure mails only; the others are one-shot security
	// events and should always be delivered when SMTP is up.
	if kind == MailLoginFailure {
		if !s.mailT.allow(user.ID) {
			return
		}
	}

	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.sendMailFn(sendCtx, user.Email, subject, body); err != nil {
			// best-effort: log but never surface to the caller
			_ = err
		}
	}()
}

// sendMailFn is the SMTP transport. Defaults to a real socket send; tests may
// replace it with a stub.
func (s *Service) sendMailFn(ctx context.Context, to, subject, body string) error {
	cfg, err := s.GetSiteConfig(ctx, false)
	if err != nil {
		return fmt.Errorf("load smtp config: %w", err)
	}
	sm := smtpConfig{Host: cfg.SmtpHost, Port: cfg.SmtpPort, User: cfg.SmtpUser, Pass: cfg.SmtpPass}
	if !sm.enabled() {
		return nil // feature off; treat as success
	}
	from := cfg.SmtpUser
	if from == "" {
		from = "echo@" + sm.Host
	}
	return smtpSend(ctx, sm, from, to, subject, body)
}

// SendTestMail synchronously delivers a test message to the admin's own email
// address so the operator can verify SMTP settings. It returns an error (unlike
// the notification sends, which swallow failures) so the admin UI can show a
// success/failure toast. When SMTP is not configured it returns a clear error
// rather than silently succeeding, because a "test" that does nothing is
// misleading.
func (s *Service) SendTestMail(ctx context.Context, actor models.User) error {
	if actor.Email == "" {
		return newErr(http.StatusBadRequest, "your account has no email address to test")
	}
	cfg, err := s.GetSiteConfig(ctx, false)
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to load site config")
	}
	sm := smtpConfig{Host: cfg.SmtpHost, Port: cfg.SmtpPort, User: cfg.SmtpUser, Pass: cfg.SmtpPass}
	if !sm.enabled() {
		return newErr(http.StatusBadRequest, "SMTP is not configured (host is empty)")
	}
	from := cfg.SmtpUser
	if from == "" {
		from = "echo@" + sm.Host
	}
	subject := "[Echo] 测试邮件 / Test message"
	body := "This is a test message from the Echo admin panel.\n" +
		"If you received it, your SMTP settings are correct.\n"
	if err := smtpSend(ctx, sm, from, actor.Email, subject, body); err != nil {
		return newErr(http.StatusBadGateway, "smtp send failed: "+err.Error())
	}
	s.audit(ctx, actor, "send_test_mail", "mail:"+actor.Email, "")
	return nil
}

// composeMail renders the subject + plain-text body for a notification kind.
func composeMail(kind MailKind, user models.User, extras map[string]string) (string, string) {
	get := func(k string) string {
		if v, ok := extras[k]; ok {
			return v
		}
		return ""
	}
	switch kind {
	case MailLoginFailure:
		subject := "[Security] 登录失败提醒 / Failed login attempt"
		lines := []string{
			"Your account had a failed login attempt.",
			"",
			"  IP:     " + get("ip"),
			"  Time:   " + get("time"),
			"  Browser:" + get("ua"),
			"",
			"If this was not you, change your password and log out of all devices.",
		}
		return subject, strings.Join(lines, "\n")
	case MailPasswordChange:
		subject := "[Security] 密码已更改 / Password changed"
		lines := []string{
			"Your account password was changed.",
			"",
			"  IP:     " + get("ip"),
			"  Time:   " + get("time"),
			"",
			"If this was not you, contact the site administrator immediately.",
		}
		return subject, strings.Join(lines, "\n")
	case MailNewLocation:
		subject := "[Security] 新设备登录提醒 / Login from new location"
		lines := []string{
			"Your account signed in from a new location.",
			"",
			"  New IP:    " + get("ip"),
			"  Previous:  " + get("prev_ip"),
			"  Time:      " + get("time"),
			"  Browser:   " + get("ua"),
			"",
			"If this was not you, review your login history and change your password.",
		}
		return subject, strings.Join(lines, "\n")
	case MailReviewResult:
		if get("action") == "reject" {
			subject := "[Echo] 文章被退回修改 / Post sent back for revision"
			lines := []string{
				"Your post has been sent back by a reviewer.",
				"",
				"  Title:   " + get("title"),
				"  Note:    " + get("note"),
				"  Time:    " + get("time"),
				"",
				"Open your drafts, revise the content, and submit it again.",
			}
			return subject, strings.Join(lines, "\n")
		}
		subject := "[Echo] 文章已通过审核 / Post approved"
		lines := []string{
			"Your post has been approved and is now published.",
			"",
			"  Title: " + get("title"),
			"  Time:  " + get("time"),
		}
		return subject, strings.Join(lines, "\n")
	}
	return "Echo", ""
}

// smtpSend opens an SMTP connection, optionally upgrades to TLS, authenticates
// (when a user is configured), and sends a plain-text message. It implements
// just enough of RFC 5321 for common providers (port 587 STARTTLS / 465
// implicit TLS / 25 plain).
