import type { HTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'
import { Surface } from '@/components/design-system'

export interface FilterPanelProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode
}

export function FilterPanel({ className, children, ...props }: FilterPanelProps) {
  return (
    <Surface
      {...props}
      variant="glass"
      className={clsx('nv-filter-panel', className)}
    >
      {children}
    </Surface>
  )
}

export interface FilterGroupProps extends HTMLAttributes<HTMLDivElement> {
  icon?: ReactNode
  label: ReactNode
  count?: ReactNode
  scrollable?: boolean
  children: ReactNode
  contentClassName?: string
}

export function FilterGroup({
  icon,
  label,
  count,
  scrollable = false,
  className,
  contentClassName,
  children,
  ...props
}: FilterGroupProps) {
  return (
    <div {...props} className={clsx('nv-filter-group', className)}>
      <div className="nv-filter-group-label">
        {icon && <span className="nv-filter-group-icon" aria-hidden="true">{icon}</span>}
        <span>{label}</span>
        {count !== undefined && count !== null && <span className="nv-filter-group-count">{count}</span>}
      </div>
      <div
        className={clsx('nv-filter-group-content', scrollable && 'is-scrollable', contentClassName)}
        data-scrollable={scrollable || undefined}
      >
        {children}
      </div>
    </div>
  )
}

export default FilterPanel
