package router

import "net/http"

// --- request body ceilings --------------------------------------------------
//
// Kept out of router.go (which sits at the 350-line architecture gate), and more
// importantly kept in ONE place: the ceiling is a property of the route table,
// so the code that applies it should be impossible to miss.

// defaultBodyLimit is the ceiling applied to every route that does not override
// it. 1 MiB covers the largest legitimate JSON payload (a 256 KiB post body
// plus JSON escaping and a tag list) with room to spare, while keeping the
// worst case a client can make this process allocate at a size that cannot
// matter on a 1.5 GB box.
const defaultBodyLimit = 1 << 20

// The ceilings below are NOT arbitrary: each one is the value the handler
// already enforced internally, promoted to the route table.
//
// That promotion is the point of this whole mechanism, but it also hides a trap
// worth stating plainly: a route ceiling that is SMALLER than the handler's own
// limit silently wins, and the handler's limit becomes dead code. The route
// layer runs first, so a 1 MiB default on the backup-import route means an
// operator can export a 2 MiB backup and then be unable to restore it — data
// loss disguised as a 413. Every route that carries a file or a whole-database
// payload therefore declares its real ceiling here, and security_check.py (S9)
// fails the build if a handler ever allows more than its route.

// maxUploadBodyLimit covers the largest image the service will accept
// (upload_max_image, clamped to 20 MiB) plus multipart framing. The handler
// applies the operator's configured limit afterwards — usually tighter.
const maxUploadBodyLimit = 24 << 20

// maxBackupImportLimit matches `services.ImportBackup`'s own cap. A restore is
// the one request whose size scales with the whole site (every post body, every
// user row), so it is the last place to be stingy: a backup that exports fine
// but cannot be imported is worse than no backup at all.
const maxBackupImportLimit = 20 << 20

// maxMarkdownImportLimit matches `handlers.ImportMarkdown`'s documented
// "200 files × 2 MB" contract.
const maxMarkdownImportLimit = 24 << 20

// limitBody wraps a handler so the request body cannot exceed n bytes.
//
// Two layers, because the two kinds of client need different treatment:
//
//  1. A client that declares Content-Length larger than n is refused before a
//     single byte is read. That is the common case (every browser, curl, and
//     HTTP library sets it), and refusing up front means an oversized request
//     costs nothing at all — not even the read buffer.
//  2. A chunked request carries no declared length, so MaxBytesReader is the
//     backstop: it fails the read once n bytes have been consumed, which caps
//     the allocation at n instead of the payload size.
//
// Answering 413 rather than letting the handler see a decode error matters for
// more than tidiness: "your request is too large" and "your JSON is malformed"
// call for different fixes, and a client that gets 400 for a 2 MB payload will
// keep retrying a request that can never succeed.
func limitBody(n int64, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > n {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_, _ = w.Write([]byte(`{"error":"request body too large"}`))
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, n)
		}
		next(w, r)
	}
}

