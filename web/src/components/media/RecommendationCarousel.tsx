import { useRef } from 'react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import type { RecommendedMedia } from '@/types'
import { Button, Tag } from '@/components/design-system'
import { Film, Play, ChevronLeft, ChevronRight, Star } from 'lucide-react'

interface RecommendationCarouselProps {
  recommendations: RecommendedMedia[]
}

export default function RecommendationCarousel({ recommendations }: RecommendationCarouselProps) {
  const scrollRef = useRef<HTMLDivElement>(null)

  const scroll = (direction: 'left' | 'right') => {
    const element = scrollRef.current
    if (!element) return
    element.scrollBy({ left: direction === 'left' ? -300 : 300, behavior: 'smooth' })
  }

  if (recommendations.length === 0) return null

  return (
    <section aria-labelledby="recommendation-title">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Film size={16} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
          <h2 id="recommendation-title" className="text-base font-semibold text-[var(--nv-text-primary)]">
            相关推荐
          </h2>
        </div>
        <div className="flex items-center gap-1" role="group" aria-label="相关推荐滚动控制">
          <Button variant="ghost" size="sm" iconOnly onClick={() => scroll('left')} aria-label="向左滚动">
            <ChevronLeft size={17} aria-hidden="true" />
          </Button>
          <Button variant="ghost" size="sm" iconOnly onClick={() => scroll('right')} aria-label="向右滚动">
            <ChevronRight size={17} aria-hidden="true" />
          </Button>
        </div>
      </div>

      <div
        ref={scrollRef}
        className="flex gap-4 overflow-x-auto pb-2 scrollbar-hide"
        style={{ scrollbarWidth: 'none' }}
        role="list"
        aria-label="相关推荐媒体列表"
      >
        {recommendations.map((item) => (
          <Link
            key={item.media.id}
            to={`/media/${item.media.id}`}
            className="group w-36 flex-shrink-0 rounded-[var(--nv-radius-card)] border border-transparent transition-[background-color,border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-surface-soft)] hover:shadow-[var(--nv-shadow-card)]"
            role="listitem"
          >
            <div className="relative aspect-[2/3] overflow-hidden rounded-[var(--nv-radius-card)] bg-[var(--nv-bg-surface-soft)]">
              <img
                src={streamApi.getPosterUrl(item.media.id)}
                alt={item.media.title}
                className="h-full w-full object-cover transition-transform duration-300 ease-out group-hover:scale-[1.025]"
                loading="lazy"
                onError={(event) => { event.currentTarget.style.display = 'none' }}
              />

              <div className="pointer-events-none absolute inset-0 -z-10 flex items-center justify-center text-[var(--nv-text-tertiary)]">
                <Film size={30} aria-hidden="true" />
              </div>

              <div className="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-black/80 via-black/25 to-transparent opacity-0 transition-opacity duration-200 group-hover:opacity-100" />
              <div className="absolute bottom-2 right-2 flex h-8 w-8 scale-95 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)] opacity-0 shadow-[var(--nv-shadow-card)] transition-[opacity,transform] duration-200 group-hover:scale-100 group-hover:opacity-100">
                <Play size={13} className="ml-0.5" fill="currentColor" aria-hidden="true" />
              </div>

              {item.reason && (
                <Tag tone="quality" className="absolute left-1.5 top-1.5 max-w-[calc(100%-12px)]">
                  <span className="truncate">{item.reason}</span>
                </Tag>
              )}
            </div>

            <div className="px-1 pb-2 pt-2.5">
              <h3 className="truncate text-xs font-medium text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]">
                {item.media.title}
              </h3>
              <div className="mt-1 flex items-center gap-1.5 text-[10px] text-[var(--nv-text-tertiary)]">
                {item.media.year > 0 && <span>{item.media.year}</span>}
                {item.media.rating > 0 && (
                  <>
                    {item.media.year > 0 && <span aria-hidden="true">·</span>}
                    <span className="inline-flex items-center gap-0.5 text-[var(--nv-status-rating)]">
                      <Star size={9} fill="currentColor" aria-hidden="true" />
                      {item.media.rating.toFixed(1)}
                    </span>
                  </>
                )}
              </div>
            </div>
          </Link>
        ))}
      </div>
    </section>
  )
}
