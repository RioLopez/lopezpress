package captcha

import (
	crand "crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"math/big"
)

// charset excludes 0/O and 1/I/l: they are the characters users misread, and a
// captcha that fails honest people is worse than no captcha at all.
const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// imageLen is the number of characters in the picture captcha.
const imageLen = 4

// newID returns a 128-bit random identifier. It is the only thing the browser
// gets to reference its challenge by, so it must not be guessable: a sequential
// id would let an attacker enumerate other people's challenges.
func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// randomString picks n characters from charset using crypto/rand, so the answer
// cannot be predicted from the clock or from previously issued codes. This is
// also why the picture driver does not pick its own answer — the library seeds
// from math/rand.
func randomString(n int) (string, error) {
	max := big.NewInt(int64(len(charset)))
	out := make([]byte, n)
	for i := range out {
		v, err := crand.Int(crand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = charset[v.Int64()]
	}
	return string(out), nil
}

// equalFoldASCII compares ignoring ASCII case only. Unicode folding would let
// two different characters compare equal, which is not what a four-character
// ASCII code means.
//
// The comparison runs over a normalised copy rather than returning at the first
// differing byte. A short-circuit leaks how many leading characters were right,
// which turns one guess into five; the copy is four bytes on a path that
// happens at most a few times per minute.
func equalFoldASCII(a, b string) bool {
	if len(a) == 0 || len(a) != len(b) {
		return false
	}
	va := make([]byte, len(a))
	vb := make([]byte, len(b))
	for i := 0; i < len(a); i++ {
		va[i] = upperASCII(a[i])
		vb[i] = upperASCII(b[i])
	}
	return subtle.ConstantTimeCompare(va, vb) == 1
}

// upperASCII folds one byte to upper case, leaving everything else alone.
func upperASCII(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - ('a' - 'A')
	}
	return c
}
