package services

import (
	"context"
	"time"

	"lopezpress/internal/models"
)

// --- dashboard ---

// dashboardTrendDays is the trend window. 30 days keeps the payload under a
// few hundred bytes and still shows a month of rhythm on a 1 vCPU box.
const dashboardTrendDays = 30

// GetDashboard returns post totals, sum of views, recent posts, and recent
// successful logins for the admin dashboard. actor decides the root-only
// slices (banned count, backup state).
//
// Kept in its own file: the overview keeps growing (reading pulse, status
// breakdown, storage, audit tail), and accounts_profile.go is already a
// long profile module — one 150-line read-only aggregator does not belong
// in the middle of it.
func (s *Service) GetDashboard(ctx context.Context, actor models.User) (DashboardData, error) {
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
	// The dashboard is a summary, not a reader. Shipping the full
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

	// 30-Day view trend. post_views.day is stored as "YYYY-MM-DD" in
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

	// Where the posts actually are. One grouped scan instead of five counts.
	var statusRows []struct {
		Status string
		N      int64
	}
	s.db.WithContext(ctx).Model(&models.Post{}).
		Select("status, COUNT(*) AS n").Group("status").Scan(&statusRows)
	counts := StatusCounts{}
	var banned int64
	for _, r := range statusRows {
		switch r.Status {
		case "published":
			counts.Published = r.N
		case "pending":
			counts.Pending = r.N
		case "draft":
			counts.Draft = r.N
		case "private":
			counts.Private = r.N
		case "banned":
			banned = r.N
		}
	}
	// The ban vault is root-only; a count would still reveal that content an
	// admin cannot see exists.
	if IsRoot(actor) {
		counts.Banned = &banned
	}

	todayStr := today.Format("2006-01-02")
	var viewsToday, views7d int64
	s.db.WithContext(ctx).Model(&models.PostView{}).
		Where("day = ?", todayStr).Count(&viewsToday)
	s.db.WithContext(ctx).Model(&models.PostView{}).
		Where("day >= ?", today.AddDate(0, 0, -6).Format("2006-01-02")).Count(&views7d)

	now := time.Now().UTC()
	var usersTotal, usersDisabled, usersNew7d int64
	s.db.WithContext(ctx).Model(&models.User{}).Count(&usersTotal)
	s.db.WithContext(ctx).Model(&models.User{}).Where("disabled = ?", true).Count(&usersDisabled)
	s.db.WithContext(ctx).Model(&models.User{}).
		Where("created_at >= ?", now.AddDate(0, 0, -7)).Count(&usersNew7d)

	// Soft-deleted uploads keep their bytes on disk (published articles may
	// still reference them), but they are not part of the usable library.
	var media struct {
		N          int64
		TotalBytes int64
	}
	s.db.WithContext(ctx).Model(&models.UploadFile{}).
		Select("COUNT(*) AS n, COALESCE(SUM(size),0) AS total_bytes").
		Where("deleted_at IS NULL").Scan(&media)

	actions, err := s.ListActions(ctx, "", 5)
	if err != nil {
		return DashboardData{}, err
	}

	// The backup surface is root-only; a plain admin gets no backup widget at
	// all rather than an empty one that implies there is nothing to see.
	var backup *AutoBackupStatus
	if IsRoot(actor) {
		if st, err := s.AutoBackupReport(ctx, actor); err == nil {
			backup = &st
		}
	}

	return DashboardData{
		PostsTotal:     postsTotal,
		ViewsTotal:     viewsTotal,
		PendingTotal:   pendingTotal,
		ViewTrend:      trend,
		TopPosts:       topPosts,
		RecentPosts:    recentPosts,
		RecentLogins:   recentLogins,
		ViewsToday:     viewsToday,
		Views7d:        views7d,
		StatusCounts:   counts,
		UsersTotal:     usersTotal,
		UsersDisabled:  usersDisabled,
		UsersNew7d:     usersNew7d,
		MediaLiveCount: media.N,
		MediaLiveBytes: media.TotalBytes,
		AutoBackup:     backup,
		RecentActions:  actions,
	}, nil
}
