package config

import (
	"fmt"

	"gorm.io/gorm"
)

// columnSpec pairs a column name (for the existence probe) with the full
// ALTER TABLE ... ADD COLUMN statement.
type columnSpec struct {
	name string
	ddl  string
}

// ensureColumns builds an "up" function that adds the given columns to an
// existing table, then runs the idempotent `after` SQL (indexes / backfills).
// On a fresh database (table not yet created) it is a no-op: AutoMigrate
// creates the full table afterwards.
func ensureColumns(model interface{}, cols []columnSpec, after ...string) func(*gorm.DB) error {
	return func(db *gorm.DB) error {
		if !db.Migrator().HasTable(model) {
			return nil
		}
		for _, c := range cols {
			if db.Migrator().HasColumn(model, c.name) {
				continue
			}
			if err := db.Exec(c.ddl).Error; err != nil {
				return fmt.Errorf("add %s: %w", c.name, err)
			}
		}
		for _, s := range after {
			if err := db.Exec(s).Error; err != nil {
				return fmt.Errorf("post-migration: %w", err)
			}
		}
		return nil
	}
}
