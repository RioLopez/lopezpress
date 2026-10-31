import type { Category } from './types'

// Category tree helpers.
//
// The API returns categories flat (id / name / parent_id) and the site allows
// exactly two levels. Three separate places need the same view of
// that tree — the sidebar, the list filter's <select>, and the article card's
// breadcrumb — so the ordering logic lives here once instead of being
// re-derived (and slowly drifting apart) in each component.

export interface CategoryNode {
  cat: Category
  // 0 = top level, 1 = child. The model enforces two levels, so this never
  // exceeds 1; the type says so and every consumer can switch exhaustively.
  depth: 0 | 1
}

// orderCategories flattens the tree into display order: each parent
// immediately followed by its children. Orphans (a parent_id pointing at a row
// that no longer exists) are promoted to top level so they stay reachable.
export function orderCategories(cats: Category[]): CategoryNode[] {
  const roots = cats.filter((c) => c.parent_id == null)
  const out: CategoryNode[] = []
  const seen = new Set<number>()

  for (const root of roots) {
    out.push({ cat: root, depth: 0 })
    seen.add(root.id)
    for (const child of cats.filter((c) => c.parent_id === root.id)) {
      out.push({ cat: child, depth: 1 })
      seen.add(child.id)
    }
  }
  for (const orphan of cats) {
    if (!seen.has(orphan.id)) out.push({ cat: orphan, depth: 0 })
  }
  return out
}

// categoryPath returns the ancestor chain for an id, root first. A top-level
// category yields one element, a child yields [parent, child], and an unknown
// id yields an empty array.
export function categoryPath(cats: Category[], id: number | null | undefined): string[] {
  if (id == null) return []
  const byId = new Map(cats.map((c) => [c.id, c]))
  const self = byId.get(id)
  if (!self) return []
  const parent = self.parent_id != null ? byId.get(self.parent_id) : undefined
  return parent ? [parent.name, self.name] : [self.name]
}

// categoryLabel renders the breadcrumb used on cards: "PHP › php教程". Without
// a parent it is just the name, so a card never looks like it lost context.
export const CATEGORY_SEPARATOR = ' › '

export function categoryLabel(cats: Category[], id: number | null | undefined): string {
  return categoryPath(cats, id).join(CATEGORY_SEPARATOR)
}

// categoryOptionLabel is the <select> variant. Options cannot carry their own
// indentation, so the hierarchy is spelled with a prefix. U+3000 (ideographic
// space) is used for the indent because it survives every browser's option
// rendering, unlike a CSS margin.
export function categoryOptionLabel(node: CategoryNode): string {
  return node.depth === 0 ? node.cat.name : `\u3000\u2514 ${node.cat.name}`
}
