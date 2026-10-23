package services

import (
	"context"
	"testing"
	"time"

	"lopezpress/internal/models"
)

// Spec 33: the dashboard must answer "is anyone reading?" — trend, pending
// queue and most-read list, all from data that already exists.
func TestDashboardTrendAndTopPosts(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "admin", "admin@example.com", "root")

	hot, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "hot", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	cold, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "cold", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	queued, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "queued", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// A post waiting for review is what the operator needs to act on today.
	if err := db.Model(&models.Post{}).Where("id = ?", queued.ID).Update("status", "pending").Error; err != nil {
		t.Fatalf("queue post: %v", err)
	}
	if err := db.Model(&models.Post{}).Where("id = ?", hot.ID).Update("view_count", 40).Error; err != nil {
		t.Fatalf("set views: %v", err)
	}

	// Two de-duplicated reads today, one five days ago.
	now := time.Now().UTC()
	rows := []models.PostView{
		{PostID: hot.ID, VisitorHash: "v1", Day: now.Format("2006-01-02")},
		{PostID: hot.ID, VisitorHash: "v2", Day: now.Format("2006-01-02")},
		{PostID: cold.ID, VisitorHash: "v3", Day: now.AddDate(0, 0, -5).Format("2006-01-02")},
	}
	for _, r := range rows {
		if err := db.Create(&r).Error; err != nil {
			t.Fatalf("seed view: %v", err)
		}
	}

	data, err := svc.GetDashboard(ctx)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}

	if len(data.ViewTrend) != dashboardTrendDays {
		t.Fatalf("expected %d trend points, got %d", dashboardTrendDays, len(data.ViewTrend))
	}
	if last := data.ViewTrend[len(data.ViewTrend)-1]; last.Views != 2 {
		t.Fatalf("today should have 2 reads, got %d", last.Views)
	}
	if fiveAgo := data.ViewTrend[len(data.ViewTrend)-6]; fiveAgo.Views != 1 {
		t.Fatalf("the day-5 point should have 1 read, got %d", fiveAgo.Views)
	}
	// Days without reads are present (gap-filled), not omitted.
	zeros := 0
	for _, d := range data.ViewTrend {
		if d.Views == 0 {
			zeros++
		}
	}
	if zeros != dashboardTrendDays-2 {
		t.Fatalf("expected %d empty days, got %d", dashboardTrendDays-2, zeros)
	}

	if data.PendingTotal != 1 {
		t.Fatalf("expected 1 post awaiting review, got %d", data.PendingTotal)
	}
	if len(data.TopPosts) == 0 || data.TopPosts[0].ID != hot.ID {
		t.Fatalf("the most-read post should rank first, got %+v", data.TopPosts)
	}
	for _, p := range data.TopPosts {
		if p.ID == queued.ID {
			t.Fatalf("a pending post must not appear in the most-read list")
		}
	}

	// The summary must not ship article bodies (spec 31 rule, kept here as a
	// regression guard: this page renders titles only).
	for _, p := range data.RecentPosts {
		if p.Body != "" {
			t.Fatalf("recent posts must not carry a body, got %d bytes", len(p.Body))
		}
	}
}
