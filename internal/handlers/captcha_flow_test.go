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

// testClient is the address every request in this file comes from. It has to
// match on issue and on verify: the store binds a challenge to the client it
// was handed to, so a test that issued from one address and solved from another
// would (correctly) be refused.
const testClient = "203.0.113.9"

// enableCaptcha installs a captcha store for the duration of one test. The
// handlers package reads a process-wide store because production wires it once
// at startup, so the test has to put it back — otherwise every later login
// test in this package would suddenly demand a picture.
func enableCaptcha(t *testing.T) *captcha.Store {
	t.Helper()
	st := captcha.New(2 * time.Minute)
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
		Username: "alice", Email: "alice@example.com", Password: "Zp9!qLm2xR",
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

// loginRequest builds a login POST carrying the given captcha fields.
func loginRequest(email, password, captchaID, captchaCode string) *http.Request {
	body := fmt.Sprintf(
		`{"email":%q,"password":%q,"captcha_id":%q,"captcha_code":%q}`,
		email, password, captchaID, captchaCode,
	)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.RemoteAddr = testClient + ":5555"
	return req
}

// TestLoginRequiresCaptcha proves the picture is a precondition: without one
// the password is never compared.
func TestLoginRequiresCaptcha(t *testing.T) {
	seedLoginUser(t)
	enableCaptcha(t)

	rec := httptest.NewRecorder()
	Login(rec, loginRequest("alice", "Zp9!qLm2xR", "", ""))

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
	if resp.Captcha.ID == "" || resp.Captcha.Image == "" {
		t.Fatalf("rejection must carry a fresh challenge: %s", rec.Body.String())
	}
}

// TestLoginWithSolvedCaptchaSucceeds is the happy path: solve, then log in.
// The answer is lower-cased on purpose — users type what they see, and failing
// them over case is hostile.
func TestLoginWithSolvedCaptchaSucceeds(t *testing.T) {
	seedLoginUser(t)
	st := enableCaptcha(t)

	ch, err := st.IssueWithAnswerForTest(testClient, "ABCD")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	rec := httptest.NewRecorder()
	Login(rec, loginRequest("alice", "Zp9!qLm2xR", ch.ID, "abcd"))
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

	ch, err := st.IssueWithAnswerForTest(testClient, "ABCD")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	first := httptest.NewRecorder()
	Login(first, loginRequest("alice", "wrong-password", ch.ID, "ABCD"))
	if first.Code != http.StatusUnauthorized {
		t.Fatalf("first attempt should reach the password check (401), got %d", first.Code)
	}

	second := httptest.NewRecorder()
	Login(second, loginRequest("alice", "Zp9!qLm2xR", ch.ID, "ABCD"))
	if second.Code != http.StatusBadRequest {
		t.Fatalf("replayed challenge must be refused, got %d (body=%s)", second.Code, second.Body.String())
	}
}

// TestCaptchaSolvedElsewhereIsRefused is the handler-level counterpart of the
// store test: an answer solved on another address cannot be spent here, so a
// captcha farm has to feed the same IP that is doing the guessing.
func TestCaptchaSolvedElsewhereIsRefused(t *testing.T) {
	seedLoginUser(t)
	st := enableCaptcha(t)

	ch, err := st.IssueWithAnswerForTest("198.51.100.7", "ABCD")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	rec := httptest.NewRecorder()
	Login(rec, loginRequest("alice", "Zp9!qLm2xR", ch.ID, "ABCD"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a foreign challenge must be refused, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

// TestWrongPasswordTightensAllowance: repeated wrong passwords cut this
// client's picture allowance, which is what actually slows a guessing loop.
func TestWrongPasswordTightensAllowance(t *testing.T) {
	seedLoginUser(t)
	st := enableCaptcha(t)

	for i := 0; i < 3; i++ {
		ch, err := st.IssueWithAnswerForTest(testClient, "ABCD")
		if err != nil {
			t.Fatalf("issue %d: %v", i, err)
		}
		rec := httptest.NewRecorder()
		Login(rec, loginRequest("alice", "definitely-wrong", ch.ID, "ABCD"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i+1, rec.Code)
		}
	}

	if got := st.FailureCount(testClient); got < 3 {
		t.Fatalf("want at least 3 recorded failures, got %d", got)
	}
}

// TestSuccessfulLoginResetsStreak: getting in clears the streak, so tomorrow's
// typo starts from a clean record.
func TestSuccessfulLoginResetsStreak(t *testing.T) {
	seedLoginUser(t)
	st := enableCaptcha(t)

	for i := 0; i < 3; i++ {
		st.NoteFailure(testClient)
	}

	ch, err := st.IssueWithAnswerForTest(testClient, "WXYZ")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	rec := httptest.NewRecorder()
	Login(rec, loginRequest("alice", "Zp9!qLm2xR", ch.ID, "WXYZ"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	if got := st.FailureCount(testClient); got != 0 {
		t.Fatalf("successful login must reset the streak, got %d", got)
	}
}

// TestCaptchaIssueEndpointReportsEnabled: the endpoint always answers, so a
// frontend can tell "no captcha" from "captcha failed to load".
func TestCaptchaIssueEndpointReportsEnabled(t *testing.T) {
	_, _ = setup(t)
	enableCaptcha(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/captcha", nil)
	req.RemoteAddr = testClient + ":5555"
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
		t.Fatalf("want the image kind, got %q", resp.Captcha.Kind)
	}
	if !strings.HasPrefix(resp.Captcha.Image, "data:image/png;base64,") {
		t.Fatalf("image must be a png data URI, got %.40q", resp.Captcha.Image)
	}
	// The answer must never travel to the browser, and the response must not be
	// cacheable: a shared cache would hand one user's picture to the next.
	if strings.Contains(rec.Body.String(), "answer") {
		t.Fatalf("payload must not mention the answer: %s", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("captcha must not be cacheable, got Cache-Control=%q", got)
	}
}

// TestCaptchaIssueEndpointRateLimits: over the burst the endpoint answers 429
// with Retry-After, not a picture.
func TestCaptchaIssueEndpointRateLimits(t *testing.T) {
	_, _ = setup(t)
	enableCaptcha(t)

	code := http.StatusOK
	for i := 0; i < 40 && code == http.StatusOK; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/auth/captcha", nil)
		req.RemoteAddr = testClient + ":5555"
		rec := httptest.NewRecorder()
		IssueCaptcha(rec, req)
		code = rec.Code
		if code == http.StatusTooManyRequests {
			if rec.Header().Get("Retry-After") == "" {
				t.Fatal("429 must carry Retry-After")
			}
			return
		}
	}
	t.Fatalf("issuing 40 pictures in a row must hit the limiter, last status %d", code)
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
	Login(rec, loginRequest("alice", "Zp9!qLm2xR", "", ""))
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
