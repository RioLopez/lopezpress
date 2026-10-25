package services

import (
	"strings"
	"unicode/utf8"
)

// --- field length ceilings --------------------------------------------------
//
// R3-07 measured what unbounded fields cost: one pending post with a 1 MB
// title and a 5 MB body produced a 6.29 MB create response, a 9.30 MB admin
// list, an 18.88 MB revision history (three 5 MB snapshots) and a 4.20 MB
// audit page, and grew the database by 27 MB from a single article. Nothing
// rejected any of it — CreatePost only checked "not empty".
//
// The ceilings live here, in one file, because they are one decision: "how big
// may a piece of user content be". Scattering them across handlers is how the
// import path ended up with no limit at all while the tag endpoint had one
// (R4-10: a 1 MB category name imported fine, and then the anonymous
// /api/categories endpoint served 1.05 MB to every visitor).
//
// SQLite does not enforce the `size:` tags on these columns, so a limit that
// exists only in the schema comment is not a limit.

const (
	// MaxTitleLen bounds a post title. The model declares size:200; the ceiling
	// is deliberately looser (500) so that a restore of a backup written by an
	// older, looser build still validates, while anything a human would
	// reasonably type stays inside it.
	MaxTitleLen = 500

	// MaxBodyLen bounds a post body. 256 KiB is roughly 85,000 CJK characters
	// — far past any real article, small enough that the worst-case page of
	// posts, revision history and RSS feed stay in the hundreds of KiB.
	MaxBodyLen = 256 * 1024

	// MaxAuditDetailLen bounds one action_log detail string. The column
	// declares size:512 and SQLite ignores it, so the detail — which embeds the
	// post title verbatim — used to be as long as the title was. The audit page
	// is read by admins on every visit; a single fat row made one page 4.26 MB.
	MaxAuditDetailLen = 512

	// MaxCategoryNameLen / MaxTagNameLen bound taxonomy names. They match the
	// column widths and the values the UI already enforced, so the only
	// behaviour change is on the import path, which had none.
	MaxCategoryNameLen = 64
	MaxTagNameLen      = 64

	// maxFriendLinkNameLen bounds a footer link's label. The URL already had a
	// 500-rune cap; the name had none, and because the site-config response is
	// public, an unbounded name was an unbounded public payload (R6-09 measured
	// ~5 KB of footer text per reader from one 5000-character label).
	maxFriendLinkNameLen = 50
	// maxFriendLinkURLLen / maxFriendLinks keep the footer list itself bounded:
	// 50 links of at most 500 runes is already a busy footer, and anything past
	// it lands in the public site-config payload.
	maxFriendLinkURLLen = 500
	maxFriendLinks      = 50

	// MaxRequestBodyBytes caps a JSON request body before it is parsed. The
	// per-field ceilings above already reject oversized content, but they run
	// AFTER json.Decode has materialised the whole payload in memory — which
	// is exactly the amplification R3-05 exploited on the login endpoint. This
	// is the belt to that braces.
	MaxRequestBodyBytes = 1 << 20 // 1 MiB

	// --- the second sweep ---------------------------------------------------
	//
	// The ceilings above were chosen by looking at the fields R3-07 attacked.
	// Auditing every user-writable string afterwards found four more with no
	// bound at all, and they are the same bug: a value that becomes part of a
	// response every reader (or every admin) downloads.
	//
	// MaxReviewNoteLen bounds the reviewer's note. It is stored in
	// posts.review_note (TEXT, so SQLite enforces nothing) and returned inside
	// the post payload the AUTHOR reads — so an admin-reject with a 1 MB note
	// is a 1 MB response to that author, on every list that includes it.
	MaxReviewNoteLen = 2000

	// MaxDisplayNameLen / MaxBioLen bound the profile. Bio renders on the
	// author byline of every post that user wrote, and the byline is embedded
	// in public list responses — so an unbounded bio multiplies across the
	// whole site.
	MaxDisplayNameLen = 64
	MaxBioLen         = 2000

	// MaxCoverURLLen bounds a post cover reference. The field had no validation
	// whatsoever: any string was accepted and later rendered into an <img src>.
	// The front end applies its own URL policy, but a stored `javascript:` or a
	// 1 MB data: URI should never reach the database in the first place.
	MaxCoverURLLen = 500

	// MaxTagsPerPost bounds how many tags one save may attach. The per-name
	// ceiling is useless without it: 10,000 legal 64-character names is a
	// 640 KB tag list, and the anonymous /api/tags endpoint returns all of it.
	MaxTagsPerPost = 20
)

// ValidatePostContent checks the two content fields of a post write. It is the
// single gate shared by CreatePost, UpdatePost and the Markdown importer, so a
// new write path cannot forget it.
func ValidatePostContent(title, body string) error {
	if strings.TrimSpace(title) == "" {
		return newErr(400, "title is required")
	}
	if len([]rune(title)) > MaxTitleLen {
		return newErr(400, "title is too long (max "+itoa(MaxTitleLen)+" characters)")
	}
	if strings.TrimSpace(body) == "" {
		return newErr(400, "post body must not be empty")
	}
	if len(body) > MaxBodyLen {
		return newErr(400, "post body is too long (max 256 KB)")
	}
	return nil
}

// ValidateTaxonomyName checks a category or tag name against its ceiling.
func ValidateTaxonomyName(name string, max int) error {
	if strings.TrimSpace(name) == "" {
		return newErr(400, "name is required")
	}
	if len([]rune(name)) > max {
		return newErr(400, "name is too long (max "+itoa(max)+" characters)")
	}
	return nil
}

// clampDetail truncates an audit detail string to the column's intent. The
// caller keeps a readable prefix ("title=…"), which is all an audit trail
// needs; the full content lives on the post itself.
//
// The cut is rune-aware: slicing at a byte offset can land inside a multi-byte
// character and leave an invalid UTF-8 fragment in the row (which then renders
// as a replacement character for every admin who reads the audit page).
func clampDetail(s string) string {
	if len(s) <= MaxAuditDetailLen {
		return s
	}
	var b strings.Builder
	b.Grow(MaxAuditDetailLen + len("…[truncated]"))
	for _, r := range s {
		if b.Len()+utf8.RuneLen(r) > MaxAuditDetailLen {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…[truncated]"
}

// auditDetail is the single place an action_log detail is built from user
// content. Keeping the clamp here — rather than at each call site — is what
// stops the next audit caller from reintroducing the 4 MB page.
func auditDetail(parts ...string) string {
	return clampDetail(strings.Join(parts, "; "))
}

// ValidateCoverURL checks a post's cover reference.
//
// Two shapes are accepted: an image already in this site's library
// ("/uploads/<name>") and an absolute http(s) URL. Everything else — a
// javascript: or data: URI, a protocol-relative "//host" — is refused.
//
// The front end has its own URL policy (isSafeSrc) and would refuse to render
// the bad shapes, so this is defence in depth rather than the only lock. It
// matters anyway: the front end is not the only reader of this field. RSS,
// the sitemap and any future client all consume `cover`, and a value kept in
// the database is a value somebody will eventually render.
func ValidateCoverURL(cover string) error {
	cover = strings.TrimSpace(cover)
	if cover == "" {
		return nil // no cover is valid
	}
	if len([]rune(cover)) > MaxCoverURLLen {
		return newErr(400, "cover url is too long (max "+itoa(MaxCoverURLLen)+" characters)")
	}
	low := strings.ToLower(cover)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return nil
	}
	if strings.HasPrefix(cover, "/uploads/") && !strings.HasPrefix(cover, "//") {
		return nil
	}
	return newErr(400, "cover must be an image already uploaded here or an http(s) url")
}

// ValidateProfileText bounds the two free-text profile fields.
//
// DisplayName appears next to every post the user wrote, and Bio on their
// author page; both are served inside public list responses. A member could
// previously store a megabyte in either, which then rode along in every list
// response that included their posts.
func ValidateProfileText(displayName, bio string) error {
	if len([]rune(displayName)) > MaxDisplayNameLen {
		return newErr(400, "display name is too long (max "+itoa(MaxDisplayNameLen)+" characters)")
	}
	if len([]rune(bio)) > MaxBioLen {
		return newErr(400, "bio is too long (max "+itoa(MaxBioLen)+" characters)")
	}
	return nil
}

// ValidateReviewNote bounds a reviewer's note. Required for reject and
// send-back; optional elsewhere.
//
// The note is not just stored: it is returned to the author inside the post
// payload, so its length is a length the author downloads — and the reviewer
// is the one party who must not be able to inflate another user's response.
func ValidateReviewNote(note string) error {
	if len([]rune(note)) > MaxReviewNoteLen {
		return newErr(400, "review note is too long (max "+itoa(MaxReviewNoteLen)+" characters)")
	}
	return nil
}
