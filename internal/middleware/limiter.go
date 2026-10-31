package middleware

import (
	"container/list"
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"

	"lopezpress/internal/clientip"
)

// Limiter is a token-bucket rate limiter keyed by "identity".
//
// Identity resolution:
//   - authenticated requests -> the user ID
//   - unauthenticated requests -> the client IP (fallback), which honours
//     X-Forwarded-For when the server sits behind a reverse proxy.
//
// The user ID comes from the bearer token, verified here, NOT from the request
// context: this middleware is installed outside the mux (see router.Build), so
// the JWT auth middleware has not run yet and the context never carries a user.
// It used to read only the context, which meant the advertised "per-identity"
// bucket was silently per-IP — an office behind one NAT shared a single
// allowance and collected 429s from each other, while a token holder rotating
// addresses was not throttled as an account at all.
//
// Each distinct identity gets its own *rate.Limiter with the configured
// RPS and burst. When an identity exceeds its allowance the request is
// rejected with 429 Too Many Requests and a Retry-After header.
//
// NOTE: the in-memory per-identity buckets are suitable for a single
// instance. For a multi-instance deployment replace the map with a
// shared counter (e.g. Redis).
type Limiter struct {
	mu          sync.Mutex
	buckets     map[string]*list.Element // key -> element wrapping *bucket
	lru         *list.List               // front = most-recently-used, back = LRU
	rps         float64
	burst       int
	maxBuckets  int
	tokenSecret string // set once at startup; read-only afterwards
}

// LimiterConfig carries the knobs for NewLimiter.
type LimiterConfig struct {
	// RPS is the sustained request rate allowed per identity (default 20).
	RPS float64
	// Burst is the maximum number of requests an identity may make in a
	// short window before being throttled (default 50).
	Burst int
}

// maxBuckets caps the number of cached per-identity buckets. Keeping it
// small (512) is deliberate for a 1 GB VPS: the cache itself should never
// become a memory hotspot. It is copied into Limiter.maxBuckets so tests
// can shrink it.
const maxBuckets = 512

// bucket pairs a rate-limit identity key with its token bucket, and is the
// element stored in the LRU list.
type bucket struct {
	key string
	lim *rate.Limiter
}

// UserIDContextKey is the context key under which the JWT auth handler
// stores the caller's user ID. The limiter reads it (via UserIDFrom) to
// key buckets per user.
const UserIDContextKey = "middleware.user.id"

// NewLimiter builds a Limiter. When RPS or Burst are left at zero (zero-
// valued LimiterConfig) they fall back to the documented defaults of
// 20 rps / 50 burst — a generous general bucket so a single signed-in admin
// mounting the console (session + tabs + review queue in parallel) is not
// throttled mid-page-load. Brute-force protection stays with the dedicated
// login limiter, so loosening the general bucket does not open a bypass.
// To tighten, set RATE_LIMIT_RPS / RATE_LIMIT_BURST in .env.
func NewLimiter(cfg LimiterConfig) *Limiter {
	if cfg.RPS <= 0 {
		cfg.RPS = 20
	}
	if cfg.Burst <= 0 {
		cfg.Burst = 50
	}
	return &Limiter{
		buckets:    make(map[string]*list.Element),
		lru:        list.New(),
		rps:        cfg.RPS,
		burst:      cfg.Burst,
		maxBuckets: maxBuckets,
	}
}

// Middleware returns a standard-library middleware that enforces the
// per-identity token bucket. A verified bearer token keys the bucket by user;
// everything else falls back to the client IP.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := l.resolveIdentity(r)
		key := "id:" + identity

		lim := l.limiterFor(key)

		if !lim.Allow() {
			// x/time/rate has no ResumeAfter; estimate the time until the
			// bucket refills one token: ceil(1 / rps) seconds.
			retry := int64(math.Ceil(1.0 / l.rps))
			if retry < 1 {
				retry = 1
			}
			w.Header().Set("Retry-After", strconv.FormatInt(retry, 10))
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"too many requests"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// limiterFor returns (and lazily creates) the bucket for a given key.
//
// To avoid unbounded memory growth the number of cached buckets is capped
// at maxBuckets. When the cap is exceeded the least-recently-used bucket is
// evicted (back of the access-ordered list) so that active identities keep
// their buckets while idle ones are reclaimed first — important on a 1 GB
// VPS where the cache must never become a memory hotspot.
func (l *Limiter) limiterFor(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	if el, ok := l.buckets[key]; ok {
		// Record this access as most-recently-used.
		l.lru.MoveToFront(el)
		return el.Value.(*bucket).lim
	}

	if l.lru.Len() >= l.maxBuckets {
		// Evict the least-recently-used bucket (back of the list).
		if back := l.lru.Back(); back != nil {
			old := back.Value.(*bucket)
			delete(l.buckets, old.key)
			l.lru.Remove(back)
		}
	}

	lim := rate.NewLimiter(rate.Limit(l.rps), l.burst)
	el := l.lru.PushFront(&bucket{key: key, lim: lim})
	l.buckets[key] = el
	return lim
}

// resolveIdentity returns the rate-limit key: the user ID when the JWT
// auth handler has stored it on the context (via WithUserID), otherwise
// the client IP.
// SetTokenSecret supplies the HS256 key used to identify authenticated
// callers. Call it once at startup, before the handler is served.
//
// Without it every request is bucketed by IP: the limiter cannot see a user,
// and a shared address beats one account's allowance into exhaustion.
func (l *Limiter) SetTokenSecret(secret string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tokenSecret = secret
}

// tokenSubject verifies a bearer token and returns its subject (user id), or
// "" when the header is absent, malformed, or signed with anything else.
//
// This is verification only — no database, no token_version check, no
// authorization decision. It runs before any handler precisely because the
// limiter must reject cheap requests before they become expensive ones; a
// revoked token that still verifies simply gets bucketed under its own subject
// and is refused a moment later by the auth middleware.
func (l *Limiter) tokenSubject(r *http.Request) string {
	l.mu.Lock()
	secret := l.tokenSecret
	l.mu.Unlock()
	if secret == "" {
		return ""
	}
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	raw := strings.TrimPrefix(header, "Bearer ")
	t, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid {
		return ""
	}
	claims, _ := t.Claims.(jwt.MapClaims)
	sub, _ := claims["sub"].(string)
	return sub
}

func (l *Limiter) resolveIdentity(r *http.Request) string {
	// Context first: if the limiter is ever moved inside the auth chain this
	// already works, and it saves the verification pass.
	if uid := UserIDFrom(r.Context()); uid != "" {
		return uid
	}
	if uid := l.tokenSubject(r); uid != "" {
		return uid
	}
	return clientIP(r)
}

// WithUserID returns a context with the user ID set. The JWT auth handler
// stores it so downstream code can identify the caller without re-parsing.
//
// The general limiter does NOT depend on this: it runs outside the mux, before
// auth, so the value is not there yet when it looks (see tokenSubject).
func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, UserIDContextKey, id)
}

// UserIDFrom reads the user ID stored by WithUserID, if any.
func UserIDFrom(ctx context.Context) string {
	if uid, ok := ctx.Value(UserIDContextKey).(string); ok {
		return uid
	}
	return ""
}

// clientIP extracts the originating client IP. The trust rules (only honour
// X-Forwarded-For from a configured proxy) live in internal/clientip so that
// the limiter, the login limiter and the handlers can never drift apart
func clientIP(r *http.Request) string {
	return clientip.Client(r)
}
