package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every write endpoint gets a body ceiling from the route table, so "a new
// handler forgot to cap its input" is a declaration error rather than an
// omission nobody notices. Before this, 18 of 21 decode sites read an unbounded
// r.Body: the login and post handlers had been capped by hand after earlier
// reports, and every other endpoint was open to a client that simply sent more.
//
// Routes that legitimately need more than the default are listed explicitly
// with the reason, because the list is also the answer to "which requests can
// make this process allocate something large" — a question that should require
// reading three lines, not grepping 84.
func TestEveryRouteHasABodyCeiling(t *testing.T) {
	if len(routes) == 0 {
		t.Fatal("route table is empty")
	}

	// path -> the only ceiling that route may declare.
	want := map[string]int64{
		// An image plus multipart framing; the handler applies the operator's
		// (tighter) configured limit afterwards.
		"/api/upload": maxUploadBodyLimit,
		// A whole-database payload: every post body and user row. This is the
		// one route where a ceiling smaller than the handler's would turn "I
		// have a backup" into "I cannot restore it".
		"/api/backup/import": maxBackupImportLimit,
		// Bulk .md upload: 200 files × 2 MB is the documented contract.
		"/api/admin/import/markdown": maxMarkdownImportLimit,
	}

	overridden := 0
	for _, rt := range routes {
		if rt.maxBody > 0 {
			overridden++
		}
		if limit, ok := want[rt.path]; ok {
			if rt.maxBody != limit {
				t.Errorf("%s must declare maxBody=%d, got %d", rt.path, limit, rt.maxBody)
			}
			continue
		}
		if rt.maxBody > defaultBodyLimit {
			t.Errorf("%s %s raises the ceiling to %d without being in the allowlist "+
				"above; add it there with a reason, or it will silently diverge from "+
				"whatever the handler expects", rt.method, rt.path, rt.maxBody)
		}
	}
	// The allowlist must not rot: an entry for a path that no longer exists
	// means the real route lost its ceiling.
	seen := map[string]bool{}
	for _, rt := range routes {
		seen[rt.path] = true
	}
	for path := range want {
		if !seen[path] {
			t.Errorf("allowlist names %s, which is not in the route table", path)
		}
	}
	if overridden == 0 {
		t.Error("no route overrides the default — did maxBody stop being read?")
	}
	if defaultBodyLimit > 4<<20 {
		t.Errorf("default body limit grew to %d; a 4 MiB cap on a 1.5 GB box is too loose",
			defaultBodyLimit)
	}
}

// A declared Content-Length over the cap is refused before a byte is read, and
// answered with 413 rather than a decode error: "too large" and "malformed"
// call for different fixes, and a client that gets 400 for a 2 MB payload will
// keep retrying something that can never succeed.
func TestOversizedBodyIsRefusedWithoutReading(t *testing.T) {
	var reached bool
	h := limitBody(1024, func(w http.ResponseWriter, r *http.Request) {
		reached = true
	})

	body := bytes.NewReader(make([]byte, 5000))
	req := httptest.NewRequest("POST", "/api/anything", body)
	req.ContentLength = 5000
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
	if reached {
		t.Fatal("the handler ran for an oversized request — it should never be reached")
	}
	if !strings.Contains(rec.Body.String(), "too large") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

// A chunked request declares no length, so MaxBytesReader is the backstop: the
// read fails once the cap is crossed, which caps the allocation at the cap
// rather than the payload size. It surfaces as a decode failure (400), which is
// acceptable for a shape browsers do not use.
func TestChunkedBodyIsCappedToo(t *testing.T) {
	var got int
	h := limitBody(1024, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 8192)
		n, _ := r.Body.Read(buf)
		got = n
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/api/anything", bytes.NewReader(make([]byte, 100000)))
	req.ContentLength = -1 // chunked
	rec := httptest.NewRecorder()
	h(rec, req)

	if got > 2048 {
		t.Fatalf("read %d bytes past a 1024-byte cap", got)
	}
}

// Under the cap nothing changes: the handler sees the body it expects.
func TestBodyUnderTheCapPassesThrough(t *testing.T) {
	var seen string
	h := limitBody(1024, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		seen = string(buf[:n])
	})

	req := httptest.NewRequest("POST", "/api/anything", strings.NewReader("hello"))
	req.ContentLength = 5
	h(httptest.NewRecorder(), req)
	if seen != "hello" {
		t.Fatalf("body did not pass through, got %q", seen)
	}
}
