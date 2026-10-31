package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"lopezpress/internal/models"
)

// Buffered view counting: counts accumulate in memory and reach the database on
// the first request that finds the window expired, or on an explicit flush.
//
// What these tests protect is the trade the buffer makes: reads must not hit the
// write path, and the number a reader sees must still be right.

func bufferedPost(t *testing.T) (*Service, *models.Post, time.Time) {
	t.Helper()
	svc, db := newTestService(t)
	t.Cleanup(func() { closeDB(db) })
	alice := makeUser(t, db, "vb-alice", "vb-alice@example.com", "user")
	post := makePostNoTime(t, db, alice, "published", "body", "buffered views")
	// Pin the flush clock to the same fixed instant the reads use below. The
	// window is measured against wall-clock time, so a service built "now" and
	// then asked about a fixed date in the future would look overdue.
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	svc.viewMu.Lock()
	svc.viewLastFlush = now
	svc.viewMu.Unlock()
	return svc, &post, now
}

// A freshly built service must not treat its first reader as overdue. The clock
// used to start at the zero value, so the first view of every process looked
// decades late and paid a write on everyone's behalf. This test touches nothing
// on the service — it fails if New() stops starting the clock.
func TestFirstViewDoesNotFlushOnItsOwn(t *testing.T) {
	svc, db := newTestService(t)
	t.Cleanup(func() { closeDB(db) })
	alice := makeUser(t, db, "vb-fresh", "vb-fresh@example.com", "user")
	post := makePostNoTime(t, db, alice, "published", "body", "fresh process view")

	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/131.0"
	res, err := svc.ReportView(context.Background(), anonymous(), post.ID,
		viewMeta("203.0.113.5", ua, time.Now()))
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if !res.Counted || res.ViewCount != 1 {
		t.Fatalf("first view should count, got %+v", res)
	}

	var stored models.Post
	if err := svc.db.Where("id = ?", post.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.ViewCount != 0 {
		t.Fatalf("one view inside the window must stay in memory, database has %d", stored.ViewCount)
	}
}

func TestViewsStayInMemoryUntilFlush(t *testing.T) {
	svc, post, now := bufferedPost(t)
	ctx := context.Background()
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/131.0"

	for i := 0; i < 5; i++ {
		res, err := svc.ReportView(ctx, anonymous(), post.ID,
			viewMeta(fmt.Sprintf("10.0.0.%d", i), ua, now.Add(time.Duration(i)*time.Second)))
		if err != nil {
			t.Fatalf("view %d: %v", i, err)
		}
		if !res.Counted || res.ViewCount != int64(i+1) {
			t.Fatalf("view %d: expected count %d, got %+v", i, i+1, res)
		}
	}

	var stored models.Post
	if err := svc.db.Where("id = ?", post.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.ViewCount != 0 {
		t.Fatalf("views must stay buffered inside the window, database already has %d", stored.ViewCount)
	}

	svc.FlushViews(ctx)
	if err := svc.db.Where("id = ?", post.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload after flush: %v", err)
	}
	if stored.ViewCount != 5 {
		t.Fatalf("flush must write all five buffered views, got %d", stored.ViewCount)
	}
	var rows int64
	svc.db.Model(&models.PostView{}).Where("post_id = ?", post.ID).Count(&rows)
	if rows != 5 {
		t.Fatalf("flush must persist the de-duplication rows, got %d", rows)
	}
}

// De-duplication must work while the counts are still in memory: otherwise a
// reader refreshing the page twenty times inside one window would be counted
// twenty times and the batching would inflate the number it was meant to leave
// alone.
func TestViewsDedupeBeforeTheFlush(t *testing.T) {
	svc, post, now := bufferedPost(t)
	ctx := context.Background()
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) Safari/605.1"

	first, err := svc.ReportView(ctx, anonymous(), post.ID, viewMeta("192.0.2.7", ua, now))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if !first.Counted || first.ViewCount != 1 {
		t.Fatalf("first read should count, got %+v", first)
	}
	again, err := svc.ReportView(ctx, anonymous(), post.ID, viewMeta("192.0.2.7", ua, now.Add(5*time.Second)))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if again.Counted || again.Reason != "duplicate" || again.ViewCount != 1 {
		t.Fatalf("a repeat read must be a duplicate even before the flush, got %+v", again)
	}

	// And the memory of that visitor survives the flush, so the same reader is
	// still not counted a second time afterwards.
	svc.FlushViews(ctx)
	after, err := svc.ReportView(ctx, anonymous(), post.ID, viewMeta("192.0.2.7", ua, now.Add(10*time.Second)))
	if err != nil {
		t.Fatalf("after flush: %v", err)
	}
	if after.Counted || after.ViewCount != 1 {
		t.Fatalf("de-duplication must survive a flush, got %+v", after)
	}
}

// One flush carries every post that was touched in the window, not just the last
// one — the whole point is one transaction instead of one per read.
func TestFlushCoversEveryTouchedPost(t *testing.T) {
	svc, first, now := bufferedPost(t)
	ctx := context.Background()
	ua := "Mozilla/5.0 (X11; Linux x86_64) Firefox/131.0"

	second := makePostNoTime(t, svc.db, models.User{ID: first.AuthorID}, "published", "body", "second buffered")
	for i, id := range []string{first.ID, first.ID, second.ID} {
		if _, err := svc.ReportView(ctx, anonymous(), id,
			viewMeta(fmt.Sprintf("198.51.100.%d", i), ua, now.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("view on %s: %v", id, err)
		}
	}
	svc.FlushViews(ctx)

	var a, b models.Post
	svc.db.Where("id = ?", first.ID).First(&a)
	svc.db.Where("id = ?", second.ID).First(&b)
	if a.ViewCount != 2 || b.ViewCount != 1 {
		t.Fatalf("flush should write 2 and 1, got %d and %d", a.ViewCount, b.ViewCount)
	}
}
