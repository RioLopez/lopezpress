package services

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"lopezpress/internal/models"
)

// newID generates the post primary key. It lives here (next to the slug logic)
// so CreatePost can build the id BEFORE insert and use it as the slug fallback.
func newID() string { return uuid.NewString() }

// Permanent links.
//
// A post is addressed by an opaque UUID in the canonical read route. That is
// fine for machines and useless for humans: it cannot be dictated, printed, or
// searched for. Every post therefore also gets a slug, and `/p/{slug}` is the
// shareable form. The UUID route stays valid forever (design decision A), so
// no existing link ever breaks.

// slugMaxRunes keeps generated links short enough to stay readable.
const slugMaxRunes = 80

// Slugify turns a title into a URL-safe slug.
//
// Kept deliberately simple and dependency-free:
//   - ASCII letters/digits are lowercased and kept;
//   - CJK characters are kept as-is (they are valid in a URL path and produce
//     readable Chinese links, which is what a Chinese blog actually wants);
//   - every other run of characters collapses into a single '-'.
//
// An input with nothing usable returns "" and the caller falls back to a
// stable id-based slug.
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	count := 0
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			count++
		case unicode.Is(unicode.Han, r), r >= 0x3040 && r <= 0x30FF: // Han + kana
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			count++
		default:
			dash = true
		}
		if count >= slugMaxRunes {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}

// ensureUniqueSlug returns base, or base-2 / base-3 ... when another post
// already claims it. excludeID lets a post keep its own slug on update.
//
// The COUNT-based probe is a best effort, not a guarantee: two concurrent
// creates of the same title both see "free" and both insert, which is how
// R6-02 ended up with four posts sharing one slug. The guarantee is the unique
// index on posts.slug (migration 0014) — so the caller retries on a constraint
// failure rather than trusting the probe, and this function is only the fast
// path that avoids a doomed insert in the common case.
//
// It deliberately does NOT take a lock or open a transaction: the index makes
// that unnecessary, and a lock here would serialise every post creation on the
// whole site.
func (s *Service) ensureUniqueSlug(ctx context.Context, base, excludeID string) (string, error) {
	if base == "" {
		return "", nil
	}
	candidate := base
	for i := 2; i < 200; i++ {
		var n int64
		q := s.db.WithContext(ctx).Model(&models.Post{}).Where("slug = ?", candidate)
		if excludeID != "" {
			q = q.Where("id <> ?", excludeID)
		}
		if err := q.Count(&n).Error; err != nil {
			return "", newErr(http.StatusInternalServerError, "failed to check slug uniqueness")
		}
		if n == 0 {
			return candidate, nil
		}
		candidate = base + "-" + itoa(i)
	}
	return "", newErr(http.StatusConflict, "could not allocate a unique slug")
}

// nextSlugCandidate returns the next suffixed form of base (base -> base-2 ->
// base-3 ...). Shared by the retry path and by the migration that
// disambiguates pre-existing duplicates, so both produce the same names.
func nextSlugCandidate(base string, attempt int) string {
	return base + "-" + strconv.Itoa(attempt)
}

// slugFor derives the slug for a new post: the requested value if the client
// sent one, otherwise the title; if both are unusable, the id itself so the
// permanent link is still stable and never empty.
func (s *Service) slugFor(ctx context.Context, requested, title, id string) (string, error) {
	base := Slugify(requested)
	if base == "" {
		base = Slugify(title)
	}
	if base == "" {
		base = id
	}
	return s.ensureUniqueSlug(ctx, base, "")
}

// GetPostBySlug resolves a permanent link. Visibility is decided by the same
// canSeePost authority as the id-based read, and a miss answers 404 with the
// same message so a slug cannot be used to probe for restricted content.
func (s *Service) GetPostBySlug(ctx context.Context, user models.User, slug string) (models.Post, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	var post models.Post
	// Ordered even though the unique index makes duplicates impossible: an
	// explicit ORDER BY is what makes the answer deterministic if that
	// guarantee is ever lost (a restore of a pre-0014 snapshot, say), instead
	// of First() returning whichever row the storage engine felt like.
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").
		Where("slug = ?", slug).Order("created_at ASC, id ASC").
		First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	if !canSeePost(user, post) {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	post.SanitizeAuthor()
	post.SanitizeReview()
	return post, nil
}

// itoa is a tiny helper to avoid importing strconv in this file twice.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// BackfillSlugs gives a permanent link to every post created before the slug
// column existed. Idempotent: it only touches rows whose slug is
// empty, so running it on every boot costs one indexed query.
//
// It lives in services (not in the migration) because slug generation belongs
// to the same code that allocates slugs on create — one implementation, no
// duplicated slugify in the config package.
func (s *Service) BackfillSlugs(ctx context.Context) (int, error) {
	var posts []models.Post
	if err := s.db.WithContext(ctx).Where("slug IS NULL OR slug = ''").Find(&posts).Error; err != nil {
		return 0, newErr(http.StatusInternalServerError, "failed to load posts without slugs")
	}
	n := 0
	for _, p := range posts {
		slug, err := s.slugFor(ctx, "", p.Title, p.ID)
		if err != nil {
			return n, err
		}
		if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", p.ID).
			Update("slug", slug).Error; err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
