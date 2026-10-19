import { useEffect, useRef, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { useAuth } from '../core/auth'
import { useI18n } from '../core/i18n'
import { useNav } from '../core/router'

// useMediaQuery: a tiny hook that tracks a media query with preact's
// useEffect + matchMedia. No extra dependency.
function useMediaQuery(q: string): boolean {
  const [matches, setMatches] = useState(() =>
    typeof window !== 'undefined' ? window.matchMedia(q).matches : false,
  )
  useEffect(() => {
    const mql = window.matchMedia(q)
    const cb = () => setMatches(mql.matches)
    cb()
    mql.addEventListener('change', cb)
    return () => mql.removeEventListener('change', cb)
  }, [q])
  return matches
}

// Top-right user menu (spec 20).
//
// PC (pointer fine / width >= 760): hover with a 300 ms close delay so the
// pointer can travel into the popover; click goes to /me; keyboard opens
// via Enter/Space/ArrowDown, Esc closes, arrow keys move between items.
//
// Mobile (touch / width < 760): tap opens a tap-only popover with a
// full-viewport overlay that closes on outside tap. No hover, no 300 ms
// delay. The same aria attributes apply (aria-haspopup/expanded, role=menu,
// role=menuitem, Esc closes, focus management on open).
export function UserMenu() {
  const { user, logout } = useAuth()
  const { t } = useI18n()
  const [, navigate] = useNav()

  // PC and mobile get different interaction models.
  const isMobile = useMediaQuery('(hover: none), (pointer: coarse), (max-width: 760px)')

  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([])
  // Distinguishes a keyboard "open" from a mouse click so the latter can
  // navigate to /me while the former just reveals the menu.
  const keyOpened = useRef(false)
  // PC hover close timer: 300 ms grace before hiding.
  const hoverCloseTimer = useRef<number | null>(null)

  const openNow = () => {
    if (hoverCloseTimer.current) {
      clearTimeout(hoverCloseTimer.current)
      hoverCloseTimer.current = null
    }
    setOpen(true)
  }
  const scheduleHoverClose = () => {
    if (hoverCloseTimer.current) clearTimeout(hoverCloseTimer.current)
    hoverCloseTimer.current = window.setTimeout(() => setOpen(false), 300)
  }

  // Close on outside click / Escape. Outside click also handles the mobile
  // overlay-tap-to-close.
  useEffect(() => {
    if (!open) return
    const onDocClick = (e: MouseEvent | TouchEvent) => {
      const target = e.target as Node
      if (containerRef.current && containerRef.current.contains(target)) return
      // Overlay click closes on mobile; document click closes on PC.
      setOpen(false)
      triggerRef.current?.focus()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpen(false)
        triggerRef.current?.focus()
      }
    }
    document.addEventListener('click', onDocClick)
    document.addEventListener('touchstart', onDocClick, { passive: true })
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('click', onDocClick)
      document.removeEventListener('touchstart', onDocClick)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  // Move focus to the first item when the menu opens via keyboard.
  useEffect(() => {
    if (open && keyOpened.current) {
      itemRefs.current[0]?.focus()
    }
  }, [open])

  const display = user?.display_name || user?.username || ''
  const initial = (display[0] || '?').toUpperCase()

  // PC trigger click: hover already opened it; a synthesized click from a
  // keyboard "open" should NOT navigate. A real PC mouse click navigates.
  const onTriggerClick = () => {
    if (isMobile) {
      // Mobile: toggle on tap.
      setOpen((o) => !o)
      return
    }
    if (keyOpened.current) {
      // Keyboard opened the menu; don't also navigate on the synthesized click.
      keyOpened.current = false
      return
    }
    navigate('/me')
  }
  const onTriggerKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') {
      if (!open) {
        e.preventDefault()
        keyOpened.current = true
        openNow()
      }
    } else if (e.key === 'ArrowUp' && open) {
      e.preventDefault()
      const last = itemRefs.current.length - 1
      itemRefs.current[last]?.focus()
    }
  }

  const doLogout = () => {
    if (!confirm(t('menu.confirmLogout'))) return
    logout()
    setOpen(false)
    navigate('/')
  }

  const items: { label: string; onClick: () => void }[] = [
    { label: t('menu.profile'), onClick: () => { setOpen(false); navigate('/me') } },
    { label: t('menu.account'), onClick: () => { setOpen(false); navigate('/settings') } },
    { label: t('menu.logout'), onClick: doLogout },
  ]

  const onItemKeyDown = (e: KeyboardEvent, idx: number) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      itemRefs.current[(idx + 1) % items.length]?.focus()
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      itemRefs.current[(idx - 1 + items.length) % items.length]?.focus()
    } else if (e.key === 'Tab' && idx === items.length - 1) {
      // Allow natural tab-out after the last item.
      setOpen(false)
    }
  }

  // PC hover handlers only attach on PC; on mobile they are no-ops so the
  // tap model stays pure.
  const hoverHandlers = isMobile
    ? {}
    : {
        onMouseEnter: openNow,
        onMouseLeave: scheduleHoverClose,
      }

  return (
    <div
      class="user-menu"
      ref={containerRef}
      {...hoverHandlers}
    >
      <button
        type="button"
        class="user-menu-trigger"
        ref={triggerRef}
        aria-haspopup="menu"
        aria-expanded={open ? 'true' : 'false'}
        aria-label={display}
        title={display}
        onClick={onTriggerClick}
        onKeyDown={onTriggerKeyDown}
      >
        {user?.avatar ? (
          <img class="user-menu-avatar" src={user.avatar} alt={display} />
        ) : (
          <span class="user-menu-avatar user-menu-avatar--fallback">{initial}</span>
        )}
        <span class="user-menu-name">{display}</span>
      </button>

      {open && (
        <>
          <div class="user-menu-pop user-menu-sheet" role="menu">
            {items.map((it, i) => (
              <button
                type="button"
                key={it.label}
                role="menuitem"
                class="user-menu-item"
                ref={(el) => { itemRefs.current[i] = el }}
                onClick={it.onClick}
                onKeyDown={(e) => onItemKeyDown(e, i)}
              >
                {it.label}
              </button>
            ))}
          </div>
          {/* Mobile only: full-viewport tap-away layer. Sits under the
          popover (z-index 190 < popover 200); tapping anywhere closes the
          menu. Hidden on PC (CSS display:none) — the outside-click listener
          already covers dismissal there. */}
          {isMobile && (
            <div
              class="user-menu-overlay"
              onClick={() => {
                setOpen(false)
                triggerRef.current?.focus()
              }}
              aria-hidden="true"
            />
          )}
        </>
      )}
    </div>
  )
}

// Renders the login/register entry when signed out (spec 20 §5).
export function LoginRegisterEntry({ children }: { children?: ComponentChildren }) {
  const { t } = useI18n()
  const [, navigate] = useNav()
  return (
    <button class="btn btn-ghost btn-sm" onClick={() => navigate('/login')}>
      {children ?? t('menu.loginOrRegister')}
    </button>
  )
}
