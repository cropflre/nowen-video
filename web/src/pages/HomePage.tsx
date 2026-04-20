import { useEffect, useState, useRef, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { mediaApi, recommendApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { usePageCache } from '@/hooks/usePageCache'
import { formatProgress } from '@/utils/format'
import type { WatchHistory, RecommendedMedia, MixedItem } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import { Play, Clock, Sparkles, ChevronLeft, ChevronRight, Star } from 'lucide-react'
import { streamApi } from '@/api'
import { motion } from 'framer-motion'
import { staggerContainerVariants, staggerItemVariants } from '@/lib/motion'
import HeroCarousel from '@/components/HeroCarousel'

interface HomeData {
  recentItems: MixedItem[]
  continueList: WatchHistory[]
  recommendations: RecommendedMedia[]
  allFailed: boolean
}

export default function HomePage() {
  const { on, off } = useWebSocket()
  const toast = useToast()
  const { t } = useTranslation()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // 跨页面共享缓存：返回首页命中缓存 → 零 loading；过期则后台静默刷新
  const { data, loading, refetch, invalidate } = usePageCache<HomeData>(
    'home:overview',
    async () => {
      const [recentResult, continueResult, recommendResult] = await Promise.allSettled([
        mediaApi.recentMixed(20),
        mediaApi.continueWatching(10),
        recommendApi.getRecommendations(12),
      ])
      return {
        recentItems: recentResult.status === 'fulfilled' ? (recentResult.value.data.data || []) : [],
        continueList: continueResult.status === 'fulfilled' ? (continueResult.value.data.data || []) : [],
        recommendations: recommendResult.status === 'fulfilled' ? (recommendResult.value.data.data || []) : [],
        allFailed: [recentResult, continueResult, recommendResult].every((r) => r.status === 'rejected'),
      }
    },
    { ttl: 30_000 },
  )

  const recentItems = data?.recentItems ?? []
  const continueList = data?.continueList ?? []
  const recommendations = data?.recommendations ?? []

  // 失败提示：仅在首次加载全部失败时展示，避免静默刷新时反复弹
  const toastRef = useRef(toast)
  const tRef = useRef(t)
  useEffect(() => { toastRef.current = toast; tRef.current = t }, [toast, t])
  useEffect(() => {
    if (data?.allFailed && !loading) {
      toastRef.current.error(tRef.current('home.loadFailed'))
    }
  }, [data?.allFailed, loading])

  // WS 事件监听：统一走 refetch(silent=true)，不触发骨架屏
  const silentRefresh = useCallback(() => {
    refetch(true)
  }, [refetch])

  useEffect(() => {
    const debouncedRefresh = () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(silentRefresh, 1000)
    }
    const handleLibraryDeleted = () => {
      invalidate()
      silentRefresh()
    }
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
  }, [on, off, invalidate, silentRefresh])

  return (
    <div className="space-y-10">
      {/* Hero Carousel — 现代化幻灯片轮播 */}
      {/* 优先使用推荐数据；推荐为空时，用最近添加的媒体作为 fallback */}
      {(recommendations.length > 0 || recentItems.length > 0) && (
        <HeroCarousel
          items={recommendations}
          fallbackItems={recentItems}
          maxItems={5}
        />
      )}

      {/* 继续观看 — 一横排横向滚动 */}
      {continueList.length > 0 && (
        <ContinueWatchingRow items={continueList} title={t('home.continueWatching')} watchedLabel={(p) => t('home.watched', { percent: String(p) })} />
      )}

      {/* 为你推荐 */}
      {recommendations.length > 0 && (
        <motion.section
          variants={staggerContainerVariants}
          initial="hidden"
          animate="visible"
        >
          <motion.h2 variants={staggerItemVariants} className="mb-5 flex items-center gap-2 font-display text-xl font-bold tracking-wide text-theme-primary">
            <Sparkles size={20} className="text-yellow-400" />
            {t('home.recommended')}
          </motion.h2>
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
                <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl bg-theme-bg-surface">
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
                        <span className="text-sm font-semibold text-white">{t('home.play')}</span>
                      </div>
                    </div>
                  </div>
                  {/* 推荐理由标签 */}
                  <span className="absolute left-2 top-2 max-w-[calc(100%-16px)] truncate rounded-md px-2 py-0.5 text-[10px] font-medium leading-tight backdrop-blur-md"
                    style={{
                      background: 'rgba(0,0,0,0.65)',
                      color: 'rgba(255,255,255,0.9)',
                      border: '1px solid rgba(255,255,255,0.15)',
                    }}
                  >
                    {item.reason}
                  </span>
                </div>
                {/* 信息区域 */}
                <div className="p-3">
                  <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon text-theme-primary">
                    {item.media.title}
                  </h3>
                  <div className="mt-1 flex items-center gap-2 text-xs text-theme-secondary">
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
        </motion.section>
      )}

      {/* 骨架屏加载状态 — 仅在首次加载、完全无数据时显示（避免与 MediaGrid 骨架叠加） */}
      {loading && recentItems.length === 0 && continueList.length === 0 && recommendations.length === 0 && (
        <motion.section
          className="space-y-10"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.3 }}
        >
          {/* 继续观看骨架屏 — 一横排 */}
          <div>
            <div className="skeleton mb-5 h-7 w-32 rounded-lg" />
            <div className="flex gap-4 overflow-hidden pb-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="w-[220px] flex-shrink-0 sm:w-[260px]">
                  <div className="skeleton aspect-video w-full rounded-xl" />
                  <div className="mt-2 space-y-2 px-1">
                    <div className="skeleton h-4 w-3/4 rounded" />
                    <div className="skeleton h-3 w-1/2 rounded" />
                  </div>
                </div>
              ))}
            </div>
          </div>
          {/* 媒体网格骨架屏 */}
          <div>
            <div className="skeleton mb-5 h-7 w-28 rounded-lg" />
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
              {Array.from({ length: 12 }).map((_, i) => (
                <div key={i} className="overflow-hidden rounded-xl" style={{ border: '1px solid var(--border-default)' }}>
                  <div className="skeleton aspect-[2/3]" />
                  <div className="space-y-2 p-3">
                    <div className="skeleton h-4 w-3/4 rounded" />
                    <div className="skeleton h-3 w-1/2 rounded" />
                  </div>
                </div>
              ))}
            </div>
          </div>
        </motion.section>
      )}

      {/* 最近添加 — 混合模式（电影+合集）。
         仅在已有数据时渲染 MediaGrid，避免"HomePage 骨架 + MediaGrid 骨架"同时出现。 */}
      {recentItems.length > 0 && (
        <MediaGrid
          mixedItems={recentItems}
          title={t('home.recentlyAdded')}
          loading={false}
        />
      )}

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
          <h3 className="font-display text-lg font-semibold tracking-wide text-theme-secondary">
            {t('home.noContent')}
          </h3>
          <p className="mt-2 text-sm text-theme-muted">
            {t('home.noContentHint')}
          </p>
        </div>
      )}
    </div>
  )
}


// ===================== 继续观看一横排组件 =====================
// 与 GenreRow 保持一致的横向滚动 + 左右箭头交互，卡片采用横向布局突出封面和进度条
function ContinueWatchingRow({
  items,
  title,
  watchedLabel,
}: {
  items: WatchHistory[]
  title: string
  watchedLabel: (percent: number) => string
}) {
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
  }, [items.length])

  const scroll = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return
    const amount = el.clientWidth * 0.7
    el.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  return (
    <motion.section
      variants={staggerContainerVariants}
      initial="hidden"
      animate="visible"
    >
      <motion.h2
        variants={staggerItemVariants}
        className="mb-5 flex items-center gap-2 font-display text-xl font-bold tracking-wide text-theme-primary"
      >
        <Clock size={20} className="text-neon" />
        {title}
      </motion.h2>

      <div className="group/row relative">
        {/* 左箭头 */}
        {canScrollLeft && (
          <button
            onClick={() => scroll('left')}
            className="absolute -left-2 top-1/2 z-10 -translate-y-1/2 rounded-full p-2 opacity-0 transition-all group-hover/row:opacity-100"
            style={{ background: 'var(--bg-surface)', boxShadow: 'var(--shadow-card)' }}
            aria-label="scroll left"
          >
            <ChevronLeft size={20} className="text-theme-primary" />
          </button>
        )}

        {/* 横向滚动容器 */}
        <div
          ref={scrollRef}
          className="scrollbar-hide flex gap-4 overflow-x-auto scroll-smooth pb-2"
        >
          {items.map((item) => {
            const percent = formatProgress(item.position, item.duration)
            const displayTitle = item.media.media_type === 'episode' && item.media.series
              ? `${item.media.series.title} S${String(item.media.season_num || 0).padStart(2, '0')}E${String(item.media.episode_num || 0).padStart(2, '0')}`
              : item.media.title
            return (
              <motion.div key={item.id} variants={staggerItemVariants} className="flex-shrink-0">
                <Link
                  to={`/play/${item.media_id}`}
                  className="media-card group block w-[220px] sm:w-[260px]"
                >
                  {/* 封面（16:9 横图，更贴近"继续观看"场景） */}
                  <div className="relative aspect-video overflow-hidden rounded-xl bg-theme-bg-surface">
                    {item.media.poster_path ? (
                      <img
                        src={streamApi.getPosterUrl(item.media_id)}
                        alt={item.media.title}
                        className="h-full w-full object-cover transition-all duration-500 group-hover:scale-105 group-hover:brightness-110"
                        loading="lazy"
                        onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                      />
                    ) : (
                      <div className="flex h-full w-full items-center justify-center text-surface-700">
                        <Play size={36} />
                      </div>
                    )}
                    {/* 悬停遮罩 + 播放按钮 */}
                    <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
                      <div className="absolute bottom-2 left-2 flex items-center gap-2">
                        <div
                          className="flex h-9 w-9 items-center justify-center rounded-full"
                          style={{
                            background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                            boxShadow: 'var(--neon-glow-shadow-md)',
                          }}
                        >
                          <Play size={16} className="ml-0.5 text-white" fill="white" />
                        </div>
                      </div>
                    </div>
                    {/* 进度百分比徽标 */}
                    <span className="absolute right-1.5 top-1.5 rounded-md bg-black/60 px-1.5 py-0.5 text-[10px] font-medium text-white backdrop-blur-sm">
                      {percent}%
                    </span>
                    {/* 底部霓虹进度条 */}
                    <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-white/10">
                      <div
                        className="h-full transition-all"
                        style={{
                          width: `${percent}%`,
                          background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                          boxShadow: 'var(--neon-glow-shadow-sm)',
                        }}
                      />
                    </div>
                  </div>

                  {/* 标题和进度文字 */}
                  <div className="px-1 py-2">
                    <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon text-theme-primary">
                      {displayTitle}
                    </h3>
                    {item.media.media_type === 'episode' && item.media.episode_title && (
                      <p className="mt-0.5 truncate text-xs text-theme-secondary">
                        {item.media.episode_title}
                      </p>
                    )}
                    <p className="mt-1 text-[11px] text-theme-tertiary">
                      {watchedLabel(percent)}
                    </p>
                  </div>
                </Link>
              </motion.div>
            )
          })}
        </div>

        {/* 右箭头 */}
        {canScrollRight && (
          <button
            onClick={() => scroll('right')}
            className="absolute -right-2 top-1/2 z-10 -translate-y-1/2 rounded-full p-2 opacity-0 transition-all group-hover/row:opacity-100"
            style={{ background: 'var(--bg-surface)', boxShadow: 'var(--shadow-card)' }}
            aria-label="scroll right"
          >
            <ChevronRight size={20} className="text-theme-primary" />
          </button>
        )}
      </div>
    </motion.section>
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
    <motion.section
      initial={{ opacity: 0, y: 16 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
    >
      <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-bold tracking-wide text-theme-primary">
        <span className="badge-accent text-xs">{genre}</span>
      </h2>

      <div className="group/row relative">
        {/* 左箭头 */}
        {canScrollLeft && (
          <button
            onClick={() => scroll('left')}
            className="absolute -left-2 top-1/2 z-10 -translate-y-1/2 rounded-full p-2 opacity-0 transition-all group-hover/row:opacity-100"
            style={{ background: 'var(--bg-surface)', boxShadow: 'var(--shadow-card)' }}
          >
            <ChevronLeft size={20} className="text-theme-primary" />
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
                <div className="relative aspect-[2/3] overflow-hidden rounded-xl bg-theme-bg-surface">
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
                  <h3 className="truncate text-xs font-medium transition-colors group-hover:text-neon text-theme-primary">
                    {media.title}
                  </h3>
                  {media.year > 0 && (
                    <p className="mt-0.5 text-[10px] text-theme-tertiary">{media.year}</p>
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
            className="absolute -right-2 top-1/2 z-10 -translate-y-1/2 rounded-full p-2 opacity-0 transition-all group-hover/row:opacity-100"
            style={{ background: 'var(--bg-surface)', boxShadow: 'var(--shadow-card)' }}
          >
            <ChevronRight size={20} className="text-theme-primary" />
          </button>
        )}
      </div>
    </motion.section>
  )
}
