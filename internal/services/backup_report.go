package services

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// --- backup directory report -------------------------------------------------
//
// The settings page needs three answers the database cannot give: how many
// snapshots are on disk, when the newest one was written, and exactly which
// folder holds the newest attachment snapshot. This file owns that directory
// walk so autobackup.go stays focused on the schedule itself.

// fillBackupDirStats counts the snapshots on disk and remembers the newest of
// each kind. A missing directory leaves every counter at its zero value.
func fillBackupDirStats(st *AutoBackupStatus) {
	entries, err := os.ReadDir(st.Dir)
	if err != nil {
		return
	}
	var newest, newestUp time.Time
	var newestUpName string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			if strings.HasPrefix(n, uploadsSnapshotPrefix) {
				st.UploadsSnapshots++
				if info, err := e.Info(); err == nil && info.ModTime().After(newestUp) {
					newestUp = info.ModTime()
					newestUpName = n
				}
			}
			continue
		}
		if !strings.HasPrefix(n, autoBackupPrefix) || !strings.HasSuffix(n, ".db") {
			continue
		}
		st.Files++
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if !newest.IsZero() {
		st.LastAt = newest.UTC().Format(time.RFC3339)
	}
	if !newestUp.IsZero() {
		st.UploadsLastAt = newestUp.UTC().Format(time.RFC3339)
	}
	if newestUpName != "" {
		st.UploadsSnapshotName = newestUpName
		st.UploadsSnapshotDir = filepath.Join(st.Dir, newestUpName)
		if abs, err := filepath.Abs(st.UploadsSnapshotDir); err == nil {
			st.UploadsSnapshotDirAbs = abs
		}
	}
}
