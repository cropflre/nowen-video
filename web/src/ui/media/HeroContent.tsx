import type { ReactNode } from 'react'
import clsx from 'clsx'
import { ChevronDown, ChevronUp } from 'lucide-react'

export type HeroHeadingLevel = 'h1' | 'h2'

export interface HeroOverviewToggle {
  expanded: boolean
  onToggle: () => void
  expandLabel: string
  collapseLabel: string
}

export interface HeroContentProps {
  title: ReactNode
  subtitle?: ReactNode
  eyebrow?: ReactNode
  meta?: ReactNode
  badges?: ReactNode
  overview?: ReactNode
  /** 传入时简介可展开/收起；不传则保持原有 3 行截断、无切换按钮。 */
  overviewToggle?: HeroOverviewToggle
  supplemental?: ReactNode
  actions?: ReactNode
  headingLevel?: HeroHeadingLevel
  className?: string
  compact?: boolean
}

export function HeroContent({
  title,
  subtitle,
  eyebrow,
  meta,
  badges,
  overview,
  overviewToggle,
  supplemental,
  actions,
  headingLevel = 'h1',
  className,
  compact = false,
}: HeroContentProps) {
  const Heading = headingLevel
  const titleText = typeof title === 'string' ? title : undefined

  return (
    <div className={clsx('nv-hero-content min-w-0', compact && 'nv-hero-content--compact', className)}>
      {eyebrow && <div className="nv-hero-content-eyebrow mb-2.5">{eyebrow}</div>}

      <Heading
        className="nv-media-hero-title max-w-[28ch] text-balance font-bold text-[var(--nv-text-primary)]"
        title={titleText}
        aria-label={titleText}
        style={{
          fontSize: compact ? 'var(--nv-type-h1)' : 'var(--nv-type-display)',
          lineHeight: 'var(--nv-line-tight)',
          letterSpacing: 'var(--nv-tracking-tight)',
        }}
      >
        {title}
      </Heading>

      {subtitle && (
        <div className="nv-media-hero-subtitle mt-1.5 line-clamp-1 max-w-3xl text-sm text-[var(--nv-text-secondary)] sm:text-base">
          {subtitle}
        </div>
      )}

      {meta && (
        <div className="nv-media-hero-meta mt-3 flex flex-wrap items-center gap-x-3 gap-y-2 text-sm text-[var(--nv-text-secondary)]">
          {meta}
        </div>
      )}

      {badges && (
        <div className="nv-media-hero-badges mt-3 flex flex-wrap items-center gap-2">
          {badges}
        </div>
      )}

      {overview && (
        <div className={clsx('nv-media-hero-overview mt-4 max-w-4xl text-sm leading-7 text-[var(--nv-text-secondary)]', !overviewToggle?.expanded && 'line-clamp-3')}>
          {overview}
        </div>
      )}

      {overview && overviewToggle && (
        <button
          type="button"
          onClick={overviewToggle.onToggle}
          aria-expanded={overviewToggle.expanded}
          className="nv-media-hero-overview-toggle mt-1.5 inline-flex w-fit items-center gap-1 text-xs font-medium text-[var(--nv-text-secondary)] transition-colors hover:text-[var(--nv-text-primary)]"
        >
          {overviewToggle.expanded ? <ChevronUp size={14} aria-hidden="true" /> : <ChevronDown size={14} aria-hidden="true" />}
          {overviewToggle.expanded ? overviewToggle.collapseLabel : overviewToggle.expandLabel}
        </button>
      )}

      {supplemental && <div className="nv-media-hero-supplemental">{supplemental}</div>}

      {actions && (
        <div className="nv-media-hero-actions mt-5 flex flex-wrap items-center gap-2.5">
          {actions}
        </div>
      )}
    </div>
  )
}

export default HeroContent
