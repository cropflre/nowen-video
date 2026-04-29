// ============================================================
// Storage UI Kit
// 存储管理模块共用的原子组件，保证 WebDAV / Alist / S3 三端
// 视觉、交互、状态反馈完全一致。
//
// v2：全面主题变量化 — 所有颜色/边框/阴影走 CSS 变量，
// 保证在深/浅两种主题下都有正确的对比度。
// ============================================================
import { ReactNode, CSSProperties } from 'react'
import clsx from 'clsx'
import { CheckCircle2, XCircle, Loader2, Wifi, WifiOff } from 'lucide-react'

// ---------------- 状态徽章 ----------------

export type ProviderState = 'connected' | 'error' | 'disabled' | 'idle'

interface StatusBadgeProps {
  state: ProviderState
  label?: string
  size?: 'sm' | 'md'
}

/** 统一的 provider 状态徽章：四种状态 = 四种色板，形状/留白/图标尺寸完全一致 */
export function StatusBadge({ state, label, size = 'md' }: StatusBadgeProps) {
  const palette: Record<ProviderState, { bg: string; text: string; icon: ReactNode; defaultLabel: string }> = {
    connected: {
      bg: 'bg-emerald-500/10 border-emerald-500/30',
      text: 'text-emerald-500 dark:text-emerald-300',
      icon: <Wifi size={size === 'sm' ? 11 : 13} />,
      defaultLabel: '已连接',
    },
    error: {
      bg: 'bg-red-500/10 border-red-500/30',
      text: 'text-red-500 dark:text-red-300',
      icon: <XCircle size={size === 'sm' ? 11 : 13} />,
      defaultLabel: '异常',
    },
    disabled: {
      bg: 'bg-black/5 dark:bg-white/5 border-black/10 dark:border-white/10',
      text: '',
      icon: <WifiOff size={size === 'sm' ? 11 : 13} />,
      defaultLabel: '未启用',
    },
    idle: {
      bg: 'bg-blue-500/10 border-blue-500/30',
      text: 'text-blue-500 dark:text-blue-300',
      icon: <CheckCircle2 size={size === 'sm' ? 11 : 13} />,
      defaultLabel: '就绪',
    },
  }
  const p = palette[state]
  return (
    <span
      className={clsx(
        'inline-flex items-center gap-1.5 rounded-full border font-medium transition-colors',
        p.bg,
        p.text,
        size === 'sm' ? 'px-2 py-0.5 text-[10px]' : 'px-2.5 py-1 text-xs'
      )}
      style={state === 'disabled' ? { color: 'var(--text-tertiary)' } : undefined}
    >
      {p.icon}
      <span>{label || p.defaultLabel}</span>
    </span>
  )
}

// ---------------- 通用开关 ----------------

interface ToggleProps {
  checked: boolean
  onChange: (next: boolean) => void
  disabled?: boolean
  accent?: 'neon' | 'purple' | 'amber'
}

/**
 * 统一的 toggle switch（主题感知）
 * - 未开启态用深灰背景，和页面底色有强烈对比
 * - 开启态填充 accent 色 + glow
 */
export function Toggle({ checked, onChange, disabled, accent = 'neon' }: ToggleProps) {
  const accentBg: Record<string, string> = {
    neon: 'linear-gradient(90deg, #06b6d4, #22d3ee)',
    purple: 'linear-gradient(90deg, #a855f7, #c084fc)',
    amber: 'linear-gradient(90deg, #f59e0b, #fbbf24)',
  }
  const accentShadow: Record<string, string> = {
    neon: '0 0 12px rgba(34,211,238,0.45), inset 0 1px 2px rgba(255,255,255,0.2)',
    purple: '0 0 12px rgba(168,85,247,0.45), inset 0 1px 2px rgba(255,255,255,0.2)',
    amber: '0 0 12px rgba(245,158,11,0.45), inset 0 1px 2px rgba(255,255,255,0.2)',
  }
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={clsx(
        'storage-toggle relative h-6 w-11 shrink-0 rounded-full transition-all duration-200',
        'focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:ring-offset-2',
        disabled && 'opacity-40 cursor-not-allowed'
      )}
      style={{
        background: checked
          ? accentBg[accent]
          : 'var(--storage-toggle-off-bg, rgba(0,0,0,0.18))',
        border: checked ? '1px solid transparent' : '1px solid var(--storage-toggle-off-border, rgba(0,0,0,0.15))',
        boxShadow: checked ? accentShadow[accent] : 'inset 0 1px 2px rgba(0,0,0,0.06)',
        padding: 0,
      }}
    >
      <span
        className={clsx(
          'absolute top-0.5 h-5 w-5 rounded-full transition-transform duration-200',
          checked ? 'translate-x-[22px]' : 'translate-x-0.5'
        )}
        style={{
          background: '#ffffff',
          boxShadow:
            '0 2px 4px rgba(0,0,0,0.25), 0 1px 2px rgba(0,0,0,0.12)',
        }}
      />
    </button>
  )
}

// ---------------- 字段分组 ----------------

interface FieldGroupProps {
  title: string
  description?: string
  children: ReactNode
  collapsible?: boolean
  defaultOpen?: boolean
}

/** 字段分组：带分组标题和描述，折叠态默认用于"高级选项" */
export function FieldGroup({ title, description, children, collapsible, defaultOpen = true }: FieldGroupProps) {
  const body = (
    <div className="space-y-4">
      {description && (
        <p className="text-xs leading-relaxed" style={{ color: 'var(--text-tertiary)' }}>
          {description}
        </p>
      )}
      {children}
    </div>
  )

  if (!collapsible) {
    return (
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <span
            className="h-3.5 w-1 rounded-full bg-gradient-to-b from-primary-400 to-accent-500"
            aria-hidden
          />
          <h3 className="text-sm font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            {title}
          </h3>
        </div>
        {body}
      </div>
    )
  }

  return (
    <details open={defaultOpen} className="group">
      <summary
        className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 -mx-2 select-none transition-colors"
        style={{ color: 'var(--text-primary)' }}
        onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--nav-hover-bg)')}
        onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
      >
        <span className="h-3.5 w-1 rounded-full bg-gradient-to-b from-primary-400 to-accent-500" aria-hidden />
        <h3 className="text-sm font-semibold tracking-wide">{title}</h3>
        <svg
          className="ml-auto h-4 w-4 transition-transform group-open:rotate-90"
          style={{ color: 'var(--text-tertiary)' }}
          viewBox="0 0 20 20"
          fill="currentColor"
          aria-hidden
        >
          <path fillRule="evenodd" d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z" clipRule="evenodd" />
        </svg>
      </summary>
      <div
        className="mt-4 pl-3"
        style={{ borderLeft: '1px solid var(--border-default)' }}
      >
        {body}
      </div>
    </details>
  )
}

// ---------------- 表单字段 ----------------

interface FieldProps {
  label: string
  required?: boolean
  hint?: string
  error?: string
  children: ReactNode
  /** 占据整行（在 grid 里） */
  fullWidth?: boolean
}

export function Field({ label, required, hint, error, children, fullWidth }: FieldProps) {
  return (
    <div className={clsx('space-y-1.5', fullWidth && 'md:col-span-2')}>
      <label
        className="flex items-center gap-1 text-xs font-medium uppercase tracking-wider"
        style={{ color: 'var(--text-secondary)' }}
      >
        <span>{label}</span>
        {required && <span className="text-red-500 text-sm leading-none">*</span>}
      </label>
      {children}
      {hint && !error && (
        <p className="text-[11px] leading-relaxed" style={{ color: 'var(--text-muted)' }}>
          {hint}
        </p>
      )}
      {error && <p className="text-[11px] text-red-500 leading-relaxed">{error}</p>}
    </div>
  )
}

// ---------------- 文本输入 ----------------

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  suffix?: ReactNode
  invalid?: boolean
}

/**
 * 统一的输入框样式（主题感知）
 * 日间：纯白底 + 深灰边 + focus 青色；深色：深底 + 细青边
 */
export function Input({ suffix, invalid, className, style, disabled, ...rest }: InputProps) {
  const baseStyle: CSSProperties = {
    background: invalid ? 'rgba(239,68,68,0.06)' : 'var(--storage-input-bg, var(--bg-input))',
    borderColor: invalid ? 'rgba(239,68,68,0.55)' : 'var(--storage-input-border, var(--border-strong))',
    color: 'var(--text-primary)',
    boxShadow: invalid
      ? '0 0 0 3px rgba(239,68,68,0.08)'
      : 'var(--storage-input-shadow, inset 0 1px 2px rgba(0,0,0,0.04))',
    ...style,
  }
  return (
    <div className="relative">
      <input
        {...rest}
        disabled={disabled}
        className={clsx(
          'storage-input w-full rounded-lg border px-3 py-2 text-sm transition-all outline-none',
          'focus:border-[var(--neon-blue)]',
          'focus:shadow-[0_0_0_3px_var(--neon-blue-15)]',
          disabled && 'opacity-60 cursor-not-allowed',
          suffix ? 'pr-10' : '',
          className
        )}
        style={baseStyle}
      />
      {suffix && <div className="absolute right-2 top-1/2 -translate-y-1/2">{suffix}</div>}
    </div>
  )
}

// ---------------- 操作栏 ----------------

interface ActionBarProps {
  /** 左侧次要操作（测试连接等） */
  secondaryActions?: ReactNode
  /** 右侧主要操作（保存等） */
  primaryActions?: ReactNode
  /** 内嵌在容器底部（有上分隔线） */
  inline?: boolean
}

export function ActionBar({ secondaryActions, primaryActions, inline }: ActionBarProps) {
  return (
    <div
      className={clsx('flex flex-wrap items-center gap-2', inline && 'pt-4 mt-2')}
      style={inline ? { borderTop: '1px solid var(--border-default)' } : undefined}
    >
      {secondaryActions}
      <div className="ml-auto flex flex-wrap items-center gap-2">{primaryActions}</div>
    </div>
  )
}

// ---------------- 按钮 ----------------

interface ActionButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'icon'
  accent?: 'neon' | 'purple' | 'amber'
  loading?: boolean
  icon?: ReactNode
}

/** 统一风格的按钮 —— 主/次/幽灵/图标四种，主文字颜色用主题变量 */
export function ActionButton({
  variant = 'secondary',
  accent = 'neon',
  loading,
  icon,
  children,
  className,
  disabled,
  style,
  ...rest
}: ActionButtonProps) {
  const base =
    'inline-flex items-center gap-2 rounded-lg font-medium transition-all duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-400/50'
  const sizeCls = variant === 'icon' ? 'p-2' : 'px-4 py-2 text-sm'

  // primary：实色按钮（深底浅字 / 渐变底 on-neon 字）
  const primaryMap = {
    neon: 'bg-gradient-to-r from-primary-500 to-primary-400 hover:shadow-[0_0_16px_rgba(0,212,224,0.35)]',
    purple: 'bg-gradient-to-r from-purple-500 to-accent-500 hover:shadow-[0_0_16px_rgba(168,85,247,0.35)]',
    amber: 'bg-gradient-to-r from-amber-500 to-orange-400 hover:shadow-[0_0_16px_rgba(245,158,11,0.35)]',
  }
  // secondary：低饱和底 + 对应色文字
  const secondaryMap = {
    neon: 'bg-primary-400/10 text-primary-600 dark:text-primary-300 hover:bg-primary-400/20 border border-primary-400/30',
    purple: 'bg-purple-500/10 text-purple-600 dark:text-purple-300 hover:bg-purple-500/20 border border-purple-500/30',
    amber: 'bg-amber-500/10 text-amber-600 dark:text-amber-300 hover:bg-amber-500/20 border border-amber-500/30',
  }

  const variantCls =
    variant === 'primary'
      ? primaryMap[accent]
      : variant === 'secondary'
      ? secondaryMap[accent]
      : variant === 'ghost'
      ? ''
      : '' // icon 单独处理

  const extraStyle: CSSProperties = { ...style }
  if (variant === 'primary') {
    // primary 强制用 --text-on-neon（深色模式=深色字，浅色模式=白字）
    extraStyle.color = 'var(--text-on-neon)'
  } else if (variant === 'ghost' || variant === 'icon') {
    extraStyle.color = 'var(--text-secondary)'
  }

  return (
    <button
      {...rest}
      disabled={disabled || loading}
      className={clsx(
        base,
        sizeCls,
        variantCls,
        (variant === 'ghost' || variant === 'icon') && 'hover:opacity-100',
        variant === 'icon' && 'rounded-md',
        'disabled:opacity-40 disabled:cursor-not-allowed disabled:shadow-none',
        className
      )}
      style={extraStyle}
      onMouseEnter={(e) => {
        if (variant === 'ghost' || variant === 'icon') {
          e.currentTarget.style.background = 'var(--nav-hover-bg)'
          e.currentTarget.style.color = 'var(--text-primary)'
        }
      }}
      onMouseLeave={(e) => {
        if (variant === 'ghost' || variant === 'icon') {
          e.currentTarget.style.background = 'transparent'
          e.currentTarget.style.color = 'var(--text-secondary)'
        }
      }}
    >
      {loading ? <Loader2 size={16} className="animate-spin" /> : icon}
      {children}
    </button>
  )
}

// ---------------- Toast 消息 ----------------

interface ToastProps {
  ok: boolean
  msg: string
  onDismiss?: () => void
}

export function Toast({ ok, msg, onDismiss }: ToastProps) {
  return (
    <div
      role="alert"
      className={clsx(
        'flex items-start gap-2.5 rounded-lg border px-3.5 py-2.5 text-sm animate-slide-down',
        ok
          ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-600 dark:text-emerald-300'
          : 'bg-red-500/10 border-red-500/30 text-red-600 dark:text-red-300'
      )}
    >
      {ok ? (
        <CheckCircle2 size={16} className="mt-0.5 flex-shrink-0" />
      ) : (
        <XCircle size={16} className="mt-0.5 flex-shrink-0" />
      )}
      <span className="flex-1 break-all leading-relaxed">{msg}</span>
      {onDismiss && (
        <button
          type="button"
          onClick={onDismiss}
          className="opacity-60 hover:opacity-100 transition-opacity"
          aria-label="关闭"
        >
          <XCircle size={14} />
        </button>
      )}
    </div>
  )
}

// ---------------- Provider 概览卡 ----------------

interface ProviderCardProps {
  icon: ReactNode
  name: string
  subtitle?: string
  state: ProviderState
  accent?: 'blue' | 'purple' | 'amber' | 'emerald'
  onClick?: () => void
  active?: boolean
}

/** 顶部概览用的 provider 卡片，也用作 Tab 切换入口 */
export function ProviderCard({ icon, name, subtitle, state, accent = 'blue', onClick, active }: ProviderCardProps) {
  const accentRing: Record<string, string> = {
    blue: 'ring-primary-400/50',
    purple: 'ring-purple-500/50',
    amber: 'ring-amber-500/50',
    emerald: 'ring-emerald-500/50',
  }
  const iconBg: Record<string, string> = {
    blue: 'bg-primary-400/10 text-primary-600 dark:text-primary-300',
    purple: 'bg-purple-500/10 text-purple-600 dark:text-purple-300',
    amber: 'bg-amber-500/10 text-amber-600 dark:text-amber-300',
    emerald: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-300',
  }
  const Component = onClick ? 'button' : 'div'
  return (
    <Component
      {...(onClick ? { onClick, type: 'button' as const } : {})}
      className={clsx(
        'glass-panel-subtle group relative rounded-xl p-4 text-left transition-all duration-200 w-full',
        onClick && 'cursor-pointer hover:-translate-y-0.5',
        active && `ring-2 ${accentRing[accent]}`
      )}
      style={active ? { borderColor: 'var(--border-strong)' } : undefined}
    >
      {active && (
        <span className="pointer-events-none absolute inset-0 rounded-xl bg-gradient-to-br from-white/[0.03] to-transparent" />
      )}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <div
            className={clsx(
              'flex h-10 w-10 shrink-0 items-center justify-center rounded-lg transition-transform',
              iconBg[accent],
              onClick && 'group-hover:scale-110'
            )}
          >
            {icon}
          </div>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              {name}
            </div>
            {subtitle && (
              <div className="truncate text-[11px] mt-0.5" style={{ color: 'var(--text-tertiary)' }}>
                {subtitle}
              </div>
            )}
          </div>
        </div>
        <StatusBadge state={state} size="sm" />
      </div>
    </Component>
  )
}

// ---------------- Section 壳 ----------------

interface SectionShellProps {
  icon: ReactNode
  title: string
  subtitle?: string
  badge?: ReactNode
  statusSlot?: ReactNode
  description?: ReactNode
  children: ReactNode
  accent?: 'neon' | 'purple' | 'amber'
}

/** provider 表单外壳：统一头部（图标+标题+描述+状态），内容区玻璃卡 */
export function SectionShell({
  icon,
  title,
  subtitle,
  badge,
  statusSlot,
  description,
  children,
  accent = 'neon',
}: SectionShellProps) {
  const accentBar: Record<string, string> = {
    neon: 'from-primary-500 to-primary-400',
    purple: 'from-purple-500 to-accent-500',
    amber: 'from-amber-500 to-orange-400',
  }
  return (
    <section className="space-y-4">
      {/* 头部 */}
      <header className="flex flex-wrap items-center gap-3">
        <div
          className={clsx(
            'flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br shadow-lg',
            accentBar[accent]
          )}
          style={{ color: 'var(--text-on-neon)' }}
        >
          {icon}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
              {title}
            </h2>
            {badge}
          </div>
          {subtitle && (
            <p className="text-[11px] mt-0.5" style={{ color: 'var(--text-tertiary)' }}>
              {subtitle}
            </p>
          )}
        </div>
        {statusSlot && <div className="ml-auto">{statusSlot}</div>}
      </header>
      {description && (
        <div
          className="rounded-lg px-3.5 py-2.5 text-xs leading-relaxed"
          style={{
            color: 'var(--text-secondary)',
            background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))',
            border: '1px solid var(--storage-enable-row-border, var(--border-strong))',
          }}
        >
          {description}
        </div>
      )}
      {/* 内容 */}
      <div className="glass-panel-subtle rounded-xl p-5 md:p-6 space-y-6">{children}</div>
    </section>
  )
}

// ---------------- 版本徽章 ----------------

export function VersionBadge({ accent = 'neon', children = 'V2.3' }: { accent?: 'neon' | 'purple' | 'amber'; children?: ReactNode }) {
  const map: Record<string, string> = {
    neon: 'bg-primary-400/15 text-primary-600 dark:text-primary-300 border-primary-400/30',
    purple: 'bg-purple-500/15 text-purple-600 dark:text-purple-300 border-purple-500/30',
    amber: 'bg-amber-500/15 text-amber-600 dark:text-amber-300 border-amber-500/30',
  }
  return (
    <span
      className={clsx(
        'inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider',
        map[accent]
      )}
    >
      {children}
    </span>
  )
}

// ---------------- 密码眼睛按钮 ----------------

export function EyeToggle({ visible, onToggle }: { visible: boolean; onToggle: () => void }) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="p-1 -mr-1 transition-colors rounded hover:opacity-100"
      style={{ color: 'var(--text-tertiary)' }}
      onMouseEnter={(e) => (e.currentTarget.style.color = 'var(--text-primary)')}
      onMouseLeave={(e) => (e.currentTarget.style.color = 'var(--text-tertiary)')}
      tabIndex={-1}
      aria-label={visible ? '隐藏' : '显示'}
    >
      {visible ? (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
          <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24" />
          <line x1="1" y1="1" x2="23" y2="23" />
        </svg>
      ) : (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
          <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
          <circle cx="12" cy="12" r="3" />
        </svg>
      )}
    </button>
  )
}

// ---------------- 启用行（Provider 主开关的外壳） ----------------

interface EnableRowProps {
  icon: ReactNode
  title: string
  description?: string
  checked: boolean
  onChange: (v: boolean) => void
  accent?: 'neon' | 'purple' | 'amber'
  iconColorClass?: string
}

/** Provider "启用开关"独立组件 —— 统一容器样式，背景/边框走主题变量 */
export function EnableRow({
  icon,
  title,
  description,
  checked,
  onChange,
  accent = 'neon',
  iconColorClass,
}: EnableRowProps) {
  return (
    <div
      className="flex items-center justify-between gap-4 rounded-lg px-4 py-3"
      style={{
        background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))',
        border: '1px solid var(--storage-enable-row-border, var(--border-strong))',
        boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.03)',
      }}
    >
      <div className="flex items-center gap-3 min-w-0">
        <span className={clsx('flex-shrink-0', iconColorClass)} style={!iconColorClass ? { color: 'var(--neon-blue)' } : undefined}>
          {icon}
        </span>
        <div className="min-w-0">
          <div className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
            {title}
          </div>
          {description && (
            <div className="text-[11px] truncate" style={{ color: 'var(--text-tertiary)' }}>
              {description}
            </div>
          )}
        </div>
      </div>
      <Toggle checked={checked} onChange={onChange} accent={accent} />
    </div>
  )
}
