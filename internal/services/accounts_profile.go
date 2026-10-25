package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// ListUserLogins returns the most recent login_log rows for a user.
//
// The root account's trail is root-only: IP, user agent and identifier of the
// super-admin are exactly the reconnaissance an admin needs to plan a
// takeover, and no admin workflow requires them.
func (s *Service) ListUserLogins(ctx context.Context, actor models.User, id string, limit int) ([]models.LoginLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var target models.User
	if err := s.db.WithContext(ctx).Select("id, role").Where("id = ?", id).First(&target).Error; err != nil {
		return nil, newErr(http.StatusNotFound, "user not found")
	}
	if target.Role == models.RoleRoot && !IsRoot(actor) {
		return nil, newErr(http.StatusForbidden, "only the root can view the root account's login history")
	}
	var logs []models.LoginLog
	if err := s.db.WithContext(ctx).Where("user_id = ?", id).
		Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load login logs")
	}
	if logs == nil {
		logs = []models.LoginLog{}
	}
	return logs, nil
}

// --- profile / author (spec 15) ---

// UpdateMe edits the caller's own profile. A nil pointer leaves a field
// unchanged; a non-nil pointer (including an empty string) overwrites it, so
// a partial save never wipes unmentioned fields while still allowing the
// caller to clear bio/avatar/display name explicitly.
//
// Username / email changes are validated for format + uniqueness. Changing
// either invalidates the login identifier: the client should treat a username
// or email change as requiring a re-login (the returned user reflects the new
// value, but the current JWT's identity is unchanged until re-issued).
func (s *Service) UpdateMe(ctx context.Context, id string, in UpdateMeInput) (models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusNotFound, "user not found")
	}

	if err := s.applyIdentityChange(ctx, &user, in); err != nil {
		return models.User{}, err
	}
	if in.DisplayName != nil {
		user.DisplayName = strings.TrimSpace(*in.DisplayName)
	}
	if in.Bio != nil {
		user.Bio = *in.Bio
	}
	// Both fields ride along in every public list response that includes this
	// user's posts (the author byline), so an unbounded one is multiplied
	// across the whole site rather than paid once (R3-07's class).
	if err := ValidateProfileText(user.DisplayName, user.Bio); err != nil {
		return models.User{}, err
	}
	if in.Avatar != nil {
		avatar := strings.TrimSpace(*in.Avatar)
		if avatar != user.Avatar {
			cfg, _ := s.GetSiteConfig(ctx, false)
			if !cfg.AllowUserChangeAvatar {
				return models.User{}, newErr(http.StatusForbidden, "changing your avatar is disabled by the site owner")
			}
			if err := s.validateAvatarValue(ctx, avatar, cfg.AllowExternalAvatar); err != nil {
				return models.User{}, err
			}
		}
		user.Avatar = avatar
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		// Spec 36: keep the avatar edge in the image ledger in step, so the
		// gallery can tell whose picture a file is (and refuse to hard-delete
		// it silently).
		return setAvatarUploadRefTx(tx, user.ID, user.Avatar)
	})
	if err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update profile")
	}
	return user, nil
}

// validateAvatarValue accepts exactly two shapes: an image that already lives
// in this site's library, or — only when the site owner allows it — an
// absolute http(s) URL.
//
// Anything else is rejected rather than stored. Before spec 36 the avatar was
// a free-form string: any member could point it at a tracking pixel, a
// javascript: payload that some future renderer might honour, or a file that
// does not exist. Since every avatar is rendered on every post the user
// authored, that is a site-wide surface, not a personal one.
func (s *Service) validateAvatarValue(ctx context.Context, value string, allowExternal bool) error {
	if value == "" {
		return nil // clearing the avatar is always allowed
	}
	if name := uploadNameFromURL(value); name != "" {
		var n int64
		if err := s.db.WithContext(ctx).Model(&models.UploadFile{}).
			Where("name = ? AND deleted_at IS NULL", name).Count(&n).Error; err != nil {
			return newErr(http.StatusInternalServerError, "failed to check the image")
		}
		if n == 0 {
			return newErr(http.StatusBadRequest, "that image is not in the library")
		}
		return nil
	}
	if allowExternal {
		low := strings.ToLower(value)
		if strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "http://") {
			if len([]rune(value)) > 500 {
				return newErr(http.StatusBadRequest, "avatar url too long")
			}
			return nil
		}
	}
	return newErr(http.StatusBadRequest, "avatar must be an image uploaded to this site")
}

// isValidEmail is a lightweight structural check: exactly one "@", non-empty
// local and domain parts, and a dot in the domain. It is intentionally loose
// (full RFC 5322 validation is overkill for a self-hosted blog) but rejects
// obviously malformed input before it hits the unique index.
func isValidEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	domain := s[at+1:]
	return strings.Contains(domain, ".")
}

// isValidUsername enforces an ASCII-only handle: 3-32 characters from
// [A-Za-z0-9_-], starting with a letter or a digit.
//
// ASCII-only is the whole point. Cyrillic "а" (U+0430) and Latin "a" render
// identically, so allowing Unicode lets an attacker register "аdmin" and
// impersonate the real account in every list that shows names. The byte loop
// below rejects anything outside ASCII by construction.
//
// Uniqueness is checked case-insensitively by the callers (LOWER(username)),
// so "Admin" and "ADMIN" cannot coexist either — a second impersonation
// channel that a plain unique index misses on SQLite.
func isValidUsername(s string) bool {
	if len(s) < 3 || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '_' || c == '-':
			if i == 0 {
				return false // no leading separator
			}
		default:
			return false
		}
	}
	return true
}

// GetAuthor returns a public author profile plus their published posts. A
// missing or disabled user yields 404 (not exposed).
func (s *Service) GetAuthor(ctx context.Context, username string) (AuthorView, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return AuthorView{}, newErr(http.StatusNotFound, "author not found")
	}
	if user.Disabled {
		return AuthorView{}, newErr(http.StatusNotFound, "author not found")
	}
	view := AuthorView{
		User: AuthorUser{
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Bio:         user.Bio,
			Avatar:      user.Avatar,
		},
	}
	var posts []models.Post
	if err := s.db.WithContext(ctx).
		Where("author_id = ? AND status = ?", user.ID, "published").
		Preload("Category").Preload("Tags").
		Order("created_at DESC").Find(&posts).Error; err != nil {
		return AuthorView{}, newErr(http.StatusInternalServerError, "failed to load posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	// The author page is a public surface: the reviewer identity must not ride
	// along with it (R1-04 / R2-07).
	models.SanitizePostReviews(posts)
	view.Posts = posts
	return view, nil
}

// --- dashboard (spec 06) ---

// dashboardTrendDays is the trend window. 30 days keeps the payload under a
// few hundred bytes and still shows a month of rhythm on a 1 vCPU box.
const dashboardTrendDays = 30

// GetDashboard returns post totals, sum of views, recent posts, and recent
// successful logins for the admin dashboard.
func (s *Service) GetDashboard(ctx context.Context) (DashboardData, error) {
	var postsTotal int64
	s.db.WithContext(ctx).Model(&models.Post{}).Count(&postsTotal)

	var viewsTotal int64
	s.db.WithContext(ctx).Model(&models.Post{}).
		Select("COALESCE(SUM(view_count),0)").Scan(&viewsTotal)

	var recentPosts []models.Post
	s.db.WithContext(ctx).Where("status = ?", "published").
		Preload("Author").Preload("Category").
		Order("created_at DESC").Limit(5).Find(&recentPosts)
	if recentPosts == nil {
		recentPosts = []models.Post{}
	}
	// Spec 31: the dashboard is a summary, not a reader. Shipping the full
	// body of the five newest posts made the admin home page the heaviest
	// response in the app (the observed demo payload was ~4 KB of Markdown for
	// a page that renders titles only).
	for i := range recentPosts {
		recentPosts[i].Body = ""
	}

	var recentLogins []models.LoginLog
	s.db.WithContext(ctx).Where("success = ?", true).
		Order("created_at DESC").Limit(5).Find(&recentLogins)
	if recentLogins == nil {
		recentLogins = []models.LoginLog{}
	}

	var pendingTotal int64
	s.db.WithContext(ctx).Model(&models.Post{}).Where("status = ?", "pending").Count(&pendingTotal)

	// Spec 33: 30-day view trend. post_views.day is stored as "YYYY-MM-DD" in
	// UTC, so GROUP BY day is a plain string aggregation (one indexed scan).
	today := time.Now().UTC().Truncate(24 * time.Hour)
	since := today.AddDate(0, 0, -(dashboardTrendDays - 1)).Format("2006-01-02")
	var dayRows []struct {
		Day string
		N   int64
	}
	s.db.WithContext(ctx).Model(&models.PostView{}).
		Select("day, COUNT(*) AS n").
		Where("day >= ?", since).
		Group("day").Scan(&dayRows)
	byDay := make(map[string]int64, len(dayRows))
	for _, r := range dayRows {
		byDay[r.Day] = r.N
	}
	trend := make([]DayViews, 0, dashboardTrendDays)
	for i := dashboardTrendDays - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i).Format("2006-01-02")
		trend = append(trend, DayViews{Day: d, Views: byDay[d]})
	}

	var topPosts []TopPost
	s.db.WithContext(ctx).Model(&models.Post{}).
		Select("id, title, slug, view_count").
		Where("status = ? AND view_count > 0", "published").
		Order("view_count DESC").Limit(5).Scan(&topPosts)
	if topPosts == nil {
		topPosts = []TopPost{}
	}

	return DashboardData{
		PostsTotal:   postsTotal,
		ViewsTotal:   viewsTotal,
		PendingTotal: pendingTotal,
		ViewTrend:    trend,
		TopPosts:     topPosts,
		RecentPosts:  recentPosts,
		RecentLogins: recentLogins,
	}, nil
}

// applyIdentityChange validates and applies the login-identifier fields
// (username / email). Split out of UpdateMe so each function stays reviewable:
// the profile update has five independent field groups, and one 70-line body
// hid which validation belonged to which.
func (s *Service) applyIdentityChange(ctx context.Context, user *models.User, in UpdateMeInput) error {
	if in.Username != nil {
		name := strings.TrimSpace(*in.Username)
		if name == "" {
			return newErr(http.StatusBadRequest, "username is required")
		}
		if !isValidUsername(name) {
			return newErr(http.StatusBadRequest, "username must be 3-32 characters: letters, digits, _ or -, starting with a letter or digit")
		}
		if name != user.Username {
			var dup models.User
			if s.db.WithContext(ctx).Where("LOWER(username) = LOWER(?)", name).First(&dup).RowsAffected == 1 {
				return newErr(http.StatusConflict, "username already in use")
			}
			user.Username = name
		}
	}
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		if email == "" {
			return newErr(http.StatusBadRequest, "email is required")
		}
		if !isValidEmail(email) {
			return newErr(http.StatusBadRequest, "invalid email")
		}
		if email != user.Email {
			// Spec 37: with email verification switched on, the address may only
			// move through the code flow (ConfirmEmailChange). Without this
			// guard the whole feature would be one PUT away from being bypassed
			// — the form is a convenience, the server is the contract.
			if s.emailCodePolicy(ctx).Enabled {
				return newErr(http.StatusForbidden, "changing your email needs a verification code; use the verified email change flow")
			}
			var dup models.User
			if s.db.WithContext(ctx).Where("LOWER(email) = LOWER(?)", email).First(&dup).RowsAffected == 1 {
				return newErr(http.StatusConflict, "email already in use")
			}
			user.Email = email
		}
	}
	return nil
}
