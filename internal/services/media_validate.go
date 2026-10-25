package services

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
)

// --- image content validation ------------------------------------------------
//
// Split out of media.go: "may these bytes be stored as an image" is a
// self-contained question with its own history of being answered too weakly,
// and it is the part an attacker aims at. Keeping it in its own file makes the
// whole rule set — sniff, decode header, decode pixels, strip the rest —
// readable in one place.

// Bounds for a decoded image. They are not a substitute for the upload byte
// cap: a small PNG can decode to a gigapixel canvas (a decompression bomb),
// which is why the geometry is checked from the header before any pixel data
// is touched.
const (
	maxImageSide   = 12000
	maxImagePixels = 40 * 1000 * 1000
)

// isAllowedImageContent reports whether the bytes are a real raster image
// (PNG / JPEG / GIF / WebP) of sane dimensions.
//
// The first check is the stdlib content sniffer (512 bytes), which already
// rejects HTML/SVG payloads renamed to .png. But a sniffed header is only a
// few bytes: an 8-byte PNG signature followed by zeros used to be accepted,
// and so was a GIF89a header glued to an HTML document (polyglot). Decoding
// the header with image.DecodeConfig proves the format actually parses and
// yields its geometry, which closes both cases without adding a dependency.
//
// DecodeConfig only reads the IHDR/SOF header, though — it never looks at the
// pixel data that follows. That is why the upload path ALSO does a full
// image.Decode and re-encodes from the decoded image (see
// reencodeStrippedImage): without it, a 64×64 PNG header followed by 1 MB of
// arbitrary bytes — or by "<?php system($_GET['c']); ?>" — was stored whole
// and served back with Content-Type: image/png, which made the site a free
// binary host with a trustworthy-looking extension (R4-09 / R6-04). Anything
// after IEND is simply not part of the image and is dropped.
//
// WebP is the exception: golang.org/x/image/webp (registered in thumbs.go) is
// a decoder, not a format police, and it refuses animated WebP outright. A
// legitimate animated sticker would be rejected if the magic bytes were not
// enough, so for WebP the sniffed header stays the authority and the geometry
// probe is best-effort. The full-decode step in UploadImage applies the same
// exception.
func isAllowedImageContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	switch http.DetectContentType(data) {
	case "image/webp":
		return true
	case "image/png", "image/jpeg", "image/gif":
	default:
		return false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return false
	}
	if cfg.Width > maxImageSide || cfg.Height > maxImageSide {
		return false
	}
	return int64(cfg.Width)*int64(cfg.Height) <= maxImagePixels
}

// isImplausiblyCompressible reports whether an image's byte size is impossibly
// small for its pixel count — the signature of a solid-colour or
// procedurally-generated canvas uploaded to exploit the fact that the quota
// counts BYTES while the cost of handling an image is measured in PIXELS.
//
// R6-05 measured it: 6000×4000 (24 MP) grey PNGs at 23.4 KB each. Twenty of
// them fit inside a 20 MB quota, and four of them — 93.8 KB of "storage" —
// drove RSS from 261 MB to 356 MB and pinned a core in the thumbnail worker
// for the better part of a minute, because each one decodes to a ~96 MB
// bitmap. On a 1.5 GB box that is a denial of service reachable by any author
// with a script.
//
// A real photograph runs the other way: a 12 MP phone shot is megabytes, i.e.
// well over a byte per hundred pixels. The threshold below (one byte per 200
// pixels) leaves every legitimate image — including flat illustration and
// line art, which land around one byte per fifty — orders of magnitude of
// headroom while catching a canvas that is, by construction, almost entirely
// one colour.
//
// Only large images are tested: a 200×150 flat icon is legitimately tiny in
// both dimensions, and rejecting it would be a false positive with no
// security benefit.
func isImplausiblyCompressible(sizeBytes int64, pixels int64) bool {
	const minPixelsToCheck = 4 * 1000 * 1000 // 4 MP
	if pixels < minPixelsToCheck {
		return false
	}
	// sizeBytes * 200 < pixels  <=>  fewer than 1 byte per 200 pixels.
	return sizeBytes*200 < pixels
}
// reencodeStrippedImage decodes the image fully and re-encodes it, returning
// only the bytes of a well-formed image. Everything the decoder ignores —
// trailing garbage after IEND, appended payloads, malformed chunk data — is
// gone, so what lands on disk is exactly what a decoder can read back.
//
// Returns (nil, false) when the format is exempt (WebP: no stdlib encoder) or
// when the full decode fails, which the caller treats as "keep the original
// bytes" for WebP and as "reject" for the formats we claim to validate.
func reencodeStrippedImage(data []byte, ext string) ([]byte, bool) {
	switch normalizeExt(ext) {
	case ".png", ".jpg", ".jpeg", ".gif":
	default:
		return nil, false // webp: no encoder in the stdlib / x-image set
	}
	// The decode is the expensive step: the pixel ceiling above caps how big one
	// decode can be, and the semaphore caps how many are running at once. The
	// slot is held across the re-encode too, so the peak memory is bounded by
	// (maxUploadPixels × 4 bytes) × 2, not by however many clients upload at the
	// same moment.
	decodeSemaphore <- struct{}{}
	defer func() { <-decodeSemaphore }()

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	switch normalizeExt(ext) {
	case ".png":
		err = png.Encode(&buf, src)
	case ".jpg", ".jpeg":
		err = jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90})
	case ".gif":
		err = gif.Encode(&buf, src, nil)
	}
	if err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// normalizeExt lower-cases an extension so the switch above is case-insensitive
// (a ".PNG" upload is the same format as ".png").
func normalizeExt(ext string) string {
	out := make([]byte, 0, len(ext))
	for i := 0; i < len(ext); i++ {
		c := ext[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return string(out)
}
// reencodeStrippedImage decodes the image fully and re-encodes it, returning
// only the bytes of a well-formed image. Everything the decoder ignores —

// prepareUploadBytes is the last gate before bytes are stored. It answers
// three questions that all amount to "is this really an image we should keep",
// and returns the bytes that should actually be written:
//
//  1. Is the byte size plausible for the pixel count? (R6-05) The quota counts
//     bytes; the cost of an image is counted in pixels. A 24 MP solid-colour
//     PNG is ~23 KB of "storage" and ~96 MB of decode, so twenty of them fit
//     in a 20 MB quota and four of them moved RSS by 95 MB. Refuse the
//     impossible combination.
//  2. Does the file decode all the way through? (R4-09 / R6-04) The header
//     checks prove it STARTS as an image; a full image.Decode proves it is one
//     ALL THE WAY THROUGH. A 64×64 PNG header followed by 1 MB of arbitrary
//     bytes — or by "<?php system($_GET['c']); ?>" — passed every earlier check
//     and was stored verbatim, then served back as image/png.
//  3. Re-encode from the decoded bitmap, so everything the decoder ignored is
//     gone and what lands on disk is exactly what a decoder can read back.
//
// The returned bytes are what the quota is charged for, which is the size that
// actually occupies the disk.
//
// WebP is exempt from (2) and (3): the registered decoder refuses animated
// WebP, and there is no WebP encoder in the standard library set, so a
// legitimate animated sticker would be rejected by a full re-encode.
func prepareUploadBytes(data []byte, ext string) ([]byte, error) {
	if w, h, _ := imageGeometry(data); w > 0 && h > 0 {
		// Hard pixel ceiling, independent of compression. The compression guard
		// below catches the "24 MP in 23 KB" lie; this catches the honest case
		// that is still too expensive — a real 30 MP panorama decodes to a
		// ~120 MB RGBA buffer, and a handful of those, concurrent, is how a
		// 1.5 GB box dies. The limit matches the thumbnail worker's decode
		// ceiling, so one upload never decodes larger than the background job
		// is already prepared to handle.
		if int64(w)*int64(h) > maxUploadPixels {
			return nil, newErr(http.StatusRequestEntityTooLarge,
				fmt.Sprintf("image is too large (max %d megapixels)", maxUploadPixels/1000000))
		}
		if isImplausiblyCompressible(int64(len(data)), int64(w)*int64(h)) {
			return nil, newErr(http.StatusBadRequest,
				"this image is too large in pixels for its file size; re-export it as a normal photo")
		}
	}
	if stripped, ok := reencodeStrippedImage(data, ext); ok {
		return stripped, nil
	}
	if normalizeExt(ext) != ".webp" {
		// The header decoded but the pixels did not: truncated or corrupt.
		// Refuse rather than store something we cannot render.
		return nil, newErr(http.StatusBadRequest, "file content is not a supported image")
	}
	return data, nil
}

// maxUploadPixels caps a single upload's decode size. A decoded image is RGBA
// (4 bytes/pixel) at minimum, so 24 MP is a ~96 MB buffer — the largest single
// allocation the request path will make, and the same ceiling the thumbnail
// worker already uses.
const maxUploadPixels = 24 * 1000 * 1000

// decodeSemaphore bounds concurrent full decodes. It does not cap the size of
// any one image (maxUploadPixels does that); it caps how many of those buffers
// exist at once. R7 measured a single 24 MP upload moving RSS by ~179 MB; two
// in flight are ~360 MB, and on a 1.5 GB box the third one kills the process.
var decodeSemaphore = make(chan struct{}, 2)
