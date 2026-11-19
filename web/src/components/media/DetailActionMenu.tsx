import { useLayoutEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'

interface DetailActionMenuProps {
  open: boolean
  anchorRef: RefObject<HTMLElement>
  ariaLabel: string
  onClose: () => void
  children: ReactNode
}

const enabledItem = '[role="menuitem"]:not(:disabled):not([aria-disabled="true"])'

export default function DetailActionMenu({ open, anchorRef, ariaLabel, onClose, children }: DetailActionMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null)
  const [position, setPosition] = useState({ top: 0, left: 0, maxWidth: 0, maxHeight: 0, ready: false })

  useLayoutEffect(() => {
    if (!open) {
      setPosition((current) => current.ready ? { ...current, ready: false } : current)
      return
    }

    const updatePosition = () => {
      const anchor = anchorRef.current
      if (!anchor) return

      const rect = anchor.getBoundingClientRect()
      const viewport = window.visualViewport
      const startX = viewport?.offsetLeft ?? 0
      const startY = viewport?.offsetTop ?? 0
      const viewportWidth = viewport?.width ?? window.innerWidth
      const viewportHeight = viewport?.height ?? window.innerHeight
      const edge = 12
      const gap = 8
      const maxWidth = Math.max(1, viewportWidth - 2 * edge)
      const maxHeight = Math.max(1, viewportHeight - 2 * edge)
      const menuWidth = Math.min(menuRef.current?.offsetWidth || 240, maxWidth)
      const menuHeight = Math.min(menuRef.current?.offsetHeight || 300, maxHeight)

      let left = rect.left
      const rightEdge = startX + viewportWidth - edge
      if (left + menuWidth > rightEdge) left = rect.right - menuWidth
      left = Math.max(startX + edge, Math.min(left, rightEdge - menuWidth))

      let top = rect.bottom + gap
      const bottomEdge = startY + viewportHeight - edge
      const roomBelow = bottomEdge - rect.bottom
      const roomAbove = rect.top - startY - edge
      if (roomBelow < menuHeight + gap && roomAbove > roomBelow) {
        top = rect.top - menuHeight - gap
      }
      top = Math.max(startY + edge, Math.min(top, bottomEdge - menuHeight))

      setPosition((current) => current.ready && current.top === top && current.left === left &&
        current.maxWidth === maxWidth && current.maxHeight === maxHeight
        ? current
        : { top, left, maxWidth, maxHeight, ready: true })

    }

    updatePosition()
    const frame = window.requestAnimationFrame(updatePosition)
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose()
        anchorRef.current?.focus({ preventScroll: true })
      }
    }

    const viewport = window.visualViewport
    viewport?.addEventListener('resize', updatePosition)
    viewport?.addEventListener('scroll', updatePosition)
    window.addEventListener('resize', updatePosition)
    window.addEventListener('scroll', updatePosition, true)
    window.addEventListener('keydown', handleKeyDown)

    return () => {
      window.cancelAnimationFrame(frame)
      viewport?.removeEventListener('resize', updatePosition)
      viewport?.removeEventListener('scroll', updatePosition)
      window.removeEventListener('resize', updatePosition)
      window.removeEventListener('scroll', updatePosition, true)
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [anchorRef, onClose, open])

  useLayoutEffect(() => {
    if (position.ready) menuRef.current?.querySelector<HTMLElement>(enabledItem)?.focus({ preventScroll: true })
  }, [position.ready])

  const handleMenuKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Tab') {
      anchorRef.current?.focus({ preventScroll: true })
      onClose()
      return
    }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return

    const items = Array.from(event.currentTarget.querySelectorAll<HTMLElement>(enabledItem))
    if (items.length === 0) return
    event.preventDefault()
    const current = items.findIndex((item) => item === document.activeElement)
    if (event.key === 'Home') items[0].focus()
    else if (event.key === 'End') items[items.length - 1].focus()
    else if (event.key === 'ArrowDown') items[(current + 1) % items.length].focus()
    else items[(current - 1 + items.length) % items.length].focus()
  }

  if (!open || typeof document === 'undefined') return null

  return createPortal(
    <>
      <button
        type="button"
        className="fixed inset-0 z-[var(--nv-z-dropdown)] cursor-default bg-transparent"
        onClick={onClose}
        aria-label="关闭菜单"
        tabIndex={-1}
      />
      <div
        ref={menuRef}
        className="nv-detail-action-menu"
        role="menu"
        aria-label={ariaLabel}
        data-ready={position.ready || undefined}
        onKeyDown={handleMenuKeyDown}
        onClick={(event) => {
          if (event.target instanceof Element && event.target.closest(enabledItem)) {
            anchorRef.current?.focus({ preventScroll: true })
          }
        }}
        style={{
          top: position.top,
          left: position.left,
          maxWidth: position.maxWidth || undefined,
          maxHeight: position.maxHeight || undefined,
          visibility: position.ready ? 'visible' : 'hidden',
        }}
      >
        {children}
      </div>
    </>,
    document.body,
  )
}
