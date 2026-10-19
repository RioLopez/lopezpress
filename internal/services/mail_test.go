package services

import (
	"context"
	"testing"
	"time"

	"lopezpress/internal/models"
)

// --- SendTestMail error paths (no real socket; SMTP not configured) ---

func TestSendTestMailNoEmail(t *testing.T) {
	s, _ := newTestService(t)
	if err := s.SendTestMail(context.Background(), models.User{}); err == nil {
		t.Fatal("expected error when user has no email")
	}
}

func TestSendTestMailNotConfigured(t *testing.T) {
	s, _ := newTestService(t)
	// Fresh DB: site config exists but smtp_host is empty -> the
	// "SMTP is not configured" branch must surface a 400.
	err := s.SendTestMail(context.Background(), models.User{Email: "a@b.c"})
	if err == nil {
		t.Fatal("expected an error when SMTP host is empty")
	}
}

// --- throttle guard: two rapid login-failure sends, only first passes ---

func TestSendMailThrottleBlocksSecond(t *testing.T) {
	tr := newMailThrottler()
	if !tr.allow("u") {
		t.Fatal("first should pass")
	}
	if tr.allow("u") {
		t.Fatal("second within window should be blocked")
	}
}

// --- composeMail ---

func TestComposeMailLoginFailure(t *testing.T) {
	user := models.User{ID: "u1", Email: "a@b.c"}
	subj, body := composeMail(MailLoginFailure, user, map[string]string{
		"ip": "1.2.3.4", "time": "2026-10-14 12:00 UTC", "ua": "Edge",
	})
	if subj == "" {
		t.Fatalf("subject empty")
	}
	for _, want := range []string{"1.2.3.4", "Edge", "2026-10-14 12:00 UTC"} {
		if !contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

func TestComposeMailPasswordChange(t *testing.T) {
	_, body := composeMail(MailPasswordChange, models.User{}, map[string]string{"time": "now"})
	if !contains(body, "now") {
		t.Errorf("body missing time:\n%s", body)
	}
}

func TestComposeMailNewLocation(t *testing.T) {
	_, body := composeMail(MailNewLocation, models.User{}, map[string]string{
		"ip": "new", "prev_ip": "old", "time": "now",
	})
	if !contains(body, "new") || !contains(body, "old") {
		t.Errorf("body missing ip fields:\n%s", body)
	}
}

// --- b64 ---

func TestB64(t *testing.T) {
	cases := map[string]string{
		"":    "",
		"a":   "YQ==",
		"ab":  "YWI=",
		"abc": "YWJj",
		"xyz": "eHl6",
	}
	for in, want := range cases {
		if got := b64(in); got != want {
			t.Errorf("b64(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- hostnameFor ---

func TestHostnameFor(t *testing.T) {
	if got := hostnameFor("smtp.example.com"); got != "smtp.example.com" {
		t.Errorf("hostnameFor plain = %q", got)
	}
	if got := hostnameFor("smtp.example.com:587"); got != "smtp.example.com" {
		t.Errorf("hostnameFor with port = %q", got)
	}
	if got := hostnameFor(""); got != "localhost" {
		t.Errorf("hostnameFor empty = %q", got)
	}
}

// --- smtpConfig ---

func TestSmtpConfigAddr(t *testing.T) {
	cases := []struct {
		cfg  smtpConfig
		want string
	}{
		{smtpConfig{Host: "h", Port: 587}, "h:587"},
		{smtpConfig{Host: "h", Port: 0}, "h:587"}, // default port
		{smtpConfig{Host: "h", Port: 465}, "h:465"},
	}
	for i, c := range cases {
		if got := c.cfg.addr(); got != c.want {
			t.Errorf("case %d: addr() = %q, want %q", i, got, c.want)
		}
	}
}

func TestSmtpConfigEnabled(t *testing.T) {
	if (smtpConfig{Host: ""}).enabled() {
		t.Error("empty host should be disabled")
	}
	if (smtpConfig{Host: " "}).enabled() {
		t.Error("whitespace host should be disabled")
	}
	if !(smtpConfig{Host: "h"}.enabled()) {
		t.Error("host should be enabled")
	}
}

// --- mailThrottler ---

func TestMailThrottler(t *testing.T) {
	tr := newMailThrottler()
	if !tr.allow("u1") {
		t.Error("first send should be allowed")
	}
	// Within the throttle window, a second send is blocked.
	if tr.allow("u1") {
		t.Error("immediate second send should be throttled")
	}
	// A different user is not affected.
	if !tr.allow("u2") {
		t.Error("different user should be allowed")
	}
}

func TestMailThrottlerExpiration(t *testing.T) {
	tr := newMailThrottler()
	tr.last["u1"] = time.Now().Add(-20 * time.Minute) // older than the 15m window
	if !tr.allow("u1") {
		t.Error("expired entry should allow again")
	}
}
