import type { Media, MixedItem } from '@/types'
import MediaCard from './MediaCard'
import { Section } from '@/components/design-system'

interface MediaGridProps {
  items?: Media[]
  mixedItems?: MixedItem[]
  title?: string
  loading?: boolean
}

export default function MediaGrid({ items, mixedItems, title, loading }: MediaGridProps) {
  if (loading) {
    return (
      <Section title={title}>
        <div className="nv-media-grid" aria-busy="true" aria-label="媒体内容加载中">
          {Array.from({ length: 12 }).map((_, i) => (
            <div key={i}>
              <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
              <div className="skeleton mt-2 h-3 w-3/4" />
              <div className="skeleton mt-1.5 h-2.5 w-1/2" />
            </div>
          ))}
        </div>
      </Section>
    )
  }

  if (mixedItems) {
    if (mixedItems.length === 0) return null
    return (
      <Section title={title}>
        <div className="nv-media-grid">
          {mixedItems.map((item) => {
            if (item.type === 'series' && item.series) {
              return <MediaCard key={`s-${item.series.id}`} series={item.series} />
            }
            if (item.media) {
              return <MediaCard key={`m-${item.media.id}`} media={item.media} />
            }
            return null
          })}
        </div>
      </Section>
    )
  }

  if (!items || items.length === 0) return null

  return (
    <Section title={title}>
      <div className="nv-media-grid">
        {items.map((media) => <MediaCard key={media.id} media={media} />)}
      </div>
    </Section>
  )
}
