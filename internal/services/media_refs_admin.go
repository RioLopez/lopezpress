package services

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"lopezpress/internal/models"
)

// --- media page helpers: per-image refs and bulk delete (B3) ---------

// maxBatchUploads caps one bulk delete. Same rationale as the post batch:
// each item opens its own transaction on a single-writer SQLite database.
const maxBatchUploads = 100

// ListUploadRefs answers "what would break if this file disappeared". It
// reads the same ledger the delete warning uses, so the detail sheet and the
// confirmation cannot disagree. Post titles are joined in two small queries
// rather than a join, because most images have one or two edges.
func (s *Service) ListUploadRefs(ctx context.Context, actor models.User, name string) ([]UploadRefInfo, error) {
	if !IsAdminOrRoot(actor) {
		return nil, newErr(http.StatusForbidden, "only an admin or the root can inspect references")
	}
	var row models.UploadFile
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&row).Error; err != nil {
		return nil, newErr(http.StatusNotFound, "image not found")
	}
	var refs []models.UploadRef
	if err := s.db.WithContext(ctx).Where("upload_id = ?", row.ID).
		Order("kind ASC, created_at ASC").Find(&refs).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to read references")
	}
	posts, users := s.refLabels(ctx, refs)

	out := make([]UploadRefInfo, 0, len(refs))
	for _, r := range refs {
		out = append(out, UploadRefInfo{
			Kind: r.Kind, PostID: r.PostID,
			PostTitle: posts[r.PostID], UserID: r.UserID, UserName: users[r.UserID],
		})
	}
	return out, nil
}

// refLabels resolves post titles and user display names for one page of edges.
func (s *Service) refLabels(ctx context.Context, refs []models.UploadRef) (map[string]string, map[string]string) {
	postIDs := make([]string, 0, len(refs))
	userIDs := make([]string, 0, len(refs))
	seenPost, seenUser := map[string]bool{}, map[string]bool{}
	for _, r := range refs {
		if r.PostID != "" && !seenPost[r.PostID] {
			seenPost[r.PostID] = true
			postIDs = append(postIDs, r.PostID)
		}
		if r.UserID != "" && !seenUser[r.UserID] {
			seenUser[r.UserID] = true
			userIDs = append(userIDs, r.UserID)
		}
	}
	posts := make(map[string]string, len(postIDs))
	users := make(map[string]string, len(userIDs))
	if len(postIDs) > 0 {
		var rows []models.Post
		if err := s.db.WithContext(ctx).Select("id", "title").
			Where("id IN ?", postIDs).Find(&rows).Error; err == nil {
			for _, p := range rows {
				posts[p.ID] = p.Title
			}
		}
	}
	if len(userIDs) > 0 {
		var rows []models.User
		if err := s.db.WithContext(ctx).Select("id", "username", "display_name").
			Where("id IN ?", userIDs).Find(&rows).Error; err == nil {
			for _, u := range rows {
				if u.DisplayName != "" {
					users[u.ID] = u.DisplayName
				} else {
					users[u.ID] = u.Username
				}
			}
		}
	}
	return posts, users
}

// BatchDeleteUploads applies one delete verb to many images. Each row goes
// through DeleteUpload, so the permission matrix, the hard-delete gate and
// the reference count are inherited rather than re-implemented.
func (s *Service) BatchDeleteUploads(ctx context.Context, actor models.User, in UploadBatchInput) (UploadBatchResult, error) {
	if len(in.Names) == 0 {
		return UploadBatchResult{}, newErr(http.StatusBadRequest, "names is required")
	}
	if len(in.Names) > maxBatchUploads {
		return UploadBatchResult{}, newErr(http.StatusBadRequest, "too many names (max 100)")
	}
	if in.Hard && !IsAdminOrRoot(actor) {
		return UploadBatchResult{}, newErr(http.StatusForbidden, "only an admin can permanently delete an image")
	}
	res := UploadBatchResult{Skipped: []UploadBatchSkip{}}
	seen := make(map[string]bool, len(in.Names))
	for _, raw := range in.Names {
		name := strings.TrimSpace(raw)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if _, err := s.DeleteUpload(ctx, actor, name, in.Hard); err != nil {
			res.Skipped = append(res.Skipped, UploadBatchSkip{Name: name, Reason: MsgOf(err)})
			continue
		}
		res.Affected++
	}
	s.audit(ctx, actor, "batch_delete_uploads", "uploads",
		"hard="+strconv.FormatBool(in.Hard)+"; affected="+strconv.Itoa(res.Affected)+
			"; skipped="+strconv.Itoa(len(res.Skipped)))
	return res, nil
}
