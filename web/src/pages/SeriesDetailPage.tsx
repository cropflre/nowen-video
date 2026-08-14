import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { adminApi, seriesApi, streamApi, userApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import EditMetadataModal from '@/components/EditMetadataModal'
import { CastGrid } from '@/components/media'
import SeriesHero from '@/components/media/SeriesHero'
import SeriesEpisodeBrowser from '@/components/media/SeriesEpisodeBrowser'
import MetadataMatchModal, { type MetadataMatchSource } from '@/components/media/MetadataMatchModal'
import ConfirmDialog from '@/components/design-system/ConfirmDialog'
import { Button, Section, Tag } from '@/components/design-system'
import { formatErrMsg } from '@/utils/error'
import { parseDirectMatchId } from '@/utils/parseDirectMatchId'
import { invalidateMediaListCaches } from '@/utils/invalidateMediaCaches'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import type { Media, MediaPerson, SeasonInfo, Series, WatchHistory } from '@/types'
import { ArrowLeft, ChevronDown, ChevronUp, Database } from 'lucide-react'

const HISTORY_PAGE_SIZE = 50
const HISTORY_FETCH_CONCURRENCY = 6

type SeriesPlaybackChoice = {
  episode: Media | null
  label: string
}

function orderedEpisodes(seasons: SeasonInfo[]) {
  return seasons
    .flatMap((season) => season.episodes || [])
    .slice()
    .sort((left, right) => left.season_num - right.season_num || left.episode_num - right.episode_num || left.id.localeCompare(right.id))
}

function isHistoryCompleted(history?: WatchHistory) {
  if (!history) return false
  if (history.completed) return true
  return history.duration > 0 && history.position / history.duration >= 0.9
}

function historyUpdatedAt(history?: WatchHistory) {
  if (!history?.updated_at) return 0
  const timestamp = Date.parse(history.updated_at)
  return Number.isFinite(timestamp) ? timestamp : 0
}

function episodeCode(episode: Media) {
  return `S${String(episode.season_num).padStart(2, '0')}E${String(episode.episode_num).padStart(2, '0')}`
}

function chooseSeriesPlayback(episodes: Media[], historyMap: Record<string, WatchHistory>): SeriesPlaybackChoice {
  if (episodes.length === 0) return { episode: null, label: '播放' }

  // Season 0 is commonly specials. Prefer the first regular episode for a fresh
  // series, but keep specials available in the episode browser.
  const regularEpisodes = episodes.filter((episode) => episode.season_num > 0)
  const playbackOrder = regularEpisodes.length > 0 ? regularEpisodes : episodes

  const partial = playbackOrder
    .map((episode) => ({ episode, history: historyMap[episode.id] }))
    .filter(({ history }) => !!history && history.position > 0 && !isHistoryCompleted(history))
    .sort((left, right) => historyUpdatedAt(right.history) - historyUpdatedAt(left.history))[0]

  if (partial) {
    return { episode: partial.episode, label: `继续播放 ${episodeCode(partial.episode)}` }
  }

  const watched = playbackOrder
    .map((episode) => ({ episode, history: historyMap[episode.id] }))
    .filter(({ history }) => isHistoryCompleted(history))
    .sort((left, right) => historyUpdatedAt(right.history) - historyUpdatedAt(left.history))

  if (watched.length > 0) {
    const latestIndex = playbackOrder.findIndex((episode) => episode.id === watched[0].episode.id)
    if (latestIndex >= 0 && latestIndex + 1 < playbackOrder.length) {
      const nextEpisode = playbackOrder[latestIndex + 1]
      if (!isHistoryCompleted(historyMap[nextEpisode.id])) {
        return { episode: nextEpisode, label: `继续播放 ${episodeCode(nextEpisode)}` }
      }
    }

    const firstUnwatched = playbackOrder.find((episode) => !isHistoryCompleted(historyMap[episode.id]))
    if (firstUnwatched) {
      return { episode: firstUnwatched, label: `继续播放 ${episodeCode(firstUnwatched)}` }
    }

    return { episode: playbackOrder[0], label: `重新播放 ${episodeCode(playbackOrder[0])}` }
  }

  return { episode: playbackOrder[0], label: '播放第一集' }
}

async function loadEpisodeHistory(episodeIds: Set<string>, onPartial?: (map: Record<string, WatchHistory>) => void) {
  const map: Record<string, WatchHistory> = {}
  if (episodeIds.size === 0) return map

  const collect = (histories: WatchHistory[]) => {
    for (const history of histories) {
      if (episodeIds.has(history.media_id)) map[history.media_id] = history
    }
  }

  const firstPage = await userApi.history(1, HISTORY_PAGE_SIZE)
  collect(firstPage.data.data || [])
  onPartial?.({ ...map })

  const totalPages = Math.max(1, Math.ceil((firstPage.data.total || 0) / HISTORY_PAGE_SIZE))
  if (totalPages <= 1 || Object.keys(map).length >= episodeIds.size) return map

  const unmatchedIds = Array.from(episodeIds).filter((episodeId) => !map[episodeId])
  const remainingHistoryRequests = totalPages - 1

  // Use whichever existing API path needs fewer requests. The history endpoint
  // is capped at 50 rows by the backend, so the old history(1, 200) call only
  // returned 20 rows. This keeps the contract unchanged while making progress
  // accurate even for long-running users and large series.
  if (remainingHistoryRequests <= unmatchedIds.length) {
    const pages = Array.from({ length: remainingHistoryRequests }, (_, index) => index + 2)
    for (let index = 0; index < pages.length; index += HISTORY_FETCH_CONCURRENCY) {
      const batch = pages.slice(index, index + HISTORY_FETCH_CONCURRENCY)
      const responses = await Promise.all(batch.map((page) => userApi.history(page, HISTORY_PAGE_SIZE).catch(() => null)))
      for (const response of responses) {
        if (response) collect(response.data.data || [])
      }
      onPartial?.({ ...map })
      if (Object.keys(map).length >= episodeIds.size) break
    }
  } else {
    for (let index = 0; index < unmatchedIds.length; index += HISTORY_FETCH_CONCURRENCY) {
      const batch = unmatchedIds.slice(index, index + HISTORY_FETCH_CONCURRENCY)
      const responses = await Promise.all(batch.map((episodeId) => userApi.getProgress(episodeId).catch(() => null)))
      for (let offset = 0; offset < responses.length; offset += 1) {
        const history = responses[offset]?.data.data
        if (history) map[batch[offset]] = history
      }
      onPartial?.({ ...map })
    }
  }

  return map
}

export default function SeriesDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const toast = useToast()

  const [series, setSeries] = useState<Series | null>(null)
  const [seasons, setSeasons] = useState<SeasonInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [isFavorited, setIsFavorited] = useState(false)
  const [overviewExpanded, setOverviewExpanded] = useState(false)
  const [posterVersion, setPosterVersion] = useState<number>(() => Date.now())
  const [historyMap, setHistoryMap] = useState<Record<string, WatchHistory>>({})
  const [persons, setPersons] = useState<MediaPerson[]>([])

  const [scraping, setScraping] = useState(false)
  const [showMatchModal, setShowMatchModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showUnmatchConfirm, setShowUnmatchConfirm] = useState(false)
  const [matchQuery, setMatchQuery] = useState('')
  const [matchResults, setMatchResults] = useState<any[]>([])
  const [matchSearching, setMatchSearching] = useState(false)
  const [matchSelecting, setMatchSelecting] = useState(false)
  const [matchSource, setMatchSource] = useState<MetadataMatchSource>('tmdb')
  const [editForm, setEditForm] = useState<{
    title: string
    orig_title: string
    year: number
    overview: string
    rating: number
    genres: string
    country: string
    language: string
    studio: string
  }>({
    title: '',
    orig_title: '',
    year: 0,
    overview: '',
    rating: 0,
    genres: '',
    country: '',
    language: '',
    studio: '',
  })

  const isAdmin = user?.role === 'admin'

  useEffect(() => {
    if (!id) return
    const abortController = new AbortController()
    setLoading(true)
    setHistoryMap({})
    setIsFavorited(false)
    setOverviewExpanded(false)

    Promise.all([
      seriesApi.detail(id),
      seriesApi.seasons(id),
    ])
      .then(([seriesRes, seasonsRes]) => {
        if (abortController.signal.aborted) return
        setSeries(seriesRes.data.data)
        setSeasons(seasonsRes.data.data || [])
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        toast.error('加载剧集详情失败')
        navigate('/')
      })
      .finally(() => {
        if (!abortController.signal.aborted) setLoading(false)
      })

    seriesApi.getPersons(id)
      .then((res) => {
        if (!abortController.signal.aborted) setPersons(res.data.data || [])
      })
      .catch(() => {
        if (!abortController.signal.aborted) setPersons([])
      })

    return () => abortController.abort()
  }, [id, navigate, toast])

  const episodes = useMemo(() => orderedEpisodes(seasons), [seasons])
  const favoriteEpisode = useMemo(() => episodes.find((episode) => episode.season_num > 0) || episodes[0] || null, [episodes])
  const favoriteEpisodeId = favoriteEpisode?.id
  const playbackChoice = useMemo(() => chooseSeriesPlayback(episodes, historyMap), [episodes, historyMap])

  useEffect(() => {
    if (episodes.length === 0) {
      setHistoryMap({})
      return
    }
    let cancelled = false
    const episodeIds = new Set(episodes.map((episode) => episode.id))
    setHistoryMap({})

    void loadEpisodeHistory(episodeIds, (partialMap) => {
      if (!cancelled) setHistoryMap(partialMap)
    }).then((map) => {
      if (!cancelled) setHistoryMap(map)
    }).catch(() => {})

    return () => { cancelled = true }
  }, [id, episodes])

  useEffect(() => {
    if (!favoriteEpisodeId) {
      setIsFavorited(false)
      return
    }
    let cancelled = false
    setIsFavorited(false)
    userApi.checkFavorite(favoriteEpisodeId)
      .then((response) => { if (!cancelled) setIsFavorited(response.data.data) })
      .catch(() => {})
    return () => { cancelled = true }
  }, [favoriteEpisodeId])

  const handleFavorite = async () => {
    if (!favoriteEpisode) return
    try {
      if (isFavorited) {
        await userApi.removeFavorite(favoriteEpisode.id)
        setIsFavorited(false)
      } else {
        await userApi.addFavorite(favoriteEpisode.id)
        setIsFavorited(true)
      }
    } catch {
      toast.error('收藏操作失败')
    }
  }

  const refreshSeriesDetail = async (seriesId: string, refreshImages = false) => {
    const [seriesRes, seasonsRes, personsRes] = await Promise.all([
      seriesApi.detail(seriesId),
      seriesApi.seasons(seriesId),
      seriesApi.getPersons(seriesId).catch(() => null),
    ])

    setSeries(seriesRes.data.data)
    setSeasons(seasonsRes.data.data || [])
    if (personsRes) setPersons(personsRes.data.data || [])

    if (refreshImages) {
      const version = Date.now()
      setPosterVersion(version)
      bumpPosterVersion()
    }
    invalidateMediaListCaches()
  }

  const handleManualMatch = () => {
    if (!series) return
    setMatchQuery(series.title)
    setMatchResults([])
    setMatchSource('tmdb')
    setShowMatchModal(true)
  }

  const handleMatchSearch = async () => {
    if (!matchQuery.trim()) return

    const direct = parseDirectMatchId(matchQuery, matchSource)
    if (direct) {
      const sourceLabel = {
        tmdb: 'TMDb',
        douban: '豆瓣',
        bangumi: 'Bangumi',
        thetvdb: 'TheTVDB',
      }[direct.source]

      if (direct.source !== matchSource) {
        setMatchSource(direct.source)
        toast.info(`已识别为 ${sourceLabel} 链接，已自动切换数据源`)
      }

      toast.success(`已识别 ${sourceLabel} ID：${direct.id}，正在绑定…`)
      const idForApply: number | string = direct.source === 'douban' ? direct.id : Number(direct.id)
      setTimeout(() => { handleMatchSelect(idForApply, direct.source) }, 0)
      return
    }

    setMatchSearching(true)
    try {
      if (matchSource === 'tmdb') {
        const res = await adminApi.searchMetadata(matchQuery, 'tv')
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) toast.info('TMDb 未找到匹配结果，请尝试其他关键词或切换数据源')
      } else if (matchSource === 'douban') {
        const res = await adminApi.searchDouban(matchQuery, series?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) toast.info('豆瓣未找到匹配结果，请尝试其他关键词')
      } else if (matchSource === 'thetvdb') {
        const res = await adminApi.searchTheTVDB(matchQuery, series?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) toast.info('TheTVDB 未找到匹配结果，请尝试其他关键词')
      } else {
        const subjectType = (series?.genres || '').includes('动画') ? 2 : 6
        const res = await adminApi.searchBangumi(matchQuery, subjectType, series?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) toast.info('Bangumi 未找到匹配结果，可尝试切换类型或更换关键词')
      }
    } catch (err) {
      const errorMap: Record<MetadataMatchSource, string> = {
        tmdb: '搜索失败，请检查 TMDb API Key 或网络/代理配置',
        douban: '豆瓣搜索失败',
        thetvdb: 'TheTVDB 搜索失败，请检查 API Key 配置',
        bangumi: 'Bangumi 搜索失败',
      }
      toast.error(formatErrMsg(err, errorMap[matchSource]))
    } finally {
      setMatchSearching(false)
    }
  }

  const handleMatchSelect = async (resultId: number | string, source = matchSource) => {
    if (!id) return
    setMatchSelecting(true)
    try {
      const sourceNameMap: Record<MetadataMatchSource, string> = {
        tmdb: 'TMDb',
        bangumi: 'Bangumi',
        douban: '豆瓣',
        thetvdb: 'TheTVDB',
      }

      if (source === 'tmdb') {
        await adminApi.matchSeriesMetadata(id, resultId as number)
      } else if (source === 'douban') {
        await adminApi.matchSeriesDouban(id, resultId as string)
      } else if (source === 'thetvdb') {
        await adminApi.matchSeriesTheTVDB(id, resultId as number)
      } else {
        await adminApi.matchSeriesBangumi(id, resultId as number)
      }

      await refreshSeriesDetail(id, true)
      setShowMatchModal(false)
      toast.success(`剧集匹配成功（来源：${sourceNameMap[source]}）`)
    } catch {
      toast.error('匹配失败')
    } finally {
      setMatchSelecting(false)
    }
  }

  const handleUnmatch = async () => {
    if (!id) return
    try {
      await adminApi.unmatchSeriesMetadata(id)
      await refreshSeriesDetail(id, true)
      setShowUnmatchConfirm(false)
      toast.success('已解除匹配')
    } catch {
      toast.error('解除匹配失败')
    }
  }

  const handleRefreshMetadata = async () => {
    if (!id) return
    setScraping(true)
    try {
      await adminApi.scrapeSeriesMetadata(id)
      await refreshSeriesDetail(id, true)
      toast.success('元数据刷新成功')
    } catch (err) {
      toast.error(formatErrMsg(err, '元数据刷新失败'))
    } finally {
      setScraping(false)
    }
  }

  const handleEditMetadata = () => {
    if (!series) return
    setEditForm({
      title: series.title || '',
      orig_title: series.orig_title || '',
      year: series.year || 0,
      overview: series.overview || '',
      rating: series.rating || 0,
      genres: series.genres || '',
      country: series.country || '',
      language: series.language || '',
      studio: series.studio || '',
    })
    setShowEditModal(true)
  }

  const handleEditSave = async () => {
    if (!id) return
    try {
      await adminApi.updateSeriesMetadata(id, editForm)
      await refreshSeriesDetail(id, true)
      setShowEditModal(false)
      toast.success('元数据已更新')
    } catch {
      toast.error('更新元数据失败')
    }
  }

  const handleDelete = async () => {
    if (!id) return
    try {
      await adminApi.deleteSeries(id)
      invalidateMediaListCaches()
      toast.success('剧集已删除')
      navigate(-1)
    } catch {
      toast.error('删除剧集失败')
    }
  }

  const handleShare = () => {
    navigator.clipboard.writeText(window.location.href)
      .then(() => toast.success('链接已复制'))
      .catch(() => {})
  }

  const handleBack = () => {
    if (window.history.length > 1) navigate(-1)
    else navigate('/')
  }

  if (loading || !series) {
    return (
      <div className="space-y-6" aria-label="剧集详情加载中">
        <div className="skeleton h-[420px] rounded-[var(--nv-radius-hero)]" />
        <div className="mx-auto flex w-full max-w-[var(--nv-content-max)] gap-6 px-[var(--nv-page-gutter)] pt-4">
          <div className="skeleton hidden h-72 w-48 rounded-[var(--nv-radius-card)] sm:block" />
          <div className="flex-1 space-y-4">
            <div className="skeleton h-10 w-2/3 rounded-[var(--nv-radius-control)]" />
            <div className="skeleton h-5 w-1/3 rounded-[var(--nv-radius-control)]" />
            <div className="flex gap-3">
              <div className="skeleton h-10 w-28 rounded-[var(--nv-radius-control)]" />
              <div className="skeleton h-10 w-24 rounded-[var(--nv-radius-control)]" />
            </div>
            <div className="skeleton h-20 w-full rounded-[var(--nv-radius-control)]" />
          </div>
        </div>
      </div>
    )
  }

  const isLongOverview = (series.overview?.length || 0) > 200
  const genres = (series.genres || '').split(',').map((item) => item.trim()).filter(Boolean)
  const hasSources = series.tmdb_id > 0 || Boolean(series.douban_id) || series.bangumi_id > 0

  return (
    <div className="relative">
      <Button
        type="button"
        variant="ghost"
        size="sm"
        iconOnly
        onClick={handleBack}
        className="absolute left-[var(--nv-page-gutter)] top-4 z-30 border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-secondary)] shadow-[var(--nv-shadow-card)]"
        aria-label="返回"
        title="返回"
      >
        <ArrowLeft size={18} aria-hidden="true" />
      </Button>

      <SeriesHero
        series={series}
        playEpisode={playbackChoice.episode}
        playLabel={playbackChoice.label}
        isFavorited={isFavorited}
        isAdmin={isAdmin}
        scraping={scraping}
        posterVersion={posterVersion}
        onFavorite={handleFavorite}
        onManualMatch={handleManualMatch}
        onUnmatch={() => setShowUnmatchConfirm(true)}
        onRefreshMetadata={handleRefreshMetadata}
        onEditMetadata={handleEditMetadata}
        onDelete={() => setShowDeleteConfirm(true)}
        onShare={handleShare}
      />

      <div className="mx-auto w-full max-w-[var(--nv-content-max)] space-y-8 px-[var(--nv-page-gutter)] py-8">
        {series.overview && (
          <Section title="剧情简介">
            <div className="border-y border-[var(--nv-border-subtle)] py-4">
              <p className={`text-sm leading-7 text-[var(--nv-text-secondary)] ${!overviewExpanded && isLongOverview ? 'line-clamp-3' : ''}`}>
                {series.overview}
              </p>
              {isLongOverview && (
                <Button type="button" variant="ghost" size="sm" className="mt-2" onClick={() => setOverviewExpanded((expanded) => !expanded)}>
                  {overviewExpanded ? <ChevronUp size={14} aria-hidden="true" /> : <ChevronDown size={14} aria-hidden="true" />}
                  {overviewExpanded ? '收起' : '展开全部'}
                </Button>
              )}
            </div>
          </Section>
        )}

        {(genres.length > 0 || hasSources) && (
          <Section title="类型与来源">
            <div className="grid gap-5 border-y border-[var(--nv-border-subtle)] py-4 sm:grid-cols-2">
              {genres.length > 0 && (
                <div>
                  <div className="mb-2 text-[11px] font-medium uppercase tracking-[0.08em] text-[var(--nv-text-tertiary)]">类型</div>
                  <div className="flex flex-wrap gap-1.5">
                    {genres.map((genre) => (
                      <Link key={genre} to={`/search?q=${encodeURIComponent(genre)}`} className="no-underline">
                        <Tag>{genre}</Tag>
                      </Link>
                    ))}
                  </div>
                </div>
              )}

              {hasSources && (
                <div>
                  <div className="mb-2 flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-[0.08em] text-[var(--nv-text-tertiary)]">
                    <Database size={12} aria-hidden="true" />
                    数据来源
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {series.tmdb_id > 0 && (
                      <a href={`https://www.themoviedb.org/tv/${series.tmdb_id}`} target="_blank" rel="noopener noreferrer" className="no-underline">
                        <Tag tone="brand">TMDb #{series.tmdb_id}</Tag>
                      </a>
                    )}
                    {series.douban_id && (
                      <a href={`https://movie.douban.com/subject/${series.douban_id}/`} target="_blank" rel="noopener noreferrer" className="no-underline">
                        <Tag>豆瓣 #{series.douban_id}</Tag>
                      </a>
                    )}
                    {series.bangumi_id > 0 && (
                      <a href={`https://bgm.tv/subject/${series.bangumi_id}`} target="_blank" rel="noopener noreferrer" className="no-underline">
                        <Tag>Bangumi #{series.bangumi_id}</Tag>
                      </a>
                    )}
                  </div>
                </div>
              )}
            </div>
          </Section>
        )}

        <CastGrid persons={persons} />

        <Section
          title="剧集"
          description={`${series.season_count} 季 · ${series.episode_count} 集`}
        >
          <SeriesEpisodeBrowser
            key={series.id}
            seasons={seasons}
            seriesTitle={series.title}
            historyMap={historyMap}
            posterVersion={posterVersion}
            preferredSeason={playbackChoice.episode?.season_num}
          />
        </Section>
      </div>

      {showMatchModal && (
        <MetadataMatchModal
          mode="immediate"
          title="手动匹配剧集"
          description="选择剧集元数据来源并搜索目标条目。点击搜索结果后会立即开始匹配。"
          source={matchSource}
          onSourceChange={(source) => {
            setMatchSource(source)
            setMatchResults([])
          }}
          query={matchQuery}
          setQuery={setMatchQuery}
          results={matchResults}
          searching={matchSearching}
          applying={matchSelecting}
          onSearch={handleMatchSearch}
          onSelect={handleMatchSelect}
          onClose={() => setShowMatchModal(false)}
          searchPlaceholder="输入名称 / TMDb·豆瓣·Bangumi·TheTVDB 链接 / ID"
          applyingLabel="正在匹配元数据..."
          applyingDescription="请稍候，正在获取并同步剧集、季与单集信息"
          sourceDescriptions={{
            tmdb: '搜索 TMDb 数据库，适合欧美电视剧。',
            douban: '搜索豆瓣数据库，适合国产剧集和电影。',
            bangumi: '搜索 Bangumi (bgm.tv) 数据库，适合日本动画与日剧。',
            thetvdb: '搜索 TheTVDB 数据库，适合各类电视剧集。',
          }}
          emptyDescription="输入关键词搜索当前数据源，或直接粘贴支持的 URL / ID。"
        />
      )}

      {showUnmatchConfirm && (
        <ConfirmDialog
          title="解除匹配剧集"
          description="确定要解除此剧集的元数据匹配吗？这会清除从外部元数据源获取的简介、海报、评分等信息，但保留原始剧集名称。"
          confirmLabel="确认解除"
          onConfirm={handleUnmatch}
          onClose={() => setShowUnmatchConfirm(false)}
          tone="warning"
        />
      )}

      {showEditModal && (
        <EditMetadataModal
          type="series"
          id={id!}
          tmdbId={series.tmdb_id}
          mediaType="tv"
          editForm={editForm}
          setEditForm={setEditForm}
          currentPoster={streamApi.getSeriesPosterUrl(series.id, posterVersion)}
          hasPoster={Boolean(series.poster_path)}
          hasBackdrop={Boolean(series.backdrop_path)}
          onSave={handleEditSave}
          onClose={() => setShowEditModal(false)}
        />
      )}

      {showDeleteConfirm && (
        <ConfirmDialog
          title="删除剧集"
          description="确定要删除此剧集合集及其所有剧集记录吗？"
          hint="此操作只从数据库移除记录，不会删除磁盘上的视频文件；重新扫描媒体库后可以恢复。"
          confirmLabel="确认删除"
          onConfirm={handleDelete}
          onClose={() => setShowDeleteConfirm(false)}
          tone="danger"
        />
      )}
    </div>
  )
}
