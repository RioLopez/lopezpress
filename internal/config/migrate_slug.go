package config

// Slug de-duplication used by migration 0014. Split out of migrate.go so the
// migration registry stays a readable list; the logic is only ever needed by
// that one migration.

import (
	"strconv"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// disambiguateDuplicateSlugs rewrites colliding slugs so a unique index can be
// built over them. The oldest row in each group keeps the slug; the rest get a
// numeric suffix, which is what a sequential allocate would have produced.
func disambiguateDuplicateSlugs(db *gorm.DB) error {
	// A brand-new database has no posts table yet — AutoMigrate creates it after
	// this pass, so there is nothing to disambiguate. Checking first keeps the
	// migration idempotent on both a fresh install and an upgrade.
	if !db.Migrator().HasTable(&models.Post{}) {
		return nil
	}
	var groups []string
	if err := db.Model(&models.Post{}).
		Where("slug <> ''").
		Group("slug").
		Having("COUNT(*) > 1").
		Pluck("slug", &groups).Error; err != nil {
		return err
	}
	for _, slug := range groups {
		var rows []models.Post
		if err := db.Where("slug = ?", slug).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
			return err
		}
		for i := 1; i < len(rows); i++ {
			// Try successive suffixes; a collision with an existing slug moves
			// to the next number rather than creating a new duplicate.
			for n := 2; n < 1000; n++ {
				candidate := slug + "-" + strconv.Itoa(n)
				var clash int64
				if err := db.Model(&models.Post{}).Where("slug = ?", candidate).Count(&clash).Error; err != nil {
					return err
				}
				if clash > 0 {
					continue
				}
				if err := db.Model(&models.Post{}).Where("id = ?", rows[i].ID).
					Update("slug", candidate).Error; err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}
