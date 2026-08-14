import { useCallback, useEffect, useRef } from 'react'
import { createPortal } from 'react-dom'

export interface ContextMenuItem {
  key: string
  label: string
  icon?: React.ReactNode
  danger?: boolean
  disabled?: boolean
  divider?: boolean
  onClick: () => void
}

interface ContextMenuProps {
  visible: boolean
  x: number
  y: number
  items: ContextMenuItem[]
  onClose: () => void
}

export default function ContextMenu({ visible, x, y, items, onClose }: ContextMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!visible) return

    const handleClick = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) onClose()
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }

    const timer = setTimeout(() => {
      document.addEventListener('click', handleClick)
      document.addEventListener('contextmenu', handleClick)
      document.addEventListener('keydown', handleKeyDown)
    }, 0)

    return () => {
      clearTimeout(timer)
      document.removeEventListener('click', handleClick)
      document.removeEventListener('contextmenu', handleClick)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [visible, onClose])

  useEffect(() => {
    if (!visible || !menuRef.current) return

    const menu = menuRef.current
    const rect = menu.getBoundingClientRect()
    const viewportWidth = window.innerWidth
    const viewportHeight = window.innerHeight

    let adjustedX = x
    let adjustedY = y

    if (x + rect.width > viewportWidth - 8) adjustedX = viewportWidth - rect.width - 8
    if (y + rect.height > viewportHeight - 8) adjustedY = viewportHeight - rect.height - 8
    if (adjustedX < 8) adjustedX = 8
    if (adjustedY < 8) adjustedY = 8

    menu.style.left = `${adjustedX}px`
    menu.style.top = `${adjustedY}px`
  }, [visible, x, y])

  const handleMenuKeyDown = useCallback((event: React.KeyboardEvent) => {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const focused = document.activeElement
      const buttons = menuRef.current?.querySelectorAll('button:not(:disabled)')
      if (!buttons || buttons.length === 0) return

      const menuItems = Array.from(buttons)
      const index = menuItems.indexOf(focused as Element)
      const next = event.key === 'ArrowDown'
        ? (index + 1) % menuItems.length
        : (index - 1 + menuItems.length) % menuItems.length
      ;(menuItems[next] as HTMLElement).focus()
    }

    if (event.key === 'Enter') {
      const focused = document.activeElement as HTMLButtonElement
      focused?.click()
    }
  }, [])

  if (!visible) return null

  return createPortal(
    <div
      ref={menuRef}
      role="menu"
      aria-label="文件操作菜单"
      className="fixed min-w-[176px] overflow-hidden rounded-[var(--nv-radius-popover)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] p-1.5 shadow-[var(--nv-shadow-elevated)] animate-in fade-in slide-in-from-top-1 duration-150 motion-reduce:animate-none"
      style={{
        left: `${x}px`,
        top: `${y}px`,
        zIndex: 'var(--nv-z-dropdown)',
      }}
      onKeyDown={handleMenuKeyDown}
    >
      {items.map((item, index) => (
        <div key={item.key}>
          {item.divider && index > 0 && (
            <div className="mx-1 my-1 border-t border-[var(--nv-border-subtle)]" role="separator" />
          )}
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              if (!item.disabled) {
                item.onClick()
                onClose()
              }
            }}
            disabled={item.disabled}
            className={`flex min-h-8 w-full items-center gap-2 rounded-[var(--nv-radius-control)] px-2.5 py-1.5 text-left text-[13px] outline-none transition-[background-color,color] duration-150 focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-40 ${
              item.danger
                ? 'text-[var(--nv-status-danger)] hover:bg-[color-mix(in_srgb,var(--nv-status-danger)_10%,transparent)]'
                : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-primary)]'
            }`}
          >
            {item.icon && (
              <span className="flex h-4 w-4 shrink-0 items-center justify-center text-[var(--nv-text-tertiary)]" aria-hidden="true">
                {item.icon}
              </span>
            )}
            <span className="flex-1">{item.label}</span>
          </button>
        </div>
      ))}
    </div>,
    document.body,
  )
}
