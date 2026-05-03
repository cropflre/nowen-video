import { useRef, useState, useEffect, useMemo } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { streamApi } from '@/api'
import { collectionApi } from '@/api'
import type { CollectionWithMedia, CollectionMediaItem } from '@/types'
import { groupByMovie, versionLabel } from '@/utils/collectionGroup'
import { Film, Play, ChevronLeft, ChevronRight, Layers, Star, Clock, ChevronDown, ChevronUp, Copy } from 'lucide-react'

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

  // 不显示：加载中、无数据、或折叠后只有 1 部电影
  // ★ 方案 C 核心：先折叠同片多版本
  const groupedMovies = useMemo(() => {
    if (!data?.media) return []
    return groupByMovie(data.media)
  }, [data?.media])

  if (loading || !data || groupedMovies.length <= 1) return null

  const { collection, media } = data
  // 当前电影在系列中的位置（基于折叠后的分组）
  const currentIndex = groupedMovies.findIndex((g) => g.versions.some((v) => v.is_current))
  const movieCount = groupedMovies.length
  const fileCount = media.length

  return (
    <section className="mt-6">
      {/* 标题栏 */}
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 className="flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Layers size={16} className="text-neon/60" />
            系列合集1
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
              {collection.name} · {movieCount}部
              {fileCount > movieCount && (
                <span className="opacity-70">/{fileCount}个文件</span>
              )}
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
                第 {currentIndex + 1}/{movieCount} 部
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
          {groupedMovies.map((group) => {
            const item = group.primary
            const isCurrent = group.versions.some((v) => v.is_current)
            return (
              <CollectionCard
                key={item.id}
                item={item}
                versionCount={group.versions.length}
                isCurrent={isCurrent}
                onClick={() => {
                  if (!isCurrent) {
                    navigate(`/media/${item.id}`)
                  }
                }}
              />
            )
          })}
        </div>
      )}

      {/* 展开的列表模式 */}
      {expanded && (
        <div className="space-y-2" role="list" aria-label="系列合集电影列表">
          {groupedMovies.map((group, index) => {
            const isCurrent = group.versions.some((v) => v.is_current)
            return (
              <CollectionListItem
                key={group.primary.id}
                group={group}
                index={index + 1}
                isCurrent={isCurrent}
              />
            )
          })}
        </div>
      )}
    </section>
  )
}

/** 横向滚动卡片 */
function CollectionCard({ item, versionCount, isCurrent, onClick }: {
  item: CollectionMediaItem
  versionCount: number
  isCurrent: boolean
  onClick: () => void
}) {
  const hasMultipleVersions = versionCount > 1
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

        {/* 多版本角标 */}
        {hasMultipleVersions && (
          <div className="absolute right-1.5 top-1.5">
            <span
              className="flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-[9px] font-bold backdrop-blur-md"
              style={{
                background: 'rgba(0,0,0,0.7)',
                color: '#fff',
              }}
              title={`共有 ${versionCount} 个版本`}
            >
              <Copy size={8} />
              {versionCount}版
            </span>
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

/** 展开列表项（支持同片多版本切换） */
function CollectionListItem({ group, index, isCurrent }: {
  group: import('@/utils/collectionGroup').GroupedMovieItem
  index: number
  isCurrent: boolean
}) {
  const item = group.primary
  const hasMultipleVersions = group.versions.length > 1
  const [showVersions, setShowVersions] = useState(false)

  return (
    <div
      className={`rounded-xl transition-all duration-200 ${isCurrent ? 'ring-1 ring-neon/30' : ''}`}
      style={{ background: isCurrent ? 'var(--neon-blue-5)' : 'var(--bg-surface)' }}
      role="listitem"
    >
      <Link
        to={isCurrent ? '#' : `/media/${item.id}`}
        className={`flex items-center gap-4 p-3 ${isCurrent ? '' : 'hover:bg-neon-blue/5'}`}
        onClick={(e) => { if (isCurrent) e.preventDefault() }}
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
          {hasMultipleVersions && (
            <span className="ml-2 inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[9px] font-bold"
              style={{
                background: 'var(--neon-blue-10)',
                color: 'var(--neon-blue)',
                border: '1px solid var(--neon-blue-20)',
              }}
              title={`共有 ${group.versions.length} 个版本`}
            >
              <Copy size={8} />
              {group.versions.length}版
            </span>
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

      {/* 展开版本按钮 */}
      {hasMultipleVersions && (
        <button
          type="button"
          onClick={(e) => {
            e.preventDefault()
            e.stopPropagation()
            setShowVersions((v) => !v)
          }}
          className="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full transition-colors"
          style={{
            background: showVersions ? 'var(--neon-blue-10)' : 'var(--bg-elevated)',
            color: showVersions ? 'var(--neon-blue)' : 'var(--text-muted)',
          }}
          title={showVersions ? '收起版本' : '展开版本'}
        >
          <ChevronDown size={12} className={showVersions ? 'rotate-180 transition-transform' : 'transition-transform'} />
        </button>
      )}

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

      {/* 版本下拉列表 */}
      {hasMultipleVersions && showVersions && (
        <div className="px-3 pb-3" style={{ borderTop: '1px dashed var(--border-default)' }}>
          <div className="mt-2 space-y-1">
            {group.versions.map((v) => {
              const label = versionLabel(v) || '默认版本'
              const vIsCurrent = v.is_current
              return (
                <Link
                  key={v.id}
                  to={vIsCurrent ? '#' : `/media/${v.id}`}
                  onClick={(e) => { if (vIsCurrent) e.preventDefault() }}
                  className="flex items-center justify-between gap-2 rounded-lg px-3 py-2 text-xs transition-colors"
                  style={{
                    background: vIsCurrent ? 'var(--neon-blue-10)' : 'var(--bg-elevated)',
                    color: vIsCurrent ? 'var(--neon-blue)' : 'var(--text-secondary)',
                  }}
                >
                  <span className="truncate">{label}</span>
                  {vIsCurrent && (
                    <span className="flex-shrink-0 rounded px-1.5 py-0.5 text-[9px] font-bold"
                      style={{
                        background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                        color: '#fff',
                      }}>当前</span>
                  )}
                </Link>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
