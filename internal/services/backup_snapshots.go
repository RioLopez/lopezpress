package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// --- snapshot file management ------------------------------------------------
//
// Split out of backup.go: taking a VACUUM INTO snapshot, deciding which files
// count as one, and pruning old ones are all file-housekeeping concerns, and
// they are shared by ImportBackup and RestoreSnapshot — two callers that live
// in different files. Keeping them together also keeps backup.go focused on
// the export/import payload itself.

// snapshotBeforeImport runs VACUUM INTO to write a file-level copy of the
// current database into the backup dir, then prunes so only the 3 most
// recent snapshots remain. It returns the new snapshot's base file name.
func (s *Service) snapshotBeforeImport(ctx context.Context) (string, error) {
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// VACUUM INTO writes a full, consistent copy of the live DB file.
	//
	// The name embeds microseconds, not just seconds. A one-second stamp made
	// two restores in the same second collide — VACUUM INTO refuses to write
	// over an existing file ("output file already exists"), so a scripted
	// restore-then-restore, or a retry after a partial failure, failed with a
	// message about snapshots rather than anything the operator did wrong.
	// The stamp stays lexicographically ordered, which is what pruneSnapshots
	// relies on to find the oldest.
	now := time.Now()
	target := filepath.Join(dir, fmt.Sprintf("snapshot-%s.db", now.UTC().Format("20060102-150405.000000")))
	if err := s.db.WithContext(ctx).Exec("VACUUM INTO ?", target).Error; err != nil {
		return "", err
	}
	// Keep only the 3 most recent snapshots.
	s.pruneSnapshots(dir, 3)
	base := filepath.Base(target)
	return base, nil
}

// isSnapshotName reports whether a file in the backup dir is a restorable
// database copy: either a pre-import snapshot or a daily auto snapshot.
func isSnapshotName(name string) bool {
	if !strings.HasSuffix(name, ".db") {
		return false
	}
	return strings.HasPrefix(name, "snapshot-") || strings.HasPrefix(name, autoBackupPrefix)
}

// pruneSnapshots removes oldest snapshot-*.db files beyond keep.
func (s *Service) pruneSnapshots(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var snaps []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "snapshot-") && strings.HasSuffix(e.Name(), ".db") {
			snaps = append(snaps, e.Name())
		}
	}
	if len(snaps) <= keep {
		return
	}
	// File names embed a monotonically increasing UTC timestamp, so a
	// lexicographic sort is chronological. Remove the oldest (len-keep).
	for _, old := range snaps[:len(snaps)-keep] {
		_ = os.Remove(filepath.Join(dir, old))
	}
}
