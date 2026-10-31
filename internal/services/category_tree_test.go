package services

// Anonymous-read switch + two-level category tree + preset tree.

import (
	"context"
	"strconv"
	"testing"

	"lopezpress/internal/models"
)

// --- anonymous read switch ---

func TestAllowAnonymousFollowsConfig(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	if !svc.AllowAnonymous(ctx) {
		t.Fatalf("fixture is public; expected anonymous reading to be allowed")
	}

	// Flip it off through the real write path (which must invalidate the cache).
	cfg, err := svc.GetSiteConfig(ctx, true)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.PublicRead = false
	if _, err := svc.UpdateSiteConfig(ctx, cfg); err != nil {
		t.Fatalf("update config: %v", err)
	}
	if svc.AllowAnonymous(ctx) {
		t.Fatalf("after UpdateSiteConfig(public_read=false) anonymous must be denied without a restart")
	}

	// And back on.
	cfg.PublicRead = true
	if _, err := svc.UpdateSiteConfig(ctx, cfg); err != nil {
		t.Fatalf("update config back: %v", err)
	}
	if !svc.AllowAnonymous(ctx) {
		t.Fatalf("re-enabling public read should take effect immediately")
	}
}

// --- category tree ---

func TestCategoryParentRules(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	parent, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "技术笔记"})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	pid := parent.ID
	child, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "后端开发", ParentID: &pid})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("child should point at the parent, got %+v", child.ParentID)
	}

	// A category cannot be its own parent.
	self := child.ID
	if _, err := svc.UpdateCategory(ctx, child.ID, UpdateCategoryInput{Name: "后端开发", ParentID: &self}); StatusOf(err) != 400 {
		t.Fatalf("self-parent must be 400, got %v", err)
	}

	// A child cannot become a parent of its own parent (two levels only).
	childID := child.ID
	if _, err := svc.UpdateCategory(ctx, parent.ID, UpdateCategoryInput{Name: "技术笔记", ParentID: &childID}); StatusOf(err) != 400 {
		t.Fatalf("grandchild attempt must be 400, got %v", err)
	}

	// A child of a child is rejected at creation time too.
	if _, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "Golang", ParentID: &childID}); StatusOf(err) != 400 {
		t.Fatalf("third level must be 400, got %v", err)
	}

	// Missing parent id.
	bogus := int64(99999)
	if _, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "x", ParentID: &bogus}); StatusOf(err) != 400 {
		t.Fatalf("missing parent must be 400, got %v", err)
	}

	// Moving a child back to top level uses the explicit 0 sentinel.
	zero := int64(0)
	moved, err := svc.UpdateCategory(ctx, child.ID, UpdateCategoryInput{Name: "后端开发", ParentID: &zero})
	if err != nil {
		t.Fatalf("move to top: %v", err)
	}
	if moved.ParentID != nil {
		t.Fatalf("explicit 0 should lift to top level, got %v", *moved.ParentID)
	}
}

func TestDeleteCategoryLiftsChildren(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	parent, _ := svc.CreateCategory(ctx, CreateCategoryInput{Name: "系统与网络"})
	pid := parent.ID
	child, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "Linux", ParentID: &pid})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	if err := svc.DeleteCategory(ctx, parent.ID); err != nil {
		t.Fatalf("delete parent: %v", err)
	}

	var reloaded models.Category
	if err := db.Where("id = ?", child.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("child should survive a parent delete: %v", err)
	}
	if reloaded.ParentID != nil {
		t.Fatalf("orphaned child must become top level, got %d", *reloaded.ParentID)
	}
}

func TestCategoryFilterIncludesDescendants(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	parent, _ := svc.CreateCategory(ctx, CreateCategoryInput{Name: "技术笔记"})
	pid := parent.ID
	child, _ := svc.CreateCategory(ctx, CreateCategoryInput{Name: "数据库", ParentID: &pid})
	other, _ := svc.CreateCategory(ctx, CreateCategoryInput{Name: "生活随笔"})

	pChild := makePostNoTime(t, db, alice, "published", "body", "in child")
	pOther := makePostNoTime(t, db, alice, "published", "body", "in other")
	if err := db.Model(&models.Post{}).Where("id = ?", pChild.ID).Update("category_id", child.ID).Error; err != nil {
		t.Fatalf("assign child cat: %v", err)
	}
	if err := db.Model(&models.Post{}).Where("id = ?", pOther.ID).Update("category_id", other.ID).Error; err != nil {
		t.Fatalf("assign other cat: %v", err)
	}

	// Name form: the frontend links with ?category=<name>, and a parent must
	// still include its children's posts.
	page, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{Page: 1, PageSize: 50, Category: parent.Name})
	if err != nil {
		t.Fatalf("list by parent: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != pChild.ID {
		t.Fatalf("parent filter must include its children's posts, got %d items", len(page.Items))
	}

	// The numeric form stays supported so bookmarks made before the switch to
	// names keep working.
	byID, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{
		Page: 1, PageSize: 50, Category: strconv.FormatInt(parent.ID, 10),
	})
	if err != nil {
		t.Fatalf("list by numeric id: %v", err)
	}
	if len(byID.Items) != 1 || byID.Items[0].ID != pChild.ID {
		t.Fatalf("numeric category filter must behave like the name form, got %d items", len(byID.Items))
	}

	// An unknown reference yields an empty page, never the whole site: the URL
	// claims a filter is applied, so falling back to "no filter" would show
	// every post under a link that looks like a category.
	none, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{
		Page: 1, PageSize: 50, Category: "没有这个分类",
	})
	if err != nil {
		t.Fatalf("list by unknown category: %v", err)
	}
	if none.Total != 0 || len(none.Items) != 0 {
		t.Fatalf("unknown category must return an empty page, got %d items", len(none.Items))
	}
}

func TestApplyPresetCategoriesIdempotent(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	total := 0
	for _, root := range models.PresetCategories {
		total += 1 + len(root.Children)
	}

	first, err := svc.ApplyPresetCategories(ctx)
	if err != nil {
		t.Fatalf("apply preset: %v", err)
	}
	if first.Created != total {
		t.Fatalf("first apply should create %d categories, got %d", total, first.Created)
	}

	var count int64
	if err := db.Model(&models.Category{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if int(count) != total {
		t.Fatalf("expected %d categories, got %d", total, count)
	}

	// Children must actually hang under their parent.
	var linux models.Category
	if err := db.Where("name = ?", "Linux").First(&linux).Error; err != nil {
		t.Fatalf("load Linux: %v", err)
	}
	if linux.ParentID == nil {
		t.Fatalf("preset child should have a parent")
	}

	// Second run: nothing new, nothing moved.
	second, err := svc.ApplyPresetCategories(ctx)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if second.Created != 0 || second.Reparented != 0 {
		t.Fatalf("second apply must be a no-op, got %+v", second)
	}
	if err := db.Model(&models.Category{}).Count(&count).Error; err != nil {
		t.Fatalf("count again: %v", err)
	}
	if int(count) != total {
		t.Fatalf("second apply must not create rows: %d", count)
	}
}

func TestApplyPresetAdoptsOrphansOnly(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	// A pre-existing top-level "Linux" (as in the live database) must be
	// adopted under 系统与网络 rather than duplicated.
	orphan := models.Category{Name: "Linux"}
	if err := db.Create(&orphan).Error; err != nil {
		t.Fatalf("seed orphan: %v", err)
	}
	// A category that is already a parent must NOT be adopted (would create a
	// third level).
	customParent := models.Category{Name: "Windows"}
	if err := db.Create(&customParent).Error; err != nil {
		t.Fatalf("seed parent: %v", err)
	}
	pid := customParent.ID
	if err := db.Create(&models.Category{Name: "Server", ParentID: &pid}).Error; err != nil {
		t.Fatalf("seed grandchild source: %v", err)
	}

	res, err := svc.ApplyPresetCategories(ctx)
	if err != nil {
		t.Fatalf("apply preset: %v", err)
	}
	if res.Reparented < 1 {
		t.Fatalf("orphan Linux should have been adopted, got %+v", res)
	}

	var linux models.Category
	if err := db.Where("name = ?", "Linux").First(&linux).Error; err != nil {
		t.Fatalf("load Linux: %v", err)
	}
	if linux.ParentID == nil {
		t.Fatalf("Linux should now have a parent")
	}
	if linux.ID != orphan.ID {
		t.Fatalf("Linux must be reused, not recreated")
	}

	var windows models.Category
	if err := db.Where("name = ?", "Windows").First(&windows).Error; err != nil {
		t.Fatalf("load Windows: %v", err)
	}
	if windows.ParentID != nil {
		t.Fatalf("a category that already has children must not be adopted")
	}
}
