package services

import (
	"lopezpress/internal/models"
)

// canSeePost reports whether the caller may view the given post, by its status.
//
// This is the SINGLE authority for post visibility (AGENTS.md hard constraint):
// handlers and the router must never branch on status themselves, and
// posts_list.go's authorOnly slice must stay in sync with this function or the
// list shows a post that the detail endpoint then 404s.
//
// The states:
//   - published: everyone, including anonymous readers.
//   - banned: root only (the vault is invisible to the author and to admins).
//   - private / draft: the author ONLY. The UI promises "仅自己可见", so admins
//     and root are deliberately excluded too — neither state is ever published,
//     so there is nothing to moderate, and an admin who could read it would make
//     the promise a lie. Draft is the retired legacy state, treated identically.
//   - pending / rejected: the author reads their own submission; admins and root
//     read it in order to review it.
//
// Anything unknown is not visible: a new status must be opted in deliberately
// rather than leaking by default.
//
// A post the caller may not see answers 404, never 403 — the response must not
// disclose that the content exists.
func canSeePost(user models.User, post models.Post) bool {
	switch post.Status {
	case models.PostPublished:
		return true // public
	case models.PostBanned:
		// Banned vault: only root sees it.
		return user.Role == models.RoleRoot
	case models.PostPrivate, models.PostDraft:
		// Strictly author-only. The UI promises "仅自己可见", so admins and
		// root are deliberately excluded too: neither state is ever published,
		// so there is nothing to moderate, and an admin who could read it
		// would make the promise a lie. Draft is the retired legacy state and
		// is treated the same way.
		return post.AuthorID == user.ID
	case models.PostPending, models.PostRejected:
		// Review flow: the author reads their own submission, admins and root
		// read it in order to review it.
		return post.AuthorID == user.ID || user.Role == models.RoleAdmin || user.Role == models.RoleRoot
	default:
		return false
	}
}
