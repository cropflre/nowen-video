import { useEffect, useState, useCallback, useRef } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { mediaApi, recommendApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { formatProgress } from '@/utils/format'
import type { WatchHistory, RecommendedMedia, MixedItem } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import { Play, Clock, Sparkles, ChevronLeft, ChevronRight, Star } from 'lucide-react'
import { streamApi } from '@/api'
import { motion } from 'framer-motion'
import { staggerContainerVariants, staggerItemVariants } from '@/lib/motion'
import HeroCarousel from '@/components/HeroCarousel'

export default function HomePage() {
  const [recentItems, setRecentItems] = useState<MixedItem[]>([])
  const [continueList, setContinueList] = useState<WatchHistory[]>([])
  const [recommendations, setRecommendations] = useState<RecommendedMedia[]>([])
  const [loading, setLoading] = useState(true)
  const location = useLocation()
  const { on, off } = useWebSocket()
  const toast = useToast()
  const { t } = useTranslation()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // 数据加载函数（可复用）— 使用 allSettled 避免单个接口失败导致全部丢失
  const fetchData = useCallback(async (showLoading = false) => {
    if (showLoading) setLoading(true)
    try {
      const [recentResult, continueResult, recommendResult] = await Promise.allSettled([
        mediaApi.recentMixed(20),
        mediaApi.continueWatching(10),
        recommendApi.getRecommendations(12),
      ])

      if (recentResult.status === 'fulfilled') {
        setRecentItems(recentResult.value.data.data || [])
      }
      if (continueResult.status === 'fulfilled') {
        setContinueList(continueResult.value.data.data || [])
      }
      if (recommendResult.status === 'fulfilled') {
        setRecommendations(recommendResult.value.data.data || [])
      }

      // 如果所有请求都失败，才显示错误提示
      const allFailed = [recentResult, continueResult, recommendResult].every(r => r.status === 'rejected')
      if (allFailed) {
        toast.error(t('home.loadFailed'))
      }
    } catch {
      toast.error(t('home.loadFailed'))
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
      {/* Hero Carousel — 现代化幻灯片轮播 */}
      {/* 优先使用推荐数据；推荐为空时，用最近添加的媒体作为 fallback */}
      {(recommendations.length > 0 || recentItems.length > 0) && (
        <HeroCarousel
          items={recommendations}
          fallbackItems={recentItems}
          maxItems={5}
        />
      )}

      {/* 继续观看 */}
      {continueList.length > 0 && (
        <motion.section
          variants={staggerContainerVariants}
          initial="hidden"
          animate="visible"
        >
          <motion.h2 variants={staggerItemVariants} className="mb-5 flex items-center gap-2 font-display text-xl font-bold tracking-wide text-theme-primary">
            <Clock size={20} className="text-neon" />
            {t('home.continueWatching')}
          </motion.h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {continueList.map((item) => (
              <motion.div key={item.id} variants={staggerItemVariants}>
              <Link
                key={item.id}
                to={`/play/${item.media_id}`}
                className="glass-panel-subtle group flex gap-3 rounded-xl p-3 transition-all duration-300 hover:border-neon-blue/20 hover:shadow-card-hover"
              >
                {/* 缩略图 */}
                <div className="relative h-20 w-32 flex-shrink-0 overflow-hidden rounded-lg bg-theme-bg-surface">
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
                  <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon text-theme-primary">
                    {item.media.title}
                  </h3>
                  <p className="mt-1 text-xs text-theme-tertiary">
                    {t('home.watched', { percent: String(formatProgress(item.position, item.duration)) })}
                  </p>
                </div>
              </Link>
              </motion.div>
            ))}
          </div>
        </motion.section>
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
                  <span className="badge-accent absolute left-2 top-2">
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

      {/* 骨架屏加载状态 */}
      {loading && recentItems.length === 0 && (
        <motion.section
          className="space-y-10"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.3 }}
        >
          {/* 继续观看骨架屏 */}
          <div>
            <div className="skeleton mb-5 h-7 w-32 rounded-lg" />
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="glass-panel-subtle flex gap-3 rounded-xl p-3">
                  <div className="skeleton h-20 w-32 flex-shrink-0 rounded-lg" />
                  <div className="flex-1 space-y-2 py-1">
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

      {/* 最近添加 — 混合模式（电影+合集） */}
      <MediaGrid
        mixedItems={recentItems}
        title={t('home.recentlyAdded')}
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
