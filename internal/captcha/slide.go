package captcha

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math/rand"
)

// The slide challenge is generated here rather than pulled from
// github.com/wenlng/go-captcha because that library ships its own font and
// background-image assets. On a 1-core box the extra megabytes and the extra
// image decode per request are not worth it for a 300x150 picture we can draw
// with the standard library in about a millisecond.
//
// What the browser gets is two images: the background, which already has a
// dimmed notch showing where the piece belongs, and the piece itself. The
// notch's horizontal position is the answer and never leaves the server — the
// browser can see *that* a gap exists, not *where* the number is.

const (
	slideWidth     = 300
	slideHeight    = 150
	slideTile      = 44
	slideMargin    = 10
	slideTolerance = 7
	// slideXScale means the browser reports its drop position in per-mille of
	// the image width rather than in CSS pixels. The form is responsive and
	// the image is scaled on narrow screens; a pixel offset would then disagree
	// with the server's coordinates by exactly the scale factor.
	slideXScale = 1000

	slideDots = 26
)

type slideChallenge struct {
	bg   string
	tile string
}

// newSlide returns the two data URIs plus the target coordinates.
func newSlide() (slideChallenge, int, int, int, error) {
	size := slideTile

	// The piece must start clear of both edges: a notch flush with the border
	// is a giveaway, and there has to be room for the piece to be dragged to.
	spanX := slideWidth - size - 2*slideMargin
	spanY := slideHeight - size - 2*slideMargin
	offX, err := randomInt(spanX)
	if err != nil {
		return slideChallenge{}, 0, 0, 0, err
	}
	offY, err := randomInt(spanY)
	if err != nil {
		return slideChallenge{}, 0, 0, 0, err
	}
	x := offX + slideMargin
	y := offY + slideMargin

	bg := renderBackground(slideWidth, slideHeight)

	// Crop the piece BEFORE punching the notch: the piece has to carry the
	// original pixels or it will never match what it covers.
	piece := crop(bg, x, y, size)
	addPieceBorder(piece)
	punchNotch(bg, x, y, size)

	bgURI, err := encodePNG(bg)
	if err != nil {
		return slideChallenge{}, 0, 0, 0, err
	}
	tileURI, err := encodePNG(piece)
	if err != nil {
		return slideChallenge{}, 0, 0, 0, err
	}
	return slideChallenge{bg: bgURI, tile: tileURI}, x, y, size, nil
}

// renderBackground paints a soft vertical gradient with scattered dots. The
// palette stays light: the site is a light theme, and a dark captcha inside a
// white form reads as a broken image.
func renderBackground(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	top := lightColor()
	bottom := lightColor()
	for y := 0; y < h; y++ {
		t := float64(y) / float64(h)
		c := color.RGBA{
			R: lerp(top.R, bottom.R, t),
			G: lerp(top.G, bottom.G, t),
			B: lerp(top.B, bottom.B, t),
			A: 255,
		}
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}

	// Dots add the high-frequency detail that makes the notch edge hard to
	// find by simply looking for a flat region.
	for i := 0; i < slideDots; i++ {
		cx := rand.Intn(w)
		cy := rand.Intn(h)
		r := 2 + rand.Intn(5)
		c := lightColor()
		c.A = 190
		fillCircle(img, cx, cy, r, c)
	}

	// A couple of slanted bands break the gradient's horizontal symmetry.
	for i := 0; i < 3; i++ {
		y0 := rand.Intn(h)
		thick := 2 + rand.Intn(4)
		c := lightColor()
		c.A = 120
		for x := 0; x < w; x++ {
			y := y0 + x/6
			for t := 0; t < thick; t++ {
				if y+t >= 0 && y+t < h {
					img.SetRGBA(x, y+t, blend(img.RGBAAt(x, y+t), c))
				}
			}
		}
	}
	return img
}

// crop copies a square region out of src.
func crop(src *image.RGBA, x, y, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	for j := 0; j < size; j++ {
		for i := 0; i < size; i++ {
			dst.SetRGBA(i, j, src.RGBAAt(x+i, y+j))
		}
	}
	return dst
}

// addPieceBorder gives the piece a visible edge so it is findable over the
// background while it is being dragged.
func addPieceBorder(piece *image.RGBA) {
	b := piece.Bounds()
	edge := color.RGBA{R: 255, G: 255, B: 255, A: 210}
	for i := 0; i < b.Dx(); i++ {
		piece.SetRGBA(i, 0, edge)
		piece.SetRGBA(i, b.Dy()-1, edge)
	}
	for j := 0; j < b.Dy(); j++ {
		piece.SetRGBA(0, j, edge)
		piece.SetRGBA(b.Dx()-1, j, edge)
	}
}

// punchNotch dims the target square so the user can see where the piece goes.
// This is the only public hint about the answer, and it is intentional: a
// slider with no visible target is unusable.
func punchNotch(bg *image.RGBA, x, y, size int) {
	shadow := color.RGBA{R: 60, G: 60, B: 70, A: 130}
	for j := 0; j < size; j++ {
		for i := 0; i < size; i++ {
			px := x + i
			py := y + j
			if px < 0 || py < 0 || px >= slideWidth || py >= slideHeight {
				continue
			}
			bg.SetRGBA(px, py, blend(bg.RGBAAt(px, py), shadow))
		}
	}
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	b := img.Bounds()
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
				continue
			}
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= r*r {
				img.SetRGBA(x, y, blend(img.RGBAAt(x, y), c))
			}
		}
	}
}

// blend composites src over dst using src's alpha.
func blend(dst, src color.RGBA) color.RGBA {
	a := float64(src.A) / 255
	return color.RGBA{
		R: uint8(float64(src.R)*a + float64(dst.R)*(1-a)),
		G: uint8(float64(src.G)*a + float64(dst.G)*(1-a)),
		B: uint8(float64(src.B)*a + float64(dst.B)*(1-a)),
		A: 255,
	}
}

// lightColor returns a random pastel. Channels are floored at 140 so the
// result stays light enough to keep the white border and dark notch visible.
func lightColor() color.RGBA {
	return color.RGBA{
		R: uint8(140 + rand.Intn(90)),
		G: uint8(140 + rand.Intn(90)),
		B: uint8(140 + rand.Intn(90)),
		A: 255,
	}
}

func lerp(a, b uint8, t float64) uint8 {
	return uint8(float64(a)*(1-t) + float64(b)*t)
}

// encodePNG renders img as a data URI the frontend can drop into an <img src>.
func encodePNG(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
