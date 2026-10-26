package captcha

import (
	crand "crypto/rand"
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

// randomInt returns a uniform value in [0, n).
func randomInt(n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}
	v, err := crand.Int(crand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// equalFoldASCII compares ignoring ASCII case only. Unicode folding would let
// two different characters compare equal, which is not what a four-character
// ASCII code means.
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'a' && ca <= 'z' {
			ca -= 'a' - 'A'
		}
		if cb >= 'a' && cb <= 'z' {
			cb -= 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
