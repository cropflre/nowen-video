import { useRef, useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { streamApi } from '@/api'
import { collectionApi } from '@/api'
import type { CollectionWithMedia, CollectionMediaItem } from '@/types'
import { Film, Play, ChevronLeft, ChevronRight, Layers, Star, Clock, ChevronDown, ChevronUp } from 'lucide-react'

interface CollectionCarouselProps {
  mediaId: string
}

export default function CollectionCarousel({ mediaId }: CollectionCarouselProps) {
  const [data, setData] = useState<CollectionWithMedia | null>(null)
  const [loading, setLoading] = useState(true)
  const [expanded, setExpanded] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    collectionApi.getMediaCollection(mediaId)
      .then(res => {
        if (!cancelled) {
          setData(res.data.data)
        }
      })
      .catch(() => {
        if (!cancelled) setData(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [mediaId])

  const scroll = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return
    const scrollAmount = 320
    el.scrollBy({ left: direction === 'left' ? -scrollAmount : scrollAmount, behavior: 'smooth' })
  }

  // 不显示：加载中、无数据、或只有1部电影
  if (loading || !data || data.media.length <= 1) return null

  const { collection, media } = data
  // 当前电影在系列中的位置
  const currentIndex = media.findIndex(m => m.is_current)

  return (
    <section className="mt-6">
      {/* 标题栏 */}
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 className="flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Layers size={16} className="text-neon/60" />
            系列合集
            <Link
              to={`/collections/${collection.id}`}
              className="ml-1 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium transition-all hover:scale-105"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue-10), var(--neon-purple-10))',
                color: 'var(--neon-blue)',
                border: '1px solid var(--neon-blue-20)',
              }}
              title="查看合集详情"
            >
              {collection.name} · {media.length}部
              <ChevronRight size={10} />
            </Link>
          </h3>
          <button
            onClick={() => setExpanded(!expanded)}
            className="rounded-lg p-1 transition-colors hover:bg-neon-blue/5"
            title={expanded ? '收起列表' : '展开列表'}
          >
            {expanded ? (
              <ChevronUp size={16} style={{ color: 'var(--text-muted)' }} />
            ) : (
              <ChevronDown size={16} style={{ color: 'var(--text-muted)' }} />
            )}
          </button>
        </div>

        {!expanded && (
          <div className="flex items-center gap-2">
            {/* 系列进度指示器 */}
            {currentIndex >= 0 && (
              <span className="text-[10px] font-medium" style={{ color: 'var(--text-muted)' }}>
                第 {currentIndex + 1}/{media.length} 部
              </span>
            )}
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
        )}
      </div>

      {/* 横向滚动卡片模式（默认） */}
      {!expanded && (
        <div
          ref={scrollRef}
          className="flex gap-4 overflow-x-auto pb-2 scrollbar-hide"
          style={{ scrollbarWidth: 'none' }}
          role="list"
          aria-label="系列合集电影列表"
        >
          {media.map((item) => (
            <CollectionCard
              key={item.id}
              item={item}
              isCurrent={item.is_current}
              onClick={() => {
                if (!item.is_current) {
                  navigate(`/media/${item.id}`)
                }
              }}
            />
          ))}
        </div>
      )}

      {/* 展开的列表模式 */}
      {expanded && (
        <div className="space-y-2" role="list" aria-label="系列合集电影列表">
          {media.map((item, index) => (
            <CollectionListItem
              key={item.id}
              item={item}
              index={index + 1}
              isCurrent={item.is_current}
            />
          ))}
        </div>
      )}
    </section>
  )
}

/** 横向滚动卡片 */
function CollectionCard({ item, isCurrent, onClick }: {
  item: CollectionMediaItem
  isCurrent: boolean
  onClick: () => void
}) {
  return (
    <div
      onClick={onClick}
      className={`media-card group w-36 flex-shrink-0 cursor-pointer transition-all duration-300 ${
        isCurrent ? 'ring-2 ring-neon scale-[1.02]' : 'hover:scale-[1.02]'
      }`}
      role="listitem"
    >
      <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl" style={{ background: 'var(--bg-surface)' }}>
        <img
          src={streamApi.getPosterUrl(item.id)}
          alt={item.title}
          className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        <div className="absolute inset-0 -z-10 flex items-center justify-center" style={{ color: 'var(--text-muted)' }}>
          <Film size={32} />
        </div>

        {/* 当前正在查看标识 */}
        {isCurrent && (
          <div className="absolute left-1.5 top-1.5">
            <span className="block rounded-md px-1.5 py-0.5 text-[9px] font-bold leading-tight backdrop-blur-md"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                color: '#fff',
                boxShadow: '0 0 8px var(--neon-blue-40)',
              }}
            >当前</span>
          </div>
        )}

        {/* 悬停播放图标 */}
        {!isCurrent && (
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
        )}
      </div>
      <div className="p-2.5">
        <h4 className={`truncate text-xs font-medium transition-colors ${
          isCurrent ? 'text-neon' : 'group-hover:text-neon'
        }`} style={isCurrent ? {} : { color: 'var(--text-primary)' }}>
          {item.title}
        </h4>
        <div className="mt-0.5 flex items-center gap-1 text-[10px]" style={{ color: 'var(--text-muted)' }}>
          {item.year > 0 && <span>{item.year}</span>}
          {item.rating > 0 && (
            <>
              <span className="text-neon-blue/20">·</span>
              <span className="text-yellow-400">★{item.rating.toFixed(1)}</span>
            </>
          )}
        </div>
      </div>
    </div>
  )
}

/** 展开列表项 */
function CollectionListItem({ item, index, isCurrent }: {
  item: CollectionMediaItem
  index: number
  isCurrent: boolean
}) {
  return (
    <Link
      to={isCurrent ? '#' : `/media/${item.id}`}
      className={`flex items-center gap-4 rounded-xl p-3 transition-all duration-200 ${
        isCurrent
          ? 'ring-1 ring-neon/30'
          : 'hover:bg-neon-blue/5'
      }`}
      style={{
        background: isCurrent ? 'var(--neon-blue-5)' : 'var(--bg-surface)',
      }}
      onClick={(e) => { if (isCurrent) e.preventDefault() }}
      role="listitem"
    >
      {/* 序号 */}
      <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg text-sm font-bold"
        style={{
          background: isCurrent
            ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))'
            : 'var(--bg-elevated)',
          color: isCurrent ? '#fff' : 'var(--text-muted)',
        }}
      >
        {index}
      </div>

      {/* 海报缩略图 */}
      <div className="relative h-16 w-11 flex-shrink-0 overflow-hidden rounded-lg" style={{ background: 'var(--bg-elevated)' }}>
        <img
          src={streamApi.getPosterUrl(item.id)}
          alt={item.title}
          className="h-full w-full object-cover"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        <div className="absolute inset-0 -z-10 flex items-center justify-center" style={{ color: 'var(--text-muted)' }}>
          <Film size={16} />
        </div>
      </div>

      {/* 信息 */}
      <div className="min-w-0 flex-1">
        <h4 className={`truncate text-sm font-medium ${isCurrent ? 'text-neon' : ''}`}
          style={isCurrent ? {} : { color: 'var(--text-primary)' }}
        >
          {item.title}
          {isCurrent && (
            <span className="ml-2 inline-block rounded px-1.5 py-0.5 text-[9px] font-bold"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                color: '#fff',
              }}
            >当前</span>
          )}
        </h4>
        <div className="mt-1 flex items-center gap-3 text-xs" style={{ color: 'var(--text-muted)' }}>
          {item.year > 0 && <span>{item.year}</span>}
          {item.rating > 0 && (
            <span className="flex items-center gap-0.5">
              <Star size={10} className="text-yellow-400" fill="currentColor" />
              {item.rating.toFixed(1)}
            </span>
          )}
          {item.runtime > 0 && (
            <span className="flex items-center gap-0.5">
              <Clock size={10} />
              {item.runtime}分钟
            </span>
          )}
        </div>
        {item.overview && (
          <p className="mt-1 line-clamp-1 text-[11px]" style={{ color: 'var(--text-muted)' }}>
            {item.overview}
          </p>
        )}
      </div>

      {/* 播放按钮 */}
      {!isCurrent && (
        <div className="flex-shrink-0 opacity-0 transition-opacity group-hover:opacity-100">
          <div className="flex h-8 w-8 items-center justify-center rounded-full"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
              boxShadow: '0 0 8px var(--neon-blue-40)',
            }}
          >
            <Play size={12} className="ml-0.5 text-white" fill="white" />
          </div>
        </div>
      )}
    </Link>
  )
}
