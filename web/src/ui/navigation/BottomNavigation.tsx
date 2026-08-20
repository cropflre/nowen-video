import type { ReactNode } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import clsx from 'clsx'

export interface BottomNavigationItem {
  to: string
  label: string
  icon: ReactNode
  end?: boolean
  /** Extra route prefixes that should keep this navigation item selected. */
  activeOn?: string[]
}

export interface BottomNavigationProps {
  items: BottomNavigationItem[]
  className?: string
}

export function BottomNavigation({ items, className }: BottomNavigationProps) {
  const location = useLocation()

  return (
    <nav
      className={clsx('nv-mobile-nav', className)}
      aria-label="移动端主导航"
      style={{
        left: 'max(8px, env(safe-area-inset-left, 0px))',
        right: 'max(8px, env(safe-area-inset-right, 0px))',
      }}
    >
      {items.map((item) => {
        const forceActive = item.activeOn?.some((prefix) => location.pathname.startsWith(prefix)) ?? false
        return (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className={({ isActive }) => clsx(
              'nv-mobile-nav-item',
              (isActive || forceActive) && 'active',
            )}
            aria-label={item.label}
            title={item.label}
            aria-current={forceActive ? 'page' : undefined}
          >
            <span className="nv-mobile-nav-icon">{item.icon}</span>
            <span className="nv-mobile-nav-label">{item.label}</span>
          </NavLink>
        )
      })}
    </nav>
  )
}

export default BottomNavigation
