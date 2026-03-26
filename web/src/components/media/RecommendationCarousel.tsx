import { useRef } from 'react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import type { RecommendedMedia } from '@/types'
import { Film, Play, ChevronLeft, ChevronRight } from 'lucide-react'

interface RecommendationCarouselProps {
  recommendations: RecommendedMedia[]
}

export default function RecommendationCarousel({ recommendations }: RecommendationCarouselProps) {
  const scrollRef = useRef<HTMLDivElement>(null)

  const scroll = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return
    const scrollAmount = 300
    el.scrollBy({ left: direction === 'left' ? -scrollAmount : scrollAmount, behavior: 'smooth' })
  }

  if (recommendations.length === 0) return null

  return (
    <section>
      <div className="mb-3 flex items-center justify-between">
        <h3 className="flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Film size={16} className="text-neon/60" />
          相关推荐
        </h3>
        <div className="flex gap-1">
          <button
            onClick={() => scroll('left')}
            className="rounded-lg p-1.5 transition-colors hover:bg-neon-blue/5"
            style={{ color: 'var(--text-muted)' }}
            aria-label="向左滚动"
          >
            <ChevronLeft size={18} />
          </button>
          <button
            onClick={() => scroll('right')}
            className="rounded-lg p-1.5 transition-colors hover:bg-neon-blue/5"
            style={{ color: 'var(--text-muted)' }}
            aria-label="向右滚动"
          >
            <ChevronRight size={18} />
          </button>
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
            className="media-card group w-36 flex-shrink-0"
            role="listitem"
          >
            <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl" style={{ background: 'var(--bg-surface)' }}>
              <img
                src={streamApi.getPosterUrl(item.media.id)}
                alt={item.media.title}
                className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110"
                loading="lazy"
                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
              />
              <div className="absolute inset-0 -z-10 flex items-center justify-center" style={{ color: 'var(--text-muted)' }}>
                <Film size={32} />
              </div>
              {/* 悬停播放图标 */}
              <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
                <div className="absolute bottom-2 left-2">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full"
                    style={{
                      background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                      boxShadow: '0 0 12px var(--neon-blue-40)',
                    }}
                  >
                    <Play size={14} className="ml-0.5 text-white" fill="white" />
                  </div>
                </div>
              </div>
              {/* 推荐理由 */}
              <div className="absolute right-1.5 top-1.5">
                <span className="badge-accent text-[9px]">{item.reason}</span>
              </div>
            </div>
            <div className="p-2.5">
              <h4 className="truncate text-xs font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
                {item.media.title}
              </h4>
              <div className="mt-0.5 flex items-center gap-1 text-[10px]" style={{ color: 'var(--text-muted)' }}>
                {item.media.year > 0 && <span>{item.media.year}</span>}
                {item.media.rating > 0 && (
                  <>
                    <span className="text-neon-blue/20">·</span>
                    <span className="text-yellow-400">★{item.media.rating.toFixed(1)}</span>
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
