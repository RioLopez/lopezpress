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
// this function only decides the resulting status. mayPublish is the
// canPublish verdict for this actor and this post's author (root always,
// admin for plain users, nobody else); it is computed by the caller so this
// function does not need a second author lookup.
func resolveStatusChange(post *models.Post, me models.User, requested string, contentChanged, mayPublish bool) {
	isAuthor := post.AuthorID == me.ID
	isRoot := IsRoot(me)

	switch requested {
	case models.PostPrivate:
		// Unpublishing. A reviewer may pull someone else's *live* post down —
		// that is moderation. They cannot reach private from pending/rejected,
		// because that would take over the author's decision about a post that
		// was never public.
		switch {
		case isRoot || isAuthor:
			post.Status = models.PostPrivate
			// A withdrawn submission carries no verdict.
			post.ReviewNote = ""
		case IsAdmin(me) && post.Status == models.PostPublished:
			post.Status = models.PostPrivate
		}

	case models.PostPublished, models.PostPending:
		// Going public. Root any time; an admin may publish a plain user's post
		// (same scope as approving it); an author only from a never-public
		// state. A live post is covered by the content rule at the end.
		switch {
		case isRoot:
			if requested == models.PostPublished {
				post.Status = models.PostPublished
				post.ReviewNote = ""
			}
		case mayPublish && requested == models.PostPublished:
			post.Status = models.PostPublished
			post.ReviewNote = ""
			// Approval drops a plain member's self-pin; publishing for them is
			// the same act through a different door.
			post.Pinned = false
		case isAuthor && (post.Status == models.PostPrivate || post.Status == models.PostRejected):
			// Both requests mean "submit for review": the author asked for
			// readers, and only a reviewer can grant that.
			post.Status = models.PostPending
			post.ReviewNote = ""
		}
	}

	// The re-review rule, deliberately outside the switch above. It fires on
	// content alone, so it applies whether the request said "published",
	// "pending", or nothing. `post.Status` is read after the switch: an author
	// who asked for 私密 already reached a state with no readers, and queuing
	// them would resurrect the "private means pending" bug this function kills.
	// Root is exempt (root is the reviewer) and an editor who did not write the
	// post is exempt because reviewers never change the words here (R5-05).
	if contentChanged && post.Status == models.PostPublished &&
		post.AuthorID == me.ID && !isRoot {
		post.Status = models.PostPending
		post.ReviewNote = ""
	}
}
