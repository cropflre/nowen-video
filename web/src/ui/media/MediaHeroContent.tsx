import type { ReactNode } from 'react'
import { Star } from 'lucide-react'
import type { Media } from '@/types'
import { Tag } from '@/components/design-system'

export interface MediaHeroContentProps {
  media: Media
  eyebrow?: ReactNode
  actions?: ReactNode
  className?: string
}

export function MediaHeroContent({ media, eyebrow, actions, className }: MediaHeroContentProps) {
  return (
    <div className={className}>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        {eyebrow && <Tag tone="brand">{eyebrow}</Tag>}
        {media.resolution && <Tag tone="quality">{media.resolution}</Tag>}
      </div>

      <h2
        className="nv-media-hero-title max-w-[18ch] font-bold text-[var(--nv-text-primary)]"
        style={{
          fontSize: 'var(--nv-type-display)',
          lineHeight: 'var(--nv-line-tight)',
          letterSpacing: 'var(--nv-tracking-tight)',
        }}
      >
        {media.title}
      </h2>

      {media.orig_title && media.orig_title !== media.title && (
        <div className="nv-media-hero-subtitle mt-1.5 line-clamp-1 text-sm text-[var(--nv-text-secondary)]">
          {media.orig_title}
        </div>
      )}

      <div className="nv-media-hero-meta mt-3 flex flex-wrap items-center gap-x-3 gap-y-2 text-sm text-[var(--nv-text-secondary)]">
        {media.year > 0 && <span>{media.year}</span>}
        {media.rating > 0 && (
          <span className="inline-flex items-center gap-1 text-[var(--nv-status-rating)]">
            <Star size={13} fill="currentColor" aria-hidden="true" />
            <span className="font-semibold">{media.rating.toFixed(1)}</span>
          </span>
        )}
        {media.genres && (
          <span className="text-[var(--nv-text-tertiary)]">
            {media.genres.split(',').slice(0, 3).join(' · ')}
          </span>
        )}
      </div>

      {media.overview && (
        <p className="nv-media-hero-overview mt-3 line-clamp-2 max-w-2xl text-sm leading-6 text-[var(--nv-text-secondary)] sm:text-[var(--nv-type-body)]">
          {media.overview}
        </p>
      )}

      {actions && <div className="nv-media-hero-actions mt-5 flex flex-wrap items-center gap-3">{actions}</div>}
    </div>
  )
}

export default MediaHeroContent
