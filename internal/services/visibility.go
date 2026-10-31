package services

import (
	"lopezpress/internal/models"
)

// canSeePost reports whether the caller may view the given post, by its status.
//
// This is the SINGLE authority for post visibility: handlers and the router
// must never branch on status themselves, and posts_list.go's applyFilters must
// express the same predicate in SQL — if the two drift apart the list shows a
// post that the detail endpoint then 404s.
//
// The states:
//   - published: everyone, including anonymous readers.
//   - banned: root only (the vault is invisible to the author and to admins).
//   - private / draft / pending / rejected: the author, and admin/root. A
//     private post reaches no reader, so it never needs review — it skips the
//     queue entirely. But "no reader" is not "no operator": an admin who cannot
//     open the article cannot investigate a report about it, verify that a
//     takedown was correct, or answer the author's question. So the operator
//     roles can read it, and the editor says so instead of promising "only you
//     can see this". `draft` is the retired legacy state and behaves
//     identically.
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
		// Banned vault: only root sees it. The author is excluded on purpose:
		// a ban is a take-down, and showing the author their content back
		// would defeat it.
		return user.Role == models.RoleRoot
	case models.PostPrivate, models.PostDraft, models.PostPending, models.PostRejected:
		return post.AuthorID == user.ID || IsAdminOrRoot(user)
	default:
		return false
	}
}
