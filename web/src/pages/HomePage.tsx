import { useEffect, useState, useRef, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { mediaApi, recommendApi, streamApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { usePageCache } from '@/hooks/usePageCache'
import { formatProgress } from '@/utils/format'
import type { WatchHistory, RecommendedMedia, MixedItem } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import MediaCard from '@/components/MediaCard'
import HeroCarousel from '@/components/HeroCarousel'
import { Button, EmptyState, Section, Tag } from '@/components/design-system'
import { Play, Clock, Sparkles, ChevronLeft, ChevronRight } from 'lucide-react'
import { motion } from 'framer-motion'
import { staggerContainerVariants, staggerItemVariants } from '@/lib/motion'

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

  const toastRef = useRef(toast)
  const tRef = useRef(t)
  useEffect(() => { toastRef.current = toast; tRef.current = t }, [toast, t])
  useEffect(() => {
    if (data?.allFailed && !loading) {
      toastRef.current.error(tRef.current('home.loadFailed'))
    }
  }, [data?.allFailed, loading])

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
    <div className="nv-section-stack">
      {(recommendations.length > 0 || recentItems.length > 0) && (
        <HeroCarousel
          items={recommendations}
          fallbackItems={recentItems}
          maxItems={5}
        />
      )}

      {continueList.length > 0 && (
        <ContinueWatchingRow
          items={continueList}
          title={t('home.continueWatching')}
          watchedLabel={(p) => t('home.watched', { percent: String(p) })}
        />
      )}

      {recommendations.length > 0 && (
        <Section
          title={(
            <span className="inline-flex items-center gap-2">
              <Sparkles size={18} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
              {t('home.recommended')}
            </span>
          )}
        >
          <motion.div
            className="nv-media-grid"
            variants={staggerContainerVariants}
            initial="hidden"
            animate="visible"
          >
            {recommendations.map((item) => (
              <motion.div key={item.media.id} variants={staggerItemVariants}>
                <MediaCard media={item.media} eyebrow={item.reason} />
              </motion.div>
            ))}
          </motion.div>
        </Section>
      )}

      {loading && recentItems.length === 0 && continueList.length === 0 && recommendations.length === 0 && (
        <motion.div
          className="nv-section-stack"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.2 }}
        >
          <Section title={t('home.continueWatching')}>
            <div className="flex gap-[var(--nv-grid-gap)] overflow-hidden pb-2">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="w-[220px] flex-shrink-0 sm:w-[260px]">
                  <div className="skeleton aspect-video w-full rounded-[var(--nv-radius-card)]" />
                  <div className="mt-2 space-y-2 px-1">
                    <div className="skeleton h-4 w-3/4" />
                    <div className="skeleton h-3 w-1/2" />
                  </div>
                </div>
              ))}
            </div>
          </Section>
          <MediaGrid title={t('home.recentlyAdded')} loading />
        </motion.div>
      )}

      {recentItems.length > 0 && (
        <MediaGrid
          mixedItems={recentItems}
          title={t('home.recentlyAdded')}
          loading={false}
        />
      )}

      {!loading && recentItems.length > 0 && (
        <GenreRows items={recentItems} />
      )}

      {!loading && recentItems.length === 0 && continueList.length === 0 && (
        <EmptyState
          icon={<Play size={26} aria-hidden="true" />}
          title={t('home.noContent')}
          description={t('home.noContentHint')}
        />
      )}
    </div>
  )
}

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

  const updateScrollState = useCallback(() => {
    const el = scrollRef.current
    if (!el) return
    setCanScrollLeft(el.scrollLeft > 10)
    setCanScrollRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 10)
  }, [])

  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    el.addEventListener('scroll', updateScrollState, { passive: true })
    updateScrollState()
    return () => el.removeEventListener('scroll', updateScrollState)
  }, [items.length, updateScrollState])

  const scroll = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return
    const amount = el.clientWidth * 0.72
    el.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  return (
    <Section
      title={(
        <span className="inline-flex items-center gap-2">
          <Clock size={18} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
          {title}
        </span>
      )}
    >
      <div className="group/row relative">
        {canScrollLeft && (
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={() => scroll('left')}
            className="absolute -left-2 top-[42%] z-30 -translate-y-1/2 opacity-0 shadow-[var(--nv-shadow-card)] group-hover/row:opacity-100 focus:opacity-100"
            aria-label="向左滚动"
          >
            <ChevronLeft size={18} aria-hidden="true" />
          </Button>
        )}

        <div
          ref={scrollRef}
          className="scrollbar-hide flex gap-[var(--nv-grid-gap)] overflow-x-auto scroll-smooth pb-2"
        >
          {items.map((item) => {
            const percent = formatProgress(item.position, item.duration)
            const displayTitle = item.media.media_type === 'episode' && item.media.series
              ? `${item.media.series.title} S${String(item.media.season_num || 0).padStart(2, '0')}E${String(item.media.episode_num || 0).padStart(2, '0')}`
              : item.media.title

            return (
              <article
                key={item.id}
                className="nv-media-card group w-[220px] flex-shrink-0 sm:w-[260px]"
              >
                <Link to={`/play/${item.media_id}`} className="block" aria-label={`继续播放 ${displayTitle}`}>
                  <div className="relative aspect-video overflow-hidden rounded-[var(--nv-radius-card)] bg-[var(--nv-bg-surface-soft)]">
                    {item.media.poster_path ? (
                      <img
                        src={streamApi.getPosterUrl(item.media_id)}
                        alt=""
                        className="h-full w-full object-cover transition-[transform,filter] duration-300 ease-out group-hover:scale-[1.025] group-hover:brightness-90"
                        loading="lazy"
                        onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                      />
                    ) : (
                      <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
                        <Play size={30} aria-hidden="true" />
                      </div>
                    )}

                    <div className="absolute inset-0 bg-gradient-to-t from-black/65 via-transparent to-transparent opacity-0 transition-opacity duration-200 group-hover:opacity-100" />
                    <div className="absolute bottom-3 left-3 flex h-8 w-8 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)] opacity-0 transition-opacity duration-200 group-hover:opacity-100">
                      <Play size={14} fill="currentColor" aria-hidden="true" />
                    </div>

                    <Tag tone="quality" className="absolute right-2 top-2">
                      {percent}%
                    </Tag>

                    <div className="absolute bottom-0 left-0 right-0 h-1 bg-black/25">
                      <div
                        className="h-full bg-[var(--nv-action-primary)] transition-[width] duration-200"
                        style={{ width: `${percent}%` }}
                      />
                    </div>
                  </div>

                  <div className="px-1 py-2">
                    <h3 className="nv-media-card-title">{displayTitle}</h3>
                    {item.media.media_type === 'episode' && item.media.episode_title && (
                      <p className="mt-0.5 truncate text-xs text-[var(--nv-text-secondary)]">
                        {item.media.episode_title}
                      </p>
                    )}
                    <p className="mt-1 text-[var(--nv-type-caption)] text-[var(--nv-text-tertiary)]">
                      {watchedLabel(percent)}
                    </p>
                  </div>
                </Link>
              </article>
            )
          })}
        </div>

        {canScrollRight && (
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={() => scroll('right')}
            className="absolute -right-2 top-[42%] z-30 -translate-y-1/2 opacity-0 shadow-[var(--nv-shadow-card)] group-hover/row:opacity-100 focus:opacity-100"
            aria-label="向右滚动"
          >
            <ChevronRight size={18} aria-hidden="true" />
          </Button>
        )}
      </div>
    </Section>
  )
}

function GenreRows({ items }: { items: MixedItem[] }) {
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

  const genreEntries = Array.from(genreMap.entries())
    .filter(([, list]) => list.length >= 3)
    .sort((a, b) => b[1].length - a[1].length)
    .slice(0, 5)

  if (genreEntries.length === 0) return null

  return (
    <div className="nv-section-stack">
      {genreEntries.map(([genre, list]) => (
        <GenreRow key={genre} genre={genre} items={list.slice(0, 20)} />
      ))}
    </div>
  )
}

function GenreRow({ genre, items }: { genre: string; items: MixedItem[] }) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(true)

  const updateScrollState = useCallback(() => {
    const el = scrollRef.current
    if (!el) return
    setCanScrollLeft(el.scrollLeft > 10)
    setCanScrollRight(el.scrollLeft < el.scrollWidth - el.clientWidth - 10)
  }, [])

  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    el.addEventListener('scroll', updateScrollState, { passive: true })
    updateScrollState()
    return () => el.removeEventListener('scroll', updateScrollState)
  }, [items.length, updateScrollState])

  const scroll = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return
    const amount = el.clientWidth * 0.72
    el.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  return (
    <Section title={genre}>
      <div className="group/row relative">
        {canScrollLeft && (
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={() => scroll('left')}
            className="absolute -left-2 top-[42%] z-30 -translate-y-1/2 opacity-0 shadow-[var(--nv-shadow-card)] group-hover/row:opacity-100 focus:opacity-100"
            aria-label={`${genre} 向左滚动`}
          >
            <ChevronLeft size={18} aria-hidden="true" />
          </Button>
        )}

        <div
          ref={scrollRef}
          className="scrollbar-hide flex gap-[var(--nv-grid-gap)] overflow-x-auto scroll-smooth pb-2"
        >
          {items.map((item) => {
            const media = item.type === 'movie' ? item.media : item.series
            if (!media) return null
            return (
              <div key={`${item.type}-${media.id}`} className="w-[140px] flex-shrink-0 sm:w-[160px]">
                {item.type === 'series' && item.series
                  ? <MediaCard series={item.series} />
                  : item.media
                    ? <MediaCard media={item.media} />
                    : null}
              </div>
            )
          })}
        </div>

        {canScrollRight && (
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={() => scroll('right')}
            className="absolute -right-2 top-[42%] z-30 -translate-y-1/2 opacity-0 shadow-[var(--nv-shadow-card)] group-hover/row:opacity-100 focus:opacity-100"
            aria-label={`${genre} 向右滚动`}
          >
            <ChevronRight size={18} aria-hidden="true" />
          </Button>
        )}
      </div>
    </Section>
  )
}
