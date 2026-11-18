package services

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"

	xdraw "golang.org/x/image/draw"
)

// --- upload options: resize + compression -----------------------------------
//
// A picture is often needed at more than one size: a 1920px version inside the
// article and a 640px version as the cover. Before this file the only choice
// was "store the original bytes", so authors either shipped a 6 MB phone photo
// into every card or edited the file outside the site.
//
// The rules are deliberately narrow:
//   - the aspect ratio is always preserved exactly (one scale factor for both
//     axes), and an image is never upscaled;
//   - PNG/JPEG are decoded and re-encoded, which also strips trailing payloads
//     (the security property of the old path, kept intact);
//   - WebP has no encoder here, so it never accepts a transform;
//   - GIF is never re-encoded. image.Decode yields only the first frame, so any
//     re-encode silently freezes an animation. Resize is refused, and the
//     bytes are stored verbatim (the same trade-off as WebP; the header checks
//     in isAllowedImageContent still prove it is a real GIF);
//   - "lossless" means JPEG quality 100; PNG stays pixel-lossless by definition
//     and ignores the quality setting.

// normalizeUploadOptions clamps every field instead of rejecting it. A slider
// that posts 49 or 101 should produce the nearest legal quality, not a failed
// upload after the bytes were sent.
func normalizeUploadOptions(o UploadOptions) UploadOptions {
	if o.MaxWidth < 0 {
		o.MaxWidth = 0
	}
	if o.MaxHeight < 0 {
		o.MaxHeight = 0
	}
	if o.MaxWidth > maxImageSide {
		o.MaxWidth = maxImageSide
	}
	if o.MaxHeight > maxImageSide {
		o.MaxHeight = maxImageSide
	}
	if o.Quality != 0 {
		if o.Quality < 50 {
			o.Quality = 50
		}
		if o.Quality > 100 {
			o.Quality = 100
		}
	}
	if o.Lossless {
		o.Quality = 100
	}
	return o
}

// jpegQualityFor resolves the encoder quality for one upload.
func (o UploadOptions) jpegQualityFor() int {
	if o.Lossless {
		return 100
	}
	if o.Quality > 0 {
		return o.Quality
	}
	return jpegQuality
}

// wantsResize reports whether either bound would actually change anything.
func (o UploadOptions) wantsResize() bool {
	return o.MaxWidth > 0 || o.MaxHeight > 0
}

// fitDimensions returns the target size for one image, preserving the aspect
// ratio and never upscaling.
//
// maxW/maxH are a bounding BOX, not independent limits: the smaller of the two
// scale factors wins, so a 4000×2000 picture with "1920×1920" becomes
// 1920×960, not 1920×2000.
func fitDimensions(w, h, maxW, maxH int) (int, int) {
	if w <= 0 || h <= 0 || (maxW <= 0 && maxH <= 0) {
		return w, h
	}
	scale := 1.0
	if maxW > 0 && w > maxW {
		scale = math.Min(scale, float64(maxW)/float64(w))
	}
	if maxH > 0 && h > maxH {
		scale = math.Min(scale, float64(maxH)/float64(h))
	}
	if scale >= 1 {
		return w, h
	}
	tw := int(math.Round(float64(w) * scale))
	th := int(math.Round(float64(h) * scale))
	if tw < 1 {
		tw = 1
	}
	if th < 1 {
		th = 1
	}
	// Rounding can land one pixel past a bound; clamp so the promise
	// "never larger than the requested size" is exact.
	if maxW > 0 && tw > maxW {
		tw = maxW
	}
	if maxH > 0 && th > maxH {
		th = maxH
	}
	return tw, th
}

// transformDescription renders the applied recipe for the library row. Empty
// means "the stored bytes are the source bytes".
func transformDescription(srcW, srcH, dstW, dstH, quality int, ext string) string {
	parts := ""
	if dstW != srcW || dstH != srcH {
		parts = fmt.Sprintf("%dx%d", dstW, dstH)
	}
	if normalizeExt(ext) == ".jpg" || normalizeExt(ext) == ".jpeg" {
		if quality != jpegQuality {
			if parts != "" {
				parts += "-"
			}
			parts += fmt.Sprintf("q%d", quality)
		}
	}
	return parts
}

// scaleImage applies the requested bounding box and returns the bitmap plus its
// final dimensions. When the box already fits, the original bitmap comes back
// untouched.
//
// CatmullRom is the quality/speed point that matches what image tools produce
// for a downscale; NearestNeighbor would alias text and ApproxBiLinear softens
// screenshots noticeably.
func scaleImage(src image.Image, w, h int, opts UploadOptions) (image.Image, int, int) {
	if !opts.wantsResize() {
		return src, w, h
	}
	dstW, dstH := fitDimensions(w, h, opts.MaxWidth, opts.MaxHeight)
	if dstW == w && dstH == h {
		return src, w, h
	}
	canvas := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return canvas, dstW, dstH
}

// transformImageBytes decodes, optionally resizes, and re-encodes the image.
// It is the single place that turns "what the author uploaded" into "what is
// stored and charged to the quota".
func transformImageBytes(data []byte, ext string, opts UploadOptions) ([]byte, string, error) {
	ext = normalizeExt(ext)
	srcW, srcH, _ := imageGeometry(data)

	// WebP: the x/image decoder is registered process-wide, but there is no
	// encoder. Passing an unchanged WebP through keeps the old behaviour;
	// accepting a resize would silently return the original dimensions.
	if ext == ".webp" {
		if opts.wantsResize() {
			return nil, "", newErr(http.StatusBadRequest,
				"WebP 不支持服务端缩放，请先转换格式或选择原图上传")
		}
		return data, "", nil
	}
	// GIF: storing verbatim is the only way to keep every frame. A re-encode
	// here would run through image.Decode (first frame) and gif.Encode (single
	// frame), turning an animation into a still image that the library would
	// still label "original". Resize is refused because it needs a re-encode.
	if ext == ".gif" {
		if opts.wantsResize() {
			return nil, "", newErr(http.StatusBadRequest,
				"GIF 不支持缩放（会丢失动画），请选择原图上传")
		}
		return data, "", nil
	}

	decodeSemaphore <- struct{}{}
	defer func() { <-decodeSemaphore }()

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", newErr(http.StatusBadRequest, "file content is not a supported image")
	}
	dst, dstW, dstH := scaleImage(src, srcW, srcH, opts)

	quality := opts.jpegQualityFor()
	var buf bytes.Buffer
	switch ext {
	case ".png":
		enc := &png.Encoder{CompressionLevel: png.BestCompression}
		err = enc.Encode(&buf, dst)
	case ".jpg", ".jpeg":
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality})
	default:
		return nil, "", newErr(http.StatusBadRequest, "unsupported image format")
	}
	if err != nil {
		return nil, "", newErr(http.StatusInternalServerError, "failed to process image")
	}
	return buf.Bytes(), transformDescription(srcW, srcH, dstW, dstH, quality, ext), nil
}
