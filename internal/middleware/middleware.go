package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Middleware wraps an http.Handler, augmenting its behaviour (logging,
// auth, rate-limiting, CORS...). The standard library provides no
// middleware chain, so we define one here.
type Middleware = func(http.Handler) http.Handler

// Chain composes middlewares and applies them to h so that mws[0] runs
// first (outermost) and the last one runs innermost, just before h.
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// statusRecorder captures the response status code so Logger can log it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Recover is a middleware that recovers from panics and writes a 500 JSON
// response instead of crashing the process.
func Recover(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				slog.Error("panic recovered", "panic", p, "path", r.URL.Path)
				JSONError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		h.ServeHTTP(w, r)
	})
}

// Logger is a middleware that logs each request via log/slog with method,
// path, status code and duration.
func Logger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).String(),
			"remote", r.RemoteAddr,
		)
	})
}

// CORS returns a middleware that adds CORS response headers. The origins
// argument is a comma-separated list; "*" allows all origins. OPTIONS
// preflight requests are short-circuited with 204.
//
// Security note: when the allow list contains "*", this middleware NEVER
// sets Access-Control-Allow-Credentials, because "*" + credentials is
// equivalent to allowing every origin (see docs/decisions/0004-cors). When
// an explicit origin list is provided, credentials are enabled so the SPA
// can rely on same-site cookies; this project still uses Bearer tokens in
// localStorage, so credentials only matter for the optional-cookie path.
func CORS(origins string) Middleware {
	wildcard := strings.TrimSpace(origins) == "*"
	allowed := map[string]bool{}
	if wildcard {
		allowed["*"] = true
	} else {
		for _, o := range strings.Split(origins, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				allowed[o] = true
			}
		}
	}

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			var corsOrigin string
			switch {
			case origin == "":
				corsOrigin = "*"
			case allowed[origin]:
				corsOrigin = origin // known origin: echo back
			case wildcard:
				corsOrigin = "*" // open dev mode: allow anything
		default:
			// Unknown origin under a restricted allow-list: do NOT echo it
			// back (that would defeat the allow-list). Leave the header unset
			// so the browser rejects the cross-origin response.
			corsOrigin = ""
		}
		if corsOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", corsOrigin)
		}
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
			if !wildcard {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			h.ServeHTTP(w, r)
		})
	}
}

// JSONError writes a JSON error response with the given status code.
// The response body is {"error":"msg"}.
func JSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
