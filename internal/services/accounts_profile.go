package services

import (
	"context"
	"net/http"
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

// UpdateMeInput is the decoded payload for PUT /api/auth/me.
type UpdateMeInput struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Avatar      string `json:"avatar"`
}

// UpdateMe edits the caller's own profile. Empty fields are left unchanged so
// a partial save never wipes the others.
func (s *Service) UpdateMe(ctx context.Context, id string, in UpdateMeInput) (models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusNotFound, "user not found")
	}
	if in.DisplayName != "" {
		user.DisplayName = in.DisplayName
	}
	if in.Bio != "" {
		user.Bio = in.Bio
	}
	if in.Avatar != "" {
		user.Avatar = in.Avatar
	}
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update profile")
	}
	return user, nil
}

// AuthorUser is the public, sanitised view of an author.
type AuthorUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Avatar      string `json:"avatar"`
}

// AuthorView is the payload for GET /api/authors/{username}.
type AuthorView struct {
	User  AuthorUser    `json:"user"`
	Posts []models.Post `json:"posts"`
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

// DashboardData aggregates the admin dashboard widgets.
type DashboardData struct {
	PostsTotal int64 `json:"posts_total"`
	ViewsTotal int64 `json:"views_total"`
	// Spec 33: the dashboard answers "is anyone reading?" not just "how many
	// posts exist". PendingTotal is what needs a human today; ViewTrend is the
	// last 30 days of de-duplicated reads; TopPosts is what people actually read.
	PendingTotal int64             `json:"pending_total"`
	ViewTrend    []DayViews        `json:"view_trend"`
	TopPosts     []TopPost         `json:"top_posts"`
	RecentPosts  []models.Post     `json:"recent_posts"`
	RecentLogins []models.LoginLog `json:"recent_logins"`
}

// DayViews is one day of the view trend. Days with no reads are present with
// Views = 0 so the client never has to fill gaps itself.
type DayViews struct {
	Day   string `json:"day"`
	Views int64  `json:"views"`
}

// TopPost is a most-read post. Only published posts qualify: a banned or
// pending post's read count is not something an author should be paid
// attention for.
type TopPost struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	ViewCount int64  `json:"view_count"`
}

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
