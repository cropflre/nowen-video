import type { HTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'

export function AdminPageHeader({
  title,
  description,
  actions,
  className,
}: {
  title: ReactNode
  description?: ReactNode
  actions?: ReactNode
  className?: string
}) {
  return (
    <header className={clsx('nv-admin-header', className)}>
      <div className="nv-admin-header-copy">
        <h1 className="nv-admin-title">{title}</h1>
        {description && <div className="nv-admin-description">{description}</div>}
      </div>
      {actions && <div className="nv-admin-toolbar">{actions}</div>}
    </header>
  )
}

export function AdminPanel({
  title,
  description,
  icon,
  actions,
  children,
  className,
  bodyClassName,
  ...props
}: Omit<HTMLAttributes<HTMLElement>, 'title'> & {
  title?: ReactNode
  description?: ReactNode
  icon?: ReactNode
  actions?: ReactNode
  bodyClassName?: string
}) {
  const hasHeader = title || description || icon || actions

  return (
    <section className={clsx('nv-admin-panel', className)} {...props}>
      {hasHeader && (
        <div className="nv-admin-panel-header">
          <div className="min-w-0">
            {title && (
              <div className="nv-admin-section-title">
                {icon}
                <span>{title}</span>
              </div>
            )}
            {description && (
              <div className="mt-1.5 text-sm leading-relaxed text-[var(--nv-text-tertiary)]">
                {description}
              </div>
            )}
          </div>
          {actions && <div className="nv-admin-toolbar shrink-0">{actions}</div>}
        </div>
      )}
      <div className={clsx('nv-admin-panel-body', bodyClassName)}>{children}</div>
    </section>
  )
}

export function AdminSectionTitle({
  icon,
  children,
  className,
}: {
  icon?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <div className={clsx('nv-admin-section-title', className)}>
      {icon}
      <span>{children}</span>
    </div>
  )
}

export type AdminStatusTone = 'neutral' | 'connected' | 'success' | 'active' | 'warning' | 'danger'

export function AdminStatus({
  tone = 'neutral',
  children,
  className,
}: {
  tone?: AdminStatusTone
  children: ReactNode
  className?: string
}) {
  return (
    <span className={clsx('nv-admin-status', className)} data-tone={tone}>
      {children}
    </span>
  )
}