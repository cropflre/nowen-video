import { useEffect, useRef, useCallback, useMemo } from 'react'
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
import { EmptyState, Section, Tag } from '@/components/design-system'
import { MediaArtwork, MediaRail } from '@/ui'
import { ChevronRight, Clock, Play, Sparkles } from 'lucide-react'

const HOME_GENRES = ['动画', '喜剧', '冒险', '家庭'] as const

type HomeGenre = typeof HOME_GENRES[number]

interface HomeData {
  recentItems: MixedItem[]
  continueList: WatchHistory[]
  recommendations: RecommendedMedia[]
  genreItems: Partial<Record<HomeGenre, MixedItem[]>>
  allFailed: boolean
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

function RailAction({ to, label = '查看全部' }: { to: string; label?: string }) {
  return (
    <Link to={to} className="nv-home-rail-action">
      {label}
      <ChevronRight size={14} aria-hidden="true" />
    </Link>
  )
}

function itemMatchesGenre(item: MixedItem, genre: string) {
  const media = item.type === 'movie' ? item.media : item.series
  if (!media?.genres) return false
  return media.genres
    .split(',')
    .map((value) => value.trim())
    .filter(Boolean)
    .some((value) => value === genre || value.includes(genre))
}

export default function HomePage() {
  const { on, off } = useWebSocket()
  const toast = useToast()
  const { t } = useTranslation()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const { data, loading, refetch, invalidate } = usePageCache<HomeData>(
    'home:overview:v2',
    async () => {
      const requests = [
        mediaApi.recentMixed(24),
        mediaApi.continueWatching(10),
        recommendApi.getRecommendations(12),
        ...HOME_GENRES.map((genre) => mediaApi.listMixed({
          page: 1,
          size: 16,
          genre,
          sort: 'added',
          order: 'desc',
        })),
      ] as const

      const results = await Promise.allSettled(requests)
      const [recentResult, continueResult, recommendResult, ...genreResults] = results
      const recentItems = recentResult.status === 'fulfilled' ? (recentResult.value.data.data || []) : []
      const genreItems: Partial<Record<HomeGenre, MixedItem[]>> = {}

      HOME_GENRES.forEach((genre, index) => {
        const result = genreResults[index]
        const serverItems = result?.status === 'fulfilled' ? (result.value.data.data || []) : []
        genreItems[genre] = serverItems.length > 0
          ? serverItems
          : recentItems.filter((item) => itemMatchesGenre(item, genre))
      })

      return {
        recentItems,
        continueList: continueResult.status === 'fulfilled' ? (continueResult.value.data.data || []) : [],
        recommendations: recommendResult.status === 'fulfilled' ? (recommendResult.value.data.data || []) : [],
        genreItems,
        allFailed: [recentResult, continueResult, recommendResult].every((result) => result.status === 'rejected'),
      }
    },
    { ttl: 30_000 },
  )

  const recentItems = data?.recentItems ?? []
  const continueList = data?.continueList ?? []
  const recommendations = data?.recommendations ?? []
  const genreItems = data?.genreItems ?? {}
  const watchStateByMediaId = useMemo(() => Object.fromEntries(
    continueList.map((item) => [item.media_id, { position: item.position, duration: item.duration }]),
  ), [continueList])

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
        <HeroCarousel
          items={recommendations}
          fallbackItems={recentItems}
          maxItems={5}
          watchStateByMediaId={watchStateByMediaId}
        />
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
              <Sparkles size={16} aria-hidden="true" />
              {t('home.recommended')}
            </span>
          )}
          ariaLabel={t('home.recommended')}
          itemCount={recommendations.length}
          action={<RailAction to="/browse" />}
        >
          {recommendations.map((item) => (
            <div key={item.media.id} className="nv-home-recommendation-slot flex-shrink-0">
              <MediaCard media={item.media} variant="landscape" showBadges={false} />
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

      {!loading && recentItems.length > 0 && (
        <HomeShelfGrid
          items={recentItems}
          genreItems={genreItems}
          recentTitle={t('home.recentlyAdded')}
        />
      )}

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
          <Clock size={16} aria-hidden="true" />
          {title}
        </span>
      )}
      ariaLabel={title}
      itemCount={items.length}
      action={<RailAction to="/history" />}
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
              <MediaArtwork
                src={artworkUrl}
                alt=""
                ratio="landscape"
                className="nv-continue-artwork"
                imageClassName="transition-[filter,transform] duration-200 group-hover:scale-[1.015] group-hover:brightness-[.82]"
                fallback={<Play size={24} aria-hidden="true" />}
              >
                <div className="nv-continue-overlay absolute inset-0 z-10 grid place-items-center opacity-0 transition-opacity duration-200 group-hover:opacity-100">
                  <span className="grid h-9 w-9 place-items-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]">
                    <Play size={14} fill="currentColor" aria-hidden="true" />
                  </span>
                </div>
                <Tag tone="quality" className="absolute right-2 top-2 z-20">{percent}%</Tag>
                <div className="nv-media-card-progress">
                  <span style={{ width: `${percent}%` }} />
                </div>
              </MediaArtwork>

              <div className="nv-continue-copy">
                <h3 className="nv-media-card-title">{displayTitle}</h3>
                {item.media.media_type === 'episode' && item.media.episode_title && (
                  <p className="nv-continue-episode-title">{item.media.episode_title}</p>
                )}
                <p className="nv-continue-progress-label">{watchedLabel(percent)}</p>
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

function HomeShelfGrid({
  items,
  genreItems,
  recentTitle,
}: {
  items: MixedItem[]
  genreItems: Partial<Record<HomeGenre, MixedItem[]>>
  recentTitle: string
}) {
  const shelves: HomeShelf[] = [
    {
      key: 'recent',
      title: recentTitle,
      to: '/browse?sort=created_desc',
      items,
    },
    ...HOME_GENRES.map((genre) => ({
      key: `genre-${genre}`,
      title: genre,
      to: `/browse?genres=${encodeURIComponent(genre)}`,
      items: genreItems[genre] || [],
    })).filter((shelf) => shelf.items.length > 0),
  ]

  return (
    <div className="nv-home-shelf-grid" aria-label="首页分类内容">
      {shelves.map((shelf) => (
        <HomePosterShelf key={shelf.key} shelf={shelf} />
      ))}
    </div>
  )
}

function HomePosterShelf({ shelf }: { shelf: HomeShelf }) {
  return (
    <MediaRail
      title={shelf.title}
      ariaLabel={shelf.title}
      itemCount={shelf.items.length}
      action={<RailAction to={shelf.to} label="更多" />}
      className="nv-home-compact-shelf"
    >
      {shelf.items.slice(0, 16).map((item) => {
        const media = item.type === 'movie' ? item.media : item.series
        if (!media) return null
        return (
          <div key={`${shelf.key}-${item.type}-${media.id}`} className="nv-home-shelf-poster-slot flex-shrink-0">
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
