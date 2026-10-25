package models

import (
	"crypto/rand"
	"math/big"
)

// base62Alphabet is the URL-safe alphabet used for opaque identifiers. No
// padding characters, no case-folding hazards beyond the usual base62 caveat
// (which does not apply here: ids are compared verbatim, never by a user).
const base62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// opaqueIDLength is the fixed width of a generated id: 62^22 > 2^128, so 16
// random bytes always fit and the string never has to grow.
const opaqueIDLength = 22

// NewOpaqueID returns a 22-character base62 identifier built from 128 bits of
// crypto/rand. It is used as both the primary key and the file-name stem of an
// upload, which is what keeps upload metadata (who, when, what) out of the URL:
// nothing in the name is derived from the clock or from user input.
//
// This is deliberately NOT encryption. A reversible scheme would add a key to
// manage and buy nothing — the requirement is "unlinkable and unguessable",
// and 128 random bits give that without a secret.
func NewOpaqueID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	n := new(big.Int).SetBytes(buf)
	base := big.NewInt(int64(len(base62Alphabet)))
	rem := new(big.Int)

	out := make([]byte, 0, opaqueIDLength)
	for n.Sign() > 0 {
		n.DivMod(n, base, rem)
		out = append(out, base62Alphabet[rem.Int64()])
	}
	// Left-pad to a constant width so ids are indistinguishable by length.
	for len(out) < opaqueIDLength {
		out = append(out, base62Alphabet[0])
	}
	// DivMod produced least-significant digit first; flip it.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out), nil
}
