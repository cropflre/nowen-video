import type { KeyboardEvent, ReactNode } from 'react'
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
  const enabledItems = items.filter((item) => !item.disabled)

  const handleKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight' && event.key !== 'Home' && event.key !== 'End') return
    const currentIndex = enabledItems.findIndex((item) => item.value === value)
    if (currentIndex < 0 || enabledItems.length === 0) return

    event.preventDefault()
    let nextIndex = currentIndex
    if (event.key === 'Home') nextIndex = 0
    else if (event.key === 'End') nextIndex = enabledItems.length - 1
    else if (event.key === 'ArrowLeft') nextIndex = (currentIndex - 1 + enabledItems.length) % enabledItems.length
    else nextIndex = (currentIndex + 1) % enabledItems.length

    const next = enabledItems[nextIndex]
    onChange(next.value)
    requestAnimationFrame(() => document.getElementById(next.tabId || `${next.panelId}-tab`)?.focus())
  }

  return (
    <nav className={clsx('nv-detail-section-tabs', className)} aria-label={ariaLabel} role="tablist" onKeyDown={handleKeyDown}>
      {items.map((item) => {
        const tabId = item.tabId || `${item.panelId}-tab`
        const selected = value === item.value
        return (
          <a
            key={item.value}
            id={tabId}
            href={`#${item.panelId}`}
            role="tab"
            aria-selected={selected}
            aria-controls={item.panelId}
            aria-disabled={item.disabled || undefined}
            tabIndex={selected ? 0 : -1}
            onClick={(event) => {
              event.preventDefault()
              if (!item.disabled) onChange(item.value)
            }}
          >
            {item.label}
          </a>
        )
      })}
    </nav>
  )
}

export default DetailTabs
