package models

import "golang.org/x/crypto/bcrypt"

// bcryptMaxConcurrency bounds how many bcrypt operations may run at once.
//
// bcrypt is deliberately expensive — that is the point of using it — but the
// asymmetry is that an attacker spends one cheap request while the server burns
// ~60 ms of CPU hashing (DefaultCost = 10). On a 1-core box a handful of
// concurrent login or register attempts saturate the CPU for seconds (R4-05
// measured ~1.4 s for 20 concurrent attempts). The semaphore caps the
// parallelism without weakening the hash or adding latency for the honest
// single login, which is the only case that matters.
const bcryptMaxConcurrency = 2

var bcryptSemaphore = make(chan struct{}, bcryptMaxConcurrency)

// acquireBcrypt blocks until a bcrypt slot is free and returns its release.
func acquireBcrypt() func() {
	bcryptSemaphore <- struct{}{}
	return func() { <-bcryptSemaphore }
}

// HashPassword returns the bcrypt hash of the given plain password.
func HashPassword(plain string) (string, error) {
	release := acquireBcrypt()
	defer release()
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword reports whether plain matches the stored hash.
func VerifyPassword(plain, hash string) bool {
	release := acquireBcrypt()
	defer release()
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// timingEqualizerHash is a real bcrypt hash of a throwaway string. It exists so
// a failed *lookup* can cost the same as a failed *password*.
//
// Without it, Login answers "no such user" in about a millisecond and "wrong
// password" only after ~60 ms of bcrypt. Both return the same 401, so the
// response body leaks nothing — but the clock does. An attacker who can time
// 200 requests has a free list of valid usernames, and against a 5-attempt
// lockout a targeted list is worth far more than blind guessing.
//
// Computed at startup so it always tracks bcrypt.DefaultCost: if the cost is
// raised, this hash is raised with it and the two paths stay in step.
var timingEqualizerHash = func() string {
	h, err := HashPassword("lopezpress timing equalizer")
	if err != nil {
		// HashPassword only fails on a cost above 31 or a password over 72
		// bytes; neither is possible here. Failing loudly at startup beats
		// running with a silent timing leak nobody will ever notice.
		panic("models: cannot build timing equalizer hash: " + err.Error())
	}
	return h
}()

// VerifyPasswordAgainstNothing burns one password check's worth of CPU and
// always fails. Call it on the "user not found" branch of a login so that path
// costs what the real one costs.
func VerifyPasswordAgainstNothing(plain string) {
	release := acquireBcrypt()
	defer release()
	_ = bcrypt.CompareHashAndPassword([]byte(timingEqualizerHash), []byte(plain))
}
