import { useEffect, useState, useRef, useCallback, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { mediaApi, recommendApi, streamApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { usePageCache } from '@/hooks/usePageCache'
import { formatProgress } from '@/utils/format'
import type { WatchHistory, RecommendedMedia, MixedItem } from '@/types'
import MediaCard from '@/components/MediaCard'
import HeroCarousel from '@/components/HeroCarousel'
import { Button, EmptyState, Section, Tag } from '@/components/design-system'
import { Play, Clock, Sparkles, ChevronLeft, ChevronRight } from 'lucide-react'

interface HomeData {
  recentItems: MixedItem[]
  continueList: WatchHistory[]
  recommendations: RecommendedMedia[]
  allFailed: boolean
}

function getContinueArtwork(item: WatchHistory): string | null {
  const media = item.media
  if (media.media_type === 'episode' && media.series_id && media.series?.backdrop_path) {
    return streamApi.getSeriesBackdropUrl(media.series_id)
  }
  if (media.media_type === 'episode' && media.series_id && media.series?.poster_path) {
    return streamApi.getSeriesPosterUrl(media.series_id)
  }
  if (media.poster_path) return streamApi.getPosterUrl(item.media_id)
  return null
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
        allFailed: [recentResult, continueResult, recommendResult].every((result) => result.status === 'rejected'),
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
    if (data?.allFailed && !loading) toastRef.current.error(tRef.current('home.loadFailed'))
  }, [data?.allFailed, loading])

  const silentRefresh = useCallback(() => refetch(true), [refetch])

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
    <div className="nv-home-page nv-section-stack">
      {(recommendations.length > 0 || recentItems.length > 0) && (
        <HeroCarousel items={recommendations} fallbackItems={recentItems} maxItems={5} />
      )}

      {continueList.length > 0 && (
        <ContinueWatchingRow
          items={continueList}
          title={t('home.continueWatching')}
          watchedLabel={(percent) => t('home.watched', { percent: String(percent) })}
        />
      )}

      {recommendations.length > 0 && (
        <MediaRail
          title={(
            <span className="inline-flex items-center gap-2">
              <Sparkles size={16} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
              {t('home.recommended')}
            </span>
          )}
          ariaLabel={t('home.recommended')}
          itemCount={recommendations.length}
        >
          {recommendations.map((item) => (
            <div key={item.media.id} className="nv-home-poster-slot flex-shrink-0">
              <MediaCard media={item.media} eyebrow={item.reason} />
            </div>
          ))}
        </MediaRail>
      )}

      {loading && recentItems.length === 0 && continueList.length === 0 && recommendations.length === 0 && (
        <div className="nv-section-stack">
          <HomeRailSkeleton title={t('home.continueWatching')} landscape />
          <HomeRailSkeleton title={t('home.recentlyAdded')} />
        </div>
      )}

      {recentItems.length > 0 && (
        <MediaRail title={t('home.recentlyAdded')} ariaLabel={t('home.recentlyAdded')} itemCount={recentItems.length}>
          {recentItems.map((item) => {
            const media = item.type === 'movie' ? item.media : item.series
            if (!media) return null
            return (
              <div key={`${item.type}-${media.id}`} className="nv-home-poster-slot flex-shrink-0">
                {item.type === 'series' && item.series
                  ? <MediaCard series={item.series} />
                  : item.media
                    ? <MediaCard media={item.media} />
                    : null}
              </div>
            )
          })}
        </MediaRail>
      )}

      {!loading && recentItems.length > 0 && <GenreRows items={recentItems} />}

      {!loading && recentItems.length === 0 && continueList.length === 0 && (
        <EmptyState
          icon={<Play size={22} aria-hidden="true" />}
          title={t('home.noContent')}
          description={t('home.noContentHint')}
        />
      )}
    </div>
  )
}

function MediaRail({
  title,
  ariaLabel,
  itemCount,
  children,
}: {
  title: ReactNode
  ariaLabel: string
  itemCount: number
  children: ReactNode
}) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)

  const updateScrollState = useCallback(() => {
    const element = scrollRef.current
    if (!element) return
    setCanScrollLeft(element.scrollLeft > 10)
    setCanScrollRight(element.scrollLeft < element.scrollWidth - element.clientWidth - 10)
  }, [])

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return
    element.addEventListener('scroll', updateScrollState, { passive: true })
    window.addEventListener('resize', updateScrollState)
    const frame = window.requestAnimationFrame(updateScrollState)
    return () => {
      window.cancelAnimationFrame(frame)
      element.removeEventListener('scroll', updateScrollState)
      window.removeEventListener('resize', updateScrollState)
    }
  }, [itemCount, updateScrollState])

  const scroll = (direction: 'left' | 'right') => {
    const element = scrollRef.current
    if (!element) return
    const amount = element.clientWidth * .78
    element.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  return (
    <Section title={title}>
      <div className="nv-home-rail relative">
        {canScrollLeft && (
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={() => scroll('left')}
            className="nv-home-rail-arrow nv-home-rail-arrow-left absolute left-1 top-[42%] z-30 -translate-y-1/2 opacity-0"
            aria-label={`${ariaLabel} 向左滚动`}
          >
            <ChevronLeft size={17} aria-hidden="true" />
          </Button>
        )}

        <div
          ref={scrollRef}
          className="nv-home-media-rail scrollbar-hide flex gap-[var(--nv-grid-gap-x)] overflow-x-auto scroll-smooth pb-3 pt-1"
          aria-label={ariaLabel}
        >
          {children}
        </div>

        {canScrollRight && (
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={() => scroll('right')}
            className="nv-home-rail-arrow nv-home-rail-arrow-right absolute right-1 top-[42%] z-30 -translate-y-1/2 opacity-0"
            aria-label={`${ariaLabel} 向右滚动`}
          >
            <ChevronRight size={17} aria-hidden="true" />
          </Button>
        )}
      </div>
    </Section>
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
  return (
    <MediaRail
      title={(
        <span className="inline-flex items-center gap-2">
          <Clock size={16} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
          {title}
        </span>
      )}
      ariaLabel={title}
      itemCount={items.length}
    >
      {items.map((item) => {
        const percent = formatProgress(item.position, item.duration)
        const displayTitle = item.media.media_type === 'episode' && item.media.series
          ? `${item.media.series.title} S${String(item.media.season_num || 0).padStart(2, '0')}E${String(item.media.episode_num || 0).padStart(2, '0')}`
          : item.media.title
        const artworkUrl = getContinueArtwork(item)

        return (
          <article key={item.id} className="nv-continue-card group flex-shrink-0">
            <Link to={`/play/${item.media_id}`} className="block" aria-label={`继续播放 ${displayTitle}`}>
              <div className="nv-continue-artwork relative aspect-video overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-poster)] transition-[transform,box-shadow] duration-200 group-hover:-translate-y-[3px] group-hover:shadow-[var(--nv-shadow-card-hover)]">
                {artworkUrl ? (
                  <img
                    src={artworkUrl}
                    alt=""
                    className="h-full w-full object-cover transition-[filter] duration-200 group-hover:brightness-[.82]"
                    loading="lazy"
                    onError={(event) => { (event.currentTarget as HTMLImageElement).style.display = 'none' }}
                  />
                ) : (
                  <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
                    <Play size={26} aria-hidden="true" />
                  </div>
                )}
                <div className="nv-continue-overlay absolute inset-0 grid place-items-center opacity-0 transition-opacity duration-200 group-hover:opacity-100">
                  <span className="grid h-9 w-9 place-items-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]">
                    <Play size={14} fill="currentColor" aria-hidden="true" />
                  </span>
                </div>
                <Tag tone="quality" className="absolute right-2 top-2">{percent}%</Tag>
                <div className="nv-media-card-progress">
                  <span style={{ width: `${percent}%` }} />
                </div>
              </div>

              <div className="py-2">
                <h3 className="nv-media-card-title">{displayTitle}</h3>
                {item.media.media_type === 'episode' && item.media.episode_title && (
                  <p className="mt-0.5 truncate text-xs text-[var(--nv-text-secondary)]">{item.media.episode_title}</p>
                )}
                <p className="mt-1 text-[var(--nv-type-caption)] text-[var(--nv-text-tertiary)]">{watchedLabel(percent)}</p>
              </div>
            </Link>
          </article>
        )
      })}
    </MediaRail>
  )
}

function HomeRailSkeleton({ title, landscape = false }: { title: string; landscape?: boolean }) {
  return (
    <Section title={title}>
      <div className="flex gap-[var(--nv-grid-gap-x)] overflow-hidden pb-3 pt-1">
        {Array.from({ length: landscape ? 5 : 8 }).map((_, index) => (
          <div key={index} className={landscape ? 'nv-continue-card flex-shrink-0' : 'nv-home-poster-slot flex-shrink-0'}>
            <div className={`skeleton w-full rounded-[var(--nv-radius-card)] ${landscape ? 'aspect-video' : 'aspect-[2/3]'}`} />
            <div className="mt-2 space-y-2">
              <div className="skeleton h-3 w-3/4" />
              <div className="skeleton h-2.5 w-1/2" />
            </div>
          </div>
        ))}
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
      const value = genre.trim()
      if (!value) return
      if (!genreMap.has(value)) genreMap.set(value, [])
      genreMap.get(value)!.push(item)
    })
  })

  const genreEntries = Array.from(genreMap.entries())
    .filter(([, list]) => list.length >= 3)
    .sort((a, b) => b[1].length - a[1].length)
    .slice(0, 4)

  if (genreEntries.length === 0) return null

  return (
    <div className="nv-home-genre-stack nv-section-stack">
      {genreEntries.map(([genre, list]) => (
        <GenreRow key={genre} genre={genre} items={list.slice(0, 20)} />
      ))}
    </div>
  )
}

function GenreRow({ genre, items }: { genre: string; items: MixedItem[] }) {
  return (
    <MediaRail title={genre} ariaLabel={genre} itemCount={items.length}>
      {items.map((item) => {
        const media = item.type === 'movie' ? item.media : item.series
        if (!media) return null
        return (
          <div key={`${item.type}-${media.id}`} className="nv-home-poster-slot nv-home-genre-slot flex-shrink-0">
            {item.type === 'series' && item.series
              ? <MediaCard series={item.series} />
              : item.media
                ? <MediaCard media={item.media} />
                : null}
          </div>
        )
      })}
    </MediaRail>
  )
}
