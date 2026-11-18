package services

// Backup policy: clamps for the values an operator may set, validation for the
// one path a form may influence, and the date arithmetic that decides whether
// the next snapshot is due. Split out of autobackup.go so the scheduler file
// stays readable (and under the size gate).

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClampBackupKeep keeps the retention inside a sane window: at least one copy,
// at most 90 (a quarter). Values outside the range silently fall back to a
// week instead of being stored as typed, so the prune step can never be told
// to delete everything.
func ClampBackupKeep(n int) int {
	if n < minBackupKeep {
		return 7
	}
	if n > maxBackupKeep {
		return maxBackupKeep
	}
	return n
}

// ClampBackupInterval keeps the schedule between daily and monthly. A zero or
// negative value would mean "every request", which is not a schedule.
func ClampBackupInterval(n int) int {
	if n < minBackupInterval {
		return minBackupInterval
	}
	if n > maxBackupInterval {
		return maxBackupInterval
	}
	return n
}

// ClampBackupHour keeps the preferred hour inside a real clock. 0 means "as
// soon as the interval is due", which is the pre-existing lazy behaviour.
func ClampBackupHour(n int) int {
	if n < 0 {
		return 0
	}
	if n > 23 {
		return 23
	}
	return n
}

// NormalizeBackupSubdir validates the optional folder under BACKUP_DIR.
//
// It is deliberately a RELATIVE path with a very small alphabet. The value is
// joined onto a server-side directory and then written to, so an absolute path,
// a drive letter, a "..", or a control character would turn a settings field
// into an arbitrary-file-write primitive. An operator who needs a different
// disk sets BACKUP_DIR in .env instead, where it is a deployment decision
// rather than a web form field.
func NormalizeBackupSubdir(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	s = strings.ReplaceAll(s, "\\", "/")
	if strings.HasPrefix(s, "/") || filepath.IsAbs(s) || strings.Contains(s, ":") {
		return "", newErr(http.StatusBadRequest, "备份子目录必须是相对路径（不能是绝对路径或盘符）")
	}
	parts := make([]string, 0, 4)
	for _, p := range strings.Split(s, "/") {
		p = strings.TrimSpace(p)
		if p == "" || p == "." {
			continue
		}
		if p == ".." || strings.ContainsAny(p, `/\:`) ||
			strings.IndexFunc(p, func(r rune) bool { return r < 0x20 }) >= 0 {
			return "", newErr(http.StatusBadRequest, "备份子目录不能包含 .. 或特殊字符")
		}
		parts = append(parts, p)
	}
	out := strings.Join(parts, "/")
	if len([]rune(out)) > 120 {
		return "", newErr(http.StatusBadRequest, "备份子目录过长")
	}
	return out, nil
}

// autoBackupDue reports whether the newest snapshot is older than the
// configured interval. It reads the DATE FROM THE FILE NAME rather than the
// mtime: the name is what the rest of the job already trusts, and a restored
// or copied file would otherwise reset the schedule.
func autoBackupDue(dir string, now time.Time, intervalDays int) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true // no directory yet: the MkdirAll above will create it
	}
	today := now.UTC().Truncate(24 * time.Hour)
	var newest time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		d, ok := autoBackupDate(e.Name())
		if !ok {
			continue
		}
		if d.After(newest) {
			newest = d
		}
	}
	if newest.IsZero() {
		return true
	}
	cutoff := today.AddDate(0, 0, -(ClampBackupInterval(intervalDays) - 1))
	return newest.Before(cutoff)
}

// autoBackupDate parses auto-YYYYMMDD.db.
func autoBackupDate(name string) (time.Time, bool) {
	if !strings.HasPrefix(name, autoBackupPrefix) || !strings.HasSuffix(name, ".db") {
		return time.Time{}, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(name, autoBackupPrefix), ".db")
	d, err := time.ParseInLocation("20060102", raw, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}
