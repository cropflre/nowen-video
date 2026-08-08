import type {
  ButtonHTMLAttributes,
  HTMLAttributes,
  ReactNode,
} from 'react'
import clsx from 'clsx'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'
export type ButtonSize = 'sm' | 'md' | 'lg'

export function buttonClassName({
  variant = 'secondary',
  size = 'md',
  iconOnly = false,
  className,
}: {
  variant?: ButtonVariant
  size?: ButtonSize
  iconOnly?: boolean
  className?: string
} = {}) {
  return clsx(
    'nv-button',
    `nv-button--${variant}`,
    `nv-button--${size}`,
    iconOnly && 'nv-button--icon-only',
    className,
  )
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  iconOnly?: boolean
  loading?: boolean
}

export function Button({
  variant = 'secondary',
  size = 'md',
  iconOnly = false,
  loading = false,
  className,
  disabled,
  children,
  ...props
}: ButtonProps) {
  return (
    <button
      {...props}
      className={buttonClassName({ variant, size, iconOnly, className })}
      data-variant={variant}
      data-size={size}
      data-icon-only={iconOnly || undefined}
      aria-busy={loading || undefined}
      disabled={disabled || loading}
    >
      {children}
    </button>
  )
}

export type TagTone = 'neutral' | 'brand' | 'quality' | 'success' | 'warning' | 'rating' | 'danger'

interface TagProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: TagTone
}

export function Tag({ tone = 'neutral', className, children, ...props }: TagProps) {
  return (
    <span {...props} className={clsx('nv-tag', className)} data-tone={tone}>
      {children}
    </span>
  )
}

interface SurfaceProps extends HTMLAttributes<HTMLDivElement> {
  as?: 'div' | 'section' | 'article'
}

export function Surface({ as: Element = 'div', className, children, ...props }: SurfaceProps) {
  return (
    <Element {...props} className={clsx('nv-surface', className)}>
      {children}
    </Element>
  )
}

interface PageContainerProps extends HTMLAttributes<HTMLDivElement> {
  width?: 'content' | 'wide'
}

export function PageContainer({ width = 'content', className, children, ...props }: PageContainerProps) {
  return (
    <div
      {...props}
      className={clsx('nv-page-container', className)}
      data-width={width === 'wide' ? 'wide' : undefined}
    >
      {children}
    </div>
  )
}

interface StackProps extends HTMLAttributes<HTMLDivElement> {
  gap?: '1' | '2' | '3' | '4' | '6' | '8'
}

export function Stack({ gap = '4', className, children, ...props }: StackProps) {
  return (
    <div {...props} className={clsx('nv-stack', className)} data-gap={gap}>
      {children}
    </div>
  )
}

interface SectionProps extends HTMLAttributes<HTMLElement> {
  title?: ReactNode
  description?: ReactNode
  action?: ReactNode
}

export function Section({ title, description, action, className, children, ...props }: SectionProps) {
  return (
    <section {...props} className={clsx('nv-section', className)}>
      {(title || description || action) && (
        <div className="nv-section-header">
          <div className="min-w-0">
            {title && <h2 className="nv-section-title">{title}</h2>}
            {description && <div className="nv-section-description">{description}</div>}
          </div>
          {action && <div className="shrink-0">{action}</div>}
        </div>
      )}
      {children}
    </section>
  )
}

interface EmptyStateProps extends HTMLAttributes<HTMLDivElement> {
  icon?: ReactNode
  title: ReactNode
  description?: ReactNode
  action?: ReactNode
}

export function EmptyState({ icon, title, description, action, className, ...props }: EmptyStateProps) {
  return (
    <div
      {...props}
      className={clsx('flex min-h-64 flex-col items-center justify-center px-6 py-12 text-center', className)}
    >
      {icon && (
        <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-tertiary)]">
          {icon}
        </div>
      )}
      <h3 className="text-base font-semibold text-[var(--nv-text-primary)]">{title}</h3>
      {description && (
        <div className="mt-2 max-w-md text-sm leading-6 text-[var(--nv-text-tertiary)]">
          {description}
        </div>
      )}
      {action && <div className="mt-5">{action}</div>}
    </div>
  )
}
