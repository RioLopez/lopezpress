package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"lopezpress/internal/services"
)

// uploadOptionsFromForm reads the optional resize/compression fields. Absent
// fields mean "no change", which is exactly what the pre-variant clients send;
// malformed numbers fall back to zero rather than rejecting the upload, because
// the service clamps every value anyway and a stale client must not lose a
// picture over a bad slider.
func uploadOptionsFromForm(r *http.Request) services.UploadOptions {
	atoi := func(key string) int {
		n, err := strconv.Atoi(strings.TrimSpace(r.FormValue(key)))
		if err != nil {
			return 0
		}
		return n
	}
	lossless := r.FormValue("lossless")
	return services.UploadOptions{
		MaxWidth:  atoi("max_width"),
		MaxHeight: atoi("max_height"),
		Quality:   atoi("quality"),
		Lossless:  lossless == "1" || strings.EqualFold(lossless, "true"),
	}
}
