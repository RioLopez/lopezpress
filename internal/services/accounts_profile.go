package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"lopezpress/internal/models"
)

func (s *Service) ListUserLogins(ctx context.Context, id string, limit int) ([]models.LoginLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
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

	if in.Username != nil {
		name := strings.TrimSpace(*in.Username)
		if name == "" {
			return models.User{}, newErr(http.StatusBadRequest, "username is required")
		}
		if name != user.Username {
			var dup models.User
			if s.db.WithContext(ctx).Where("username = ?", name).First(&dup).RowsAffected == 1 {
				return models.User{}, newErr(http.StatusConflict, "username already in use")
			}
			user.Username = name
		}
	}
	if in.Email != nil {
		email := strings.TrimSpace(*in.Email)
		if email == "" {
			return models.User{}, newErr(http.StatusBadRequest, "email is required")
		}
		if !isValidEmail(email) {
			return models.User{}, newErr(http.StatusBadRequest, "invalid email")
		}
		if email != user.Email {
			var dup models.User
			if s.db.WithContext(ctx).Where("email = ?", email).First(&dup).RowsAffected == 1 {
				return models.User{}, newErr(http.StatusConflict, "email already in use")
			}
			user.Email = email
		}
	}
	if in.DisplayName != nil {
		user.DisplayName = strings.TrimSpace(*in.DisplayName)
	}
	if in.Bio != nil {
		user.Bio = *in.Bio
	}
	if in.Avatar != nil {
		user.Avatar = strings.TrimSpace(*in.Avatar)
	}
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update profile")
	}
	return user, nil
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
