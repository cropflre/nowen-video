import { useCallback, useEffect, useRef, type PointerEvent as ReactPointerEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { EyeOff, GripVertical, Minus, RotateCcw } from 'lucide-react'
import { containAssistantPosition, moveAssistant, resizeAssistant, type AssistantPoint, type AssistantSize } from '@/utils/assistantWindow'
import { assistantViewport } from '@/stores/assistantWindow'

const DRAG_THRESHOLD = 3
const CONTROL_SELECTOR = 'button, a, input, textarea, select, [data-nv-window-control]'

interface AssistantWindowProps {
  open: boolean
  position: AssistantPoint
  size: AssistantSize
  onMove: (position: AssistantPoint) => void
  onResize: (size: AssistantSize) => void
  onMinimize: () => void
  onHide: () => void
  onReset: () => void
  children: ReactNode
}

/**
 * Non-modal floating shell. It deliberately has no backdrop, never locks page
 * scroll and never declares aria-modal: the movie keeps playing underneath.
 */
export default function AssistantWindow({ open, position, size, onMove, onResize, onMinimize, onHide, onReset, children }: AssistantWindowProps) {
  const windowRef = useRef<HTMLElement | null>(null)
  const drag = useRef<{ pointerId: number; startX: number; startY: number; origin: AssistantPoint; latest: AssistantPoint; moved: boolean } | null>(null)
  const resize = useRef<{ pointerId: number; startX: number; startY: number; origin: AssistantSize; latest: AssistantSize; moved: boolean } | null>(null)

  const startDrag = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    if (event.pointerType === 'mouse' && event.button !== 0) return
    if ((event.target as HTMLElement).closest(CONTROL_SELECTOR)) return
    const element = windowRef.current
    if (!element) return
    event.preventDefault()
    element.setPointerCapture(event.pointerId)
    drag.current = { pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, origin: position, latest: position, moved: false }
  }, [position])

  const startResize = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    if (event.pointerType === 'mouse' && event.button !== 0) return
    const element = windowRef.current
    if (!element) return
    event.preventDefault()
    event.stopPropagation()
    element.setPointerCapture(event.pointerId)
    resize.current = { pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, origin: size, latest: size, moved: false }
  }, [size])

  const move = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    const element = windowRef.current
    if (!element) return
    const viewport = assistantViewport()
    const current = drag.current
    if (current && current.pointerId === event.pointerId) {
      const dx = event.clientX - current.startX
      const dy = event.clientY - current.startY
      if (!current.moved && Math.abs(dx) < DRAG_THRESHOLD && Math.abs(dy) < DRAG_THRESHOLD) return
      current.moved = true
      element.dataset.dragging = 'true'
      current.latest = moveAssistant(current.origin, { x: dx, y: dy }, size, viewport)
      // Written straight to the node: a pointer move must not re-render the chat.
      element.style.left = `${current.latest.x}px`
      element.style.top = `${current.latest.y}px`
      return
    }
    const resizing = resize.current
    if (resizing && resizing.pointerId === event.pointerId) {
      const dx = event.clientX - resizing.startX
      const dy = event.clientY - resizing.startY
      if (!resizing.moved && Math.abs(dx) < DRAG_THRESHOLD && Math.abs(dy) < DRAG_THRESHOLD) return
      resizing.moved = true
      element.dataset.resizing = 'true'
      resizing.latest = resizeAssistant(resizing.origin, { x: dx, y: dy }, viewport)
      element.style.width = `${resizing.latest.width}px`
      element.style.height = `${resizing.latest.height}px`
    }
  }, [size])

  const finish = useCallback((event: ReactPointerEvent<HTMLElement>) => {
    const element = windowRef.current
    const current = drag.current
    const resizing = resize.current
    drag.current = null
    resize.current = null
    if (element) {
      delete element.dataset.dragging
      delete element.dataset.resizing
      if (element.hasPointerCapture(event.pointerId)) element.releasePointerCapture(event.pointerId)
    }
    if (current && current.pointerId === event.pointerId && current.moved) {
      // Settle fully inside the viewport. The value is written back to the node
      // first: React skips the style write when the clamped value happens to
      // equal the position it already had before the drag.
      const settled = containAssistantPosition(current.latest, size, assistantViewport())
      if (element) {
        element.style.left = `${settled.x}px`
        element.style.top = `${settled.y}px`
      }
      onMove(settled)
    }
    if (resizing && resizing.pointerId === event.pointerId && resizing.moved) onResize(resizing.latest)
  }, [onMove, onResize, size])

  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) return
      // A real modal stacked above owns Escape while it is open.
      if (document.querySelector('[aria-modal="true"]')) return
      event.preventDefault()
      onMinimize()
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onMinimize, open])

  useEffect(() => {
    if (!open) return
    const focusTimer = window.setTimeout(() => {
      const input = windowRef.current?.querySelector<HTMLElement>('input, textarea')
      input?.focus()
    }, 0)
    return () => window.clearTimeout(focusTimer)
  }, [open])

  if (!open) return null

  const shell = (
    <section
      ref={windowRef}
      role="dialog"
      aria-label="观影助手"
      className="nv-assistant-window"
      style={{ left: position.x, top: position.y, width: size.width, height: size.height }}
      onPointerMove={move}
      onPointerUp={finish}
      onPointerCancel={finish}
    >
      <header className="nv-assistant-window__bar" onPointerDown={startDrag}>
        <span className="nv-assistant-window__grip" aria-hidden="true"><GripVertical size={15} /></span>
        <div className="nv-assistant-window__titles">
          <h2 className="nv-assistant-window__title">观影助手</h2>
          <p className="nv-assistant-window__hint">拖动标题栏移动</p>
        </div>
        <div className="nv-assistant-window__controls" data-nv-window-control>
          <button type="button" className="nv-assistant-window__control" onClick={onReset} aria-label="恢复默认位置和大小" title="恢复默认位置和大小"><RotateCcw size={15} aria-hidden="true" /></button>
          <button type="button" className="nv-assistant-window__control" onClick={onMinimize} aria-label="最小化助手" title="最小化"><Minus size={16} aria-hidden="true" /></button>
          <button type="button" className="nv-assistant-window__control" onClick={onHide} aria-label="隐藏助手" title="隐藏助手"><EyeOff size={15} aria-hidden="true" /></button>
        </div>
      </header>
      <div className="nv-assistant-window__body">{children}</div>
      <div className="nv-assistant-window__resize" onPointerDown={startResize} aria-hidden="true" />
    </section>
  )

  return typeof document === 'undefined' ? shell : createPortal(shell, document.body)
}
