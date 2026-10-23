package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lopezpress/internal/models"
)

// ViewMeta carries the request-derived inputs for view de-duplication.
//
// Now is injectable so tests can move between days without sleeping; a zero
// value means time.Now().
type ViewMeta struct {
	IP  string
	UA  string
	Now time.Time
}

// ViewResult is the body of POST /api/posts/{id}/view (spec 23).
//
// Counted=false comes with a Reason ("duplicate" / "author" / "bot") so the
// frontend can decide whether to show anything; the count is always the
// post's current value so the UI can update itself even when not counted.
type ViewResult struct {
	Counted   bool   `json:"counted"`
	ViewCount int64  `json:"view_count"`
	Reason    string `json:"reason,omitempty"`
}

// viewRetentionDays is how long de-duplication rows are kept. They exist only
// to answer "did this visitor already read this post today", so anything older
// than a few months is dead weight on a 5 GB disk.
const viewRetentionDays = 90

// botUAHints are substrings (lowercase) that identify non-human readers. View
// counts should reflect people; crawlers, monitoring probes and CLI clients
// are excluded.
var botUAHints = []string{
	"bot", "spider", "crawler", "crawl", "slurp", "scrapy",
	"headlesschrome", "phantomjs", "puppeteer", "playwright",
	"curl/", "wget/", "python-requests", "python-urllib", "go-http-client",
	"java/", "okhttp", "libwww", "httpclient",
	"facebookexternalhit", "embedly", "quora link preview", "whatsapp",
	"uptimerobot", "pingdom", "monitoring",
}

// isBotUA reports whether ua looks like an automated client. An empty UA is
// treated as human-ish (some privacy browsers omit it) — the daily
// de-duplication still caps the damage at one count per day.
func isBotUA(ua string) bool {
	if ua == "" {
		return false
	}
	lower := strings.ToLower(ua)
	for _, hint := range botUAHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// visitorHash derives the opaque de-duplication key. JWT_SECRET is mixed in so
// the hash cannot be reproduced (and IPv4 space enumerated) from outside.
func (s *Service) visitorHash(ip, ua string) string {
	secret := ""
	if s.cfg != nil {
		secret = s.cfg.JWTSecret
	}
	sum := sha256.Sum256([]byte(ip + "|" + ua + "|" + secret))
	return hex.EncodeToString(sum[:])
}

// ReportView records one read of a post, de-duplicated per visitor per UTC
// day (spec 23).
//
// Returns 404 (not 403) for posts the caller may not read and for any
// non-published status, matching the public read path so the endpoint never
// leaks the existence of private/pending/rejected/banned content.
//
// The counter is incremented only when the INSERT actually created a row:
// the unique index on (post_id, visitor_hash, day) is the de-duplication
// mechanism, not application logic.
func (s *Service) ReportView(ctx context.Context, user models.User, postID string, meta ViewMeta) (ViewResult, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", postID).First(&post).Error; err != nil {
		return ViewResult{}, newErr(http.StatusNotFound, "post not found")
	}
	if post.Status != models.PostPublished {
		return ViewResult{}, newErr(http.StatusNotFound, "post not found")
	}

	res := ViewResult{ViewCount: post.ViewCount}

	// The author reading their own post is not traffic.
	if user.ID != "" && user.ID == post.AuthorID {
		res.Reason = "author"
		return res, nil
	}
	if isBotUA(meta.UA) {
		res.Reason = "bot"
		return res, nil
	}

	now := meta.Now
	if now.IsZero() {
		now = time.Now()
	}
	day := now.UTC().Format("2006-01-02")

	rec := models.PostView{
		PostID:      post.ID,
		VisitorHash: s.visitorHash(meta.IP, meta.UA),
		Day:         day,
	}
	ins := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rec)
	if ins.Error != nil {
		return ViewResult{}, newErr(http.StatusInternalServerError, "failed to record view")
	}
	if ins.RowsAffected == 0 {
		// Same visitor, same post, same day: already counted.
		res.Reason = "duplicate"
		return res, nil
	}

	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", post.ID).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
		return ViewResult{}, newErr(http.StatusInternalServerError, "failed to update view count")
	}
	res.Counted = true
	res.ViewCount = post.ViewCount + 1

	s.pruneViews(ctx, now)
	return res, nil
}

// pruneViews deletes de-duplication rows past the retention window. It runs at
// most once per process per UTC day (guarded by viewPruneDay) so the write path
// stays a single INSERT in the common case.
func (s *Service) pruneViews(ctx context.Context, now time.Time) {
	day := now.UTC().Format("2006-01-02")
	s.viewPruneMu.Lock()
	if s.viewPruneDay == day {
		s.viewPruneMu.Unlock()
		return
	}
	s.viewPruneDay = day
	s.viewPruneMu.Unlock()

	cutoff := now.UTC().AddDate(0, 0, -viewRetentionDays).Format("2006-01-02")
	// Best-effort: a failed prune must never fail the view report.
	_ = s.db.WithContext(ctx).Where("day < ?", cutoff).Delete(&models.PostView{}).Error
}
