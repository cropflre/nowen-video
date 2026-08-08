import type { ReactNode } from 'react'
import { Zap } from 'lucide-react'
import { Surface, Tag } from '@/components/design-system'

interface AuthShellProps {
  title: ReactNode
  description?: ReactNode
  icon?: ReactNode
  eyebrow?: ReactNode
  children: ReactNode
  footer?: ReactNode
}

export default function AuthShell({ title, description, icon, eyebrow, children, footer }: AuthShellProps) {
  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-[var(--nv-bg-canvas)] px-4 py-10 text-[var(--nv-text-primary)]">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -left-24 top-[-8rem] h-[30rem] w-[30rem] rounded-full bg-[var(--nv-ambient-cyan)] blur-[90px]" />
        <div className="absolute -right-24 bottom-[-10rem] h-[28rem] w-[28rem] rounded-full bg-[var(--nv-ambient-purple-soft)] blur-[100px]" />
      </div>

      <div className="relative z-10 w-full max-w-md">
        <div className="mb-7 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-[var(--nv-radius-container)] border border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)] shadow-[var(--nv-shadow-card)]">
            {icon ?? <Zap size={27} fill="currentColor" aria-hidden="true" />}
          </div>
          {eyebrow && <div className="mb-2 flex justify-center"><Tag tone="brand">{eyebrow}</Tag></div>}
          <h1 className="text-2xl font-semibold tracking-[-0.02em] text-[var(--nv-text-primary)]">{title}</h1>
          {description && <p className="mx-auto mt-2 max-w-sm text-sm leading-6 text-[var(--nv-text-secondary)]">{description}</p>}
        </div>

        <Surface className="border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] p-6 shadow-[var(--nv-shadow-elevated)] sm:p-7">
          {children}
        </Surface>

        {footer && <div className="mt-4 text-center text-xs text-[var(--nv-text-tertiary)]">{footer}</div>}
      </div>
    </div>
  )
}
