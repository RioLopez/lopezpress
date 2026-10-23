// Role predicates — the single source of truth for front-end role checks.
// Mirrors the backend's `services/roles.go` helpers. Every `role === 'admin'`
// / `role === 'root'` comparison must go through these, so the three-tier
// model (user / admin / root) only has to change in ONE place if a new role
// is ever introduced (ARCH_REVIEW F3).

import type { Role } from './types'

// Admin-or-root: any role that may enter the /admin console.
export const isAdmin = (role?: Role): boolean => role === 'admin' || role === 'root'

// Super-admin only.
export const isRoot = (role?: Role): boolean => role === 'root'

// Plain admin (NOT root): used to distinguish the two badges and the
// management tier where root outranks admin.
export const isAdminOnly = (role?: Role): boolean => role === 'admin'
