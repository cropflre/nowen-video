import type { Media } from '@/types'
import MediaCard from './MediaCard'

interface MediaGridProps {
  items: Media[]
  title?: string
  loading?: boolean
}

export default function MediaGrid({ items, title, loading }: MediaGridProps) {
  if (loading) {
    return (
      <div className="animate-fade-in">
        {title && (
          <h2 className="mb-4 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            {title}
          </h2>
        )}
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {Array.from({ length: 12 }).map((_, i) => (
            <div key={i}>
              <div className="skeleton aspect-[2/3] rounded-xl" />
              <div className="skeleton mt-2 h-4 w-3/4 rounded" />
              <div className="skeleton mt-1 h-3 w-1/2 rounded" />
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (items.length === 0) {
    return null
  }

  return (
    <div className="animate-fade-in">
      {title && (
        <h2 className="mb-4 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          {title}
        </h2>
      )}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        {items.map((media) => (
          <MediaCard key={media.id} media={media} />
        ))}
      </div>
    </div>
  )
}
