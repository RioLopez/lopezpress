package services

import (
	"net/http"
	"strings"

	"lopezpress/internal/models"
)

// --- post body format -------------------------------------------------------
//
// Two formats share one field:
//   - markdown (default): everyone; rendered by marked on the reader.
//   - html: root/admin only; sanitized here at write time.
//
// The permission is enforced in the service layer, not only by hiding the
// toggle in the SPA: an API client can send body_format=html directly.

// storedBodyFormat normalizes rows written before migration 0018 (empty) to
// the only meaning they could have had: Markdown.
func storedBodyFormat(format string) string {
	if strings.EqualFold(strings.TrimSpace(format), models.BodyFormatHTML) {
		return models.BodyFormatHTML
	}
	return models.BodyFormatMarkdown
}

func validateBodyFormatName(format string) error {
	if format != models.BodyFormatMarkdown && format != models.BodyFormatHTML {
		return newErr(http.StatusBadRequest, "body_format must be markdown or html")
	}
	return nil
}

// resolveCreateBodyFormat decides the format of a new post. Empty means
// Markdown. HTML requires an admin/root author.
func resolveCreateBodyFormat(me models.User, requested string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(requested))
	if format == "" {
		return models.BodyFormatMarkdown, nil
	}
	if err := validateBodyFormatName(format); err != nil {
		return "", err
	}
	if format == models.BodyFormatHTML && !IsAdminOrRoot(me) {
		return "", newErr(http.StatusForbidden, "only an admin can author HTML posts")
	}
	return format, nil
}

// resolveUpdateBodyFormat decides the format of an edit. The requested value
// is optional: absent keeps the current format. A post that is already HTML
// may only be saved by admin/root — otherwise an author demoted to "user"
// would retain an HTML editing path.
func resolveUpdateBodyFormat(me models.User, current, requested string) (string, error) {
	cur := storedBodyFormat(current)
	if cur == models.BodyFormatHTML && !IsAdminOrRoot(me) {
		return "", newErr(http.StatusForbidden, "only an admin can edit an HTML post")
	}
	format := strings.ToLower(strings.TrimSpace(requested))
	if format == "" {
		return cur, nil
	}
	if err := validateBodyFormatName(format); err != nil {
		return "", err
	}
	if format == models.BodyFormatHTML && !IsAdminOrRoot(me) {
		return "", newErr(http.StatusForbidden, "only an admin can author HTML posts")
	}
	return format, nil
}

// preparePostBody sanitizes HTML and refuses a body whose visible text is
// empty. A payload made only of <script> is a valid fragment but not an
// article, and storing it would produce a blank page that looks like a bug.
func preparePostBody(format, body string) (string, error) {
	if storedBodyFormat(format) != models.BodyFormatHTML {
		return body, nil
	}
	out, err := SanitizeHTML(body)
	if err != nil {
		return "", newErr(http.StatusBadRequest, "post body is not valid HTML")
	}
	if strings.TrimSpace(HTMLToText(out)) == "" {
		return "", newErr(http.StatusBadRequest, "post body must not be empty")
	}
	return out, nil
}
