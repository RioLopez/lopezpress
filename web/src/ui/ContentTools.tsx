import type { ComponentChildren } from 'preact'

export type SortDir = 'asc' | 'desc'

interface SortOption {
  value: string
  label: string
}

interface ContentToolsProps {
  q: string
  onQ: (value: string) => void
  placeholder: string
  sorts: SortOption[]
  sort: string
  onSort: (value: string) => void
  dir: SortDir
  onDir: (value: SortDir) => void
  descLabel: string
  ascLabel: string
  children?: ComponentChildren
}

// Shared toolbar for the content lists that filter in the browser (review
// queue, trash, ban vault). Posts uses the server-side paged endpoint and
// keeps its own richer filter bar, but the field/direction split is the same
// so the two feel like one product.
export function ContentTools({ q, onQ, placeholder, sorts, sort, onSort, dir, onDir, descLabel, ascLabel, children }: ContentToolsProps) {
  return (
    <div class="list-tools">
      <input
        class="search-input"
        type="search"
        value={q}
        placeholder={placeholder}
        onInput={(e) => onQ((e.currentTarget as HTMLInputElement).value)}
      />
      {children}
      <select class="select" value={sort} onChange={(e) => onSort((e.currentTarget as HTMLSelectElement).value)}>
        {sorts.map((s) => (
          <option key={s.value} value={s.value}>{s.label}</option>
        ))}
      </select>
      <select class="select" value={dir} onChange={(e) => onDir((e.currentTarget as HTMLSelectElement).value as SortDir)}>
        <option value="desc">{descLabel}</option>
        <option value="asc">{ascLabel}</option>
      </select>
    </div>
  )
}
