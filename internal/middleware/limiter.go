package middleware

import (
	"container/list"
	"context"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// Limiter is a token-bucket rate limiter keyed by "identity".
//
// Identity resolution (per ARCHITECTURE.md §6):
//   - authenticated requests -> the user ID (highest priority)
//   - unauthenticated requests -> the client IP (fallback), which honours
//     X-Forwarded-For when the server sits behind a reverse proxy.
//
// Each distinct identity gets its own *rate.Limiter with the configured
// RPS and burst. When an identity exceeds its allowance the request is
// rejected with 429 Too Many Requests and a Retry-After header.
//
// NOTE: the in-memory per-identity buckets are suitable for a single
// instance. For a multi-instance deployment replace the map with a
// shared counter (e.g. Redis) — see ARCHITECTURE.md §7.
type Limiter struct {
	mu         sync.Mutex
	buckets    map[string]*list.Element // key -> element wrapping *bucket
	lru        *list.List               // front = most-recently-used, back = LRU
	rps        float64
	burst      int
	maxBuckets int
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
// per-identity token bucket. The user id (when the JWT auth handler has
// stored it on the context via WithUserID) keys the bucket; otherwise the
// client IP is used.
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
func (l *Limiter) resolveIdentity(r *http.Request) string {
	if uid := UserIDFrom(r.Context()); uid != "" {
		return uid
	}
	return clientIP(r)
}

// WithUserID returns a context with the user ID set, so the limiter (and
// anything downstream) can key buckets by user. The JWT auth handler calls
// it to wrap the request context before invoking the next handler.
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

// clientIP extracts the originating client IP, honouring X-Forwarded-For
// first entry when present (typical behind Nginx / a load balancer).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Use the first hop (the original client); proxies append after it.
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" { // Cloudflare
		return cf
	}
	host := r.RemoteAddr
	if ip, _, err := net.SplitHostPort(host); err == nil {
		return ip
	}
	return host
}
