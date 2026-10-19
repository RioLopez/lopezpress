// Thin routing layer over wouter (zero-config, URL-based).
// wouter's React types are structurally compatible with Preact's JSX, so it
// works directly under Preact without any adapter.

import { Link as WLink, useLocation } from 'wouter'
import type { ComponentChildren } from 'preact'

// [currentPath, navigate] — call inside any component that needs to read the
// URL or push a new route.
export function useNav(): [string, (to: string) => void] {
  const [location, navigate] = useLocation()
  return [location, navigate as unknown as (to: string) => void]
}

interface AppLinkProps {
  to: string
  class?: string
  onClick?: (e: Event) => void
  children: ComponentChildren
}

// A link that keeps the SPA in-page (no full reload), with optional styling.
export function AppLink({ to, class: className, onClick, children }: AppLinkProps) {
  return (
    <WLink to={to} class={className} onClick={onClick as any}>
      {children}
    </WLink>
  )
}
