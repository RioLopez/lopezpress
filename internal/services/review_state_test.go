package services

import (
	"context"
	"net/http"
	"testing"
)

// TestReviewPostStatePrecondition is the regression test for as-built audit
// finding Q1: ReviewPost / BanPost had no state precondition, so a published
// post could be suddenly rejected (public content vanishes) and a banned post
// could be approved straight back to published, skipping the mandatory
// send-back-to-author fix step.
func TestReviewPostStatePrecondition(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	author := makeUser(t, db, "author", "author@example.com", "user")
	admin := makeUser(t, db, "admin", "admin@example.com", "admin")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// --- pending is a valid review target ---------------------------------
	pending, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "t", Body: "x"})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if pending.Status != "pending" {
		t.Fatalf("expected pending, got %q", pending.Status)
	}
	// approve a pending post → published (happy path).
	if _, err := svc.ReviewPost(ctx, admin, pending.ID, ReviewPostInput{Action: "approve"}); err != nil {
		t.Fatalf("approve pending: expected success, got %v", err)
	}
	// reject on an already-published post is NOT allowed (would unpublish).
	if _, err := svc.ReviewPost(ctx, admin, pending.ID, ReviewPostInput{Action: "reject", Note: "take down"}); StatusOf(err) != http.StatusConflict {
		t.Errorf("reject published: expected 409, got %v (err=%v)", StatusOf(err), err)
	}
	// the post must still be published (reject was refused).
	if p, _ := svc.GetPost(ctx, admin, pending.ID); p.Status != "published" {
		t.Errorf("published post should be unchanged after refused reject, got %q", p.Status)
	}

	// --- private is NOT a review target -----------------------------------
	priv, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "p", Body: "x"})
	if err != nil {
		t.Fatalf("CreatePost(private seed): %v", err)
	}
	// Promote to private via root edit (author cannot set private on a pending
	// post, so use root who may set any status).
	if _, err := svc.UpdatePost(ctx, root, priv.ID, UpdatePostInput{Status: "private"}); err != nil {
		t.Fatalf("set private: %v", err)
	}
	if _, err := svc.ReviewPost(ctx, admin, priv.ID, ReviewPostInput{Action: "approve"}); StatusOf(err) != http.StatusConflict {
		t.Errorf("review private: expected 409, got %v (err=%v)", StatusOf(err), err)
	}

	// --- rejected is a valid review target (admin override / re-review) ----
	rej, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "r", Body: "x"})
	if err != nil {
		t.Fatalf("CreatePost(reject seed): %v", err)
	}
	if _, err := svc.ReviewPost(ctx, admin, rej.ID, ReviewPostInput{Action: "reject", Note: "fix it"}); err != nil {
		t.Fatalf("reject pending: expected success, got %v", err)
	}
	if p, _ := svc.GetPost(ctx, admin, rej.ID); p.Status != "rejected" {
		t.Fatalf("expected rejected, got %q", p.Status)
	}
	// admin may re-review a rejected post and publish it.
	if _, err := svc.ReviewPost(ctx, admin, rej.ID, ReviewPostInput{Action: "approve"}); err != nil {
		t.Fatalf("approve rejected: expected success, got %v", err)
	}

	// --- banned is NOT a review target (must go through send-back) ---------
	banned, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "b", Body: "x"})
	if err != nil {
		t.Fatalf("CreatePost(banned seed): %v", err)
	}
	if _, err := svc.BanPost(ctx, root, banned.ID); err != nil {
		t.Fatalf("BanPost: %v", err)
	}
	// Neither admin nor root may ReviewPost a banned post — it would skip the
	// mandatory author-fix step and republish without changes.
	if _, err := svc.ReviewPost(ctx, root, banned.ID, ReviewPostInput{Action: "approve"}); StatusOf(err) != http.StatusConflict {
		t.Errorf("approve banned (root): expected 409, got %v (err=%v)", StatusOf(err), err)
	}
	if _, err := svc.ReviewPost(ctx, admin, banned.ID, ReviewPostInput{Action: "reject", Note: "x"}); StatusOf(err) != http.StatusConflict {
		t.Errorf("reject banned (admin): expected 409, got %v (err=%v)", StatusOf(err), err)
	}
	// The proper exit is send-back, which this test does not exercise but the
	// post must remain banned after the refused reviews.
	if p, _ := svc.GetPost(ctx, root, banned.ID); p.Status != "banned" {
		t.Errorf("banned post should remain banned after refused reviews, got %q", p.Status)
	}

	// --- BanPost on an already-banned post fails fast ----------------------
	if _, err := svc.BanPost(ctx, root, banned.ID); StatusOf(err) != http.StatusConflict {
		t.Errorf("re-ban banned post: expected 409, got %v (err=%v)", StatusOf(err), err)
	}
}
