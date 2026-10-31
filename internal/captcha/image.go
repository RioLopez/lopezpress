package captcha

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"sync"

	"github.com/mojocn/base64Captcha"
)

// The image captcha delegates only the drawing to base64Captcha. The answer is
// generated here with crypto/rand instead of letting the library pick it,
// because the library's character picker is seeded from math/rand — fine for a
// demo, useless against anyone who can predict it.
//
// The driver is a process-wide singleton: constructing a DriverString parses
// every embedded TTF, which is measurable work on a 1-core box. DrawCaptcha
// only reads the driver's fields, so sharing it is safe.

const (
	imageHeight = 50
	// Width is not cosmetic. The driver sizes each glyph at
	// height*(7..16)/16 — up to 40px here — and lays the characters out in
	// equal width/len slots starting at each slot's left edge, with no right
	// margin. At 180px the slots were 45px wide and a wide glyph in the last
	// slot ran past the canvas: test renders cut the fourth character in half.
	// 200px gives 50px per slot, which leaves roughly ten pixels of slack for
	// the widest glyph plus its random growth.
	imageWidth = 200
	// noiseCount is zero on purpose.
	//
	// The library's noise is *characters* drawn from the same alphabet as the
	// answer, distinguished only by being paler. At 6 and even at 3 that
	// produced pictures where the eye counted five glyphs and had to guess
	// which four were real — a captcha that honest people fail is worse than
	// none, because the operator switches it off and nothing is protected.
	//
	// Interference lines (see showLineOptions) do the anti-OCR work instead:
	// a line can cross a character without ever being mistaken for one.
	noiseCount = 0
	// showLineOptions adds one interference line. The choice is between two
	// extremes in this library and nothing in between:
	//
	//	OptionShowSineLine    a deep-coloured band ten pixels thick. In a test
	//	                      render it crossed the characters like a bent bar
	//	                      and hid their lower halves — a captcha that
	//	                      honest people fail.
	//	OptionShowHollowLine  a light two-pixel curve sweeping the whole width.
	//
	// The hollow one is chosen: it still breaks the connected-component
	// assumption a naive OCR pass makes, while leaving every glyph readable.
	// Anti-OCR is not the line's job here — that work is done server side by
	// single use, three attempts, a two-minute life, binding to the client and
	// a per-client issue limit, none of which an OCR improvement touches.
	showLineOptions = base64Captcha.OptionShowHollowLine
)

var (
	imageOnce   sync.Once
	imageDriver *base64Captcha.DriverString
)

// imageBackground is near-white: the reader is a light theme, and a dark
// captcha next to a white form looks like a rendering bug.
var imageBackground = color.RGBA{R: 250, G: 250, B: 250, A: 255}

func imageInit() {
	imageOnce.Do(func() {
		imageDriver = base64Captcha.NewDriverString(
			imageHeight, imageWidth,
			noiseCount,
			showLineOptions,
			imageLen,
			charset,
			&imageBackground,
			nil, // default embedded fonts
			nil, // load all of them
		)
	})
}

// newImage renders one challenge and returns its data URI plus the answer.
func newImage() (b64 string, answer string, err error) {
	answer, err = randomString(imageLen)
	if err != nil {
		return "", "", err
	}
	b64, err = newImageWith(answer)
	return b64, answer, err
}

// newImageWith renders a picture for a caller-chosen answer. Only Issue's
// fixed-answer path uses it, which exists for tests: they need a challenge
// whose answer they know, and the alternative — exporting a way to read the
// answer of a live challenge — would be a real hole rather than a test hook.
func newImageWith(answer string) (string, error) {
	imageInit()
	item, err := imageDriver.DrawCaptcha(answer)
	if err != nil {
		return "", err
	}
	return padHorizontally(item.EncodeB64string(), padX)
}

// padX is the background margin added on each side, in pixels.
//
// The driver starts the first glyph at x=0 and gives each slot no right margin,
// so the outermost characters sit flush against the border. Nothing is actually
// missing — the glyph is inside the canvas — but a character touching the edge
// reads as cut off, and a captcha that looks broken is a captcha the operator
// disables. Four pixels of background on each side cost nothing and remove the
// doubt. Width on the frontend is imageWidth + 2*padX; the two must agree, or
// object-fit rescales and reintroduces the very edges this is protecting.
const padX = 4

// padHorizontally re-encodes a data-URI PNG onto a wider canvas of the same
// background colour.
func padHorizontally(dataURI string, pad int) (string, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURI, prefix) || pad <= 0 {
		return dataURI, nil
	}
	raw, err := base64.StdEncoding.DecodeString(dataURI[len(prefix):])
	if err != nil {
		return "", err
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()+2*pad, b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: imageBackground}, image.Point{}, draw.Src)
	draw.Draw(dst, image.Rect(pad, 0, pad+b.Dx(), b.Dy()), src, b.Min, draw.Over)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return "", err
	}
	return prefix + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
