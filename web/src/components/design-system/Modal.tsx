import { useCallback, useEffect, useRef, type HTMLAttributes, type MouseEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import clsx from 'clsx'

export type ModalSize = 'sm' | 'md' | 'lg' | 'xl' | 'video'

interface ModalProps {
  open?: boolean
  onClose: () => void
  children: ReactNode
  size?: ModalSize
  ariaLabel?: string
  closeOnBackdrop?: boolean
  closeOnEscape?: boolean
  className?: string
  panelClassName?: string
}

const sizeClass: Record<ModalSize, string> = {
  sm: 'max-w-md',
  md: 'max-w-2xl',
  lg: 'max-w-3xl',
  xl: 'max-w-5xl',
  video: 'max-w-5xl',
}

const responsiveSafePadding = 'max(10px, min(20px, 2vw), env(safe-area-inset-top, 0px))'

export function Modal({
  open = true,
  onClose,
  children,
  size = 'md',
  ariaLabel = '对话框',
  closeOnBackdrop = true,
  closeOnEscape = true,
  className,
  panelClassName,
}: ModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null)

  const closeRef = useRef(onClose)
  closeRef.current = onClose

  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    const dialogs = document.querySelectorAll('[aria-modal="true"]')
    if (dialogs[dialogs.length - 1] !== overlayRef.current) return
    if (closeOnEscape && event.key === 'Escape') { event.preventDefault(); closeRef.current(); return }
    if (event.key !== 'Tab') return
    const controls = Array.from(overlayRef.current?.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), summary, [tabindex="0"]') || []).filter((element) => element.getClientRects().length > 0)
    const first = controls[0]
    const last = controls[controls.length - 1]
    if (!first) { event.preventDefault(); overlayRef.current?.focus(); return }
    if (event.shiftKey && (document.activeElement === first || !overlayRef.current?.contains(document.activeElement))) { event.preventDefault(); last.focus() }
    else if (!event.shiftKey && (document.activeElement === last || !overlayRef.current?.contains(document.activeElement))) { event.preventDefault(); first.focus() }
  }, [closeOnEscape])

  useEffect(() => {
    if (!open) return
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    document.addEventListener('keydown', handleKeyDown)
    const frame = requestAnimationFrame(() => overlayRef.current?.querySelector<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled)')?.focus())
    return () => {
      cancelAnimationFrame(frame)
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', handleKeyDown)
      if (previousFocus?.isConnected) previousFocus.focus()
    }
  }, [handleKeyDown, open])

  if (!open) return null

  const handleBackdrop = (event: MouseEvent<HTMLDivElement>) => {
    if (closeOnBackdrop && event.target === overlayRef.current) onClose()
  }

  const modal = (
    <div
      ref={overlayRef}
      className={clsx('nv-modal-backdrop', className)}
      style={{
        paddingTop: responsiveSafePadding,
        paddingRight: 'max(10px, min(20px, 2vw), env(safe-area-inset-right, 0px))',
        paddingBottom: 'max(10px, min(20px, 2vw), env(safe-area-inset-bottom, 0px))',
        paddingLeft: 'max(10px, min(20px, 2vw), env(safe-area-inset-left, 0px))',
      }}
      onMouseDown={handleBackdrop}
      tabIndex={-1}
      role="dialog"
      aria-modal="true"
      aria-label={ariaLabel}
    >
      <div
        className={clsx('nv-modal-sheet', sizeClass[size], panelClassName)}
        onMouseDown={(event) => event.stopPropagation()}
      >
        {children}
      </div>
    </div>
  )

  // Always mount dialogs under document.body so page-level stacking contexts
  // (hero isolation, transformed rails, sticky panels, etc.) can never paint
  // above an active modal.
  return typeof document === 'undefined' ? modal : createPortal(modal, document.body)
}

interface ModalHeaderProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  title: ReactNode
  description?: ReactNode
  onClose?: () => void
  icon?: ReactNode
}

export function ModalHeader({ title, description, onClose, icon, className, ...props }: ModalHeaderProps) {
  return (
    <div {...props} className={clsx('nv-modal-header', className)}>
      {icon && <div className="nv-modal-icon">{icon}</div>}
      <div className="min-w-0 flex-1">
        <h2 className="nv-modal-title">{title}</h2>
        {description && <div className="nv-modal-description">{description}</div>}
      </div>
      {onClose && (
        <button
          type="button"
          onClick={onClose}
          className="nv-button nv-button--ghost nv-button--sm nv-button--icon-only"
          data-variant="ghost"
          data-size="sm"
          data-icon-only="true"
          aria-label="关闭"
        >
          <X size={17} aria-hidden="true" />
        </button>
      )}
    </div>
  )
}

export function ModalBody({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div {...props} className={clsx('nv-modal-body', className)} />
}

export function ModalFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div {...props} className={clsx('nv-modal-footer', className)} />
}
