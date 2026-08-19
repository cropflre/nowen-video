import type { ReactNode } from 'react'
import { Star } from 'lucide-react'
import type { Media } from '@/types'
import { Tag } from '@/components/design-system'
import { HeroContent, type HeroHeadingLevel } from './HeroContent'

export interface MediaHeroContentProps {
  media: Media
  eyebrow?: ReactNode
  actions?: ReactNode
  supplemental?: ReactNode
  className?: string
  title?: ReactNode
  subtitle?: ReactNode
  extraMeta?: ReactNode
  extraBadges?: ReactNode
  headingLevel?: HeroHeadingLevel
  compact?: boolean
  inlineBadges?: boolean
}

function formatHeroDuration(media: Media) {
  const seconds = media.duration > 0
    ? media.duration
    : media.runtime > 0
      ? media.runtime * 60
      : 0
  if (!seconds) return ''
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  if (hours > 0 && minutes > 0) return `${hours}小时${minutes}分钟`
  if (hours > 0) return `${hours}小时`
  return `${minutes}分钟`
}

export function MediaHeroContent({
  media,
  eyebrow,
  actions,
  supplemental,
  className,
  title,
  subtitle,
  extraMeta,
  extraBadges,
  headingLevel = 'h2',
  compact = false,
  inlineBadges = false,
}: MediaHeroContentProps) {
  const resolvedSubtitle = subtitle ?? (
    media.orig_title && media.orig_title !== media.title ? media.orig_title : undefined
  )
  const durationLabel = formatHeroDuration(media)
  const genres = media.genres
    ? media.genres.split(',').slice(0, 3).map((genre) => genre.trim()).filter(Boolean)
    : []

  return (
    <HeroContent
      className={className}
      compact={compact}
      headingLevel={headingLevel}
      eyebrow={eyebrow ? <Tag tone="brand">{eyebrow}</Tag> : undefined}
      title={title ?? media.title}
      subtitle={resolvedSubtitle}
      meta={(
        <>
          {media.rating > 0 && (
            <span className="inline-flex items-center gap-1 text-[var(--nv-status-rating)]">
              <Star size={13} fill="currentColor" aria-hidden="true" />
              <span className="font-semibold">{media.rating.toFixed(1)}</span>
            </span>
          )}
          {media.year > 0 && <span>{media.year}</span>}
          {durationLabel && <span>{durationLabel}</span>}
          {inlineBadges ? (
            <>
              {media.resolution && <Tag tone="quality">{media.resolution}</Tag>}
              {genres.map((genre) => <Tag key={genre}>{genre}</Tag>)}
              {extraBadges}
            </>
          ) : genres.length > 0 ? (
            <span className="text-[var(--nv-text-tertiary)]">{genres.join(' · ')}</span>
          ) : null}
          {extraMeta}
        </>
      )}
      badges={inlineBadges ? undefined : (
        <>
          {media.resolution && <Tag tone="quality">{media.resolution}</Tag>}
          {extraBadges}
        </>
      )}
      overview={media.overview || undefined}
      supplemental={supplemental}
      actions={actions}
    />
  )
}

export default MediaHeroContent
