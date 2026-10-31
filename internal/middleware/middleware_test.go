package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRecoverTurnsPanicInto500 verifies a panicking handler is caught and the
// client gets a 500 JSON body instead of a crashed process.
func TestRecoverTurnsPanicInto500(t *testing.T) {
	panicky := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	h := Recover(panicky)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("expected non-empty error message, got %q", body["error"])
	}
}

// TestRecoverPassesThroughHealthyHandlers verifies a non-panicking handler
// returns its own status untouched.
func TestRecoverPassesThroughHealthyHandlers(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
	})
	h := Recover(ok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", rec.Code)
	}
}

// TestCORSPreflightShortCircuits verifies OPTIONS requests return 204 with
// the CORS allow headers and do not reach the inner handler.
//
// The Origin header is not decoration: a browser preflight ALWAYS carries one,
// and this test used to omit it — which meant it was asserting behaviour for a
// request no browser sends. A bare OPTIONS with no Origin is not a preflight,
// it is just an OPTIONS, and answering it with `Access-Control-Allow-Origin: *`
// was how every anonymous request ended up advertising a wildcard policy.
func TestCORSPreflightShortCircuits(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})
	h := CORS("*")(inner)

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://reader.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected ACAO=*, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Fatalf("expected Vary: Origin, got %q", rec.Header().Get("Vary"))
	}
	if called {
		t.Fatalf("preflight must not reach the inner handler")
	}
}

// TestCORSStaysSilentWithoutOrigin pins the fix for the wildcard leak: a
// same-origin or non-browser request carries no Origin, so it must receive no
// CORS headers at all. Anything else tells every site on the internet that the
// API is readable cross-origin.
func TestCORSStaysSilentWithoutOrigin(t *testing.T) {
	var got http.Header
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		got = w.Header().Clone()
	})
	// Wildcard config: the loosest possible setting, and it must STILL stay
	// quiet when there is no Origin to answer.
	h := CORS("*")(inner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts", nil))

	if v := got.Get("Access-Control-Allow-Origin"); v != "" {
		t.Fatalf("no Origin must mean no ACAO, got %q", v)
	}
	if v := got.Get("Access-Control-Allow-Credentials"); v != "" {
		t.Fatalf("no Origin must mean no Allow-Credentials, got %q", v)
	}
}

// TestCORSEchoesAllowedOrigin verifies a known allowed origin is echoed back
// (not a wildcard) and the request reaches the inner handler.
func TestCORSEchoesAllowedOrigin(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := CORS("https://app.example.com")(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("expected echoed origin, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// TestCORSRestrictedEchoesKnownNotWildcard verifies that in a restricted
// (non-"*") allow-list, a KNOWN origin is echoed back exactly (not a
// wildcard), which is the intended behaviour when origins are enumerated.
func TestCORSRestrictedEchoesKnownNotWildcard(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := CORS("https://allowed.com")(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://allowed.com" {
		t.Fatalf("expected exact allowed origin, got %q", got)
	}
}

// TestCORSWildcardOmitsCredentials verifies that in "*" mode the
// Access-Control-Allow-Credentials header is NEVER emitted (wildcard +
// credentials is a footgun: browsers allow every origin).
func TestCORSWildcardOmitsCredentials(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := CORS("*")(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://random.example.org")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("wildcard mode should set ACAO=*, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("wildcard mode must NOT set Access-Control-Allow-Credentials, got %q", got)
	}
}

// TestCORSRestrictedSetsCredentials verifies that a non-wildcard allow-list
// emits Access-Control-Allow-Credentials: true.
func TestCORSRestrictedSetsCredentials(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := CORS("https://allowed.com")(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://allowed.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("restricted mode should set credentials=true, got %q", got)
	}
}

// TestChainOrderVerifiesOuterFirst verifies Chain applies mws[0] outermost
// (runs first) and the last middleware innermost (runs last, just before h).
func TestChainOrder(t *testing.T) {
	var order []string
	mk := func(tag string) Middleware {
		return func(h http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, tag)
				h.ServeHTTP(w, r)
			})
		}
	}
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	})

	h := Chain(inner, mk("outer"), mk("mid"), mk("inner"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"outer", "mid", "inner", "handler"}
	if len(order) != len(want) {
		t.Fatalf("expected execution order %v, got %v", want, order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full=%v)", i, order[i], want[i], order)
		}
	}
}

// TestLoggerRecordsStatus verifies Logger does not panic and lets the
// underlying status through.
func TestLoggerPassesThroughStatus(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	h := Logger(inner)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 to pass through Logger, got %d", rec.Code)
	}
}

// TestJSONErrorWritesJSONBody verifies JSONError emits an error body and
// the given status.
func TestJSONErrorWritesJSONBody(t *testing.T) {
	rec := httptest.NewRecorder()
	JSONError(rec, http.StatusBadRequest, "bad input")

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if body["error"] != "bad input" {
		t.Fatalf("error = %q, want %q", body["error"], "bad input")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// Compile-time guard: make sure httptest/recorder body is consumed as text.
var _ = io.Discard
