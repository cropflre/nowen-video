import type { ReactNode } from 'react'
import { Film } from 'lucide-react'
import { Tag } from '@/components/design-system'

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
    <div className="flex min-h-screen items-center justify-center bg-[var(--nv-bg-canvas)] px-4 py-10 text-[var(--nv-text-primary)]">
      <main className="w-full max-w-sm">
        <header className="mb-7">
          <div className="mb-5 grid h-10 w-10 place-items-center rounded-[var(--nv-radius-control)] bg-[var(--nv-fill-active)] text-[var(--nv-text-secondary)]">
            {icon ?? <Film size={19} aria-hidden="true" />}
          </div>
          {eyebrow && <div className="mb-2"><Tag>{eyebrow}</Tag></div>}
          <h1 className="text-xl font-semibold tracking-[-0.015em] text-[var(--nv-text-primary)]">{title}</h1>
          {description && <p className="mt-1.5 text-xs leading-5 text-[var(--nv-text-tertiary)]">{description}</p>}
        </header>

        <section className="border-t border-[var(--nv-border-default)] pt-5">
          {children}
        </section>

        {footer && <div className="mt-5 border-t border-[var(--nv-border-subtle)] pt-4 text-[11px] leading-5 text-[var(--nv-text-tertiary)]">{footer}</div>}
      </main>
    </div>
  )
}
