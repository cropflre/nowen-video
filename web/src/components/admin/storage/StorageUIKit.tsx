import type { ButtonHTMLAttributes, CSSProperties, InputHTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'
import {
  CheckCircle2,
  ChevronRight,
  Eye,
  EyeOff,
  Loader2,
  Wifi,
  WifiOff,
  X,
  XCircle,
} from 'lucide-react'
import { Button, Input as DesignInput, Tag } from '@/components/design-system'
import { AdminPanel, AdminStatus, type AdminStatusTone } from '@/components/admin/AdminPrimitives'

export type ProviderState = 'connected' | 'error' | 'disabled' | 'idle'

type LegacyAccent = 'neon' | 'purple' | 'amber'
type LegacyProviderAccent = 'blue' | 'purple' | 'amber' | 'emerald'

const STATUS_CONFIG: Record<
  ProviderState,
  { tone: AdminStatusTone; label: string; icon: ReactNode }
> = {
  connected: {
    tone: 'success',
    label: '已连接',
    icon: <Wifi size={12} />,
  },
  error: {
    tone: 'danger',
    label: '异常',
    icon: <XCircle size={12} />,
  },
  disabled: {
    tone: 'neutral',
    label: '未启用',
    icon: <WifiOff size={12} />,
  },
  idle: {
    tone: 'active',
    label: '就绪',
    icon: <CheckCircle2 size={12} />,
  },
}

interface StatusBadgeProps {
  state: ProviderState
  label?: string
  size?: 'sm' | 'md'
}

export function StatusBadge({ state, label, size = 'md' }: StatusBadgeProps) {
  const config = STATUS_CONFIG[state]
  return (
    <AdminStatus
      tone={config.tone}
      className={clsx(size === 'sm' ? 'gap-1 px-2 py-0.5 text-[10px]' : 'gap-1.5')}
    >
      {config.icon}
      <span>{label || config.label}</span>
    </AdminStatus>
  )
}

interface ToggleProps {
  checked: boolean
  onChange: (next: boolean) => void
  disabled?: boolean
  accent?: LegacyAccent
}

/**
 * Storage keeps the historical accent prop for call-site compatibility, but
 * all enabled switches intentionally use the single semantic primary action.
 */
export function Toggle({ checked, onChange, disabled, accent: _accent = 'neon' }: ToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={clsx(
        'relative h-6 w-11 shrink-0 rounded-full border outline-none transition-[background-color,border-color,opacity] duration-200',
        'focus-visible:border-[var(--nv-action-primary)] focus-visible:shadow-[var(--nv-shadow-focus)]',
        disabled && 'cursor-not-allowed opacity-40',
      )}
      style={{
        background: checked ? 'var(--nv-action-primary)' : 'var(--nv-bg-control)',
        borderColor: checked ? 'var(--nv-action-primary)' : 'var(--nv-border-default)',
      }}
    >
      <span
        className={clsx(
          'absolute top-0.5 h-5 w-5 rounded-full transition-transform duration-200',
          checked ? 'translate-x-[22px]' : 'translate-x-0.5',
        )}
        style={{
          background: checked ? 'var(--nv-text-on-brand)' : 'var(--nv-text-tertiary)',
          boxShadow: '0 1px 3px rgba(0,0,0,.2)',
        }}
      />
    </button>
  )
}

interface FieldGroupProps {
  title: string
  description?: string
  children: ReactNode
  collapsible?: boolean
  defaultOpen?: boolean
}

function FieldGroupBody({ description, children }: Pick<FieldGroupProps, 'description' | 'children'>) {
  return (
    <div className="space-y-4">
      {description && (
        <p className="text-xs leading-relaxed text-[var(--nv-text-tertiary)]">{description}</p>
      )}
      {children}
    </div>
  )
}

export function FieldGroup({
  title,
  description,
  children,
  collapsible,
  defaultOpen = true,
}: FieldGroupProps) {
  if (!collapsible) {
    return (
      <div className="space-y-3">
        <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">{title}</h3>
        <FieldGroupBody description={description}>{children}</FieldGroupBody>
      </div>
    )
  }

  return (
    <details
      open={defaultOpen}
      className="group rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]"
    >
      <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2.5 text-sm font-semibold text-[var(--nv-text-primary)] transition-colors hover:bg-[var(--nv-bg-hover)]">
        <ChevronRight
          size={15}
          className="text-[var(--nv-text-tertiary)] transition-transform duration-200 group-open:rotate-90"
        />
        <span>{title}</span>
      </summary>
      <div className="border-t border-[var(--nv-border-subtle)] px-3 py-4">
        <FieldGroupBody description={description}>{children}</FieldGroupBody>
      </div>
    </details>
  )
}

interface FieldProps {
  label: string
  required?: boolean
  hint?: string
  error?: string
  children: ReactNode
  fullWidth?: boolean
}

export function Field({ label, required, hint, error, children, fullWidth }: FieldProps) {
  return (
    <div className={clsx('space-y-1.5', fullWidth && 'md:col-span-2')}>
      <label className="flex items-center gap-1 text-xs font-medium text-[var(--nv-text-secondary)]">
        <span>{label}</span>
        {required && <span className="text-[var(--nv-status-danger)]">*</span>}
      </label>
      {children}
      {hint && !error && (
        <p className="text-[11px] leading-relaxed text-[var(--nv-text-tertiary)]">{hint}</p>
      )}
      {error && (
        <p className="text-[11px] leading-relaxed text-[var(--nv-status-danger)]">{error}</p>
      )}
    </div>
  )
}

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  suffix?: ReactNode
  invalid?: boolean
}

export function Input({ suffix, invalid, className, disabled, style, ...rest }: InputProps) {
  return (
    <div className="relative">
      <DesignInput
        {...rest}
        invalid={invalid}
        disabled={disabled}
        className={clsx(suffix && 'pr-10', className)}
        style={style}
      />
      {suffix && <div className="absolute right-2 top-1/2 -translate-y-1/2">{suffix}</div>}
    </div>
  )
}

interface ActionBarProps {
  secondaryActions?: ReactNode
  primaryActions?: ReactNode
  inline?: boolean
}

export function ActionBar({ secondaryActions, primaryActions, inline }: ActionBarProps) {
  return (
    <div
      className={clsx(
        'flex flex-wrap items-center gap-2',
        inline && 'mt-2 border-t border-[var(--nv-border-subtle)] pt-4',
      )}
    >
      {secondaryActions}
      <div className="ml-auto flex flex-wrap items-center gap-2">{primaryActions}</div>
    </div>
  )
}

interface ActionButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'icon'
  accent?: LegacyAccent
  loading?: boolean
  icon?: ReactNode
}

export function ActionButton({
  variant = 'secondary',
  accent: _accent = 'neon',
  loading = false,
  icon,
  children,
  className,
  disabled,
  ...rest
}: ActionButtonProps) {
  const designVariant =
    variant === 'primary' ? 'primary' : variant === 'ghost' || variant === 'icon' ? 'ghost' : 'secondary'
  const iconOnly = variant === 'icon' && !children

  return (
    <Button
      {...rest}
      variant={designVariant}
      size="sm"
      iconOnly={iconOnly}
      loading={loading}
      disabled={disabled}
      className={className}
    >
      {loading ? <Loader2 size={15} className="animate-spin" /> : icon}
      {children}
    </Button>
  )
}

interface ToastProps {
  ok: boolean
  msg: string
  onDismiss?: () => void
}

export function Toast({ ok, msg, onDismiss }: ToastProps) {
  const statusColor = ok ? 'var(--nv-status-success)' : 'var(--nv-status-danger)'
  const style: CSSProperties = {
    background: `color-mix(in srgb, ${statusColor} 8%, var(--nv-bg-surface))`,
    borderColor: `color-mix(in srgb, ${statusColor} 28%, var(--nv-border-subtle))`,
  }

  return (
    <div
      role="alert"
      className="flex items-start gap-2.5 rounded-[var(--nv-radius-control)] border px-3.5 py-2.5 text-sm text-[var(--nv-text-primary)]"
      style={style}
    >
      {ok ? (
        <CheckCircle2 size={16} className="mt-0.5 shrink-0 text-[var(--nv-status-success)]" />
      ) : (
        <XCircle size={16} className="mt-0.5 shrink-0 text-[var(--nv-status-danger)]" />
      )}
      <span className="min-w-0 flex-1 break-all leading-relaxed">{msg}</span>
      {onDismiss && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          iconOnly
          onClick={onDismiss}
          aria-label="关闭"
          className="-mr-1 -mt-1"
        >
          <X size={14} />
        </Button>
      )}
    </div>
  )
}

interface ProviderCardProps {
  icon: ReactNode
  name: string
  subtitle?: string
  state: ProviderState
  accent?: LegacyProviderAccent
  onClick?: () => void
  active?: boolean
}

function ProviderCardContent({
  icon,
  name,
  subtitle,
  state,
  active,
}: Omit<ProviderCardProps, 'accent' | 'onClick'>) {
  return (
    <div className="flex items-start justify-between gap-3">
      <div className="flex min-w-0 items-center gap-3">
        <div
          className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border"
          style={{
            color: active ? 'var(--nv-action-primary)' : 'var(--nv-text-tertiary)',
            background: active ? 'var(--nv-bg-active)' : 'var(--nv-bg-surface-soft)',
            borderColor: active ? 'var(--nv-border-default)' : 'var(--nv-border-subtle)',
          }}
        >
          {icon}
        </div>
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-[var(--nv-text-primary)]">{name}</div>
          {subtitle && (
            <div className="mt-0.5 truncate text-[11px] text-[var(--nv-text-tertiary)]">{subtitle}</div>
          )}
        </div>
      </div>
      <StatusBadge state={state} size="sm" />
    </div>
  )
}

export function ProviderCard({
  icon,
  name,
  subtitle,
  state,
  accent: _accent = 'blue',
  onClick,
  active = false,
}: ProviderCardProps) {
  const className = clsx(
    'w-full rounded-[var(--nv-radius-card)] border bg-[var(--nv-bg-surface)] p-4 text-left transition-[background-color,border-color,transform,box-shadow] duration-200',
    onClick && 'cursor-pointer hover:-translate-y-0.5 hover:bg-[var(--nv-bg-hover)]',
  )
  const style: CSSProperties = {
    borderColor: active ? 'var(--nv-action-primary)' : 'var(--nv-border-subtle)',
    boxShadow: active ? 'var(--nv-shadow-card)' : 'none',
  }
  const content = (
    <ProviderCardContent icon={icon} name={name} subtitle={subtitle} state={state} active={active} />
  )

  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={className} style={style} aria-pressed={active}>
        {content}
      </button>
    )
  }

  return (
    <div className={className} style={style}>
      {content}
    </div>
  )
}

interface SectionShellProps {
  icon: ReactNode
  title: string
  subtitle?: string
  badge?: ReactNode
  statusSlot?: ReactNode
  description?: ReactNode
  children: ReactNode
  accent?: LegacyAccent
}

export function SectionShell({
  icon,
  title,
  subtitle,
  badge,
  statusSlot,
  description,
  children,
  accent: _accent = 'neon',
}: SectionShellProps) {
  return (
    <AdminPanel
      icon={<span className="text-[var(--nv-action-primary)]">{icon}</span>}
      title={
        <span className="flex flex-wrap items-center gap-2">
          <span>{title}</span>
          {badge}
        </span>
      }
      description={subtitle}
      actions={statusSlot}
      bodyClassName="space-y-6"
    >
      {description && (
        <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3.5 py-2.5 text-xs leading-relaxed text-[var(--nv-text-secondary)]">
          {description}
        </div>
      )}
      {children}
    </AdminPanel>
  )
}

export function VersionBadge({
  accent: _accent = 'neon',
  children = 'V2.3',
}: {
  accent?: LegacyAccent
  children?: ReactNode
}) {
  return (
    <Tag tone="brand" className="text-[10px] font-semibold uppercase tracking-wide">
      {children}
    </Tag>
  )
}

export function EyeToggle({ visible, onToggle }: { visible: boolean; onToggle: () => void }) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      iconOnly
      onClick={onToggle}
      tabIndex={-1}
      aria-label={visible ? '隐藏' : '显示'}
    >
      {visible ? <EyeOff size={15} /> : <Eye size={15} />}
    </Button>
  )
}

interface EnableRowProps {
  icon: ReactNode
  title: string
  description?: string
  checked: boolean
  onChange: (v: boolean) => void
  accent?: LegacyAccent
  iconColorClass?: string
}

export function EnableRow({
  icon,
  title,
  description,
  checked,
  onChange,
  accent: _accent = 'neon',
  iconColorClass: _iconColorClass,
}: EnableRowProps) {
  return (
    <div className="flex items-center justify-between gap-4 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3">
      <div className="flex min-w-0 items-center gap-3">
        <span className="shrink-0 text-[var(--nv-action-primary)]">{icon}</span>
        <div className="min-w-0">
          <div className="text-sm font-medium text-[var(--nv-text-primary)]">{title}</div>
          {description && (
            <div className="mt-0.5 text-[11px] leading-relaxed text-[var(--nv-text-tertiary)]">
              {description}
            </div>
          )}
        </div>
      </div>
      <Toggle checked={checked} onChange={onChange} />
    </div>
  )
}
