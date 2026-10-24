import type { Tag } from './types'

// Tag suggestion engine for the editor.
//
// Kept as a pure function (no Preact, no DOM) so it can be unit-tested and so
// the matching rules live in exactly one place. The rules mirror the server's
// tag resolution (services.normalizeTagName + case-insensitive lookup):
//   - names compare case-insensitively, so "go" matches the existing "Go"
//   - leading/trailing and repeated whitespace is ignored while matching
//   - an exact (case-insensitive) match always wins over partial matches
//   - when nothing matches, the caller is offered a "create" row instead of
//     being forced to guess what spelling the site already uses

export interface TagSuggestion {
  // The name to submit. For an existing tag this is the stored spelling (so
  // picking "go" attaches "Go"), for a new one it is what the author typed.
  name: string
  // false = this row creates a new tag.
  exists: boolean
}

// normalizeTagInput mirrors the server: trim and collapse inner whitespace.
export function normalizeTagInput(raw: string): string {
  return raw.trim().replace(/\s+/g, ' ')
}

// suggestTags returns up to `limit` existing tags matching `input`, plus a
// create-row when nothing matches exactly. Selected tags are excluded so the
// dropdown never offers something already attached.
export function suggestTags(
  all: Tag[],
  input: string,
  selected: string[] = [],
  limit = 8,
): TagSuggestion[] {
  const needle = normalizeTagInput(input).toLowerCase()
  if (!needle) return []

  const taken = new Set(selected.map((s) => s.toLowerCase()))
  // When the typed name is already on the post, offering to "create" it would
  // read as if the earlier pick had failed — but other suggestions (a longer
  // name that happens to start the same way) are still useful, so only the
  // create row is suppressed.
  const alreadyAttached = taken.has(needle)
  const prefix: TagSuggestion[] = []
  const partial: TagSuggestion[] = []
  let exact: TagSuggestion | null = null

  for (const tag of all) {
    const lower = tag.name.toLowerCase()
    if (taken.has(lower)) continue
    if (lower === needle) {
      exact = { name: tag.name, exists: true }
      continue
    }
    if (lower.startsWith(needle)) prefix.push({ name: tag.name, exists: true })
    else if (lower.includes(needle)) partial.push({ name: tag.name, exists: true })
  }

  const matches = [...prefix, ...partial].slice(0, limit)
  if (exact) {
    // Selecting the exact match is the overwhelmingly common case, so it goes
    // first — and it is the same row Enter would pick.
    return [exact, ...matches].slice(0, limit)
  }
  // No existing tag matches: offer creating one with the author's spelling.
  // The create row is reserved a slot so a long match list can never push the
  // only way forward off the bottom of the dropdown.
  if (alreadyAttached) return matches
  return [...matches.slice(0, limit - 1), { name: normalizeTagInput(input), exists: false }]
}
