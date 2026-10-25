package services

// The two "merge a submitted config into the stored row" policies live here so
// site_config.go stays about validation + persistence. Each spec that adds
// numeric settings (36 storage quotas, 37 code timings) contributes one, and
// they share the same absent-means-inherit rule.

import "lopezpress/internal/models"

// applyStoragePolicy folds the spec-36 byte quotas into existing.
//
// These are the only settings where "absent" and "zero" are the same thing in
// JSON, and where zero means "no limit". A settings form that predates spec 36
// (or any API client sending a partial body) would therefore silently switch
// the disk protection off on save. So a non-positive value inherits the stored
// one, and whatever survives is clamped to a range that cannot brick the site:
// an upload ceiling below 64 KB would reject real avatars, and a site quota
// above 10 GB would defeat the purpose on a VPS with a 20 GB disk.
func applyStoragePolicy(existing *models.SiteConfig, in models.SiteConfig) {
	if in.UploadMaxImage <= 0 {
		in.UploadMaxImage = existing.UploadMaxImage
	}
	if in.UploadMaxAvatar <= 0 {
		in.UploadMaxAvatar = existing.UploadMaxAvatar
	}
	if in.UserQuotaBytes <= 0 {
		in.UserQuotaBytes = existing.UserQuotaBytes
	}
	if in.SiteQuotaBytes <= 0 {
		in.SiteQuotaBytes = existing.SiteQuotaBytes
	}
	existing.UploadMaxImage = clampInt64(in.UploadMaxImage, 64<<10, 20<<20)
	existing.UploadMaxAvatar = clampInt64(in.UploadMaxAvatar, 32<<10, 5<<20)
	existing.UserQuotaBytes = clampInt64(in.UserQuotaBytes, 1<<20, 5<<30)
	existing.SiteQuotaBytes = clampInt64(in.SiteQuotaBytes, 20<<20, 10<<30)
	existing.AllowUserDeleteUpload = in.AllowUserDeleteUpload
	existing.AllowUserChangeAvatar = in.AllowUserChangeAvatar
	existing.AllowExternalAvatar = in.AllowExternalAvatar
}

// applyEmailVerifyPolicy folds the spec-37 timings into existing, applying the
// same "absent means inherit" rule as the quotas: a settings form that predates
// this feature would otherwise rewrite a tuned TTL to zero on every save.
func applyEmailVerifyPolicy(existing *models.SiteConfig, in models.SiteConfig) {
	existing.EmailVerifyEnabled = in.EmailVerifyEnabled
	if in.EmailCodeTTLMin <= 0 {
		in.EmailCodeTTLMin = existing.EmailCodeTTLMin
	}
	if in.EmailCodeCooldown <= 0 {
		in.EmailCodeCooldown = existing.EmailCodeCooldown
	}
	existing.EmailCodeTTLMin = int(clampInt64(int64(in.EmailCodeTTLMin), 2, 60))
	existing.EmailCodeCooldown = int(clampInt64(int64(in.EmailCodeCooldown), 30, 600))
	// A cooldown at or beyond the lifetime leaves the visitor unable to request
	// a second code before the first expires — the resend button would be a
	// lie. Halve it rather than reject the save.
	if existing.EmailCodeCooldown >= existing.EmailCodeTTLMin*60 {
		existing.EmailCodeCooldown = existing.EmailCodeTTLMin * 30
	}
}
