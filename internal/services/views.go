package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"lopezpress/internal/models"
)

// ViewResult is the body of POST /api/posts/{id}/view.
//
// Counted=false comes with a Reason ("duplicate" / "author" / "bot") so the
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
// day.
//
// Returns 404 (not 403) for posts the caller may not read and for any
// non-published status, matching the public read path so the endpoint never
// leaks the existence of private/pending/rejected/banned content.
//
// The counting itself is buffered in memory and written in batches; see
// views_flush.go for why and for what a crash costs.
func (s *Service) ReportView(ctx context.Context, user models.User, postID string, meta ViewMeta) (ViewResult, error) {
	post, err := s.countablePost(ctx, postID)
	if err != nil {
		return ViewResult{}, err
	}

	// The author reading their own post is not traffic.
	if user.ID != "" && user.ID == post.AuthorID {
		return ViewResult{ViewCount: post.ViewCount, Reason: "author"}, nil
	}
	if isBotUA(meta.UA) {
		return ViewResult{ViewCount: post.ViewCount, Reason: "bot"}, nil
	}

	now := meta.Now
	if now.IsZero() {
		now = time.Now()
	}
	counted, total := s.countView(ctx, post, s.visitorHash(meta.IP, meta.UA), now)
	res := ViewResult{ViewCount: total}
	if !counted {
		res.Reason = "duplicate"
		return res, nil
	}
	res.Counted = true
	s.pruneViews(ctx, now)
	return res, nil
}

// countablePost loads a post only if a view of it can be counted.
func (s *Service) countablePost(ctx context.Context, postID string) (models.Post, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", postID).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	if post.Status != models.PostPublished {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	return post, nil
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
