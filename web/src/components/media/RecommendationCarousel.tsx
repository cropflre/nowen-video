import { useRef } from 'react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import type { RecommendedMedia } from '@/types'
import { Button, Tag } from '@/components/design-system'
import { ChevronLeft, ChevronRight, Film, Play, Star } from 'lucide-react'

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
          <Film size={15} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
          <h2 id="recommendation-title" className="text-base font-semibold text-[var(--nv-text-primary)]">
            相关推荐
          </h2>
        </div>
        <div className="flex items-center gap-0.5" role="group" aria-label="相关推荐滚动控制">
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
            className="group w-36 flex-shrink-0 transition-transform duration-150 hover:-translate-y-0.5"
            role="listitem"
          >
            <div className="relative aspect-[2/3] overflow-hidden rounded-[var(--nv-radius-card)] bg-[var(--nv-bg-surface-soft)] shadow-[var(--nv-shadow-card)] transition-shadow duration-150 group-hover:shadow-[var(--nv-shadow-card-hover)]">
              <img
                src={streamApi.getPosterUrl(item.media.id)}
                alt={item.media.title}
                className="h-full w-full object-cover"
                loading="lazy"
                onError={(event) => { event.currentTarget.style.display = 'none' }}
              />

              <div className="pointer-events-none absolute inset-0 -z-10 flex items-center justify-center text-[var(--nv-text-tertiary)]">
                <Film size={30} aria-hidden="true" />
              </div>

              <div className="absolute inset-0 flex items-center justify-center bg-black/25 opacity-0 transition-opacity duration-150 group-hover:opacity-100">
                <span className="grid h-9 w-9 place-items-center rounded-full bg-white/90 text-black shadow-[var(--nv-shadow-card)]">
                  <Play size={14} className="ml-0.5" fill="currentColor" aria-hidden="true" />
                </span>
              </div>

              {item.reason && (
                <Tag className="absolute left-1.5 top-1.5 max-w-[calc(100%-12px)]">
                  <span className="truncate">{item.reason}</span>
                </Tag>
              )}
            </div>

            <div className="px-0.5 pt-2.5">
              <h3 className="truncate text-xs font-medium text-[var(--nv-text-primary)]">
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
