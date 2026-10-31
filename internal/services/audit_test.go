package services

// P2: audit is written by the use case, not the transport layer.
//
// These tests exist because the refactor moved 18 handler-side recordAudit
// calls into the service methods. "The handler used to do it" is not a
// guarantee; what has to hold is that calling the use case is *sufficient* —
// a new endpoint that forgets about auditing still gets an audit row.

import (
	"context"
	"strconv"
	"testing"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// auditRows returns every action_log row for one action, oldest first.
func auditRows(t *testing.T, db *gorm.DB, action string) []models.ActionLog {
	t.Helper()
	var rows []models.ActionLog
	if err := db.Where("action = ?", action).Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("read action_log: %v", err)
	}
	return rows
}

// AC-1: calling a write use case is enough — the row appears without the
// caller doing anything audit-related.
func TestWriteUseCaseAuditsItself(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	if err := db.AutoMigrate(&models.ActionLog{}); err != nil {
		t.Fatalf("migrate action_log: %v", err)
	}

	admin := makeUser(t, db, "admin", "admin@example.com", "admin")
	ctx := WithActor(context.Background(), admin, "203.0.113.9")

	cat, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "News"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}

	rows := auditRows(t, db, "create_category")
	if len(rows) != 1 {
		t.Fatalf("expected exactly one create_category row, got %d", len(rows))
	}
	got := rows[0]
	if got.ActorID != admin.ID || got.ActorName != admin.Username {
		t.Errorf("actor = %q/%q, want %q/%q", got.ActorID, got.ActorName, admin.ID, admin.Username)
	}
	if got.IP != "203.0.113.9" {
		t.Errorf("ip = %q, want the client IP carried on the context", got.IP)
	}
	if want := "category:" + strconv.FormatInt(cat.ID, 10); got.Target != want {
		t.Errorf("target = %q, want %q", got.Target, want)
	}
}

// AC-2: an explicit actor beats the context. Audit authorship must not silently
// follow the request when the use case was told who acted — that is how the two
// used to disagree.
func TestAuditPrefersExplicitActorOverContext(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	if err := db.AutoMigrate(&models.ActionLog{}); err != nil {
		t.Fatalf("migrate action_log: %v", err)
	}

	admin := makeUser(t, db, "admin", "admin@example.com", "admin")
	root := makeUser(t, db, "root", "root@example.com", "root")
	victim := makeUser(t, db, "victim", "victim@example.com", "user")

	// The context says "admin acted"; the call says "root did". The row must
	// name root, because root is who the service actually authorised.
	ctx := WithActor(context.Background(), admin, "198.51.100.4")
	if err := svc.DeleteUser(ctx, root, victim.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	rows := auditRows(t, db, "delete_user")
	if len(rows) != 1 {
		t.Fatalf("expected one delete_user row, got %d", len(rows))
	}
	if rows[0].ActorID != root.ID {
		t.Errorf("actor = %q, want the explicit actor %q (not the context user %q)",
			rows[0].ActorID, root.ID, admin.ID)
	}
}

// AC-3: a use case with no actor in its signature still writes a row. That is
// the fallback which let all 18 handler call sites move without changing a
// single service signature, and it must keep working for background jobs.
func TestAuditWritesRowWithoutRequestIdentity(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	if err := db.AutoMigrate(&models.ActionLog{}); err != nil {
		t.Fatalf("migrate action_log: %v", err)
	}

	// context.Background(): no request, no identity — exactly the cron case.
	// The zero-value actor is deliberate: the point is that audit() still writes
	// a row when nobody claims the action. Calling it directly keeps the test
	// about the fallback rather than about one use case's signature.
	svc.audit(context.Background(), models.User{}, "empty_trash", "trash", "deleted=0")

	rows := auditRows(t, db, "empty_trash")
	if len(rows) != 1 {
		t.Fatalf("expected one empty_trash row, got %d", len(rows))
	}
	if rows[0].ActorID != "" || rows[0].IP != "" {
		t.Errorf("row should carry no actor/IP outside a request, got actor=%q ip=%q",
			rows[0].ActorID, rows[0].IP)
	}
	if rows[0].Detail != "deleted=0" {
		t.Errorf("detail = %q, want deleted=0", rows[0].Detail)
	}
}
