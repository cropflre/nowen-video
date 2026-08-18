import { useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, ChevronLeft, ChevronRight, Clock, GalleryHorizontal, LayoutList, Play } from 'lucide-react'
import type { Media, SeasonInfo, WatchHistory } from '@/types'
import { streamApi } from '@/api'
import { Button, EmptyState, Tag } from '@/components/design-system'
import Pagination from '@/components/Pagination'
import { usePagination } from '@/hooks/usePagination'
import { MediaArtwork } from '@/ui'

interface SeriesEpisodeBrowserProps {
  seasons: SeasonInfo[]
  seriesTitle: string
  historyMap: Record<string, WatchHistory>
  posterVersion: number
  preferredSeason?: number
}

type ViewMode = 'season' | 'all'
type DisplayMode = 'slide' | 'list'

const EPISODE_PAGE_THRESHOLD = 50

function resolveDefaultSeason(seasons: SeasonInfo[], preferredSeason?: number) {
  if (preferredSeason !== undefined && seasons.some((season) => season.season_num === preferredSeason)) return preferredSeason
  return seasons.find((season) => season.season_num > 0)?.season_num ?? seasons[0]?.season_num ?? 1
}

export default function SeriesEpisodeBrowser({ seasons, seriesTitle, historyMap, posterVersion, preferredSeason }: SeriesEpisodeBrowserProps) {
  const [viewMode, setViewMode] = useState<ViewMode>('season')
  const [displayMode, setDisplayMode] = useState<DisplayMode>('slide')
  const [activeSeason, setActiveSeason] = useState<number>(() => resolveDefaultSeason(seasons, preferredSeason))
  const manuallySelectedSeasonRef = useRef(false)
  const pagination = usePagination({ initialSize: 50 })
  const allPagination = usePagination({ initialSize: 50 })

  useEffect(() => {
    const activeExists = seasons.some((season) => season.season_num === activeSeason)
    if (!activeExists) {
      manuallySelectedSeasonRef.current = false
      setActiveSeason(resolveDefaultSeason(seasons, preferredSeason))
      return
    }
    if (!manuallySelectedSeasonRef.current && preferredSeason !== undefined && preferredSeason !== activeSeason && seasons.some((season) => season.season_num === preferredSeason)) {
      setActiveSeason(preferredSeason)
    }
  }, [activeSeason, preferredSeason, seasons])

  useEffect(() => {
    pagination.setPage(1)
  }, [activeSeason, pagination.setPage])

  useEffect(() => {
    if (viewMode === 'all') allPagination.setPage(1)
  }, [allPagination.setPage, viewMode])

  const activeSeasonData = seasons.find((season) => season.season_num === activeSeason)
  const episodeCount = activeSeasonData?.episodes?.length ?? 0
  const needsPagination = episodeCount > EPISODE_PAGE_THRESHOLD

  const seasonTotalPages = Math.max(1, Math.ceil(episodeCount / pagination.size))
  useEffect(() => {
    if (pagination.page > seasonTotalPages) pagination.setPage(seasonTotalPages)
  }, [pagination.page, pagination.setPage, seasonTotalPages])

  const pagedEpisodes = useMemo(() => {
    if (!activeSeasonData?.episodes) return []
    if (!needsPagination) return activeSeasonData.episodes
    const start = (pagination.page - 1) * pagination.size
    return activeSeasonData.episodes.slice(start, start + pagination.size)
  }, [activeSeasonData?.episodes, needsPagination, pagination.page, pagination.size])

  const allEpisodes = useMemo(() => seasons
    .flatMap((season) => season.episodes || [])
    .slice()
    .sort((left, right) => left.season_num - right.season_num || left.episode_num - right.episode_num || left.id.localeCompare(right.id)), [seasons])
  const allNeedsPagination = allEpisodes.length > EPISODE_PAGE_THRESHOLD
  const allTotalPages = Math.max(1, Math.ceil(allEpisodes.length / allPagination.size))

  useEffect(() => {
    if (allPagination.page > allTotalPages) allPagination.setPage(allTotalPages)
  }, [allPagination.page, allPagination.setPage, allTotalPages])

  const pagedAllEpisodes = useMemo(() => {
    if (!allNeedsPagination) return allEpisodes
    const start = (allPagination.page - 1) * allPagination.size
    return allEpisodes.slice(start, start + allPagination.size)
  }, [allEpisodes, allNeedsPagination, allPagination.page, allPagination.size])

  const allEpisodeGroups = useMemo(() => {
    const grouped = new Map<number, Media[]>()
    for (const episode of pagedAllEpisodes) {
      const episodes = grouped.get(episode.season_num) || []
      episodes.push(episode)
      grouped.set(episode.season_num, episodes)
    }
    return Array.from(grouped.entries()).map(([seasonNum, episodes]) => ({ seasonNum, episodes }))
  }, [pagedAllEpisodes])

  const watchedCount = useMemo(() => allEpisodes.filter((episode) => getWatchStatus(historyMap[episode.id]).watched).length, [allEpisodes, historyMap])
  const inProgressCount = useMemo(() => allEpisodes.filter((episode) => {
    const status = getWatchStatus(historyMap[episode.id])
    return !status.watched && status.progress > 0
  }).length, [allEpisodes, historyMap])

  if (seasons.length === 0) {
    return <EmptyState title="暂无剧集" description="当前剧集还没有可展示的季或单集。" className="min-h-52" />
  }

  return (
    <section className="nv-series-episode-browser space-y-5">
      <div className="nv-series-episode-toolbar flex flex-col gap-3 border-b border-[var(--nv-border-subtle)] pb-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-wrap items-center gap-2" role="tablist" aria-label="剧集视图">
          <Button type="button" variant={viewMode === 'season' ? 'primary' : 'ghost'} size="sm" onClick={() => setViewMode('season')}>季视图</Button>
          <Button type="button" variant={viewMode === 'all' ? 'primary' : 'ghost'} size="sm" onClick={() => setViewMode('all')}>全部剧集</Button>
          {watchedCount > 0 && <Tag tone="success">已看 {watchedCount}/{allEpisodes.length}</Tag>}
          {inProgressCount > 0 && <Tag tone="brand">进行中 {inProgressCount}</Tag>}
        </div>

        {viewMode === 'season' && (
          <div className="flex items-center gap-1 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-1">
            <Button type="button" variant={displayMode === 'slide' ? 'secondary' : 'ghost'} size="sm" iconOnly onClick={() => setDisplayMode('slide')} aria-label="幻灯片模式" title="幻灯片模式">
              <GalleryHorizontal size={15} aria-hidden="true" />
            </Button>
            <Button type="button" variant={displayMode === 'list' ? 'secondary' : 'ghost'} size="sm" iconOnly onClick={() => setDisplayMode('list')} aria-label="列表模式" title="列表模式">
              <LayoutList size={15} aria-hidden="true" />
            </Button>
          </div>
        )}
      </div>

      {viewMode === 'season' ? (
        <div className="space-y-5">
          {seasons.length > 1 && (
            <div className="flex flex-wrap gap-2">
              {seasons.map((season) => {
                const active = activeSeason === season.season_num
                return (
                  <button
                    key={season.season_num}
                    type="button"
                    onClick={() => {
                      manuallySelectedSeasonRef.current = true
                      setActiveSeason(season.season_num)
                    }}
                    className={`nv-season-chip rounded-[var(--nv-radius-control)] border px-3.5 py-2 text-sm font-medium transition-colors ${active ? 'border-[var(--nv-action-primary)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] text-[var(--nv-text-secondary)] hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)]'}`}
                    aria-pressed={active}
                  >
                    {seasonLabel(season.season_num)}
                    <span className="ml-1.5 text-xs opacity-70">{season.episode_count} 集</span>
                  </button>
                )
              })}
            </div>
          )}

          <div className="flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-[var(--nv-text-primary)]">{seasonLabel(activeSeason)}</h2>
              <p className="mt-0.5 text-xs text-[var(--nv-text-tertiary)]">共 {activeSeasonData?.episode_count || 0} 集</p>
            </div>
          </div>

          {displayMode === 'slide' ? (
            <EpisodeSlider
              episodes={pagedEpisodes}
              seriesTitle={seriesTitle}
              historyMap={historyMap}
              posterVersion={posterVersion}
            />
          ) : (
            <div className="space-y-2">
              {pagedEpisodes.map((episode) => (
                <EpisodeListCard key={episode.id} episode={episode} seriesTitle={seriesTitle} historyRecord={historyMap[episode.id]} posterVersion={posterVersion} />
              ))}
            </div>
          )}

          {needsPagination && activeSeasonData && (
            <Pagination
              page={pagination.page}
              totalPages={seasonTotalPages}
              total={activeSeasonData.episodes.length}
              pageSize={pagination.size}
              pageSizeOptions={[20, 50, 100, 200]}
              onPageChange={pagination.setPage}
              onPageSizeChange={pagination.setSize}
            />
          )}
        </div>
      ) : (
        <div className="space-y-8">
          {allEpisodeGroups.map(({ seasonNum, episodes: groupEpisodes }) => (
            <section key={`${seasonNum}-${allPagination.page}`} className="space-y-3">
              <div className="flex items-baseline gap-2">
                <h2 className="text-base font-semibold text-[var(--nv-text-primary)]">{seasonLabel(seasonNum)}</h2>
                <span className="text-xs text-[var(--nv-text-tertiary)]">本页 {groupEpisodes.length} 集</span>
              </div>
              <div className="space-y-2">
                {groupEpisodes.map((episode) => (
                  <EpisodeListCard key={episode.id} episode={episode} seriesTitle={seriesTitle} historyRecord={historyMap[episode.id]} posterVersion={posterVersion} />
                ))}
              </div>
            </section>
          ))}

          {allNeedsPagination && (
            <Pagination
              page={allPagination.page}
              totalPages={allTotalPages}
              total={allEpisodes.length}
              pageSize={allPagination.size}
              pageSizeOptions={[20, 50, 100, 200]}
              onPageChange={allPagination.setPage}
              onPageSizeChange={allPagination.setSize}
            />
          )}
        </div>
      )}
    </section>
  )
}

function EpisodeSlider({
  episodes,
  seriesTitle,
  historyMap,
  posterVersion,
}: {
  episodes: Media[]
  seriesTitle: string
  historyMap: Record<string, WatchHistory>
  posterVersion: number
}) {
  const sliderRef = useRef<HTMLDivElement>(null)
  const scrollBy = (left: number) => sliderRef.current?.scrollBy({ left, behavior: 'smooth' })

  if (episodes.length === 0) {
    return <EmptyState title="暂无单集" description="这一季暂时没有可展示的单集。" className="min-h-44" />
  }

  return (
    <div className="group/series-slider relative">
      <Button type="button" variant="secondary" size="sm" iconOnly onClick={() => scrollBy(-360)} className="absolute -left-2 top-1/2 z-10 -translate-y-1/2 opacity-0 shadow-[var(--nv-shadow-card)] transition-opacity group-hover/series-slider:opacity-100" aria-label="向左滚动">
        <ChevronLeft size={16} aria-hidden="true" />
      </Button>

      <div ref={sliderRef} className="flex snap-x snap-mandatory gap-3 overflow-x-auto pb-2 scrollbar-hide" style={{ scrollbarWidth: 'none' }}>
        {episodes.map((episode) => (
          <EpisodeSlideCard key={episode.id} episode={episode} seriesTitle={seriesTitle} historyRecord={historyMap[episode.id]} posterVersion={posterVersion} />
        ))}
      </div>

      <Button type="button" variant="secondary" size="sm" iconOnly onClick={() => scrollBy(360)} className="absolute -right-2 top-1/2 z-10 -translate-y-1/2 opacity-0 shadow-[var(--nv-shadow-card)] transition-opacity group-hover/series-slider:opacity-100" aria-label="向右滚动">
        <ChevronRight size={16} aria-hidden="true" />
      </Button>
    </div>
  )
}

function EpisodeListCard({
  episode,
  seriesTitle,
  historyRecord,
  posterVersion,
}: {
  episode: Media
  seriesTitle: string
  historyRecord?: WatchHistory
  posterVersion: number
}) {
  const status = getWatchStatus(historyRecord)

  return (
    <Link
      to={`/media/${episode.id}`}
      className="nv-episode-list-card group flex items-center gap-3 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] p-3 transition-[background-color,border-color,box-shadow,transform] hover:-translate-y-px hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)] hover:shadow-[var(--nv-shadow-card-hover)]"
    >
      <EpisodeThumb episode={episode} status={status} posterVersion={posterVersion} className="h-16 w-28" />

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <Tag tone="brand">S{pad(episode.season_num)}E{pad(episode.episode_num)}</Tag>
          <h3 className={`min-w-0 flex-1 truncate text-sm font-medium ${status.watched ? 'text-[var(--nv-text-tertiary)]' : 'text-[var(--nv-text-primary)]'}`}>
            {episodeTitle(episode, seriesTitle)}
          </h3>
          {status.watched && <Tag tone="success">已看</Tag>}
        </div>

        <div className="mt-1.5 flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
          {episode.duration > 0 && <span className="inline-flex items-center gap-1"><Clock size={12} />{formatDuration(episode.duration)}</span>}
          {!status.watched && status.progress > 0 && <span className="text-[var(--nv-action-primary)]">{status.progress}%</span>}
          {episode.resolution && <Tag tone="quality">{episode.resolution}</Tag>}
          {episode.video_codec && <Tag>{episode.video_codec}</Tag>}
        </div>

        {episode.overview && <p className="mt-1.5 line-clamp-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{episode.overview}</p>}
      </div>

      <ChevronRight size={16} className="shrink-0 text-[var(--nv-text-tertiary)] transition-colors group-hover:text-[var(--nv-action-primary)]" aria-hidden="true" />
    </Link>
  )
}

function EpisodeSlideCard({
  episode,
  seriesTitle,
  historyRecord,
  posterVersion,
}: {
  episode: Media
  seriesTitle: string
  historyRecord?: WatchHistory
  posterVersion: number
}) {
  const status = getWatchStatus(historyRecord)

  return (
    <Link
      to={`/media/${episode.id}`}
      className="nv-episode-slide-card group w-[13.5rem] shrink-0 snap-start overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] transition-[background-color,border-color,box-shadow,transform] hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:shadow-[var(--nv-shadow-card-hover)]"
    >
      <EpisodeThumb episode={episode} status={status} posterVersion={posterVersion} className="aspect-video w-full !rounded-none !border-0" showEpisodeLabel />

      <div className="p-3">
        <h3 className={`truncate text-sm font-medium ${status.watched ? 'text-[var(--nv-text-tertiary)]' : 'text-[var(--nv-text-primary)]'}`}>
          {episodeTitle(episode, seriesTitle)}
        </h3>
        <div className="mt-2 flex flex-wrap items-center gap-1.5 text-xs text-[var(--nv-text-tertiary)]">
          {status.watched ? <Tag tone="success">已看</Tag> : status.progress > 0 ? <span className="text-[var(--nv-action-primary)]">{status.progress}%</span> : null}
          {episode.resolution && <Tag tone="quality">{episode.resolution}</Tag>}
          {episode.duration > 0 && <span>{formatDuration(episode.duration)}</span>}
        </div>
        {episode.overview && <p className="mt-2 line-clamp-2 text-[11px] leading-5 text-[var(--nv-text-tertiary)]">{episode.overview}</p>}
      </div>
    </Link>
  )
}

function EpisodeThumb({
  episode,
  status,
  posterVersion,
  className,
  showEpisodeLabel = false,
}: {
  episode: Media
  status: { watched: boolean; progress: number }
  posterVersion: number
  className: string
  showEpisodeLabel?: boolean
}) {
  return (
    <MediaArtwork
      src={episode.poster_path ? streamApi.getPosterUrl(episode.id, posterVersion) : null}
      alt={episode.title}
      ratio="landscape"
      className={`nv-episode-thumb shrink-0 ${className}`}
      imageClassName="transition-transform duration-300 group-hover:scale-[1.025]"
      fallback={<Play size={22} aria-hidden="true" />}
    >
      <div className="absolute inset-0 z-20 flex items-center justify-center bg-black/35 opacity-0 transition-opacity group-hover:opacity-100">
        <div className="flex h-9 w-9 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)] shadow-[var(--nv-shadow-card)]">
          <Play size={16} fill="currentColor" className="ml-0.5" aria-hidden="true" />
        </div>
      </div>

      {showEpisodeLabel && <div className="absolute left-2 top-2 z-30"><Tag tone="brand">E{pad(episode.episode_num)}</Tag></div>}

      {status.watched && (
        <div className="absolute inset-0 z-30 flex items-center justify-center bg-black/45">
          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--nv-status-success)] text-white"><Check size={16} aria-hidden="true" /></div>
        </div>
      )}

      {!status.watched && status.progress > 0 && (
        <div className="nv-episode-progress absolute inset-x-0 bottom-0 z-30 h-1 bg-black/35">
          <div className="h-full bg-[var(--nv-action-primary)]" style={{ width: `${status.progress}%` }} />
        </div>
      )}
    </MediaArtwork>
  )
}

function getWatchStatus(historyRecord?: WatchHistory) {
  if (!historyRecord) return { watched: false, progress: 0 }
  const ratio = historyRecord.duration > 0 ? historyRecord.position / historyRecord.duration : 0
  return {
    watched: historyRecord.completed || ratio >= 0.9,
    progress: Math.max(0, Math.min(100, Math.round(ratio * 100))),
  }
}

function seasonLabel(seasonNumber: number) {
  return seasonNumber === 0 ? '特别篇' : `第 ${seasonNumber} 季`
}

function episodeTitle(episode: Media, seriesTitle: string) {
  return episode.episode_title || (episode.episode_num > 0 ? `第 ${episode.episode_num} 集` : seriesTitle)
}

function formatDuration(seconds: number) {
  if (!seconds) return ''
  return `${Math.floor(seconds / 60)}分钟`
}

function pad(value: number) {
  return String(value || 0).padStart(2, '0')
}
