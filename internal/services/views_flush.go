package services

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lopezpress/internal/models"
)

// Buffered view counting.
//
// Reporting a view used to cost two writes — the de-duplication INSERT and the
// counter UPDATE — on every single read. SQLite has one writer, and each
// committed write is a WAL append plus an fsync, so on a 1 vCPU box the busiest
// read path on the site was also its busiest write path: a hundred readers a
// minute produced two hundred serialised transactions to maintain a number
// nobody reads in real time.
//
// The counting now happens in memory and reaches the database in batches.
// Nothing is lost to a slow batch, and the worst case after a crash is the last
// flush window (60 seconds) of views. Two consequences are deliberate:
//
//   - De-duplication state is memory-only until the batch lands, so a process
//     that dies mid-window forgets which visitors it already counted and may
//     count them again after restart. That is a handful of views, once, in a
//     situation that already means the site went down.
//   - There is no background goroutine. The buffer flushes on the request that
//     notices it is due, which keeps the process free of a ticker that would
//     have to be reasoned about at every shutdown. A quiet site keeps its last
//     few counts in memory until the next reader arrives — and shutdown calls
//     FlushViews explicitly, so a clean stop writes them.

const (
	// viewFlushInterval bounds how long a counted view exists only in memory.
	viewFlushInterval = 60 * time.Second
	// viewFlushRows forces a flush when the buffer grows past this, so one busy
	// minute cannot turn into a single enormous transaction.
	viewFlushRows = 500
	// viewSeenCap bounds the per-day de-duplication set. Each entry is roughly
	// a UUID plus a 64-character hash, so this caps the map at a few megabytes
	// on a 1.5 GB box. Past the cap the set stops growing: the visitor is still
	// counted, but not remembered, so an extremely busy day can double-count a
	// few readers. Losing a real view would be worse.
	viewSeenCap = 100000
)

// countView applies in-memory de-duplication, buffers the increment, and
// returns the post's view count including everything not yet flushed.
func (s *Service) countView(ctx context.Context, post models.Post, hash string, now time.Time) (bool, int64) {
	day := now.UTC().Format("2006-01-02")

	s.viewMu.Lock()
	if s.viewSeenDay != day {
		// A new UTC day: yesterday's keys can never match again, so the set is
		// dropped. The pending counters and rows are NOT dropped — those are
		// views already counted and owed to the database, possibly from the
		// last minute of the previous day.
		s.viewSeenDay = day
		s.viewSeen = make(map[string]struct{}, 256)
	}
	key := post.ID + "|" + hash
	if _, seen := s.viewSeen[key]; seen {
		total := post.ViewCount + s.viewPending[post.ID]
		s.viewMu.Unlock()
		return false, total
	}
	if len(s.viewSeen) < viewSeenCap {
		s.viewSeen[key] = struct{}{}
	}
	s.viewPending[post.ID]++
	s.viewRows = append(s.viewRows, models.PostView{PostID: post.ID, VisitorHash: hash, Day: day})
	// The count this reader should see includes every buffered increment for
	// this post, not just their own: the returned number is the site's number.
	total := post.ViewCount + s.viewPending[post.ID]

	due := len(s.viewRows) >= viewFlushRows || now.Sub(s.viewLastFlush) >= viewFlushInterval
	var delta map[string]int64
	var rows []models.PostView
	if due {
		delta, rows = s.takeViewBufferLocked(now)
	}
	s.viewMu.Unlock()

	if due {
		s.flushViews(ctx, delta, rows)
	}
	return true, total
}

// takeViewBufferLocked hands the buffer to the caller and resets it. The caller
// must hold viewMu. Swapping the maps rather than clearing them means the flush
// writes a snapshot no concurrent request can mutate.
func (s *Service) takeViewBufferLocked(now time.Time) (map[string]int64, []models.PostView) {
	delta, rows := s.viewPending, s.viewRows
	s.viewPending = make(map[string]int64, 32)
	s.viewRows = nil
	s.viewLastFlush = now
	return delta, rows
}

// FlushViews writes whatever is buffered. Safe to call at any time; called on
// shutdown so a restart does not discard up to one flush interval of traffic.
func (s *Service) FlushViews(ctx context.Context) {
	s.viewMu.Lock()
	delta, rows := s.takeViewBufferLocked(time.Now())
	s.viewMu.Unlock()
	s.flushViews(ctx, delta, rows)
}

// flushViews writes one snapshot of counters and de-duplication rows.
//
// One transaction for the whole batch: the point of buffering is to pay the
// fsync once, and N autocommitted UPDATEs would pay it N times.
func (s *Service) flushViews(ctx context.Context, delta map[string]int64, rows []models.PostView) {
	if len(delta) == 0 && len(rows) == 0 {
		return
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, n := range delta {
			if e := tx.Model(&models.Post{}).Where("id = ?", id).
				UpdateColumn("view_count", gorm.Expr("view_count + ?", n)).Error; e != nil {
				return e
			}
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(&rows, viewFlushRows).Error
	})
	if err == nil {
		return
	}
	// Still owed to the database: put the snapshot back and let the next flush
	// retry. Re-inserting a de-duplication row is harmless (ON CONFLICT DO
	// NOTHING) and the counters are additive, so a retry can only repeat work,
	// never a count.
	s.viewMu.Lock()
	for id, n := range delta {
		s.viewPending[id] += n
	}
	s.viewRows = append(s.viewRows, rows...)
	s.viewMu.Unlock()
}
