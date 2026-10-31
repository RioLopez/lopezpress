package services

import (
	"context"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// Re-review: editing a live post puts it back in the queue.
//
// The defect these tests cover was a silent no-op. resolveStatusChange's
// published/pending case had two branches — root, and an author moving out of
// private/rejected — and an author editing an already-published post matched
// neither. The body changed, the status did not, and the article stayed public
// carrying text no reviewer had ever seen: one approved submission bought
// unlimited later rewrites, which is the exact hole review exists to close.

// livePost seeds a published post owned by author.
func livePost(t *testing.T, db *gorm.DB, author models.User, title string) models.Post {
	t.Helper()
	return makePostNoTime(t, db, author, models.PostPublished, "original body", title)
}

// inQueue reports whether the post is in the reviewer's pending list.
func inQueue(t *testing.T, svc *Service, reviewer models.User, postID string) bool {
	t.Helper()
	pending, err := svc.ListPending(context.Background(), reviewer)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	for _, p := range pending {
		if p.ID == postID {
			return true
		}
	}
	return false
}

func TestLivePostContentEditRequeues(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "rr-author", "rr-author@example.com", models.RoleUser)
	admin := makeUser(t, db, "rr-admin", "rr-admin@example.com", models.RoleAdmin)

	// The three fields that change what a reader sees, plus the case where the
	// client sends no status at all — that one must not be a way around the
	// rule, which is why the check runs on content rather than on the request.
	cases := []struct {
		name string
		in   UpdatePostInput
	}{
		{"body", UpdatePostInput{Body: "a different article", Status: models.PostPublished}},
		{"title", UpdatePostInput{Title: "A different title", Status: models.PostPublished}},
		{"cover", UpdatePostInput{Cover: strPtr("/uploads/another-cover.png"), Status: models.PostPublished}},
		{"body with no status field", UpdatePostInput{Body: "no status sent"}},
	}
	for _, c := range cases {
		// Start from a real approval rather than a hand-written published row:
		// the row must carry reviewed_by/reviewed_at for the "clear the old
		// verdict" assertion below to mean anything. A fixture with NULL
		// metadata would pass even if the clear never ran.
		post := pendingPost(t, svc, author, "original "+c.name)
		if _, err := svc.ReviewPost(ctx, admin, post.ID, ReviewPostInput{Action: "approve"}); err != nil {
			t.Fatalf("%s: approve before edit: %v", c.name, err)
		}
		out, err := svc.UpdatePost(ctx, author, post.ID, c.in)
		if err != nil {
			t.Fatalf("%s: edit failed: %v", c.name, err)
		}
		if out.Status != models.PostPending {
			t.Fatalf("%s: editing live content must queue for review, got %q", c.name, out.Status)
		}
		if out.ReviewedBy != "" || out.ReviewedAt != nil {
			t.Fatalf("%s: re-entering the queue must clear the previous verdict, got by=%q at=%v",
				c.name, out.ReviewedBy, out.ReviewedAt)
		}
		if !inQueue(t, svc, admin, post.ID) {
			t.Fatalf("%s: the re-queued post is not in the reviewer's queue", c.name)
		}
		// And it left the public site: a submission under review is not
		// readable by visitors, which is the whole point of the transition.
		if _, err := svc.GetPostPublic(ctx, post.ID); StatusOf(err) != http.StatusNotFound {
			t.Fatalf("%s: a re-queued post must be hidden from visitors, got err=%v", c.name, err)
		}
	}
}

// TestFilingEditsDoNotRequeue: category, tags and slug are how a post is filed,
// not what it says. Re-queuing on those would take the front page offline every
// time an editor fixed a misfiled article.
func TestFilingEditsDoNotRequeue(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "rr-filer", "rr-filer@example.com", models.RoleUser)

	post := livePost(t, db, author, "filed not rewritten")
	out, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{
		Tags: []string{"filed"}, Slug: "a-new-permalink", Status: models.PostPublished,
	})
	if err != nil {
		t.Fatalf("filing-only edit failed: %v", err)
	}
	if out.Status != models.PostPublished {
		t.Fatalf("a filing-only edit must keep the post live, got %q", out.Status)
	}
	if out.Slug != "a-new-permalink" {
		t.Fatalf("the slug edit should have been applied, got %q", out.Slug)
	}
}

// TestRootEditOfLivePostStaysPublished: root is the reviewer, so there is
// nobody above it to ask.
func TestRootEditOfLivePostStaysPublished(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "rr-owner", "rr-owner@example.com", models.RoleUser)
	root := makeUser(t, db, "rr-root", "rr-root@example.com", models.RoleRoot)

	post := livePost(t, db, author, "root fixes the text")
	out, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{
		Body: "root-corrected body", Status: models.PostPublished,
	})
	if err != nil {
		t.Fatalf("root edit failed: %v", err)
	}
	if out.Status != models.PostPublished {
		t.Fatalf("a root edit must stay published, got %q", out.Status)
	}
}

// TestAdminFilingEditOfLivePostStaysPublished: an admin cannot change the words
// of someone else's post at all (R5-05), so a filing edit by an admin must not
// knock that post out of the public site either.
func TestAdminFilingEditOfLivePostStaysPublished(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "rr-member", "rr-member@example.com", models.RoleUser)
	admin := makeUser(t, db, "rr-staff", "rr-staff@example.com", models.RoleAdmin)

	post := livePost(t, db, author, "admin re-files it")
	out, err := svc.UpdatePost(ctx, admin, post.ID, UpdatePostInput{
		Tags: []string{"recategorised"}, Status: models.PostPublished,
	})
	if err != nil {
		t.Fatalf("admin filing edit failed: %v", err)
	}
	if out.Status != models.PostPublished {
		t.Fatalf("an admin's filing edit must not unpublish, got %q", out.Status)
	}
}

// TestPrivateRequestWinsOverReReview: an author who rewrites a live post and
// asks for 私密 in the same save is withdrawing it, not submitting it. Forcing
// that into the queue would resurrect the "私密 means pending" bug that
// resolveStatusChange was written to kill.
func TestPrivateRequestWinsOverReReview(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "rr-withdraw", "rr-withdraw@example.com", models.RoleUser)

	post := livePost(t, db, author, "withdrawn while editing")
	out, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{
		Body: "revised privately", Status: models.PostPrivate,
	})
	if err != nil {
		t.Fatalf("edit-with-private failed: %v", err)
	}
	if out.Status != models.PostPrivate {
		t.Fatalf("an explicit private request must win, got %q", out.Status)
	}
}

// TestReReviewIsRepeatable: approval puts the post back on the site, and the
// next content edit queues it again. A one-shot approval would let an author
// wait for the first pass and then rewrite freely.
func TestReReviewIsRepeatable(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "rr-cycle", "rr-cycle@example.com", models.RoleUser)
	admin := makeUser(t, db, "rr-cycle-admin", "rr-cycle-admin@example.com", models.RoleAdmin)

	post := livePost(t, db, author, "cycles through review")
	for round := 1; round <= 2; round++ {
		if _, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{
			Body: "revision " + string(rune('0'+round)), Status: models.PostPublished,
		}); err != nil {
			t.Fatalf("round %d: edit failed: %v", round, err)
		}
		if _, err := svc.ReviewPost(ctx, admin, post.ID, ReviewPostInput{Action: "approve"}); err != nil {
			t.Fatalf("round %d: approve failed: %v", round, err)
		}
		got, err := svc.GetPostPublic(ctx, post.ID)
		if err != nil {
			t.Fatalf("round %d: approved post should be public: %v", round, err)
		}
		if got.Status != models.PostPublished {
			t.Fatalf("round %d: expected published after approval, got %q", round, got.Status)
		}
	}
}
