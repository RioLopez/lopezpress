package services

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lopezpress/internal/models"
)

// Markdown import (spec 32).
//
// The single biggest reason a buyer walks away: their existing posts live in
// another system and retyping them is not an option. This is the smallest
// useful importer: N .md files in, N posts out, with frontmatter honoured and
// the original publish date preserved (otherwise the archive timeline is
// destroyed and the site looks like it started today).

// Import limits: small enough to stay inside one request on a 1 vCPU box.
const (
	maxMarkdownFiles = 200
	maxImportBytes = 2 << 20 // 2 MB per file
)

// MarkdownSkip records a file that was not imported, with the reason.
// frontmatter is the parsed YAML-ish header. Only the handful of keys a blog
// export actually carries are supported; anything else is ignored.
type frontmatter struct {
	Title    string
	Date     time.Time
	Category string
	Tags     []string
	Draft    bool
}

// parseFrontmatter splits `---` ... `---` off the top and reads flat
// `key: value` pairs. Deliberately not a YAML parser: pulling in a YAML
// dependency for six keys would violate the "3 frontend deps / 6 backend deps"
// budget, and exports use only flat scalars.
func parseFrontmatter(raw string) (frontmatter, string) {
	var fm frontmatter
	if !strings.HasPrefix(raw, "---") {
		return fm, raw
	}
	rest := raw[3:]
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[i+1:]
	} else {
		return fm, raw
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return fm, raw
	}
	header := rest[:end]
	body := rest[end+4:]
	if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}

	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		switch key {
		case "title":
			fm.Title = val
		case "date", "published", "created":
			fm.Date = parseImportDate(val)
		case "category", "categories":
			fm.Category = strings.TrimSpace(strings.Split(val, ",")[0])
		case "tags", "tag":
			fm.Tags = parseTagList(val)
		case "draft":
			fm.Draft = val == "true" || val == "yes" || val == "1"
		}
	}
	return fm, strings.TrimLeft(body, "\n")
}

// parseTagList accepts `a, b` and `[a, b]`.
func parseTagList(val string) []string {
	val = strings.Trim(val, "[]")
	parts := strings.FieldsFunc(val, func(r rune) bool { return r == ',' || r == '，' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseImportDate accepts YYYY-MM-DD, YYYY/MM/DD and RFC3339.
func parseImportDate(val string) time.Time {
	val = strings.TrimSpace(val)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02", "2006/01/02"} {
		if t, err := time.Parse(layout, val); err == nil {
			return t
		}
	}
	return time.Time{}
}

// titleFromBody extracts a leading `# Heading` and removes that line.
func titleFromBody(body string) (string, string) {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			rest := append(append([]string{}, lines[:i]...), lines[i+1:]...)
			return title, strings.TrimLeft(strings.Join(rest, "\n"), "\n")
		}
		break // first non-empty line is not a heading
	}
	return "", body
}

// titleFromFilename turns `my-first-post.md` into `my first post`.
func titleFromFilename(name string) string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	base = strings.NewReplacer("-", " ", "_", " ").Replace(base)
	return strings.TrimSpace(base)
}

// ImportMarkdown creates one post per file.
//
// Per-file commits (no global transaction): importing 200 posts in one
// transaction would hold the SQLite write lock for the whole upload, and a
// single bad file would roll back everything. Partial success is the better
// contract here, and the response reports exactly what was skipped.
func (s *Service) ImportMarkdown(ctx context.Context, actor models.User, files []MarkdownFile) (MarkdownImportResult, error) {
	if len(files) == 0 {
		return MarkdownImportResult{}, newErr(http.StatusBadRequest, "no files uploaded")
	}
	if len(files) > maxMarkdownFiles {
		return MarkdownImportResult{}, newErr(http.StatusBadRequest, "too many files (max 200)")
	}

	res := MarkdownImportResult{Skipped: []MarkdownSkip{}}
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".md" && ext != ".markdown" {
			res.Skipped = append(res.Skipped, MarkdownSkip{Name: f.Name, Reason: "unsupported extension"})
			continue
		}
		if len(f.Body) == 0 {
			res.Skipped = append(res.Skipped, MarkdownSkip{Name: f.Name, Reason: "empty file"})
			continue
		}
		if len(f.Body) > maxImportBytes {
			res.Skipped = append(res.Skipped, MarkdownSkip{Name: f.Name, Reason: "file too large"})
			continue
		}

		fm, body := parseFrontmatter(string(f.Body))
		title := fm.Title
		if title == "" {
			var fromBody string
			fromBody, body = titleFromBody(body)
			title = fromBody
		}
		if title == "" {
			title = titleFromFilename(f.Name)
		}
		if strings.TrimSpace(body) == "" {
			res.Skipped = append(res.Skipped, MarkdownSkip{Name: f.Name, Reason: "empty body"})
			continue
		}

		// Status follows the same rule as manual creation: only root publishes
		// directly, everyone else lands in the review queue. A `draft: true`
		// file becomes private (this build has no draft state).
		status := models.PostPending
		if actor.Role == models.RoleRoot {
			status = models.PostPublished
		}
		if fm.Draft {
			status = models.PostPrivate
		}

		post := models.Post{
			ID:       newID(),
			Title:    truncateRunes(title, 200),
			Body:     body,
			Status:   status,
			AuthorID: actor.ID,
		}
		if !fm.Date.IsZero() {
			post.CreatedAt = fm.Date
			post.UpdatedAt = fm.Date
		}
		slug, err := s.slugFor(ctx, "", post.Title, post.ID)
		if err != nil {
			res.Skipped = append(res.Skipped, MarkdownSkip{Name: f.Name, Reason: MsgOf(err)})
			continue
		}
		post.Slug = slug
		if fm.Category != "" {
			if catID, cerr := s.findOrCreateCategoryByName(ctx, fm.Category); cerr == nil {
				post.CategoryID = &catID
			}
		}
		if err := s.db.WithContext(ctx).Create(&post).Error; err != nil {
			res.Skipped = append(res.Skipped, MarkdownSkip{Name: f.Name, Reason: "failed to create post"})
			continue
		}
		if len(fm.Tags) > 0 {
			s.attachTagsByName(ctx, post.ID, fm.Tags)
		}
		res.Created++
	}
	s.audit(ctx, actor, "import_markdown", "posts",
		"created="+strconv.Itoa(res.Created)+"; skipped="+strconv.Itoa(len(res.Skipped)))
	return res, nil
}

// findOrCreateCategoryByName reuses a top-level category with the same name.
//
// The name is length-checked before it reaches the database. Without this the
// importer was the one write path with no ceiling at all: R4-10 imported a
// category whose name was ~1 MB, after which the ANONYMOUS /api/categories
// endpoint served 1.05 MB to every visitor (and /api/tags 0.92 MB from a
// 900 KB tag name) — a single admin action amplifying into a public
// bandwidth/memory sink.
func (s *Service) findOrCreateCategoryByName(ctx context.Context, name string) (int64, error) {
	name = strings.TrimSpace(name)
	if err := ValidateTaxonomyName(name, MaxCategoryNameLen); err != nil {
		return 0, err
	}
	var cat models.Category
	if s.db.WithContext(ctx).Where("name = ?", name).First(&cat).Error == nil {
		return cat.ID, nil
	}
	cat = models.Category{Name: name}
	if err := s.db.WithContext(ctx).Create(&cat).Error; err != nil {
		return 0, err
	}
	return cat.ID, nil
}

// attachTagsByName links existing tags and creates the missing ones.
//
// Same ceiling as categories (R4-10): an unbounded tag name becomes an
// unbounded public response the moment the tag list is served.
func (s *Service) attachTagsByName(ctx context.Context, postID string, names []string) {
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if err := ValidateTaxonomyName(name, MaxTagNameLen); err != nil {
			continue // skip this tag, keep importing the rest of the file
		}
		var tag models.Tag
		if s.db.WithContext(ctx).Where("name = ?", name).First(&tag).Error != nil {
			tag = models.Tag{Name: name}
			if err := s.db.WithContext(ctx).Create(&tag).Error; err != nil {
				continue
			}
		}
		// Duplicate links are harmless only if the PK is composite; guard with
		// a existence check so a re-import cannot error out here.
		var n int64
		s.db.WithContext(ctx).Model(&models.PostTag{}).
			Where("post_id = ? AND tag_id = ?", postID, tag.ID).Count(&n)
		if n == 0 {
			_ = s.db.WithContext(ctx).Create(&models.PostTag{PostID: postID, TagID: tag.ID}).Error
		}
	}
}

// truncateRunes cuts s to at most n runes (titles are VARCHAR 200 in SQLite,
// but a multi-byte title could still overflow a strict driver).
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
