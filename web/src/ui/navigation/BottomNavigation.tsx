import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'
import clsx from 'clsx'

export interface BottomNavigationItem {
  to: string
  label: string
  icon: ReactNode
  end?: boolean
}

export interface BottomNavigationProps {
  items: BottomNavigationItem[]
  className?: string
}

export function BottomNavigation({ items, className }: BottomNavigationProps) {
  return (
    <nav
      className={clsx('nv-mobile-nav', className)}
      aria-label="移动端主导航"
      style={{
        left: 'max(8px, env(safe-area-inset-left, 0px))',
        right: 'max(8px, env(safe-area-inset-right, 0px))',
      }}
    >
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          className="nv-mobile-nav-item"
          aria-label={item.label}
          title={item.label}
        >
          <span className="nv-mobile-nav-icon">{item.icon}</span>
          <span className="nv-mobile-nav-label">{item.label}</span>
        </NavLink>
      ))}
    </nav>
  )
}

export default BottomNavigation
