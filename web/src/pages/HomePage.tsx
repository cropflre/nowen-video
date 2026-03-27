import { useEffect, useState, useCallback, useRef, useMemo } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { mediaApi, recommendApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import { formatProgress } from '@/utils/format'
import type { WatchHistory, RecommendedMedia, MixedItem } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import { Play, Clock, Sparkles, ChevronLeft, ChevronRight, Star } from 'lucide-react'
import { streamApi } from '@/api'
import clsx from 'clsx'

export default function HomePage() {
  const [recentItems, setRecentItems] = useState<MixedItem[]>([])
  const [continueList, setContinueList] = useState<WatchHistory[]>([])
  const [recommendations, setRecommendations] = useState<RecommendedMedia[]>([])
  const [loading, setLoading] = useState(true)
  const location = useLocation()
  const { on, off } = useWebSocket()
  const toast = useToast()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // 数据加载函数（可复用）
  const fetchData = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true)
    try {
      const [recentRes, continueRes, recommendRes] = await Promise.all([
        mediaApi.recentMixed(20),
        mediaApi.continueWatching(10),
        recommendApi.getRecommendations(12),
      ])
      setRecentItems(recentRes.data.data || [])
      setContinueList(continueRes.data.data || [])
      setRecommendations(recommendRes.data.data || [])
    } catch {
      toast.error('加载数据失败，请刷新页面重试')
    } finally {
      setLoading(false)
    }
  }, [])

  // 初始加载 + 每次导航回首页时自动刷新
  useEffect(() => {
    fetchData(true)
  }, [fetchData, location.key])

  // 监听 WebSocket 媒体库变更事件，自动刷新首页数据
  useEffect(() => {
    // 防抨动刷新：收到事件后延迟 1 秒再刷新，避免短时间内多次刷新
    const debouncedRefresh = () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(() => fetchData(false), 1000)
    }

    // 媒体库删除时立即清空并重新加载
    const handleLibraryDeleted = () => {
      setRecentItems([])
      setContinueList([])
      setRecommendations([])
      fetchData(true)
    }

    // 扫描/刮削完成时静默刷新
    const handleContentChanged = () => debouncedRefresh()

    on(WS_EVENTS.LIBRARY_DELETED, handleLibraryDeleted)
    on(WS_EVENTS.LIBRARY_UPDATED, handleContentChanged)
    on(WS_EVENTS.SCAN_COMPLETED, handleContentChanged)
    on(WS_EVENTS.SCRAPE_COMPLETED, handleContentChanged)

    return () => {
      off(WS_EVENTS.LIBRARY_DELETED, handleLibraryDeleted)
      off(WS_EVENTS.LIBRARY_UPDATED, handleContentChanged)
      off(WS_EVENTS.SCAN_COMPLETED, handleContentChanged)
      off(WS_EVENTS.SCRAPE_COMPLETED, handleContentChanged)
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
    }
  }, [on, off, fetchData])

  return (
    <div className="space-y-10">
      {/* Hero Banner — 高分推荐轮播 */}
      {recommendations.length > 0 && <HeroBanner items={recommendations.slice(0, 5)} />}

      {/* 继续观看 */}
      {continueList.length > 0 && (
        <section className="animate-fade-in">
          <h2 className="mb-5 flex items-center gap-2 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Clock size={20} className="text-neon" />
            继续观看
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {continueList.map((item) => (
              <Link
                key={item.id}
                to={`/play/${item.media_id}`}
                className="glass-panel-subtle group flex gap-3 rounded-xl p-3 transition-all duration-300 hover:border-neon-blue/20 hover:shadow-card-hover"
              >
                {/* 缩略图 */}
                <div className="relative h-20 w-32 flex-shrink-0 overflow-hidden rounded-lg" style={{ background: 'var(--bg-surface)' }}>
                  {item.media.poster_path ? (
                    <img
                      src={streamApi.getPosterUrl(item.media_id)}
                      alt={item.media.title}
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <div className="flex h-full w-full items-center justify-center text-surface-700">
                      <Play size={24} />
                    </div>
                  )}
                  {/* 播放图标 */}
                  <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
                    <Play size={24} className="text-white" fill="white" />
                  </div>
                  {/* 霓虹进度条 */}
                  <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-white/10">
                    <div
                      className="h-full transition-all"
                      style={{
                        width: `${formatProgress(item.position, item.duration)}%`,
                        background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                        boxShadow: 'var(--neon-glow-shadow-sm)',
                      }}
                    />
                  </div>
                </div>

                {/* 信息 */}
                <div className="min-w-0 flex-1">
                  <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
                    {item.media.title}
                  </h3>
                  <p className="mt-1 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                    已观看 {formatProgress(item.position, item.duration)}%
                  </p>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* 为你推荐 */}
      {recommendations.length > 0 && (
        <section className="animate-fade-in">
          <h2 className="mb-5 flex items-center gap-2 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Sparkles size={20} className="text-yellow-400" />
            为你推荐
          </h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
            {recommendations.map((item) => (
              <Link
                key={item.media.id}
                to={item.media.series_id
                  ? `/series/${item.media.series_id}`
                  : `/media/${item.media.id}`
                }
                className="media-card group block"
              >
                {/* 海报区域 */}
                <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl" style={{ background: 'var(--bg-surface)' }}>
                <img
                    src={item.media.series_id
                      ? streamApi.getSeriesPosterUrl(item.media.series_id)
                      : streamApi.getPosterUrl(item.media.id)
                    }
                    alt={item.media.title}
                    className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110 group-hover:brightness-110"
                    loading="lazy"
                    onError={(e) => {
                      (e.target as HTMLImageElement).style.display = 'none'
                    }}
                  />
                  <div className="absolute inset-0 -z-10 flex items-center justify-center text-surface-700">
                    <Play size={48} />
                  </div>
                  {/* 悬停遮罩 */}
                  <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
                    <div className="absolute bottom-3 left-3 right-3">
                      <div className="flex items-center gap-2">
                        <div
                          className="flex h-10 w-10 items-center justify-center rounded-full transition-all duration-300"
                          style={{
                            background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                            boxShadow: 'var(--neon-glow-shadow-lg)',
                          }}
                        >
                          <Play size={18} className="ml-0.5 text-white" fill="white" />
                        </div>
                        <span className="text-sm font-semibold text-white">播放</span>
                      </div>
                    </div>
                  </div>
                  {/* 推荐理由标签 */}
                  <span className="badge-accent absolute left-2 top-2">
                    {item.reason}
                  </span>
                </div>
                {/* 信息区域 */}
                <div className="p-3">
                  <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
                    {item.media.title}
                  </h3>
                  <div className="mt-1 flex items-center gap-2 text-xs" style={{ color: 'var(--text-secondary)' }}>
                    {item.media.year > 0 && <span>{item.media.year}</span>}
                    {item.media.rating > 0 && (
                      <>
                        <span className="text-neon-blue/30">·</span>
                        <span className="text-yellow-400">★ {item.media.rating.toFixed(1)}</span>
                      </>
                    )}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* 最近添加 — 混合模式（电影+合集） */}
      <MediaGrid
        mixedItems={recentItems}
        title="最近添加"
        loading={loading}
      />

      {/* 分类推荐行 — 按类型分组横向滚动 */}
      {!loading && recentItems.length > 0 && (
        <GenreRows items={recentItems} />
      )}

      {/* 空状态 */}
      {!loading && recentItems.length === 0 && continueList.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div
            className="mb-6 flex h-20 w-20 items-center justify-center rounded-2xl animate-float"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue-10), var(--neon-purple-10))',
              border: '1px solid var(--neon-blue-10)',
            }}
          >
            <Play size={36} className="text-surface-600" />
          </div>
          <h3 className="font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-secondary)' }}>
            暂无媒体内容
          </h3>
          <p className="mt-2 text-sm" style={{ color: 'var(--text-muted)' }}>
            前往管理页面添加媒体库并扫描文件
          </p>
        </div>
      )}
    </div>
  )
}

// ===================== Hero Banner 轮播组件 =====================
function HeroBanner({ items }: { items: RecommendedMedia[] }) {
  const [current, setCurrent] = useState(0)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const [loadedImages, setLoadedImages] = useState<Set<string>>(new Set())

  // 缓存海报URL，避免每次渲染重新生成（token变化时才更新）
  // 剧集类型使用 Series 海报URL，电影使用 Media 海报URL
  const posterUrls = useMemo(() => {
    const urls = new Map<string, string>()
    items.forEach((rec) => {
      const url = rec.media.series_id
        ? streamApi.getSeriesPosterUrl(rec.media.series_id)
        : streamApi.getPosterUrl(rec.media.id)
      urls.set(rec.media.id, url)
    })
    return urls
  }, [items])

  // 自动轮播
  useEffect(() => {
    timerRef.current = setInterval(() => {
      setCurrent((prev) => (prev + 1) % items.length)
    }, 6000)
    return () => { if (timerRef.current) clearInterval(timerRef.current) }
  }, [items.length])

  const goTo = (index: number) => {
    setCurrent(index)
    if (timerRef.current) clearInterval(timerRef.current)
    timerRef.current = setInterval(() => {
      setCurrent((prev) => (prev + 1) % items.length)
    }, 6000)
  }

  const goPrev = () => goTo((current - 1 + items.length) % items.length)
  const goNext = () => goTo((current + 1) % items.length)

  const handleImageLoad = useCallback((mediaId: string) => {
    setLoadedImages((prev) => {
      const next = new Set(prev)
      next.add(mediaId)
      return next
    })
  }, [])

  const item = items[current]
  if (!item) return null

  return (
    <section className="animate-fade-in -mx-4 -mt-6 mb-4 sm:-mx-6 lg:-mx-8">
      <div className="relative h-[280px] overflow-hidden rounded-b-2xl sm:h-[340px] lg:h-[400px]">
        {/* 背景图 */}
        {items.map((rec, i) => (
          <div
            key={rec.media.id}
            className={clsx(
              'absolute inset-0 transition-all duration-1000',
              i === current ? 'opacity-100 scale-100' : 'opacity-0 scale-105'
            )}
          >
            <img
              src={posterUrls.get(rec.media.id)}
              alt=""
              className={clsx(
                'h-full w-full object-cover transition-opacity duration-500',
                loadedImages.has(rec.media.id) ? 'opacity-100' : 'opacity-0'
              )}
              loading={i === 0 ? 'eager' : 'lazy'}
              onLoad={() => handleImageLoad(rec.media.id)}
              onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
            />
          </div>
        ))}

        {/* 渐变遮罩 */}
        <div className="absolute inset-0" style={{ background: 'linear-gradient(to top, var(--bg-base) 5%, var(--bg-base)/70 40%, transparent 80%)' }} />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(to right, var(--bg-base)/80, transparent 60%)' }} />

        {/* 内容 */}
        <div className="absolute bottom-0 left-0 right-0 px-4 pb-8 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-7xl">
            {/* 推荐理由标签 */}
            <span className="badge-accent mb-3 inline-block text-xs">{item.reason}</span>

            {/* 标题 */}
            <h2 className="mb-2 font-display text-2xl font-bold tracking-wide text-white drop-shadow-lg sm:text-3xl lg:text-4xl">
              {item.media.title}
            </h2>

            {/* 元数据 */}
            <div className="mb-4 flex flex-wrap items-center gap-3 text-sm text-white/70">
              {item.media.year > 0 && <span>{item.media.year}</span>}
              {item.media.rating > 0 && (
                <span className="flex items-center gap-1 text-yellow-400">
                  <Star size={14} fill="currentColor" />
                  {item.media.rating.toFixed(1)}
                </span>
              )}
              {item.media.genres && (
                <span>{item.media.genres.split(',').slice(0, 3).join(' / ')}</span>
              )}
            </div>

            {/* 简介 */}
            {item.media.overview && (
              <p className="mb-5 line-clamp-2 max-w-2xl text-sm leading-relaxed text-white/60">
                {item.media.overview}
              </p>
            )}

            {/* 播放按钮 */}
            <div className="flex items-center gap-3">
              <Link
                to={item.media.media_type === 'episode' && item.media.series_id
                  ? `/series/${item.media.series_id}`
                  : `/play/${item.media.id}`
                }
                className="group inline-flex items-center gap-2.5 rounded-2xl px-7 py-3 text-sm font-bold transition-all duration-300 hover:-translate-y-0.5"
                style={{
                  background: 'linear-gradient(135deg, var(--neon-blue), rgba(0, 180, 220, 0.95))',
                  boxShadow: 'var(--shadow-neon), 0 4px 15px var(--neon-blue-15)',
                  color: 'var(--text-on-neon)',
                }}
              >
                <Play size={18} fill="currentColor" />
                立即播放
              </Link>
              <Link
                to={`/media/${item.media.id}`}
                className="rounded-2xl px-5 py-3 text-sm font-medium text-white/80 transition-all hover:text-white"
                style={{
                  background: 'rgba(255,255,255,0.08)',
                  border: '1px solid rgba(255,255,255,0.12)',
                  backdropFilter: 'blur(8px)',
                }}
              >
                查看详情
              </Link>
            </div>
          </div>
        </div>

        {/* 左右切换按钮 */}
        {items.length > 1 && (
          <>
            <button
              onClick={goPrev}
              className="absolute left-3 top-1/2 -translate-y-1/2 rounded-full p-2 text-white/40 transition-all hover:text-white hover:bg-white/10 sm:left-4"
            >
              <ChevronLeft size={24} />
            </button>
            <button
              onClick={goNext}
              className="absolute right-3 top-1/2 -translate-y-1/2 rounded-full p-2 text-white/40 transition-all hover:text-white hover:bg-white/10 sm:right-4"
            >
              <ChevronRight size={24} />
            </button>
          </>
        )}

        {/* 底部指示器 */}
        {items.length > 1 && (
          <div className="absolute bottom-3 left-1/2 -translate-x-1/2 flex items-center gap-2">
            {items.map((_, i) => (
              <button
                key={i}
                onClick={() => goTo(i)}
                className={clsx(
                  'rounded-full transition-all duration-500',
                  i === current
                    ? 'h-2 w-6'
                    : 'h-2 w-2 hover:bg-white/40'
                )}
                style={i === current
                  ? { background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))', boxShadow: 'var(--neon-glow-shadow-md)' }
                  : { background: 'rgba(255,255,255,0.2)' }
                }
              />
            ))}
          </div>
        )}
      </div>
    </section>
  )
}

// ===================== 分类推荐行组件 =====================
function GenreRows({ items }: { items: MixedItem[] }) {
  // 按类型分组
  const genreMap = new Map<string, MixedItem[]>()

  items.forEach((item) => {
    const media = item.type === 'movie' ? item.media : item.series
    if (!media) return
    const genres = (media.genres || '').split(',').filter(Boolean)
    genres.forEach((genre: string) => {
      const g = genre.trim()
      if (!g) return
      if (!genreMap.has(g)) genreMap.set(g, [])
      genreMap.get(g)!.push(item)
    })
  })

  // 只展示至少有3个项目的分类
  const genreEntries = Array.from(genreMap.entries())
    .filter(([, list]) => list.length >= 3)
    .sort((a, b) => b[1].length - a[1].length)
    .slice(0, 5) // 最多展示5个分类

  if (genreEntries.length === 0) return null

  return (
    <div className="space-y-8">
      {genreEntries.map(([genre, list]) => (
        <GenreRow key={genre} genre={genre} items={list.slice(0, 20)} />
      ))}
    </div>
  )
}

// ===================== 单个分类横向滚动行 =====================
function GenreRow({ genre, items }: { genre: string; items: MixedItem[] }) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(true)

  const updateScrollState = () => {
    const el = scrollRef.current
    if (!el) return
    setCanScrollLeft(el.scrollLeft > 10)
    setCanScrollRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 10)
  }

  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    el.addEventListener('scroll', updateScrollState, { passive: true })
    updateScrollState()
    return () => el.removeEventListener('scroll', updateScrollState)
  }, [])

  const scroll = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return
    const amount = el.clientWidth * 0.7
    el.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  return (
    <section className="animate-fade-in">
      <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        <span className="badge-accent text-xs">{genre}</span>
      </h2>

      <div className="group relative">
        {/* 左箭头 */}
        {canScrollLeft && (
          <button
            onClick={() => scroll('left')}
            className="absolute -left-2 top-1/2 z-10 -translate-y-1/2 rounded-full p-2 opacity-0 transition-all group-hover:opacity-100"
            style={{ background: 'var(--bg-surface)', boxShadow: 'var(--shadow-card)' }}
          >
            <ChevronLeft size={20} style={{ color: 'var(--text-primary)' }} />
          </button>
        )}

        {/* 横向滚动容器 */}
        <div
          ref={scrollRef}
          className="scrollbar-hide flex gap-4 overflow-x-auto scroll-smooth pb-2"
        >
          {items.map((item) => {
            const media = item.type === 'movie' ? item.media : item.series
            if (!media) return null
            const linkTo = item.type === 'series'
              ? `/series/${media.id}`
              : `/media/${media.id}`

            return (
              <Link
                key={media.id}
                to={linkTo}
                className="media-card group w-[140px] flex-shrink-0 sm:w-[160px]"
              >
                <div className="relative aspect-[2/3] overflow-hidden rounded-xl" style={{ background: 'var(--bg-surface)' }}>
                  <img
                    src={item.type === 'series' && media.id
                      ? streamApi.getSeriesPosterUrl(media.id)
                      : streamApi.getPosterUrl(media.id)
                    }
                    alt={media.title}
                    className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110"
                    loading="lazy"
                    onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                  />
                  <div className="absolute inset-0 -z-10 flex items-center justify-center text-surface-700">
                    <Play size={36} />
                  </div>
                  {/* 悬停播放按钮 */}
                  <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
                    <div className="absolute bottom-2 left-2">
                      <div
                        className="flex h-8 w-8 items-center justify-center rounded-full"
                        style={{
                          background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                          boxShadow: 'var(--neon-glow-shadow-md)',
                        }}
                      >
                        <Play size={14} className="ml-0.5 text-white" fill="white" />
                      </div>
                    </div>
                  </div>
                  {/* 评分标签 */}                  {media.rating > 0 && (
                    <span className="absolute right-1.5 top-1.5 flex items-center gap-0.5 rounded-md bg-black/60 px-1.5 py-0.5 text-[10px] text-yellow-400 backdrop-blur-sm">
                      <Star size={10} fill="currentColor" />
                      {media.rating.toFixed(1)}
                    </span>
                  )}
                </div>
                <div className="px-1 py-2">
                  <h3 className="truncate text-xs font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
                    {media.title}
                  </h3>
                  {media.year > 0 && (
                    <p className="mt-0.5 text-[10px]" style={{ color: 'var(--text-tertiary)' }}>{media.year}</p>
                  )}
                </div>
              </Link>
            )
          })}
        </div>

        {/* 右箭头 */}
        {canScrollRight && (
          <button
            onClick={() => scroll('right')}
            className="absolute -right-2 top-1/2 z-10 -translate-y-1/2 rounded-full p-2 opacity-0 transition-all group-hover:opacity-100"
            style={{ background: 'var(--bg-surface)', boxShadow: 'var(--shadow-card)' }}
          >
            <ChevronRight size={20} style={{ color: 'var(--text-primary)' }} />
          </button>
        )}
      </div>
    </section>
  )
}
