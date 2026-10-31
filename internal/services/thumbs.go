package services

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/image/draw"
	// Registering the WebP decoder is process-wide: after this import,
	// image.Decode and image.DecodeConfig understand WebP everywhere,
	// including the geometry probe in media.go, which used to report
	// "0 x 0" for every WebP upload.
	_ "golang.org/x/image/webp"

	"lopezpress/internal/models"
)

// --- thumbnail pipeline ---------------------------------------
//
// The queue exists because of a resource constraint, not a correctness one: a
// 1C / 1.5 GB VPS that decodes eight 12-megapixel JPEGs at once is a VPS that
// stops answering HTTP. One goroutine, one image at a time, with a pause in
// between, keeps the decoder out of the request path and out of the way.
//
// The TODO list is the database, not the channel. `thumb_status = 'pending'`
// is the durable "not done yet" marker; the channel is only a wake-up hint
// with a bounded capacity. If it is full, the job is dropped and the row is
// still pending — the next boot re-scans and picks it up. Nothing is lost by
// a restart, and nothing blocks an upload when the queue is busy.

// Thumbnail states stored in upload_files.thumb_status.
const (
	ThumbStatusPending = "pending"
	ThumbStatusReady   = "ready"
	ThumbStatusSkip    = "skip"   // source already small: serve the original
	ThumbStatusFailed  = "failed" // undecodable or oversized: serve the original
)

const (
	// thumbMaxSide is the long-edge size of a generated thumbnail. Gallery
	// cards render at ~200 CSS px, so 480 covers a 2x display without
	// shipping a second copy that is nearly as big as the original.
	thumbMaxSide = 480
	thumbQuality = 82
	// thumbKeepBytes / thumbKeepPixels: below both thresholds the original is
	// already cheaper to send than a re-encode would be.
	thumbKeepBytes  = 96 * 1024
	thumbKeepPixels = 200 * 1000
	// thumbMaxPixels is the decode ceiling for the worker. media.go accepts
	// up to 40 MP at upload time (a phone photo is ~12 MP); decoding anything
	// near that inside a background goroutine on 1.5 GB is asking for an OOM
	// kill, so the worker is stricter and simply leaves the original in place.
	thumbMaxPixels = 24 * 1000 * 1000
	// thumbQueueCap bounds the in-memory hint queue.
	thumbQueueCap = 32
	// thumbGap is the pause between two jobs. It lets the HTTP server answer
	// between images on a single-core box.
	thumbGap = 150 * time.Millisecond
	// thumbResumeBatch caps one startup scan so a library with thousands of
	// pending rows does not build one huge slice on boot. Anything beyond it
	// is picked up by the periodic sweep below.
	thumbResumeBatch = 200
	// thumbSweep is how often an idle worker re-reads the durable queue. This
	// is what makes the batch cap safe: without it, a backlog larger than
	// thumbResumeBatch would only drain after a restart. On a steady-state
	// system the query returns nothing and costs less than a request.
	thumbSweep = 30 * time.Second
)

// StartThumbWorker launches the single consumer goroutine and re-queues
// whatever a previous run left unfinished. Idempotent: calling it twice on the
// same Service is a no-op.
func (s *Service) StartThumbWorker(ctx context.Context) {
	s.thumbsOnce.Do(func() { go s.runThumbWorker(ctx) })
}

// enqueueThumb hands one row id to the worker without ever blocking. A full
// queue simply drops the hint; the row stays pending and is retried on the
// next boot, which is the whole reason the state lives in the database.
func (s *Service) enqueueThumb(id string) {
	if id == "" || s.thumbs == nil {
		return
	}
	select {
	case s.thumbs <- id:
	default:
	}
}

// runThumbWorker is the loop. It drains jobs one by one and sleeps in between,
// so peak cost is a single decode rather than "as many as were uploaded".
//
// The sweep ticker is what turns the bounded startup scan into a complete one:
// each pass queues at most thumbResumeBatch rows, and the next pass picks up
// the next batch. Select picks uniformly among ready cases, so a busy queue
// cannot starve the sweep.
func (s *Service) runThumbWorker(ctx context.Context) {
	s.resumePendingThumbs(ctx, true)
	sweep := time.NewTicker(thumbSweep)
	defer sweep.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-s.thumbs:
			s.buildThumb(ctx, id)
			select {
			case <-ctx.Done():
				return
			case <-time.After(thumbGap):
			}
		case <-sweep.C:
			s.resumePendingThumbs(ctx, false)
		}
	}
}

// resumePendingThumbs re-queues rows left unfinished by a previous process.
//
// The `thumb_status = ”` branch covers rows written before the worker existed
// (the column is empty for them); once a row is ready/skip/failed it never
// matches again, so the steady-state cost of this query is one indexed scan
// that returns nothing. announce is false for the periodic sweep, which must
// not fill the log with the same line every half minute.
func (s *Service) resumePendingThumbs(ctx context.Context, announce bool) {
	var ids []string
	err := s.db.WithContext(ctx).Model(&models.UploadFile{}).
		Where("deleted_at IS NULL AND (thumb_status = ? OR thumb_status = '')", ThumbStatusPending).
		Order("created_at ASC").Limit(thumbResumeBatch).Pluck("id", &ids).Error
	if err != nil {
		log.Printf("WARN thumbnail queue: scan failed: %v", err)
		return
	}
	for _, id := range ids {
		s.enqueueThumb(id)
	}
	if announce && len(ids) > 0 {
		log.Printf("thumbnail queue: %d image(s) waiting", len(ids))
	}
}

// buildThumb renders one thumbnail. Every failure path leaves the original
// image usable — a missing thumbnail is a cosmetic regression, a broken image
// is not.
func (s *Service) buildThumb(ctx context.Context, id string) {
	var row models.UploadFile
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return
	}
	// The row may have been withdrawn or deleted between queueing and now.
	if row.DeletedAt != nil || row.ThumbStatus == ThumbStatusReady {
		return
	}

	data, err := os.ReadFile(filepath.Join(s.uploadDir(), row.Name))
	if err != nil {
		s.markThumb(ctx, id, ThumbStatusFailed, "")
		return
	}

	out, ext, status := renderThumb(data)
	if status != ThumbStatusReady {
		s.markThumb(ctx, id, status, "")
		return
	}
	name := row.ID + "_t" + ext
	if err := s.writeUploadFile(name, out); err != nil {
		s.markThumb(ctx, id, ThumbStatusFailed, "")
		return
	}
	s.markThumb(ctx, id, ThumbStatusReady, name)
}

// markThumb records the outcome. Errors are ignored on purpose: the column is
// an optimisation hint, and failing to update it must never fail the upload or
// the delete that triggered it.
func (s *Service) markThumb(ctx context.Context, id, status, name string) {
	if err := s.db.WithContext(ctx).Model(&models.UploadFile{}).Where("id = ?", id).
		Updates(map[string]any{"thumb_status": status, "thumb_name": name}).Error; err != nil {
		log.Printf("WARN thumbnail queue: cannot record %s for %s: %v", status, id, err)
	}
}

// renderThumb is the pure part: bytes in, bytes out. It returns a status
// rather than an error because "no thumbnail" and "no thumbnail, and stop
// trying" are different outcomes (skip vs failed) and both are normal.
//
// The format decision is made after scaling, on the result: if the scaled
// image has no transparency left, JPEG is roughly a third of the size of PNG
// at visually identical quality. Sources that do have transparency stay PNG,
// because JPEG would turn every transparent pixel black.
func renderThumb(data []byte) (out []byte, ext string, status string) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, "", ThumbStatusFailed
	}
	if int64(cfg.Width)*int64(cfg.Height) > thumbMaxPixels {
		return nil, "", ThumbStatusFailed
	}
	long := cfg.Width
	if cfg.Height > long {
		long = cfg.Height
	}
	if long <= thumbMaxSide && len(data) <= thumbKeepBytes {
		return nil, "", ThumbStatusSkip
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", ThumbStatusFailed
	}
	w, h := fitWithin(cfg.Width, cfg.Height, thumbMaxSide)
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	// CatmullRom rather than NearestNeighbor: thumbnails are viewed at size,
	// and the difference on photographic content is the difference between
	// "small picture" and "mud".
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)

	var buf bytes.Buffer
	if dst.Opaque() {
		ext = ".jpg"
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbQuality})
	} else {
		ext = ".png"
		err = png.Encode(&buf, dst)
	}
	if err != nil {
		return nil, "", ThumbStatusFailed
	}
	// A re-encode that came out bigger is not worth a second file: a highly
	// compressible PNG at 600px can be smaller than the same picture as a
	// 480px JPEG.
	if buf.Len() >= len(data) {
		return nil, "", ThumbStatusSkip
	}
	return buf.Bytes(), ext, ThumbStatusReady
}

// fitWithin scales (w, h) down so the long edge is at most max, preserving the
// aspect ratio and never returning zero.
func fitWithin(w, h, max int) (int, int) {
	if w <= max && h <= max {
		return w, h
	}
	if w >= h {
		nh := h * max / w
		if nh < 1 {
			nh = 1
		}
		return max, nh
	}
	nw := w * max / h
	if nw < 1 {
		nw = 1
	}
	return nw, max
}

// thumbURL is the URL the UI should load for a card, or "" when the original
// is the right thing to serve (not generated yet, skipped, or failed).
func thumbURL(f models.UploadFile) string {
	if f.ThumbStatus == ThumbStatusReady && f.ThumbName != "" {
		return "/uploads/" + f.ThumbName
	}
	return ""
}

// needsThumb reports whether a row still deserves a worker pass. The three
// terminal states (ready / skip / failed) must not be re-queued, or every
// duplicate upload of the same bytes would schedule a decode that ends in the
// same place.
func needsThumb(f models.UploadFile) bool {
	switch f.ThumbStatus {
	case ThumbStatusReady, ThumbStatusSkip, ThumbStatusFailed:
		return false
	}
	return true
}

// removeThumb deletes the generated file of a row that is being purged. A
// soft delete keeps it: the original is still served to published articles,
// and the thumbnail is just as harmless sitting there.
func (s *Service) removeThumb(ctx context.Context, name string) {
	if name == "" {
		return
	}
	_ = s.DeleteLocalUpload(ctx, name)
}

// attachCoverThumbs fills CoverThumb on posts whose cover is a local image that
// already has a generated thumbnail. Best-effort and read-only: a cover that is
// an external URL, or whose thumbnail the worker has not produced yet, simply
// leaves CoverThumb empty and the client falls back to the original.
//
// One query for the whole page rather than one per post: listings are the most
// read endpoint on the site, and a per-card lookup would add ten round trips to
// a single page view on a 1 vCPU box.
func (s *Service) attachCoverThumbs(ctx context.Context, posts []models.Post) {
	names := make([]string, 0, len(posts))
	for i := range posts {
		if n := uploadNameFromURL(posts[i].Cover); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return
	}
	var rows []models.UploadFile
	if err := s.db.WithContext(ctx).Select("name, thumb_name, thumb_status").
		Where("name IN ?", names).Find(&rows).Error; err != nil {
		return
	}
	thumbs := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.ThumbStatus == ThumbStatusReady && r.ThumbName != "" {
			thumbs[r.Name] = "/uploads/" + r.ThumbName
		}
	}
	for i := range posts {
		if n := uploadNameFromURL(posts[i].Cover); n != "" {
			posts[i].CoverThumb = thumbs[n]
		}
	}
}
