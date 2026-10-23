package services

import (
	"context"
	"log/slog"

	"lopezpress/internal/models"
)

// --- request identity -------------------------------------------------------
//
// "Who is acting, and from where" is resolved once by the transport layer
// (handlers.WithAuth) and carried on the request context. The service layer
// never inspects *http.Request, but it does need both values to write an audit
// row, so this file is the hand-off point between the two layers.

type identityKey struct{}

type identity struct {
	user models.User
	ip   string
}

// WithActor attaches the authenticated user and the originating client IP to
// ctx. Called once per request by the JWT middleware; nothing else should call
// it. An IP is carried alongside the user because an audit row is useless
// without knowing where the action came from.
func WithActor(ctx context.Context, user models.User, ip string) context.Context {
	return context.WithValue(ctx, identityKey{}, identity{user: user, ip: ip})
}

// ActorFromContext returns the authenticated user, or the zero user when the
// caller is anonymous (or there is no request at all, as in a background job).
func ActorFromContext(ctx context.Context) models.User {
	if id, ok := ctx.Value(identityKey{}).(identity); ok {
		return id.user
	}
	return models.User{}
}

// ClientIPFromContext returns the IP recorded by WithActor, or "" outside a
// request.
func ClientIPFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(identityKey{}).(identity); ok {
		return id.ip
	}
	return ""
}

// SystemActor is the actor attributed to scheduled work (the daily snapshot).
// It deliberately has no ID: nobody clicked anything, and materialising a
// "system" row in the users table just to satisfy a foreign key would be worse
// than an empty actor_id.
func SystemActor() models.User { return models.User{Username: "system"} }

// audit appends one row to action_log.
//
// This is the ONLY place in the codebase that constructs models.ActionLog —
// scripts/arch_check.py rule R10 fails the build if another file does. That
// single-writer rule is the whole point of the P2 refactor:
//
//   - Every write use case records itself, so a new handler cannot forget to
//     audit (handlers no longer have any audit API to forget to call).
//   - The detail string is built by the layer that performed the write, from
//     what actually happened, instead of from what the handler assumed the
//     service did.
//   - The row lands right next to the mutation, which makes "audited every
//     write path" reviewable by reading one function instead of two files.
//
// actor is the acting user; passing the zero user means "take the identity
// from ctx". That fallback exists for use cases whose signature does not carry
// an actor (site config, categories, tags, trash, uploads); new use cases
// should pass it explicitly so the audit stays independent of the request.
//
// Auditing is best-effort by design: a failed insert is logged and never fails
// the operation that triggered it.
//
// What is deliberately NOT audited: the self-service identity operations
// (Login, Register, UpdateMe). Login already writes login_log — including the
// failures — and duplicating it into action_log would bury the administrative
// trail under per-session noise. Password changes are the exception: an
// admin-forced reset and a self-service change are both audited because
// "the password of account X changed" is exactly what an operator investigates.
func (s *Service) audit(ctx context.Context, actor models.User, action, target, detail string) {
	if actor.ID == "" && actor.Username == "" {
		actor = ActorFromContext(ctx)
	}
	row := models.ActionLog{
		ActorID:   actor.ID,
		ActorName: actor.Username,
		Action:    action,
		Target:    target,
		Detail:    detail,
		IP:        ClientIPFromContext(ctx),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		slog.Warn("audit write failed", "action", action, "target", target, "err", err)
	}
}
