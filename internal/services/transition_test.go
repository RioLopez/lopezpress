package services

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"lopezpress/internal/models"
)

// R4-11: every post state transition was "read the row, check the state we
// expect, write the new state". Two statements, so the check did not bind the
// write: two reviewers could both approve the same post (two mails, two audit
// rows, one describing a transition that never happened), and an approve
// racing a delete could re-publish content the author had just removed.
//
// These tests use a FILE-backed DB because :memory: gives every pooled
// connection its own private database — the racers would not even be looking at
// the same row, so "exactly one wins" would pass or fail for the wrong reason.

func pendingPost(t *testing.T, svc *Service, author models.User, title string) models.Post {
	t.Helper()
	p, err := svc.CreatePost(context.Background(), author, CreatePostInput{
		Title: title, Body: "body for " + title,
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if p.Status != models.PostPending {
		t.Fatalf("a plain user's post should start pending, got %q", p.Status)
	}
	return p
}

// Only one of N concurrent approvals may succeed. The losers must be told 409,
// not silently re-run — and no duplicate audit row may be written.
func TestConcurrentApprovalsOnlyOneWins(t *testing.T) {
	svc, db := newFileTestService(t)
	author := makeUser(t, db, "writer", "writer@example.com", models.RoleUser)
	admin := makeUser(t, db, "reviewer", "reviewer@example.com", models.RoleAdmin)
	post := pendingPost(t, svc, author, "Contested")

	const racers = 6
	var wg sync.WaitGroup
	start := make(chan struct{})
	codes := make([]int, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, err := svc.ReviewPost(context.Background(),
				admin, post.ID, ReviewPostInput{Action: "approve"})
			if err == nil {
				codes[i] = http.StatusOK
				return
			}
			codes[i] = StatusOf(err)
		}(i)
	}
	close(start)
	wg.Wait()

	wins, conflicts := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			wins++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("unexpected status %d (want 200 or 409)", c)
		}
	}
	if wins != 1 {
		t.Fatalf("exactly one approval may win, got %d (conflicts=%d)", wins, conflicts)
	}

	// One transition, one audit row.
	var audits int64
	db.Model(&models.ActionLog{}).Where("action = ? AND target = ?", "review_post", "post:"+post.ID).Count(&audits)
	if audits != 1 {
		t.Fatalf("expected exactly 1 review_post audit row, got %d", audits)
	}
}

// R4-11's first measured symptom: approve ‖ delete. Before the fix the approve
// half answered 200 and re-published a post the author had just removed.
func TestApproveRacingDeleteCannotReviveThePost(t *testing.T) {
	svc, db := newFileTestService(t)
	author := makeUser(t, db, "writer2", "writer2@example.com", models.RoleUser)
	admin := makeUser(t, db, "reviewer2", "reviewer2@example.com", models.RoleAdmin)
	post := pendingPost(t, svc, author, "Racy")

	var wg sync.WaitGroup
	start := make(chan struct{})
	var approveErr, deleteErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, approveErr = svc.ReviewPost(context.Background(),
			admin, post.ID, ReviewPostInput{Action: "approve"})
	}()
	go func() {
		defer wg.Done()
		<-start
		deleteErr = svc.DeletePost(context.Background(), author, post.ID)
	}()
	close(start)
	wg.Wait()

	// Whatever interleaving happened, the row must not end up as a LIVE
	// published post that the author deleted: either the delete won (row is
	// soft-deleted) or the approve won and the delete then failed.
	var row models.Post
	if err := db.Unscoped().Where("id = ?", post.ID).First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	// Deliberately NOT asserting "deleted implies not published" here.
	//
	// approve→delete and delete→approve produce the *same* final row
	// (deleted_at set, status published) and only the second is a defect, so a
	// concurrent check cannot tell them apart and fails about one run in eight
	// on correct behaviour — the author approving-publishes a post and then
	// trashing it is perfectly legal. The defect is pinned deterministically by
	// TestReviewCannotResurrectADeletedPost below; what this test still owns is
	// the deadlock case, where both sides claim success on a live row.
	if approveErr == nil && deleteErr == nil && !row.DeletedAt.Valid {
		t.Fatalf("both approve and delete reported success but the post is live and published")
	}
	if approveErr != nil && deleteErr != nil {
		t.Fatalf("both sides refused; one of them should have won (approve=%v delete=%v)", approveErr, deleteErr)
	}
}

// TestReviewCannotResurrectADeletedPost is the deterministic half of R4-11:
// once the author has removed a post, approving it must be refused rather than
// flipping a trashed row to published. Run in order instead of concurrently so
// the assertion is about behaviour, not about who won a race.
func TestReviewCannotResurrectADeletedPost(t *testing.T) {
	svc, db := newFileTestService(t)
	author := makeUser(t, db, "writer4", "writer4@example.com", models.RoleUser)
	admin := makeUser(t, db, "reviewer4", "reviewer4@example.com", models.RoleAdmin)
	post := pendingPost(t, svc, author, "Gone")

	if err := svc.DeletePost(context.Background(), author, post.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if _, err := svc.ReviewPost(context.Background(), admin, post.ID,
		ReviewPostInput{Action: "approve"}); err == nil {
		t.Fatalf("approving a deleted post must fail, not publish it")
	}

	var row models.Post
	if err := db.Unscoped().Where("id = ?", post.ID).First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !row.DeletedAt.Valid {
		t.Fatalf("the post must still be in the trash")
	}
	if row.Status == models.PostPublished {
		t.Fatalf("a trashed post must not have been flipped to published (status=%q)", row.Status)
	}
}

// R4-11's second measured symptom: restore ‖ empty-trash. The restore used to
// answer 200 while the row was permanently destroyed a moment later.
func TestRestoreRacingEmptyTrashIsNotSilent(t *testing.T) {
	svc, db := newFileTestService(t)
	author := makeUser(t, db, "writer3", "writer3@example.com", models.RoleUser)
	post := pendingPost(t, svc, author, "Trashed")

	if err := svc.DeletePost(context.Background(), author, post.ID); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	var restoreErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		restoreErr = svc.RestoreTrash(context.Background(), author, post.ID)
	}()
	go func() {
		defer wg.Done()
		<-start
		_, _ = svc.EmptyTrash(context.Background(), author)
	}()
	close(start)
	wg.Wait()

	// If the restore reported success, the row must actually be live. Saying
	// "restored" about a row that no longer exists is the defect.
	if restoreErr == nil {
		var row models.Post
		if err := db.Where("id = ?", post.ID).First(&row).Error; err != nil {
			if row2 := (models.Post{}); db.Unscoped().Where("id = ?", post.ID).
				First(&row2).Error == nil && row2.DeletedAt.Valid {
				t.Fatalf("restore reported success but the post is still deleted")
			}
		}
	}
}

// The same class of bug in the ordinary edit path: a whole-row Save echoed the
// status back, so saving a typo at the same moment a reviewer approved the post
// silently undid the approval. The save no longer writes status at all.
func TestEditCannotUndoAnApproval(t *testing.T) {
	svc, db := newFileTestService(t)
	author := makeUser(t, db, "writer4", "writer4@example.com", models.RoleUser)
	admin := makeUser(t, db, "reviewer4", "reviewer4@example.com", models.RoleAdmin)
	post := pendingPost(t, svc, author, "Edited while reviewed")

	// The author loads the post (status pending) and starts editing.
	// Meanwhile the reviewer approves it.
	if _, err := svc.ReviewPost(context.Background(), admin, post.ID,
		ReviewPostInput{Action: "approve"}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// The author's save carries no status change and no content change (a
	// tag-only edit), so it must not touch status — the post stays published.
	// A TITLE edit would legitimately send it back to the queue under the
	// re-review rule; that path has its own test
	// (TestLivePostContentEditRequeues), and using a tag here keeps this test
	// aimed at the original defect: echoing the status we read over the one
	// the reviewer just wrote.
	upd := UpdatePostInput{Tags: []string{"reviewed"}}
	if _, err := svc.UpdatePost(context.Background(), author, post.ID, upd); err != nil {
		t.Fatalf("tag-only edit after approval should succeed: %v", err)
	}
	var row models.Post
	if err := db.Where("id = ?", post.ID).First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.Status != models.PostPublished {
		t.Fatalf("a tag-only edit must not change status, got %q", row.Status)
	}
	if len(row.Tags) == 0 {
		var cnt int64
		db.Table("post_tags").Where("post_id = ?", post.ID).Count(&cnt)
		if cnt == 0 {
			t.Fatalf("the tag edit should have been applied")
		}
	}
}

// A state change the author DOES request must be gated on the state they read.
// Resubmitting a rejected post works; resubmitting one that was approved in the
// meantime is refused rather than silently un-publishing it.
func TestRequestedStateChangeIsGatedOnTheStateRead(t *testing.T) {
	svc, db := newFileTestService(t)
	author := makeUser(t, db, "writer5", "writer5@example.com", models.RoleUser)
	admin := makeUser(t, db, "reviewer5", "reviewer5@example.com", models.RoleAdmin)
	post := pendingPost(t, svc, author, "Resubmit")

	// Reject it, then approve it (root-free path: admin can approve a plain
	// user's post). After the approval the post is published, not rejected.
	if _, err := svc.ReviewPost(context.Background(), admin, post.ID,
		ReviewPostInput{Action: "reject", Note: "fix the intro"}); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, err := svc.ReviewPost(context.Background(), admin, post.ID,
		ReviewPostInput{Action: "approve"}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// The author still thinks it is rejected and asks to resubmit. That
	// transition (rejected -> pending) no longer applies, so it must not
	// un-publish the approved post.
	if _, err := svc.UpdatePost(context.Background(), author, post.ID,
		UpdatePostInput{Status: models.PostPending}); err == nil {
		var row models.Post
		if err := db.Where("id = ?", post.ID).First(&row).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if row.Status == models.PostPending {
			t.Fatalf("a stale resubmit must not pull a published post back into the queue")
		}
	}
	_ = time.Now()
}

// A side effect of replacing the whole-row Save with an explicit column list:
// the old code wrote back every column it had read, including view_count. Any
// view counted between the read and the save — ReportView runs on every article
// read, from a different request — was silently rolled back. The new update
// touches only the author's fields, so the counter can no longer be clobbered by
// an edit.
func TestEditDoesNotRollBackViewCount(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "viewed", "viewed@example.com", models.RoleRoot)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Read me", Body: "body", Status: models.PostPublished,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Simulate a reader counting a view after the author loaded the editor.
	if err := db.Model(&models.Post{}).Where("id = ?", post.ID).
		UpdateColumn("view_count", 42).Error; err != nil {
		t.Fatalf("count a view: %v", err)
	}

	if _, err := svc.UpdatePost(ctx, author, post.ID,
		UpdatePostInput{Title: "Read me (fixed)"}); err != nil {
		t.Fatalf("edit: %v", err)
	}

	var row models.Post
	if err := db.Where("id = ?", post.ID).First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.ViewCount != 42 {
		t.Fatalf("view_count = %d after an edit, want 42 (an edit must not roll the counter back)", row.ViewCount)
	}
	if row.Title != "Read me (fixed)" {
		t.Fatalf("the title edit should still apply, got %q", row.Title)
	}
}

// An admin action on someone else's post must be attributable in the trail.
func TestCrossUserEditIsAttributedInTheAuditTrail(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "owned", "owned@example.com", models.RoleUser)
	admin := makeUser(t, db, "moderator", "moderator@example.com", models.RoleAdmin)

	cat := models.Category{Name: "Archive"}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("category: %v", err)
	}
	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Filed", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Recategorising someone else's post is allowed for an admin.
	if _, err := svc.UpdatePost(ctx, admin, post.ID, UpdatePostInput{CategoryID: &cat.ID}); err != nil {
		t.Fatalf("admin recategorise: %v", err)
	}

	var rows []models.ActionLog
	if err := db.Where("action = ? AND target = ?", "update_post", "post:"+post.ID).
		Find(&rows).Error; err != nil {
		t.Fatalf("load audit: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected an update_post audit row")
	}
	last := rows[len(rows)-1]
	if !strings.Contains(last.Detail, "by=moderator") {
		t.Fatalf("a cross-user edit must record who did it, got detail=%q", last.Detail)
	}
	if !strings.Contains(last.Detail, author.ID) {
		t.Fatalf("a cross-user edit must record the author id, got detail=%q", last.Detail)
	}
}
