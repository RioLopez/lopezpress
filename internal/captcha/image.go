package captcha

import (
	"image/color"
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
	// height*(7..13)/16 — up to 40px here — and lays them out in equal
	// width/len slots. At the previous 132px the slots were 33px, so the
	// widest glyphs overlapped their neighbours and four characters became
	// unreadable. 180px gives 45px per slot, which clears the largest glyph.
	imageWidth = 180
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
	// showLineOptions mixes a hollow line with a sine wave. Both are thin
	// enough to leave the four characters readable and irregular enough to
	// break the connected-component assumptions a simple OCR pass makes.
	showLineOptions = base64Captcha.OptionShowHollowLine | base64Captcha.OptionShowSineLine
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
	return item.EncodeB64string(), nil
}
