package services

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"lopezpress/internal/models"
)

// Trash operations used to take no actor at all. An admin who was refused
// permission to delete a root's article one call earlier (403 from DeletePost)
// could empty the trash and destroy it irreversibly, or restore it back onto
// the site. Listing was equally open: it showed every soft-deleted post in the
// database regardless of who wrote it.
//
// These tests pin the hierarchy to all three doors, because fixing only the one
// an auditor reported would leave the other two as the same hole with a
// different name.
func TestTrashRespectsAuthorHierarchy(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	adminA := makeUser(t, db, "admina", "a@example.com", models.RoleAdmin)
	adminB := makeUser(t, db, "adminb", "b@example.com", models.RoleAdmin)
	writer := makeUser(t, db, "writer", "writer@example.com", models.RoleUser)

	rootPost := newPost(t, svc, root, "Root article", "root body", "published")
	peerPost := newPost(t, svc, adminB, "Peer article", "peer body", "published")
	userPost := newPost(t, svc, writer, "User article", "user body", "published")

	// Each author deletes their own, so the trash is populated without relying
	// on the very permission being tested.
	for _, pair := range []struct {
		post   models.Post
		author models.User
	}{
		{rootPost, root}, {peerPost, adminB}, {userPost, writer},
	} {
		if err := svc.DeletePost(ctx, pair.author, pair.post.ID); err != nil {
			t.Fatalf("delete %q: %v", pair.post.Title, err)
		}
	}

	// Listing: an admin sees plain users' posts, never a peer's or root's.
	listed, err := svc.ListTrash(ctx, adminA)
	if err != nil {
		t.Fatalf("list trash: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != userPost.ID {
		got := make([]string, 0, len(listed))
		for _, row := range listed {
			got = append(got, row.Title)
		}
		t.Fatalf("admin should only see plain users' trashed posts, got %v", got)
	}

	// root sees everything.
	all, err := svc.ListTrash(ctx, root)
	if err != nil {
		t.Fatalf("list trash as root: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("root should see all 3 trashed posts, got %d", len(all))
	}

	// Restore: refused for a peer's and for root's content.
	for _, target := range []models.Post{rootPost, peerPost} {
		if err := svc.RestoreTrash(ctx, adminA, target.ID); StatusOf(err) != http.StatusForbidden {
			t.Fatalf("restoring %q as a plain admin should be 403, got %v (err=%v)",
				target.Title, StatusOf(err), err)
		}
	}
	// ...allowed for a plain user's.
	if err := svc.RestoreTrash(ctx, adminA, userPost.ID); err != nil {
		t.Fatalf("restoring a plain user's post should work: %v", err)
	}

	// Empty: scoped to what the caller may manage, so the two forbidden rows
	// survive even though the endpoint reports success.
	n, err := svc.EmptyTrash(ctx, adminA)
	if err != nil {
		t.Fatalf("empty trash: %v", err)
	}
	if n != 0 {
		t.Fatalf("admin had nothing manageable left to purge, got %d", n)
	}
	if left, _ := svc.ListTrash(ctx, root); len(left) != 2 {
		t.Fatalf("root's and the peer's posts must survive an admin's empty-trash, %d left", len(left))
	}

	// The admin can still purge their own content.
	ownPost := newPost(t, svc, adminA, "Admin own article", "own body", "published")
	if err := svc.DeletePost(ctx, adminA, ownPost.ID); err != nil {
		t.Fatalf("delete own post: %v", err)
	}
	if n, err := svc.EmptyTrash(ctx, adminA); err != nil || n != 1 {
		t.Fatalf("admin should purge their own trashed post, got n=%d err=%v", n, err)
	}

	// root may purge the rest.
	if n, err := svc.EmptyTrash(ctx, root); err != nil || n != 2 {
		t.Fatalf("root should purge the remaining two, got n=%d err=%v", n, err)
	}
}

// UpdateUser must not be a second door for enable/disable. It used to accept a
// `disabled` field and applied only half the rule set: disabling another admin
// was refused, but re-enabling one that root had shut down was not.
func TestUpdateUserCannotReEnableAnAdmin(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	adminA := makeUser(t, db, "admina", "a@example.com", models.RoleAdmin)
	adminB := makeUser(t, db, "adminb", "b@example.com", models.RoleAdmin)

	if _, err := svc.SetUserDisabled(ctx, root, adminB.ID, true); err != nil {
		t.Fatalf("root disables admin B: %v", err)
	}

	// admin A replays the exact payload the old endpoint accepted. Parsed from
	// JSON on purpose: if anyone adds the field back, the decoder will populate
	// it and this test goes red instead of quietly passing on an empty struct.
	var in UpdateUserInput
	if err := json.Unmarshal([]byte(`{"role":"","disabled":false}`), &in); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, err := svc.UpdateUser(ctx, adminA, adminB.ID, in); err != nil {
		t.Fatalf("update user: %v", err)
	}

	var reloaded models.User
	if err := db.Where("id = ?", adminB.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.Disabled {
		t.Fatal("root's disable was undone through the role endpoint")
	}
}
