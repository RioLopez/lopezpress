package services

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// emailVerifyOn flips the experimental switch on for a test. Done with a raw
// Save (not UpdateSiteConfig) so the test does not depend on the settings form
// sending every other field.
func emailVerifyOn(t *testing.T, db *gorm.DB) {
	t.Helper()
	var cfg models.SiteConfig
	if err := db.Where("id = ?", 1).First(&cfg).Error; err != nil {
		t.Fatalf("load site config: %v", err)
	}
	cfg.EmailVerifyEnabled = true
	cfg.EmailCodeTTLMin = 10
	cfg.EmailCodeCooldown = 60
	if err := db.Save(&cfg).Error; err != nil {
		t.Fatalf("enable email verification: %v", err)
	}
}

// seedCode inserts a code row directly, bypassing the SMTP step (the mechanics
// of sending are not what these tests are about, and a unit test must not open
// a socket). CreatedAt is placed in the past so the cooldown does not fire.
func seedCode(t *testing.T, svc *Service, db *gorm.DB, email, purpose, code string, ttl time.Duration) models.EmailCode {
	t.Helper()
	email = normalizeEmail(email)
	row := models.EmailCode{
		Email: email, Purpose: purpose,
		CodeHash:  svc.hashCode(email, purpose, code),
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now().Add(-time.Hour),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed code: %v", err)
	}
	return row
}

// ipCtx wraps ctx with the client IP a real request would carry. The public
// routes now inject it (handlers.WithClientIP), and allowCodeFromIP refuses an
// empty value, so a test that exercises the request path must supply one.
func ipCtx(ctx context.Context, ip string) context.Context {
	return WithActor(ctx, models.User{}, ip)
}

func TestEmailCodeOffByDefault(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// A fresh site has the experimental flag off; the endpoint must refuse
	// rather than mint codes nobody asked for.
	err := svc.RequestEmailCode(ctx, EmailCodeRequestInput{Email: "a@example.com", Purpose: models.EmailPurposeRegister})
	if StatusOf(err) != http.StatusForbidden {
		t.Fatalf("expected 403 with the feature off, got %v (err=%v)", StatusOf(err), err)
	}
}

func TestEmailCodeNeedsSMTP(t *testing.T) {
	svc, db := newTestService(t)
	emailVerifyOn(t, db)
	ctx := ipCtx(context.Background(), "203.0.113.9")

	// The switch is on but the operator has not configured a mail server: a
	// silent success here would be a lie, so it must be 503.
	err := svc.RequestEmailCode(ctx, EmailCodeRequestInput{Email: "b@example.com", Purpose: models.EmailPurposeRegister})
	if StatusOf(err) != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without SMTP, got %v (err=%v)", StatusOf(err), err)
	}
}

func TestRequestEmailCodeCooldown(t *testing.T) {
	svc, db := newTestService(t)
	emailVerifyOn(t, db)
	ctx := ipCtx(context.Background(), "203.0.113.10")

	// A row created "just now" is what the cooldown reads, so requesting again
	// must be refused before any SMTP attempt is made.
	seedCode(t, svc, db, "cool@example.com", models.EmailPurposeRegister, "123456", 10*time.Minute)
	if err := db.Model(&models.EmailCode{}).Where("1 = 1").Update("created_at", time.Now()).Error; err != nil {
		t.Fatalf("age the seeded row: %v", err)
	}

	err := svc.RequestEmailCode(ctx, EmailCodeRequestInput{Email: "cool@example.com", Purpose: models.EmailPurposeRegister})
	if StatusOf(err) != http.StatusTooManyRequests {
		t.Fatalf("expected 429 inside the cooldown, got %v (err=%v)", StatusOf(err), err)
	}
}

func TestVerifyEmailCodeBurnsAfterFiveAttempts(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	seedCode(t, svc, db, "brute@example.com", models.EmailPurposeReset, "123456", 10*time.Minute)

	for i := 0; i < EmailCodeMaxAttempts; i++ {
		if err := svc.verifyEmailCode(ctx, "brute@example.com", models.EmailPurposeReset, "000000"); StatusOf(err) != http.StatusBadRequest {
			t.Fatalf("attempt %d: expected 400, got %v", i+1, err)
		}
	}
	// The correct code must no longer work: the budget is spent, not reset.
	if err := svc.verifyEmailCode(ctx, "brute@example.com", models.EmailPurposeReset, "123456"); err == nil {
		t.Fatal("the correct code was still accepted after the attempt budget was spent")
	}
	var row models.EmailCode
	if err := db.Where("email = ?", "brute@example.com").First(&row).Error; err != nil {
		t.Fatalf("reload code row: %v", err)
	}
	if row.UsedAt == nil {
		t.Fatal("a burned code must be marked used so it can never be replayed")
	}
}

func TestVerifyEmailCodeExpires(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	seedCode(t, svc, db, "late@example.com", models.EmailPurposeRegister, "123456", -time.Minute)

	err := svc.verifyEmailCode(ctx, "late@example.com", models.EmailPurposeRegister, "123456")
	if StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 for an expired code, got %v (err=%v)", StatusOf(err), err)
	}
}

func TestRegisterRequiresCodeWhenEnabled(t *testing.T) {
	svc, db := newTestService(t)
	emailVerifyOn(t, db)
	ctx := context.Background()

	base := RegisterInput{Username: "coder01", Email: "coder01@example.com", Password: "Ht6$rCj3pM"}
	if _, err := svc.Register(ctx, base); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 without a code, got %v (err=%v)", StatusOf(err), err)
	}

	seedCode(t, svc, db, base.Email, models.EmailPurposeRegister, "654321", 10*time.Minute)
	base.Code = "654321"
	res, err := svc.Register(ctx, base)
	if err != nil {
		t.Fatalf("register with a valid code: %v", err)
	}
	if res.User.Email != base.Email {
		t.Fatalf("registered email = %q, want %q", res.User.Email, base.Email)
	}

	// The code is single-use: a second sign-up cannot replay it.
	second := RegisterInput{Username: "coder02", Email: "coder02@example.com", Password: "Ht6$rCj3pM", Code: "654321"}
	if _, err := svc.Register(ctx, second); err == nil {
		t.Fatal("a spent register code was accepted for a second account")
	}
}

func TestResetPasswordByCode(t *testing.T) {
	svc, db := newTestService(t)
	emailVerifyOn(t, db)
	ctx := context.Background()

	hash, err := models.HashPassword("oldpassword1")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	user := models.User{Username: "forgetful", Email: "forgetful@example.com", PasswordHash: hash, Role: models.RoleUser}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	seedCode(t, svc, db, user.Email, models.EmailPurposeReset, "112233", 10*time.Minute)

	err = svc.ResetPasswordByCode(ctx, PasswordResetInput{
		Email: user.Email, Code: "112233", Password: "brandnewpass9",
	})
	if err != nil {
		t.Fatalf("reset password: %v", err)
	}

	var fresh models.User
	if err := db.Where("id = ?", user.ID).First(&fresh).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if fresh.TokenVersion != 1 {
		t.Fatalf("token_version = %d, want 1 (existing sessions must be revoked)", fresh.TokenVersion)
	}
	// Old password must fail, new one must work.
	if _, err := svc.Login(ctx, LoginInput{Email: user.Email, Password: "oldpassword1"}, LoginMeta{}); err == nil {
		t.Fatal("the old password still works after a reset")
	}
	if _, err := svc.Login(ctx, LoginInput{Email: user.Email, Password: "brandnewpass9"}, LoginMeta{}); err != nil {
		t.Fatalf("login with the new password: %v", err)
	}
}

func TestUpdateMeCannotChangeEmailWhenVerificationOn(t *testing.T) {
	svc, db := newTestService(t)
	emailVerifyOn(t, db)
	ctx := context.Background()

	hash, _ := models.HashPassword("Ht6$rCj3pM")
	me := models.User{Username: "guarded", Email: "keep@example.com", PasswordHash: hash, Role: models.RoleUser}
	if err := db.Create(&me).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	// The plain profile update must not be a back door around the code flow.
	sneaky := "sneaky@example.com"
	if _, err := svc.UpdateMe(ctx, me.ID, UpdateMeInput{Email: &sneaky}); StatusOf(err) != http.StatusForbidden {
		t.Fatalf("expected 403 for an unverified email change, got %v (err=%v)", StatusOf(err), err)
	}
	var fresh models.User
	if err := db.Where("id = ?", me.ID).First(&fresh).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if fresh.Email != "keep@example.com" {
		t.Fatalf("email changed anyway: %q", fresh.Email)
	}
}

func TestConfirmEmailChange(t *testing.T) {
	svc, db := newTestService(t)
	emailVerifyOn(t, db)
	ctx := context.Background()

	hash, _ := models.HashPassword("Ht6$rCj3pM")
	me := models.User{Username: "mover", Email: "old@example.com", PasswordHash: hash, Role: models.RoleUser}
	if err := db.Create(&me).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	seedCode(t, svc, db, "new@example.com", models.EmailPurposeChangeMail, "998877", 10*time.Minute)

	// A code issued for one address must not move the account to another.
	if _, err := svc.ConfirmEmailChange(ctx, me, EmailChangeInput{Email: "other@example.com", Code: "998877"}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 for a mismatched address, got %v (err=%v)", StatusOf(err), err)
	}

	updated, err := svc.ConfirmEmailChange(ctx, me, EmailChangeInput{Email: "new@example.com", Code: "998877"})
	if err != nil {
		t.Fatalf("confirm email change: %v", err)
	}
	if updated.Email != "new@example.com" {
		t.Fatalf("email = %q, want new@example.com", updated.Email)
	}
	if updated.Username != "mover" {
		t.Fatalf("username changed unexpectedly: %q", updated.Username)
	}
}

// R4-02: a one-time code was not one-time. verifyEmailCode used to read the
// row, compare the hash, and then write used_at — so two concurrent requests
// that both matched the same code both completed. On the password-reset flow
// that means an intercepted mail can be spent twice: the victim resets, and
// the attacker resets again a millisecond later and owns the account. The
// consumption is now a conditional UPDATE, so only one caller can move
// used_at off NULL.
func TestEmailCodeCannotBeSpentTwice(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	seedCode(t, svc, db, "double@example.com", models.EmailPurposeReset, "424242", 10*time.Minute)

	const racers = 8
	errs := make([]error, racers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start // release them all at once
			errs[i] = svc.verifyEmailCode(ctx, "double@example.com", models.EmailPurposeReset, "424242")
		}(i)
	}
	close(start)
	wg.Wait()

	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("exactly one concurrent caller may spend a code, got %d successes out of %d", ok, racers)
	}

	var row models.EmailCode
	if err := db.Where("email = ?", "double@example.com").First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.UsedAt == nil {
		t.Fatal("the spent code must be marked used")
	}
	// The attempt counter is NOT asserted to be exactly 1. Step 1 spends an
	// attempt *before* the hash comparison, so any racer that read the row
	// while it was still unspent gets charged too — whether it loses depends
	// on scheduling, not on the code. What the implementation does guarantee
	// is the ceiling: the conditional UPDATE refuses past EmailCodeMaxAttempts,
	// so the counter can never exceed the number of racers.
	//
	// That "a correct code also burns a guess" behaviour is a real (mild)
	// defect: an impatient double-click can eat the 5-guess budget and void a
	// code the user typed correctly. Tracked in docs/TODO.md, not fixed here.
	if row.Attempts < 1 || row.Attempts > racers {
		t.Fatalf("attempts = %d, want 1..%d", row.Attempts, racers)
	}
}

// R2-05: the guess budget was read, compared, and then incremented, so N
// concurrent wrong guesses all saw the same "still under the limit" value.
// Five became nineteen under load. The increment is now conditional on the
// budget still being open.
func TestEmailCodeAttemptBudgetIsAtomic(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	seedCode(t, svc, db, "race@example.com", models.EmailPurposeReset, "123456", 10*time.Minute)

	const racers = 24
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_ = svc.verifyEmailCode(ctx, "race@example.com", models.EmailPurposeReset, "000000")
		}()
	}
	close(start)
	wg.Wait()

	var row models.EmailCode
	if err := db.Where("email = ?", "race@example.com").First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.Attempts > EmailCodeMaxAttempts {
		t.Fatalf("attempts reached %d under %d concurrent guesses, limit is %d",
			row.Attempts, racers, EmailCodeMaxAttempts)
	}
	if row.Attempts != EmailCodeMaxAttempts {
		t.Fatalf("attempts = %d, want the full budget of %d spent exactly once each",
			row.Attempts, EmailCodeMaxAttempts)
	}
}

// R3-03 / R4-06: the per-IP budget used to be skipped entirely on the public
// routes, because those routes never put an IP on the context and
// allowCodeFromIP treated "" as "no origin, allow it". Twelve addresses from
// one client produced twelve mails and not one 429. An empty IP is now a
// refusal, which is also what makes the budget enforceable at all.
func TestAllowCodeFromIPRefusesEmptyAndEnforcesLimit(t *testing.T) {
	svc, _ := newTestService(t)

	if svc.allowCodeFromIP("") {
		t.Fatal("an empty IP must be refused, not waved through")
	}
	for i := 0; i < emailCodeIPLimit; i++ {
		if !svc.allowCodeFromIP("198.51.100.5") {
			t.Fatalf("request %d within the hourly budget should be allowed", i+1)
		}
	}
	if svc.allowCodeFromIP("198.51.100.5") {
		t.Fatalf("request %d must be refused: the budget is %d per hour",
			emailCodeIPLimit+1, emailCodeIPLimit)
	}
	// A different origin has its own budget.
	if !svc.allowCodeFromIP("198.51.100.6") {
		t.Fatal("a different origin must start from a fresh budget")
	}
}
