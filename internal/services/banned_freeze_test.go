package services

import (
	"context"
	"net/http"
	"testing"
)

// TestBannedPostFrozenForAuthor is the regression test for the as-built audit
// finding Q2: an author must not be able to keep editing (or soft-deleting)
// their own post once the root has pulled it into the banned vault. Root keeps
// editorial access, but even root must leave the vault through the dedicated
// vault endpoints (send-back / permanent delete), never through DeletePost's
// soft delete — which would hide the row from every list while leaving it
// banned, and 404 the permanent-delete path.
func TestBannedPostFrozenForAuthor(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	author := makeUser(t, db, "author", "author@example.com", "user")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// A post that lands in the vault: user creates (→ pending), root bans.
	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "nasty", Body: "x"})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if post.Status != "pending" {
		t.Fatalf("expected pending for user-authored post, got %q", post.Status)
	}
	if _, err := svc.BanPost(ctx, root, post.ID); err != nil {
		t.Fatalf("BanPost: %v", err)
	}

	// 1. Author cannot edit their own banned post.
	if _, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Title: "sneaky edit"}); StatusOf(err) != http.StatusForbidden {
		t.Errorf("author UpdatePost on banned post: expected 403, got %v (err=%v)", StatusOf(err), err)
	}
	// 2. Author cannot soft-delete their own banned post.
	if err := svc.DeletePost(ctx, author, post.ID); StatusOf(err) != http.StatusForbidden {
		t.Errorf("author DeletePost on banned post: expected 403, got %v (err=%v)", StatusOf(err), err)
	}
	// 3. Root may still edit a banned post (corrections stay possible).
	if _, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Title: "corrected"}); err != nil {
		t.Errorf("root UpdatePost on banned post: expected success, got %v", err)
	}
	// 4. Even root must not soft-delete a banned post; the vault's permanent
	//    delete is the only exit. Otherwise the row vanishes from every list
	//    while staying banned, and DeleteBannedPermanently would 404 on it.
	if err := svc.DeletePost(ctx, root, post.ID); StatusOf(err) != http.StatusForbidden {
		t.Errorf("root DeletePost on banned post: expected 403, got %v (err=%v)", StatusOf(err), err)
	}
	// 5. The row is still intact in the vault after all the rejected attempts.
	vault, err := svc.ListBanned(ctx, root)
	if err != nil {
		t.Fatalf("ListBanned: %v", err)
	}
	if len(vault) != 1 || vault[0].ID != post.ID {
		t.Fatalf("expected the banned post to remain in the vault, got %d entries", len(vault))
	}
	// 6. The root's permanent-delete path still works on the untouched row.
	if err := svc.DeleteBannedPermanently(ctx, root, post.ID); err != nil {
		t.Errorf("DeleteBannedPermanently after frozen attempts: expected success, got %v", err)
	}
}
