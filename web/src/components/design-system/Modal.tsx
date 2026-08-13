import { useCallback, useEffect, useRef, type HTMLAttributes, type MouseEvent, type ReactNode } from 'react'
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

  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    if (closeOnEscape && event.key === 'Escape') onClose()
  }, [closeOnEscape, onClose])

  useEffect(() => {
    if (!open) return
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.body.style.overflow = previousOverflow
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [handleKeyDown, open])

  if (!open) return null

  const handleBackdrop = (event: MouseEvent<HTMLDivElement>) => {
    if (closeOnBackdrop && event.target === overlayRef.current) onClose()
  }

  return (
    <div
      ref={overlayRef}
      className={clsx('nv-modal-backdrop', className)}
      onMouseDown={handleBackdrop}
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
