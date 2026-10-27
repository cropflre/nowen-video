import { useEffect, useRef, useCallback, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { libraryApi, mediaApi, recommendApi, streamApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useTranslation } from '@/i18n'
import { usePageCache } from '@/hooks/usePageCache'
import { useAuthStore } from '@/stores/auth'
import { episodeLabel, remainingMinutes } from '@/utils/viewing'
import { formatProgress } from '@/utils/format'
import type { Library, WatchHistory, RecommendedMedia, MixedItem } from '@/types'
import MediaCard from '@/components/MediaCard'
import SuggestOne from '@/components/SuggestOne'
import HeroCarousel from '@/components/HeroCarousel'
import { Button, EmptyState, Section } from '@/components/design-system'
import { MediaArtwork, MediaRail } from '@/ui'
import { ChevronRight, Play } from 'lucide-react'

interface HomeData {
  recentItems: MixedItem[]
  continueList: WatchHistory[]
  recommendations: RecommendedMedia[]
  libraries: Library[]
  contentFailed: boolean
  historyFailed: boolean
  librariesFailed: boolean
}

interface HomeShelf {
  key: string
  title: string
  to: string
  items: MixedItem[]
}

function getContinueArtwork(item: WatchHistory): string | null {
  const media = item.media
  if (media.media_type === 'episode' && media.series_id && media.series?.backdrop_path) {
    return streamApi.getSeriesBackdropUrl(media.series_id)
  }
  if (media.media_type === 'episode' && media.series_id && media.series?.poster_path) {
    return streamApi.getSeriesPosterUrl(media.series_id)
  }
  if (media.backdrop_path) return streamApi.getBackdropUrl(item.media_id)
  if (media.poster_path) return streamApi.getPosterUrl(item.media_id)
  return null
}

function RailTitleLink({ to, label }: { to: string; label: string }) {
  return (
    <Link to={to} className="nv-home-rail-title-link">
      <span>{label}</span>
      <ChevronRight size={15} strokeWidth={1.7} aria-hidden="true" />
    </Link>
  )
}

export default function HomePage() {
  const { on, off } = useWebSocket()
  const user = useAuthStore((state) => state.user)
  const { t, locale } = useTranslation()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const { data, loading, refreshing, error, refetch, invalidate } = usePageCache<HomeData>(
    `home:family:${user?.id || 'guest'}`,
    async () => {
      const requests = [
        mediaApi.recentMixed(24),
        mediaApi.continueWatching(10),
        recommendApi.getRecommendations(5),
        libraryApi.list(),
      ] as const

      const results = await Promise.allSettled(requests)
      const [recentResult, continueResult, recommendResult, libraryResult] = results
      const recentItems = recentResult.status === 'fulfilled' ? (recentResult.value.data.data || []) : []
      return {
        recentItems,
        continueList: continueResult.status === 'fulfilled' ? (continueResult.value.data.data || []) : [],
        recommendations: recommendResult.status === 'fulfilled' ? (recommendResult.value.data.data || []) : [],
        libraries: libraryResult.status === 'fulfilled' ? (libraryResult.value.data.data || []) : [],
        contentFailed: recentResult.status === 'rejected',
        historyFailed: continueResult.status === 'rejected',
        librariesFailed: libraryResult.status === 'rejected',
      }
    },
    { ttl: 30_000 },
  )

  const recentItems = data?.recentItems ?? []
  const continueList = data?.continueList ?? []
  const recommendations = data?.recommendations ?? []
  const libraries = data?.libraries ?? []
  const watchStateByMediaId = useMemo(() => Object.fromEntries(
    continueList.map((item) => [item.media_id, { position: item.position, duration: item.duration }]),
  ), [continueList])

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

  useEffect(() => {
    if (!user?.child_mode) return
    const refresh = () => { if (document.visibilityState === 'visible') void silentRefresh() }
    const timer = window.setInterval(refresh, 30_000)
    window.addEventListener('focus', refresh)
    return () => { window.clearInterval(timer); window.removeEventListener('focus', refresh) }
  }, [user?.child_mode, silentRefresh])

  const noContent = recentItems.length === 0 && continueList.length === 0 && recommendations.length === 0
  const loadFailed = Boolean(error || data?.contentFailed)
  const isAdmin = user?.role === 'admin'
  const emptyHint = user?.child_mode ? t('family.emptyChild') : !isAdmin ? t('family.emptyViewer')
    : libraries.length > 0 ? t('family.emptyLibrary') : t('family.emptyAdmin')

  return (
    <div className="nv-home-page">
      <div className="nv-home-content-flow">
        {user?.child_mode && <p className="text-sm text-[var(--nv-text-tertiary)]">儿童空间 · 只显示家长已确认的影片</p>}
        {continueList.length > 0 && (
          <ContinueWatchingRow items={continueList} title={t('home.continueWatching')} locale={locale}
            remainingLabel={(minutes) => minutes === null ? t('family.resume') : t('family.remaining', { minutes })} />
        )}
        {!loading && data?.historyFailed && (
          <div className="flex flex-wrap items-center gap-3 text-sm" role="status">
            <span>{t('family.historyFailed')}</span>
            <Button type="button" variant="secondary" disabled={refreshing} onClick={() => void refetch()}>{t('family.retry')}</Button>
          </div>
        )}
        {(recommendations.length > 0 || recentItems.length > 0) && (
          <section aria-label={t('family.today')}>
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3"><h2 className="text-lg font-semibold">{t('family.today')}</h2><SuggestOne key={user?.id} items={recentItems} /></div>
            <HeroCarousel items={recommendations} fallbackItems={recentItems} maxItems={5} watchStateByMediaId={watchStateByMediaId} />
          </section>
        )}
        {loading && noContent && (
          <div className="nv-section-stack">
            <HomeRailSkeleton title={t('home.continueWatching')} landscape />
            <HomeRailSkeleton title={t('home.recentlyAdded')} />
          </div>
        )}
        {recentItems.length > 0 && (
          <HomePosterShelf shelf={{ key: 'recent', title: t('home.recentlyAdded'), to: '/browse?sort=created_desc', items: recentItems }} />
        )}
        {!loading && noContent && (
          <EmptyState icon={<Play size={22} aria-hidden="true" />}
            title={loadFailed ? t('family.loadFailed') : t('family.emptyTitle')}
            description={loadFailed ? t('family.loadFailedHint') : emptyHint}
            action={loadFailed || data?.librariesFailed ? (
              <Button type="button" variant="primary" disabled={refreshing} onClick={() => void refetch()}>{t('family.retry')}</Button>
            ) : isAdmin ? (
              <Link className="nv-button" data-variant="primary" to="/admin#library">{libraries.length ? t('family.manageLibrary') : t('family.addFolder')}</Link>
            ) : user?.child_mode ? <Button disabled={refreshing} onClick={() => void refetch()}>再看看</Button> : undefined} />
        )}
        {!loading && !noContent && loadFailed && (
          <div className="flex flex-wrap items-center gap-3 text-sm" role="status">
            <span>{t('family.partialFailure')}</span>
            <Button type="button" variant="secondary" disabled={refreshing} onClick={() => void refetch()}>{t('family.retry')}</Button>
          </div>
        )}
      </div>
    </div>
  )
}

function ContinueWatchingRow({
  items,
  title,
  remainingLabel,
  locale,
}: {
  items: WatchHistory[]
  title: string
  remainingLabel: (minutes: number | null) => string
  locale: string
}) {
  return (
    <MediaRail
      title={<RailTitleLink to="/history" label={title} />}
      ariaLabel={title}
      itemCount={items.length}
      className="nv-home-continue-rail"
    >
      {items.map((item) => {
        const percent = formatProgress(item.position, item.duration)
        const displayTitle = item.media.media_type === 'episode' && item.media.series
          ? `${item.media.series.title} · ${episodeLabel(item.media.season_num, item.media.episode_num, locale)}`
          : item.media.title
        const artworkUrl = getContinueArtwork(item)

        return (
          <article key={item.id} className="nv-continue-card group flex-shrink-0">
            <Link to={`/play/${item.media_id}`} className="block" aria-label={`继续播放 ${displayTitle}`}>
              <MediaArtwork
                src={artworkUrl}
                alt=""
                ratio="landscape"
                className="nv-continue-artwork"
                imageClassName="transition-[filter,transform] duration-200 group-hover:scale-[1.01] group-hover:brightness-[.9]"
                fallback={<Play size={22} aria-hidden="true" />}
              >
                <div className="nv-media-card-progress"><span style={{ width: `${percent}%` }} /></div>
              </MediaArtwork>

              <div className="nv-continue-copy">
                <h3 className="nv-media-card-title">{displayTitle}</h3>
                <p className="nv-continue-progress-label">{remainingLabel(remainingMinutes(item.position, item.duration))}</p>
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
        {Array.from({ length: landscape ? 6 : 9 }).map((_, index) => (
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

function HomePosterShelf({ shelf }: { shelf: HomeShelf }) {
  return (
    <MediaRail
      title={<RailTitleLink to={shelf.to} label={shelf.title} />}
      ariaLabel={shelf.title}
      itemCount={shelf.items.length}
      className="nv-home-compact-shelf"
    >
      {shelf.items.slice(0, 16).map((item) => {
        const media = item.type === 'movie' ? item.media : item.series
        if (!media) return null
        return (
          <div key={`${shelf.key}-${item.type}-${media.id}`} className="nv-home-shelf-poster-slot flex-shrink-0">
            {item.type === 'series' && item.series
              ? <MediaCard series={item.series} showBadges={false} />
              : item.media
                ? <MediaCard media={item.media} showBadges={false} />
                : null}
          </div>
        )
      })}
    </MediaRail>
  )
}
