/**
 * Fluent v9 风格轻量 primitives（M5）
 *
 * 设计意图：
 * - 不依赖 @fluentui/react-components 的 makeStyles，避免 runtime CSS 体积膨胀
 * - 通过 Tailwind + 已注入的 Fluent Design Tokens（FluentAppProvider）
 *   让组件和整站视觉语言保持一致
 * - 同时是后续把老组件向 Fluent 迁移的过渡层
 */
import { CSSProperties, forwardRef, HTMLAttributes, ReactNode } from 'react'
import clsx from 'clsx'

// ================= Surface（基础毛玻璃面） =================

export type SurfaceTone = 'subtle' | 'default' | 'bold' | 'ghost'

interface SurfaceProps extends HTMLAttributes<HTMLDivElement> {
  tone?: SurfaceTone
  padding?: 'none' | 'sm' | 'md' | 'lg'
  radius?: 'md' | 'lg' | 'xl' | '2xl'
  bordered?: boolean
}

const toneClasses: Record<SurfaceTone, string> = {
  subtle: 'bg-white/[0.03] backdrop-blur-xl',
  default: 'bg-white/[0.06] backdrop-blur-xl',
  bold: 'bg-white/[0.10] backdrop-blur-2xl',
  ghost: 'bg-transparent',
}

const padClasses = {
  none: '',
  sm: 'p-3',
  md: 'p-5',
  lg: 'p-7',
}

const radiusClasses = {
  md: 'rounded-md',
  lg: 'rounded-lg',
  xl: 'rounded-xl',
  '2xl': 'rounded-2xl',
}

/**
 * 通用毛玻璃面板。对标 Fluent v9 `Surface` + Windows Acrylic 质感。
 */
export const FluentSurface = forwardRef<HTMLDivElement, SurfaceProps>(function FluentSurface(
  { tone = 'default', padding = 'md', radius = 'xl', bordered = true, className, children, ...rest },
  ref,
) {
  return (
    <div
      ref={ref}
      {...rest}
      className={clsx(
        'relative',
        toneClasses[tone],
        padClasses[padding],
        radiusClasses[radius],
        bordered && 'border border-white/10',
        'transition-colors duration-200',
        className,
      )}
    >
      {children}
    </div>
  )
})

// ================= Card（可交互卡片） =================

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  interactive?: boolean
  selected?: boolean
}

/**
 * Fluent Card：悬浮 + 霓虹边发光 hover 效果
 */
export const FluentCard = forwardRef<HTMLDivElement, CardProps>(function FluentCard(
  { interactive = true, selected = false, className, children, ...rest },
  ref,
) {
  return (
    <FluentSurface
      ref={ref}
      tone="default"
      radius="xl"
      padding="md"
      className={clsx(
        interactive &&
          'cursor-pointer hover:bg-white/[0.09] hover:border-[color:var(--neon-blue,_#00F0FF)]/30 hover:shadow-[0_8px_30px_rgba(0,240,255,0.12)]',
        selected && 'ring-2 ring-[color:var(--neon-blue,_#00F0FF)]/60 border-transparent',
        className,
      )}
      {...(rest as HTMLAttributes<HTMLDivElement>)}
    >
      {children}
    </FluentSurface>
  )
})

// ================= SectionTitle（栏目标题） =================

interface SectionTitleProps {
  icon?: ReactNode
  title: ReactNode
  subtitle?: ReactNode
  action?: ReactNode
  className?: string
}

/**
 * 栏目标题行：左 icon+标题，右 action，Fluent 轻视觉
 */
export function FluentSectionTitle({
  icon,
  title,
  subtitle,
  action,
  className,
}: SectionTitleProps) {
  return (
    <div className={clsx('flex items-end justify-between mb-4 md:mb-5', className)}>
      <div className="flex items-center gap-3">
        {icon && (
          <span className="w-9 h-9 rounded-lg bg-white/5 border border-white/10 flex items-center justify-center text-[color:var(--neon-blue,_#00F0FF)]">
            {icon}
          </span>
        )}
        <div>
          <h2 className="text-lg md:text-xl font-semibold text-white tracking-wide">{title}</h2>
          {subtitle && <p className="text-xs md:text-sm text-white/55 mt-0.5">{subtitle}</p>}
        </div>
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  )
}

// ================= Badge =================

export type BadgeTone = 'neutral' | 'brand' | 'success' | 'warning' | 'danger' | 'hdr'

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone
  size?: 'sm' | 'md'
}

const badgeToneMap: Record<BadgeTone, string> = {
  neutral: 'bg-white/10 text-white/80 border-white/15',
  brand: 'bg-[color:var(--neon-blue,_#00F0FF)]/15 text-[color:var(--neon-blue,_#00F0FF)] border-[color:var(--neon-blue,_#00F0FF)]/30',
  success: 'bg-emerald-500/15 text-emerald-300 border-emerald-400/30',
  warning: 'bg-amber-500/20 text-amber-200 border-amber-400/40',
  danger: 'bg-rose-500/15 text-rose-300 border-rose-400/30',
  hdr: 'bg-gradient-to-r from-amber-500/25 to-orange-500/25 text-amber-100 border-amber-400/40',
}

/**
 * Fluent Badge：用于 HDR、4K、新增、独家等标注
 */
export function FluentBadge({
  tone = 'neutral',
  size = 'sm',
  className,
  children,
  ...rest
}: BadgeProps) {
  return (
    <span
      {...rest}
      className={clsx(
        'inline-flex items-center gap-1 border rounded-md font-medium tracking-wide',
        badgeToneMap[tone],
        size === 'sm' ? 'text-[11px] px-1.5 py-0.5' : 'text-xs px-2 py-1',
        className,
      )}
    >
      {children}
    </span>
  )
}

// ================= 工具：浮雕发光字 =================

interface NeonTextProps {
  children: ReactNode
  intensity?: 'low' | 'mid' | 'high'
  className?: string
  style?: CSSProperties
}

export function NeonText({ children, intensity = 'mid', className, style }: NeonTextProps) {
  const glow = {
    low: '0 0 4px rgba(0,240,255,0.25)',
    mid: '0 0 10px rgba(0,240,255,0.45)',
    high: '0 0 16px rgba(0,240,255,0.75)',
  }[intensity]
  return (
    <span
      className={clsx('text-[color:var(--neon-blue,_#00F0FF)] font-semibold', className)}
      style={{ textShadow: glow, ...style }}
    >
      {children}
    </span>
  )
}
