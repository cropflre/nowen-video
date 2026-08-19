import { Link } from 'react-router-dom'
import type { RecommendedMedia } from '@/types'
import MediaCard from '@/components/MediaCard'
import { EmptyState } from '@/components/design-system'
import { MediaRail } from '@/ui'
import { ChevronRight, Film } from 'lucide-react'

interface RecommendationCarouselProps {
  recommendations: RecommendedMedia[]
}

export default function RecommendationCarousel({ recommendations }: RecommendationCarouselProps) {
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
    <section className="nv-recommendation-section nv-detail-recommendation-section" aria-labelledby="recommendation-title">
      <MediaRail
        title={(
          <span className="inline-flex items-center gap-2">
            <Film size={16} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
            <span id="recommendation-title">相似推荐</span>
          </span>
        )}
        ariaLabel="相似推荐"
        itemCount={recommendations.length}
        fullItemsOnly
        className="nv-detail-recommendation-rail"
        action={(
          <Link to="/browse" className="nv-detail-section-more">
            查看更多
            <ChevronRight size={14} aria-hidden="true" />
          </Link>
        )}
      >
        {recommendations.map((item) => (
          <div key={item.media.id} className="nv-detail-recommendation-slot min-w-0">
            <MediaCard media={item.media} eyebrow={item.reason} variant="recommendation" />
          </div>
        ))}
      </MediaRail>
    </section>
  )
}
