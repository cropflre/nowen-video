import { useEffect, useRef, useCallback } from 'react'
import { createPortal } from 'react-dom'
import clsx from 'clsx'

export interface ContextMenuItem {
  key: string
  label: string
  icon?: React.ReactNode
  danger?: boolean
  disabled?: boolean
  divider?: boolean // 在此项前显示分割线
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

  // 点击外部关闭
  useEffect(() => {
    if (!visible) return
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose()
      }
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    // 延迟绑定，避免触发右键的 click 事件立即关闭
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

  // 自动调整位置，防止超出视口
  useEffect(() => {
    if (!visible || !menuRef.current) return
    const menu = menuRef.current
    const rect = menu.getBoundingClientRect()
    const vw = window.innerWidth
    const vh = window.innerHeight

    let adjustedX = x
    let adjustedY = y

    if (x + rect.width > vw - 8) {
      adjustedX = vw - rect.width - 8
    }
    if (y + rect.height > vh - 8) {
      adjustedY = vh - rect.height - 8
    }
    if (adjustedX < 8) adjustedX = 8
    if (adjustedY < 8) adjustedY = 8

    menu.style.left = `${adjustedX}px`
    menu.style.top = `${adjustedY}px`
  }, [visible, x, y])

  // 键盘导航
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    const enabledItems = items.filter(i => !i.disabled)
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      const focused = document.activeElement
      const buttons = menuRef.current?.querySelectorAll('button:not(:disabled)')
      if (!buttons || buttons.length === 0) return
      const arr = Array.from(buttons)
      const idx = arr.indexOf(focused as Element)
      const next = e.key === 'ArrowDown'
        ? (idx + 1) % arr.length
        : (idx - 1 + arr.length) % arr.length
      ;(arr[next] as HTMLElement).focus()
    }
    if (e.key === 'Enter') {
      const focused = document.activeElement as HTMLButtonElement
      focused?.click()
    }
  }, [items])

  if (!visible) return null

  return createPortal(
    <div
      ref={menuRef}
      className="fixed z-[9999] min-w-[180px] py-1.5 rounded-xl shadow-2xl border animate-in fade-in zoom-in-95 duration-150"
      style={{
        left: `${x}px`,
        top: `${y}px`,
        background: 'var(--bg-elevated, rgba(30, 32, 40, 0.98))',
        borderColor: 'var(--border-default, rgba(255,255,255,0.08))',
        backdropFilter: 'blur(20px)',
      }}
      onKeyDown={handleKeyDown}
    >
      {items.map((item, idx) => (
        <div key={item.key}>
          {/* 分割线 */}
          {item.divider && idx > 0 && (
            <div className="my-1 mx-2 border-t" style={{ borderColor: 'var(--border-default, rgba(255,255,255,0.06))' }} />
          )}
          <button
            onClick={() => {
              if (!item.disabled) {
                item.onClick()
                onClose()
              }
            }}
            disabled={item.disabled}
            className={clsx(
              'w-full flex items-center gap-2.5 px-3 py-2 text-sm text-left transition-colors',
              item.disabled
                ? 'opacity-40 cursor-not-allowed'
                : item.danger
                  ? 'text-red-400 hover:bg-red-500/10'
                  : 'hover:bg-white/[0.06]',
              !item.danger && !item.disabled && 'text-surface-200'
            )}
          >
            {item.icon && (
              <span className="flex-shrink-0 w-4 h-4 flex items-center justify-center">
                {item.icon}
              </span>
            )}
            <span className="flex-1">{item.label}</span>
          </button>
        </div>
      ))}
    </div>,
    document.body
  )
}
