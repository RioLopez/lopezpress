// Package imghost talks to external image hosts on behalf of the application.
//
// Why this package exists: the upload path used to live inside
// internal/services, which meant the business layer imported net/http to build
// an outbound request and hold a *http.Client. The 2026-10-23 architecture
// review (docs/ARCH_REVIEW_2026-10-23.md, finding A1) flagged that import as
// the thing making the service layer impossible to exercise without a
// transport. Outbound HTTP belongs to an adapter, not to domain logic; this is
// that adapter, and it has no knowledge of posts, users or permissions.
//
// Currently one host is supported (SM.MS v2). Adding a second host means one
// more file here plus a case in the caller's config switch -- still zero
// net/http in the service layer.
package imghost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// UploadTimeout is a hard ceiling on a remote upload. A hung image host must
// not pin a goroutine -- and the enclosing request -- indefinitely on a
// low-core VPS (as-built audit Q11). Kept as a named constant so the value is
// greppable and testable.
const UploadTimeout = 10 * time.Second

// client is deliberately NOT http.DefaultClient: the default has no timeout.
var client = &http.Client{Timeout: UploadTimeout}

// Upload proxies one image to the SM.MS v2 API and returns its public URL.
// The token is held by the server, so this is a privileged, browser-free call.
//
// The context is honoured end to end: a client disconnect cancels the outbound
// request immediately, and client.Timeout acts as a backstop against a remote
// that accepts the connection then stalls.
//
// The declared MIME type is intentionally not forwarded: the original
// in-service implementation did not use it either, and SM.MS sniffs the
// multipart part. Keeping the signature minimal avoids a dead parameter.
func Upload(ctx context.Context, filename string, data []byte, token string) (string, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("smfile", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	_ = w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://sm.ms/api/v2/upload", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		Success bool `json:"success"`
		Data    struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || !out.Success || out.Data.URL == "" {
		return "", fmt.Errorf("smms upload failed (status %d)", resp.StatusCode)
	}
	return out.Data.URL, nil
}
