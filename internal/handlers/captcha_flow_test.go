package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"lopezpress/internal/captcha"
	"lopezpress/internal/services"
)

// enableCaptcha installs a captcha store for the duration of one test. The
// handlers package reads a process-wide store because production wires it once
// at startup, so the test has to put it back — otherwise every later login
// test in this package would suddenly demand a picture.
func enableCaptcha(t *testing.T) *captcha.Store {
	t.Helper()
	st := captcha.New(2*time.Minute, 3)
	captcha.Init(st)
	t.Cleanup(func() { captcha.Init(nil) })
	return st
}

// seedLoginUser creates a user whose password is known to the test. setup()
// is what installs the package-level svc the Login handler calls.
func seedLoginUser(t *testing.T) {
	t.Helper()
	s, _ := setup(t)
	if _, err := s.Register(context.Background(), services.RegisterInput{
		Username: "alice", Email: "alice@example.com", Password: "secret123",
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

// loginRequest builds a login POST carrying the given captcha fields.
func loginRequest(email, password, captchaID, captchaCode string, captchaX int) *http.Request {
	body := fmt.Sprintf(
		`{"email":%q,"password":%q,"captcha_id":%q,"captcha_code":%q,"captcha_x":%d}`,
		email, password, captchaID, captchaCode, captchaX,
	)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:5555"
	return req
}

// TestLoginRequiresCaptcha proves the picture is a precondition: without one
// the password is never compared.
func TestLoginRequiresCaptcha(t *testing.T) {
	seedLoginUser(t)
	enableCaptcha(t)

	rec := httptest.NewRecorder()
	Login(rec, loginRequest("alice", "secret123", "", "", 0))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 without a captcha, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Error           string            `json:"error"`
		CaptchaRequired bool              `json:"captcha_required"`
		Captcha         captcha.Challenge `json:"captcha"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.CaptchaRequired {
		t.Fatalf("response must flag captcha_required: %s", rec.Body.String())
	}
	// A rejected attempt hands back a usable challenge, so a human who misread
	// four letters does not have to retype their password to get another one.
	if resp.Captcha.ID == "" {
		t.Fatalf("rejection must carry a fresh challenge: %s", rec.Body.String())
	}
}

// TestLoginWithSolvedCaptchaSucceeds is the happy path: solve, then log in.
// The answer is lower-cased on purpose — users type what they see, and failing
// them over case is hostile.
func TestLoginWithSolvedCaptchaSucceeds(t *testing.T) {
	seedLoginUser(t)
	st := enableCaptcha(t)

	ch, err := st.IssueWithAnswerForTest("203.0.113.9", "ABCD")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	rec := httptest.NewRecorder()
	Login(rec, loginRequest("alice", "secret123", ch.ID, "abcd", 0))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with a solved captcha, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

// TestCaptchaCannotBeReplayed: a solved challenge authorises exactly one
// attempt. Otherwise one picture plus a stolen password allows unlimited
// logins regardless of the limiter.
func TestCaptchaCannotBeReplayed(t *testing.T) {
	seedLoginUser(t)
	st := enableCaptcha(t)

	ch, err := st.IssueWithAnswerForTest("203.0.113.9", "ABCD")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	first := httptest.NewRecorder()
	Login(first, loginRequest("alice", "wrong-password", ch.ID, "ABCD", 0))
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("first attempt should reach the password check (401), got %d", first.Code)
	}

	second := httptest.NewRecorder()
	Login(second, loginRequest("alice", "secret123", ch.ID, "ABCD", 0))
	if second.Code != http.StatusBadRequest {
		t.Fatalf("replayed challenge must be refused, got %d (body=%s)", second.Code, second.Body.String())
	}
}

// TestWrongPasswordEscalatesToSlide: repeated wrong passwords move this client
// from the four-character image to the sliding puzzle.
func TestWrongPasswordEscalatesToSlide(t *testing.T) {
	seedLoginUser(t)
	st := enableCaptcha(t)

	for i := 0; i < 3; i++ {
		ch, err := st.IssueWithAnswerForTest("203.0.113.9", "ABCD")
		if err != nil {
			t.Fatalf("issue %d: %v", i, err)
		}
		rec := httptest.NewRecorder()
		Login(rec, loginRequest("alice", "definitely-wrong", ch.ID, "ABCD", 0))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i+1, rec.Code)
		}
	}

	next, err := st.Issue("203.0.113.9")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if next.Kind != captcha.KindSlide {
		t.Fatalf("after three wrong passwords the next challenge must be a slide, got %q", next.Kind)
	}
	if next.Bg == "" || next.Tile == "" {
		t.Fatalf("slide challenge incomplete: %+v", next)
	}
}

// TestSuccessfulLoginResetsEscalation: getting in clears the streak, so the
// next login is back to the cheap picture.
func TestSuccessfulLoginResetsEscalation(t *testing.T) {
	seedLoginUser(t)
	st := enableCaptcha(t)

	// Three failures already puts the client on the slide tier.
	for i := 0; i < 3; i++ {
		st.NoteFailure("203.0.113.9")
	}
	if got := st.KindFor("203.0.113.9"); got != captcha.KindSlide {
		t.Fatalf("precondition: want slide tier, got %q", got)
	}

	ch, err := st.IssueWithAnswerForTest("203.0.113.9", "WXYZ")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	rec := httptest.NewRecorder()
	Login(rec, loginRequest("alice", "secret123", ch.ID, "WXYZ", 0))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	if got := st.KindFor("203.0.113.9"); got != captcha.KindImage {
		t.Fatalf("successful login must reset the tier, got %q", got)
	}
}

// TestCaptchaIssueEndpointReportsEnabled: the endpoint always answers, so a
// frontend can tell "no captcha" from "captcha failed to load".
func TestCaptchaIssueEndpointReportsEnabled(t *testing.T) {
	_, _ = setup(t)
	enableCaptcha(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/captcha", nil)
	req.RemoteAddr = "203.0.113.9:5555"
	rec := httptest.NewRecorder()
	IssueCaptcha(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	var resp struct {
		Enabled bool               `json:"enabled"`
		Captcha *captcha.Challenge `json:"captcha"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Enabled || resp.Captcha == nil || resp.Captcha.ID == "" {
		t.Fatalf("unexpected payload: %s", rec.Body.String())
	}
	if resp.Captcha.Kind != captcha.KindImage {
		t.Fatalf("clean client must get the image kind, got %q", resp.Captcha.Kind)
	}
	if !strings.HasPrefix(resp.Captcha.Image, "data:image/png;base64,") {
		t.Fatalf("image must be a png data URI, got %.40q", resp.Captcha.Image)
	}
}

// TestCaptchaDisabledLetsOldClientsThrough: with the feature off the login
// behaves exactly as before, which is what a rollback needs.
func TestCaptchaDisabledLetsOldClientsThrough(t *testing.T) {
	seedLoginUser(t)
	enableCaptcha(t)

	prev := cfg.CaptchaEnabled
	cfg.CaptchaEnabled = false
	t.Cleanup(func() { cfg.CaptchaEnabled = prev })

	rec := httptest.NewRecorder()
	Login(rec, loginRequest("alice", "secret123", "", "", 0))
	if rec.Code != http.StatusOK {
		t.Fatalf("captcha off must allow a plain login, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	// And the endpoint advertises the switch so the UI can hide the widget.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/captcha", nil)
	rec2 := httptest.NewRecorder()
	IssueCaptcha(rec2, req)
	if !strings.Contains(rec2.Body.String(), `"enabled":false`) {
		t.Fatalf("disabled endpoint must say so: %s", rec2.Body.String())
	}
}
