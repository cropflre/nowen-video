import type { HTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'

export interface AppShellProps extends HTMLAttributes<HTMLDivElement> {
  sidebar: ReactNode
  mobileNavigation?: ReactNode
  sidebarCollapsed?: boolean
}

export function AppShell({
  sidebar,
  mobileNavigation,
  sidebarCollapsed = false,
  className,
  children,
  ...props
}: AppShellProps) {
  return (
    <div
      {...props}
      className={clsx('nv-app-shell relative flex h-full min-h-0 overflow-hidden', className)}
      data-sidebar-collapsed={sidebarCollapsed ? 'true' : 'false'}
    >
      {sidebar}
      {children}
      {mobileNavigation}
    </div>
  )
}

export default AppShell
