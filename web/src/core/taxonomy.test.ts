import { describe, expect, it } from 'vitest'
import {
  CATEGORY_SEPARATOR,
  categoryLabel,
  categoryOptionLabel,
  categoryPath,
  orderCategories,
} from './taxonomy'
import type { Category } from './types'

const cat = (id: number, name: string, parent_id: number | null = null): Category => ({
  id,
  name,
  parent_id,
  created_at: '2026-01-01T00:00:00Z',
})

// A deliberately unsorted list: children interleaved with roots, which is what
// the API actually returns (ORDER BY name).
const tree: Category[] = [
  cat(17, 'php教程', 2),
  cat(2, 'PHP'),
  cat(4, '技术'),
  cat(11, '运维部署', 7),
  cat(7, '技术笔记'),
]

describe('orderCategories', () => {
  it('puts each parent immediately before its children', () => {
    const flat = orderCategories(tree).map((n) => `${n.depth}:${n.cat.name}`)
    expect(flat).toEqual([
      '0:PHP',
      '1:php教程',
      '0:技术',
      '0:技术笔记',
      '1:运维部署',
    ])
  })

  it('promotes orphans to top level so they stay reachable', () => {
    const withOrphan = [...tree, cat(99, '野孩子', 12345)]
    const nodes = orderCategories(withOrphan)
    const orphan = nodes.find((n) => n.cat.id === 99)
    expect(orphan?.depth).toBe(0)
    expect(nodes).toHaveLength(withOrphan.length)
  })

  it('never drops or duplicates a category', () => {
    const ids = orderCategories(tree).map((n) => n.cat.id)
    expect(new Set(ids).size).toBe(tree.length)
  })
})

describe('categoryPath / categoryLabel', () => {
  it('builds the ancestor chain for a child', () => {
    expect(categoryPath(tree, 17)).toEqual(['PHP', 'php教程'])
  })

  it('returns a single element for a root category', () => {
    expect(categoryPath(tree, 2)).toEqual(['PHP'])
  })

  it('returns nothing for null or unknown ids', () => {
    expect(categoryPath(tree, null)).toEqual([])
    expect(categoryPath(tree, 404)).toEqual([])
  })

  it('joins the path with the breadcrumb separator', () => {
    expect(categoryLabel(tree, 17)).toBe(`PHP${CATEGORY_SEPARATOR}php教程`)
    expect(categoryLabel(tree, 2)).toBe('PHP')
  })

  it('falls back to an empty string when the category list has not loaded', () => {
    // The card renders `categoryLabel(...) || category.name` so an empty list
    // must not throw or produce a stray separator.
    expect(categoryLabel([], 17)).toBe('')
  })
})

describe('categoryOptionLabel', () => {
  it('indents children and prefixes them with an elbow', () => {
    const nodes = orderCategories(tree)
    const root = nodes.find((n) => n.cat.id === 4)!
    const child = nodes.find((n) => n.cat.id === 17)!
    expect(categoryOptionLabel(root)).toBe('技术')
    // U+3000 indent + U+2514 elbow: options cannot be styled, so the indent
    // has to be in the text.
    expect(categoryOptionLabel(child)).toBe('\u3000\u2514 php教程')
  })
})
