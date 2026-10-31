package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"lopezpress/internal/models"
)

// Login must not answer faster for a username that does not exist than for one
// with a wrong password.
//
// Both paths return the same 401 body, so the response *content* was already
// safe. The clock was not: "no such user" used to return in about a
// microsecond while "wrong password" spent ~60 ms inside bcrypt. Timing 200
// requests therefore produced a list of valid usernames for free, which is
// exactly what an attacker needs to spend the five-attempt lockout budget
// wisely instead of blindly.
//
// The assertion is a floor, not equality: wall-clock equality on a shared CI
// box is not something a test can promise. bcrypt at DefaultCost takes tens of
// milliseconds, so anything above a few milliseconds proves a real comparison
// ran. A regression that removes the dummy comparison lands near 0 ms and fails
// this immediately.
func TestLoginTimingDoesNotRevealAccountExistence(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", models.RoleUser)

	// Warm the bcrypt path once so neither measurement pays first-call cost.
	_, _ = svc.Login(ctx, LoginInput{Email: alice.Email, Password: "wrong-password"}, LoginMeta{})

	const floor = 3 * time.Millisecond

	start := time.Now()
	_, err := svc.Login(ctx, LoginInput{Email: "nobody@example.com", Password: "whatever1"}, LoginMeta{})
	missing := time.Since(start)
	if err == nil {
		t.Fatalf("login for an unknown account must fail")
	}
	if StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("unknown account must answer 401, got %d", StatusOf(err))
	}
	if missing < floor {
		t.Fatalf("unknown-account login returned in %v: no password comparison ran, so the "+
			"response time reveals whether the account exists", missing)
	}

	start = time.Now()
	_, err = svc.Login(ctx, LoginInput{Email: alice.Email, Password: "wrong-password"}, LoginMeta{})
	wrongPw := time.Since(start)
	if err == nil {
		t.Fatalf("wrong password must fail")
	}
	if wrongPw < floor {
		t.Fatalf("wrong-password login returned in %v, unexpectedly cheap", wrongPw)
	}
}

// A suspended account must not confirm its own existence to a caller who has
// not proved ownership of it.
//
// The old order checked Disabled before the password, so anyone who typed a
// suspended user's address got 403 "account is disabled" while everyone else
// got 401 — an existence oracle that costs no password attempt and never
// touches the lockout counter. Ownership is what earns the explanation.
func TestDisabledAccountIsNotDisclosedWithoutThePassword(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", models.RoleUser)
	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	if _, err := svc.SetUserDisabled(ctx, root, alice.ID, true); err != nil {
		t.Fatalf("disable: %v", err)
	}

	// Wrong password on a suspended account: must look like any other failure.
	_, err := svc.Login(ctx, LoginInput{Email: alice.Email, Password: "not-her-password"}, LoginMeta{})
	if err == nil {
		t.Fatalf("login on a disabled account must fail")
	}
	if StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("wrong password on a disabled account must answer 401 like any other, got %d (%s)",
			StatusOf(err), MsgOf(err))
	}

	// The real owner, with the real password, is told the truth.
	if _, err := svc.Login(ctx, LoginInput{Email: alice.Email, Password: "Zp9!qLm2xR"}, LoginMeta{}); err == nil {
		t.Fatalf("a disabled account must not be able to log in")
	} else if StatusOf(err) != http.StatusForbidden {
		t.Fatalf("the owner should see 403 account disabled, got %d (%s)", StatusOf(err), MsgOf(err))
	}

	// And an unknown address is indistinguishable from a suspended one.
	_, err = svc.Login(ctx, LoginInput{Email: "ghost@example.com", Password: "not-her-password"}, LoginMeta{})
	if StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("unknown address must answer 401, got %d", StatusOf(err))
	}
}
