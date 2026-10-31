package services

import (
	"lopezpress/internal/models"
)

// resolveStatusChange applies one save request's status intent to post.
//
// The governing rule is about readers, not roles: a post nobody else can read
// needs no review, and a post that becomes readable must have one. So private
// is always honoured for the author, and "make it public" always routes
// through the queue unless the caller is root.
//
// Why this is not just "honour the field": the editor offers two radios, and
// before this function they meant the same thing. A non-root author who picked
// 私密 got a pending post (CreatePost forced it), and approving that post
// published it — the exact opposite of the promise printed under the radio.
// Silent no-ops are what made that possible, so every branch below is explicit
// and the cases that change nothing say so.
//
// contentChanged says whether this save altered the words (title, body or
// cover), which the caller computes before calling. It matters more than the
// requested status: an approval covers the text that was reviewed, not the row
// forever. A non-root author editing a LIVE post therefore goes back into the
// queue — otherwise "reviewed" would mean "reviewed once, editable at will",
// and a perfectly good first submission buys unlimited later rewrites. That
// rule is enforced at the end of this function, not inside the requested-status
// switch, precisely so it cannot be dodged by omitting the field.
//
// Callers have already enforced edit permission and the banned-post freeze;
// this function only decides the resulting status.
func resolveStatusChange(post *models.Post, me models.User, requested string, contentChanged bool) {
	isAuthor := post.AuthorID == me.ID
	isRoot := me.Role == models.RoleRoot

	switch requested {
	case models.PostPrivate:
		// Unpublishing. A reviewer may pull someone else's *live* post down —
		// that is moderation, and it is the one private transition an admin
		// gets on content they did not write. They cannot reach private from
		// pending/rejected, because that would be taking over the author's
		// decision about a post that was never public.
		switch {
		case isRoot || isAuthor:
			post.Status = models.PostPrivate
			// A withdrawn submission carries no verdict: keeping the old note
			// would show a rejection reason on a post no longer under review.
			post.ReviewNote = ""
		case me.Role == models.RoleAdmin && post.Status == models.PostPublished:
			post.Status = models.PostPrivate
		}

	case models.PostPublished, models.PostPending:
		// Going public. Root at any time; an author only from a state that was
		// never public (private or rejected). A live post is handled by the
		// content-changed rule at the end of this function: it stays published
		// when nothing but metadata moved, and re-enters the queue when the
		// words did.
		switch {
		case isRoot:
			if requested == models.PostPublished {
				post.Status = models.PostPublished
				post.ReviewNote = ""
			}
		case isAuthor && (post.Status == models.PostPrivate || post.Status == models.PostRejected):
			// Both requests mean the same thing here: submit for review. The
			// author asked for readers, and only a reviewer can grant that.
			post.Status = models.PostPending
			post.ReviewNote = ""
		}
	}

	// The re-review rule, deliberately outside the switch above.
	//
	// It fires on content alone, so it applies whether the request said
	// "published", "pending", or said nothing at all. `post.Status` is read
	// after the switch: an author who asked for 私密 in the same save already
	// reached a state with no readers, and pulling them into the queue would
	// resurrect the "private means pending" bug this function exists to kill.
	//
	// Root is exempt because root is the reviewer — there is no one above it
	// to ask — and an editor who did not write the post is exempt because
	// reviewers never change the words here (R5-05 refuses that earlier).
	if contentChanged && post.Status == models.PostPublished &&
		post.AuthorID == me.ID && !isRoot {
		post.Status = models.PostPending
		post.ReviewNote = ""
	}
}
