import type { RecommendedMedia } from '@/types'
import MediaCard from '@/components/MediaCard'
import { EmptyState } from '@/components/design-system'
import { Film } from 'lucide-react'

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
    <section className="nv-recommendation-section" aria-labelledby="recommendation-title">
      <div className="nv-recommendation-header">
        <div className="flex items-center gap-2">
          <Film size={15} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
          <h2 id="recommendation-title" className="nv-section-title">
            相关推荐
          </h2>
          <span className="text-[10px] text-[var(--nv-text-tertiary)]">{recommendations.length}</span>
        </div>
      </div>

      <div
        className="grid justify-start gap-x-[13px] gap-y-6"
        style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(138px, 166px))' }}
        role="list"
        aria-label="相关推荐媒体列表"
      >
        {recommendations.map((item) => (
          <div key={item.media.id} className="min-w-0" role="listitem">
            <MediaCard media={item.media} eyebrow={item.reason} />
          </div>
        ))}
      </div>
    </section>
  )
}
