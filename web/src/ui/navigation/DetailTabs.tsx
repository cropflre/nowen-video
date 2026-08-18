import type { ReactNode } from 'react'
import clsx from 'clsx'

export interface DetailTabItem<T extends string> {
  value: T
  label: ReactNode
  panelId: string
  tabId?: string
  disabled?: boolean
}

export interface DetailTabsProps<T extends string> {
  items: DetailTabItem<T>[]
  value: T
  onChange: (value: T) => void
  ariaLabel: string
  className?: string
}

export function DetailTabs<T extends string>({
  items,
  value,
  onChange,
  ariaLabel,
  className,
}: DetailTabsProps<T>) {
  return (
    <nav className={clsx('nv-detail-section-tabs', className)} aria-label={ariaLabel} role="tablist">
      {items.map((item) => {
        const tabId = item.tabId || `${item.panelId}-tab`
        const selected = value === item.value
        return (
          <button
            key={item.value}
            id={tabId}
            type="button"
            role="tab"
            aria-selected={selected}
            aria-controls={item.panelId}
            tabIndex={selected ? 0 : -1}
            disabled={item.disabled}
            onClick={() => onChange(item.value)}
          >
            {item.label}
          </button>
        )
      })}
    </nav>
  )
}

export default DetailTabs
