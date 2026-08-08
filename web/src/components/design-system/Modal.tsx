import { useCallback, useEffect, useRef, type HTMLAttributes, type MouseEvent, type ReactNode } from 'react'
import { X } from 'lucide-react'
import clsx from 'clsx'
import { Button } from './index'

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
      className={clsx(
        'fixed inset-0 z-[var(--nv-z-modal)] flex items-center justify-center overflow-y-auto bg-black/70 p-4 backdrop-blur-sm',
        className,
      )}
      onMouseDown={handleBackdrop}
      role="dialog"
      aria-modal="true"
      aria-label={ariaLabel}
    >
      <div
        className={clsx(
          'relative flex max-h-[min(90vh,900px)] w-full flex-col overflow-hidden rounded-[var(--nv-radius-container)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-elevated)] shadow-[var(--nv-shadow-modal)]',
          sizeClass[size],
          panelClassName,
        )}
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
    <div
      {...props}
      className={clsx(
        'flex shrink-0 items-start gap-3 border-b border-[var(--nv-border-subtle)] px-5 py-4 sm:px-6',
        className,
      )}
    >
      {icon && (
        <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
          {icon}
        </div>
      )}
      <div className="min-w-0 flex-1">
        <h2 className="text-base font-semibold text-[var(--nv-text-primary)] sm:text-lg">{title}</h2>
        {description && <div className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)] sm:text-sm">{description}</div>}
      </div>
      {onClose && (
        <Button type="button" variant="ghost" size="sm" iconOnly onClick={onClose} aria-label="关闭">
          <X size={18} aria-hidden="true" />
        </Button>
      )}
    </div>
  )
}

export function ModalBody({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div {...props} className={clsx('min-h-0 flex-1 overflow-y-auto px-5 py-5 sm:px-6', className)} />
}

export function ModalFooter({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      {...props}
      className={clsx(
        'flex shrink-0 flex-wrap items-center justify-end gap-2 border-t border-[var(--nv-border-subtle)] px-5 py-4 sm:px-6',
        className,
      )}
    />
  )
}
