import type { ReactNode } from 'react'
import { NavLink } from 'react-router-dom'

export interface NavigationRailLinkProps {
  to: string
  icon: ReactNode
  label: string
  end?: boolean
  meta?: ReactNode
}

export function NavigationRailLink({
  to,
  icon,
  label,
  end = false,
  meta,
}: NavigationRailLinkProps) {
  return (
    <NavLink
      to={to}
      end={end}
      className="nv-rail-item"
      aria-label={label}
      title={label}
      data-label={label}
    >
      <span className="nv-rail-icon">{icon}</span>
      <span className="nv-rail-label">{label}</span>
      {meta !== undefined && <span className="nv-rail-meta">{meta}</span>}
    </NavLink>
  )
}

export interface NavigationRailSectionProps {
  title?: ReactNode
  children: ReactNode
}

export function NavigationRailSection({ title, children }: NavigationRailSectionProps) {
  return (
    <section className="nv-rail-section">
      {title && <div className="nv-rail-section-title">{title}</div>}
      <div className="nv-rail-section-links">{children}</div>
    </section>
  )
}

export default NavigationRailLink
