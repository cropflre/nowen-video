import { useRef } from 'react'
import type { RecommendedMedia } from '@/types'
import MediaCard from '@/components/MediaCard'
import { Button, EmptyState } from '@/components/design-system'
import { ChevronLeft, ChevronRight, Film } from 'lucide-react'

interface RecommendationCarouselProps {
  recommendations: RecommendedMedia[]
}

export default function RecommendationCarousel({ recommendations }: RecommendationCarouselProps) {
  const scrollRef = useRef<HTMLDivElement>(null)

  const scroll = (direction: 'left' | 'right') => {
    const element = scrollRef.current
    if (!element) return
    const amount = Math.max(280, element.clientWidth * 0.72)
    element.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  if (recommendations.length === 0) {
    return (
      <EmptyState
        className="nv-detail-tab-empty-state"
        icon={<Film size={23} aria-hidden="true" />}
        title="暂无相关推荐"
        description="当前媒体暂时没有可展示的相似内容推荐。"
      />
    )
  }

  return (
    <section className="nv-recommendation-section" aria-labelledby="recommendation-title">
      <div className="nv-recommendation-header">
        <div className="flex items-center gap-2">
          <Film size={15} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
          <h2 id="recommendation-title" className="nv-section-title">
            相关推荐
          </h2>
          <span className="text-[10px] text-[var(--nv-text-tertiary)]">{recommendations.length}</span>
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
        className="nv-recommendation-row scrollbar-hide"
        style={{ scrollbarWidth: 'none' }}
        role="list"
        aria-label="相关推荐媒体列表"
      >
        {recommendations.map((item) => (
          <div key={item.media.id} className="nv-recommendation-item" role="listitem">
            <MediaCard media={item.media} eyebrow={item.reason} />
          </div>
        ))}
      </div>
    </section>
  )
}
