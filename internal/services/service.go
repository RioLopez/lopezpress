package services

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/config"
	"lopezpress/internal/models"
)

// Service carries the shared dependencies (db + cfg) and implements every
// business operation. Handlers are thin: they decode/encode JSON and map
// ServiceErrors to HTTP status codes; they never touch gorm directly.
type Service struct {
	db  *gorm.DB
	cfg *config.Config

	// mailT is the per-user throttle state for login-failure notifications.
	// Guarded by the fact that SendMail is the only writer and reads are
	// confined to the single goroutine that spawned it; for simplicity the
	// map is only touched from the request goroutine (single instance).
	mailT *mailThrottler

	// viewPruneMu / viewPruneDay throttle the post_views retention sweep to
	// once per UTC day per process.
	viewPruneMu  sync.Mutex
	viewPruneDay string

	// Buffered view counting (views_flush.go). viewMu guards all four fields
	// below: the per-post increments, the de-duplication rows waiting to be
	// written, the current day's "already counted" set, and when the buffer was
	// last emptied.
	viewMu        sync.Mutex
	viewPending   map[string]int64
	viewRows      []models.PostView
	viewSeen      map[string]struct{}
	viewSeenDay   string
	viewLastFlush time.Time

	// publicRead caches site_configs.public_read so the anonymous-read gate
	// does not hit the database on every read. Invalidated by
	// UpdateSiteConfig.
	publicReadMu  sync.RWMutex
	publicRead    bool
	publicReadSet bool

	// thumbs is the bounded wake-up channel of the thumbnail worker
	// (thumbs.go). The durable queue is the thumb_status column; this channel
	// only carries hints, so a full one is safe to drop.
	thumbs     chan string
	thumbsOnce sync.Once

	// codeIPMu / codeIP count how many verification codes one IP asked for in
	// the current hour (email_codes.go). The per-address cooldown lives in the
	// database and stops a single inbox being spammed; this stops one client
	// from walking a list of a thousand addresses instead.
	codeIPMu sync.Mutex
	codeIP   map[string]*ipCodeWindow

	// Per-account upload buckets (upload_limits.go). Quotas cap total bytes;
	// these cap the rate, so a script with one session cannot keep the decoder
	// busy indefinitely.
	uploadMu    sync.Mutex
	uploadGates map[string]*uploadBuckets
}

// New builds a Service backed by db and cfg.
func New(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{
		db:          db,
		cfg:         cfg,
		mailT:       newMailThrottler(),
		thumbs:      make(chan string, thumbQueueCap),
		viewPending: make(map[string]int64),
		viewSeen:    make(map[string]struct{}),
		// Start the flush clock at construction. Left at the zero value the
		// first counted view of every process would look overdue by decades
		// and pay a transaction on behalf of everyone who follows.
		viewLastFlush: time.Now(),
	}
}

// --- error plumbing: map domain errors to HTTP status codes ---

// ServiceError is the typed error returned by every Service method. Handlers
// map Status to a concrete HTTP code (400/401/403/404/409/500) and use Msg
// for the JSON "error" body.
type ServiceError struct {
	Status int    // HTTP status code
	Msg    string // human-readable message for the client
}

func (e *ServiceError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("[%d] %s", e.Status, e.Msg)
}

// newErr is a small constructor.
func newErr(status int, msg string) *ServiceError {
	return &ServiceError{Status: status, Msg: msg}
}

// AsServiceError returns the *ServiceError inside err, or nil when there is
// none. Callers that need to tell "the operation was refused on purpose" from
// "the operation blew up" use this instead of a type assertion, so an error
// wrapped by a transaction still unwraps correctly.
func AsServiceError(err error) *ServiceError {
	var se *ServiceError
	if errors.As(err, &se) {
		return se
	}
	return nil
}

// StatusOf extracts the HTTP status from a ServiceError. Non-ServiceError
// values (e.g. raw gorm errors that leaked through) map to 500.
func StatusOf(err error) int {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.Status
	}
	return http.StatusInternalServerError
}

// MsgOf extracts the client-facing message. It unwraps ServiceError; for any
// other error it returns a generic message so internals are not leaked.
func MsgOf(err error) string {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.Msg
	}
	return "internal server error"
}
