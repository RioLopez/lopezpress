/**
 * Client-side mirror of the server's password policy
 * (`internal/services/password_policy.go`).
 *
 * It deliberately checks only the two *mechanical* rules — length floor and
 * the bcrypt byte ceiling — and not the blocklist. The blocklist is ~200
 * entries that would drift the moment either side is edited, and the server
 * already answers with a message that says exactly what is wrong
 * ("password is too weak: it is one of the most commonly used passwords"),
 * which `errMessage` shows verbatim. Duplicating it here would mean two lists
 * to keep in sync and one of them silently wrong.
 *
 * The length rules are worth duplicating because they are cheap, stable, and
 * catch the failure *before* a round trip: a 73-byte passphrase pasted into the
 * form used to reach bcrypt and come back as a 500.
 */

/** Minimum length in characters (not UTF-16 units, not bytes). */
export const PASSWORD_MIN_CHARS = 8

/** Maximum length in UTF-8 bytes — bcrypt refuses anything longer. */
export const PASSWORD_MAX_BYTES = 72

export type PasswordProblem = 'short' | 'long'

/**
 * Returns the mechanical problem with a password, or null when it passes.
 *
 * `[...s].length` counts code points rather than UTF-16 units, so an emoji
 * counts as one character and matches the server's rune count. `length` alone
 * would count it as two and let a 7-character password through.
 */
export function passwordProblem(pw: string): PasswordProblem | null {
  if ([...pw].length < PASSWORD_MIN_CHARS) return 'short'
  if (new TextEncoder().encode(pw).length > PASSWORD_MAX_BYTES) return 'long'
  return null
}
