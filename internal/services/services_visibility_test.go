package services

// Spec #9: service-layer unit tests covering the three visibility
// branches, GetPostPublic's private->404, the sort-column whitelist
// fallback, and the public-only scoping of ListArchive / GetAuthor.

import (
	"context"
	"net/http"
	"testing"

	"lopezpress/internal/models"
)

// --- ListPostsPage: the three visibility branches (spec 22 + #9) ---

func TestListPostsPageVisibilityThreeBranches(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	alice := makeUser(t, db, "alice", "alice@example.com", "user")

	pubA := makePostNoTime(t, db, alice, "published", "public body", "Alice Public")
	privA := makePostNoTime(t, db, alice, "private", "alice secret", "Alice Private")
	_ = makePostNoTime(t, db, admin, "private", "boss secret", "Boss Private")

	// 1) Anonymous visitor (ID ""): sees ONLY published.
	anonPage, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{PageSize: 100})
	if err != nil {
		t.Fatalf("anon list: %v", err)
	}
	if len(anonPage.Items) != 1 {
		t.Fatalf("anon should see 1 published post, got %d", len(anonPage.Items))
	}
	if anonPage.Items[0].ID != pubA.ID {
		t.Fatalf("anon saw wrong post: %+v", anonPage.Items[0])
	}

	// 2) Signed-in non-admin (alice): all published + her own private.
	alicePage, err := svc.ListPostsPage(ctx, alice, ListPostsQuery{PageSize: 100})
	if err != nil {
		t.Fatalf("alice list: %v", err)
	}
	seen := map[string]bool{}
	for _, p := range alicePage.Items {
		seen[p.ID] = true
	}
	if !seen[pubA.ID] || !seen[privA.ID] {
		t.Fatalf("alice should see her public + her private, got ids %v", seen)
	}
	if seen[privA.ID] && len(alicePage.Items) > 2 {
		t.Fatalf("alice leaked another user's private post")
	}
	if len(alicePage.Items) != 2 {
		t.Fatalf("alice should see exactly 2 posts (1 pub + 1 own private), got %d", len(alicePage.Items))
	}

	// 3) Admin: sees everything (public + all privates).
	adminPage, err := svc.ListPostsPage(ctx, admin, ListPostsQuery{PageSize: 100})
	if err != nil {
		t.Fatalf("admin list: %v", err)
	}
	if len(adminPage.Items) != 3 {
		t.Fatalf("admin should see all 3 posts, got %d", len(adminPage.Items))
	}
}

// --- GetPostPublic: private posts must 404 (spec #9) ---

func TestGetPostPublicPrivateReturns404(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	priv := makePostNoTime(t, db, alice, "private", "secret", "Hidden")
	pub := makePostNoTime(t, db, alice, "published", "open", "Open")

	// Private post -> 404 even for the anonymous caller.
	if _, err := svc.GetPostPublic(ctx, priv.ID); err == nil {
		t.Fatalf("expected 404 for private post")
	} else if StatusOf(err) != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (err=%v)", StatusOf(err), err)
	}

	// Public post -> found.
	got, err := svc.GetPostPublic(ctx, pub.ID)
	if err != nil {
		t.Fatalf("public post should load: %v", err)
	}
	if got.ID != pub.ID {
		t.Fatalf("loaded wrong post")
	}
}

// TestListPostsPageSortWhitelistFallback verifies that an unknown SortBy value
// is ignored by the whitelist (falls back to created_at) and that "views" is
// honoured when requested. The injection-safety is what matters: a hostile
// SortBy like "DROP TABLE posts" must not break the query.
func TestListPostsPageSortWhitelistFallback(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	p1 := makePostNoTime(t, db, alice, "published", "old", "Older")
	_ = makePostNoTime(t, db, alice, "published", "new", "Newer")

	// 1) Hostile/unknown SortBy must not error and must not corrupt the table.
	_, err := svc.ListPostsPage(ctx, alice, ListPostsQuery{SortBy: "DROP TABLE posts", Order: "desc", PageSize: 100})
	if err != nil {
		t.Fatalf("unknown sort must be ignored, not error: %v", err)
	}
	var still int64
	db.Model(&models.Post{}).Count(&still)
	if still < 2 {
		t.Fatalf("posts table was damaged by the whitelist bypass; count=%d", still)
	}

	// 2) "views" is the one other whitelisted column; it must actually reorder.
	var v int64 = 42
	db.Model(&models.Post{}).Where("id = ?", p1.ID).UpdateColumn("view_count", v)
	page, err := svc.ListPostsPage(ctx, alice, ListPostsQuery{SortBy: "views", Order: "desc", PageSize: 100})
	if err != nil {
		t.Fatalf("views sort: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(page.Items))
	}
	if page.Items[0].ID != p1.ID {
		t.Fatalf("p1 has 42 views, should sort first under views DESC; got %q", page.Items[0].Title)
	}
}

// --- ListArchive: public-only + body stripped ---

func TestListArchivePublicOnly(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	pub := makePostNoTime(t, db, alice, "published", "LONG PUBLIC BODY", "Pub")
	priv := makePostNoTime(t, db, alice, "private", "LONG PRIVATE BODY", "Priv")

	posts, err := svc.ListArchive(ctx, anonymous())
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("archive should list only 1 published post, got %d", len(posts))
	}
	if posts[0].ID != pub.ID {
		t.Fatalf("archive returned private post")
	}
	if posts[0].Body != "" {
		t.Fatalf("archive must strip body, got %q", posts[0].Body)
	}
	_ = priv
}

// --- GetAuthor: public-only profile, 404 on missing/disabled ---

func TestGetAuthorPublicOnlyAnd404(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	_ = makePostNoTime(t, db, alice, "published", "pub", "Alice Pub")
	priv := makePostNoTime(t, db, alice, "private", "sec", "Alice Priv")

	view, err := svc.GetAuthor(ctx, "alice")
	if err != nil {
		t.Fatalf("GetAuthor(alice): %v", err)
	}
	if len(view.Posts) != 1 {
		t.Fatalf("author page should show only 1 published post, got %d", len(view.Posts))
	}
	for _, p := range view.Posts {
		if p.ID == priv.ID {
			t.Fatalf("author page leaked a private post")
		}
	}

	// Missing username -> 404.
	if _, err := svc.GetAuthor(ctx, "ghost"); err == nil {
		t.Fatalf("expected 404 for unknown author")
	} else if StatusOf(err) != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", StatusOf(err))
	}

	// Disabled user -> 404 (not exposed).
	disabled := makeUser(t, db, "gone", "gone@example.com", "user")
	db.Model(&disabled).Update("disabled", true)
	if _, err := svc.GetAuthor(ctx, "gone"); err == nil {
		t.Fatalf("expected 404 for disabled author")
	} else if StatusOf(err) != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", StatusOf(err))
	}
}
