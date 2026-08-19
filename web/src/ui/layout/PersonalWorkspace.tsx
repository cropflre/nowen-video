import type { ReactNode } from 'react'
import clsx from 'clsx'

export interface PersonalWorkspaceProps {
  className?: string
  children: ReactNode
}

export function PersonalWorkspace({ className, children }: PersonalWorkspaceProps) {
  return <div className={clsx('nv-personal-workspace', className)}>{children}</div>
}

export interface PersonalWorkspaceHeaderProps {
  icon: ReactNode
  eyebrow: ReactNode
  title: ReactNode
  description?: ReactNode
  statValue?: ReactNode
  statLabel?: ReactNode
  statAriaLabel?: string
  actions?: ReactNode
}

export function PersonalWorkspaceHeader({
  icon,
  eyebrow,
  title,
  description,
  statValue,
  statLabel,
  statAriaLabel,
  actions,
}: PersonalWorkspaceHeaderProps) {
  const stat = statValue !== undefined ? (
    <div className="nv-personal-workspace-stat" aria-label={statAriaLabel}>
      <strong>{statValue}</strong>
      {statLabel !== undefined && <span>{statLabel}</span>}
    </div>
  ) : null

  return (
    <header className={clsx('nv-personal-workspace-header', actions && 'nv-personal-workspace-header--actions')}>
      <div className="nv-page-title-lockup">
        <div className="nv-page-title-icon" aria-hidden="true">{icon}</div>
        <div className="min-w-0">
          <span className="nv-personal-workspace-eyebrow">{eyebrow}</span>
          <h1 className="nv-page-title">{title}</h1>
          {description !== undefined && <p className="nv-page-subtitle">{description}</p>}
        </div>
      </div>

      {actions ? (
        <div className="nv-personal-workspace-header-actions">
          {stat}
          {actions}
        </div>
      ) : stat}
    </header>
  )
}

export interface PersonalWorkspacePanelProps {
  title: ReactNode
  titleId: string
  description?: ReactNode
  count?: ReactNode
  children: ReactNode
}

export function PersonalWorkspacePanel({
  title,
  titleId,
  description,
  count,
  children,
}: PersonalWorkspacePanelProps) {
  return (
    <section className="nv-personal-workspace-panel" aria-labelledby={titleId}>
      <div className="nv-personal-workspace-toolbar">
        <div>
          <h2 id={titleId}>{title}</h2>
          {description !== undefined && <p>{description}</p>}
        </div>
        {count !== undefined && <span className="nv-personal-workspace-count">{count}</span>}
      </div>
      {children}
    </section>
  )
}

export default PersonalWorkspace
