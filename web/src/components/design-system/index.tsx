import {
  forwardRef,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
} from 'react'
import clsx from 'clsx'
import { Search } from 'lucide-react'

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

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  iconOnly?: boolean
  loading?: boolean
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button({
  variant = 'secondary',
  size = 'md',
  iconOnly = false,
  loading = false,
  className,
  disabled,
  children,
  ...props
}, ref) {
  return (
    <button
      {...props}
      ref={ref}
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
})

export interface IconButtonProps extends Omit<ButtonProps, 'iconOnly'> {
  label: string
}

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton({
  label,
  title,
  children,
  ...props
}, ref) {
  return (
    <Button
      {...props}
      ref={ref}
      iconOnly
      aria-label={label}
      title={title ?? label}
    >
      {children}
    </Button>
  )
})

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

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  invalid?: boolean
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input({ invalid = false, className, ...props }, ref) {
  return (
    <input
      {...props}
      ref={ref}
      className={clsx('nv-input', className)}
      aria-invalid={invalid || props['aria-invalid'] || undefined}
    />
  )
})

interface SearchFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  wrapperClassName?: string
}

export const SearchField = forwardRef<HTMLInputElement, SearchFieldProps>(function SearchField({
  wrapperClassName,
  className,
  ...props
}, ref) {
  return (
    <label className={clsx('nv-search', wrapperClassName)}>
      <Search size={15} aria-hidden="true" />
      <input {...props} ref={ref} type="search" className={className} />
    </label>
  )
})

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea({ invalid = false, className, ...props }, ref) {
  return (
    <textarea
      {...props}
      ref={ref}
      className={clsx('nv-textarea', className)}
      aria-invalid={invalid || props['aria-invalid'] || undefined}
    />
  )
})

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  invalid?: boolean
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select({ invalid = false, className, children, ...props }, ref) {
  return (
    <select
      {...props}
      ref={ref}
      className={clsx('nv-select', className)}
      aria-invalid={invalid || props['aria-invalid'] || undefined}
    >
      {children}
    </select>
  )
})

export type SurfaceVariant = 'base' | 'raised' | 'glass' | 'overlay'

interface SurfaceProps extends HTMLAttributes<HTMLDivElement> {
  as?: 'div' | 'section' | 'article'
  variant?: SurfaceVariant
}

export function Surface({
  as: Element = 'div',
  variant = 'base',
  className,
  children,
  ...props
}: SurfaceProps) {
  return (
    <Element
      {...props}
      className={clsx('nv-surface', className)}
      data-variant={variant === 'base' ? undefined : variant}
    >
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

interface SectionProps extends Omit<HTMLAttributes<HTMLElement>, 'title'> {
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

interface EmptyStateProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  icon?: ReactNode
  title: ReactNode
  description?: ReactNode
  action?: ReactNode
}

export function EmptyState({ icon, title, description, action, className, ...props }: EmptyStateProps) {
  return (
    <div
      {...props}
      className={clsx('nv-empty-state flex min-h-56 flex-col items-center justify-center px-6 py-10 text-center', className)}
    >
      {icon && (
        <div className="nv-empty-state-icon mb-3 flex h-10 w-10 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-fill-hover)] text-[var(--nv-text-tertiary)]">
          {icon}
        </div>
      )}
      <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">{title}</h3>
      {description && (
        <div className="mt-1.5 max-w-md text-xs leading-5 text-[var(--nv-text-tertiary)]">
          {description}
        </div>
      )}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}

export { Modal, ModalBody, ModalFooter, ModalHeader, type ModalSize } from './Modal'
