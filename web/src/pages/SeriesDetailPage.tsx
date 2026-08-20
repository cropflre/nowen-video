import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { adminApi, seriesApi, streamApi, userApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import EditMetadataModal from '@/components/EditMetadataModal'
import { CastGrid } from '@/components/media'
import SeriesHero from '@/components/media/SeriesHero'
import SeriesEpisodeBrowser from '@/components/media/SeriesEpisodeBrowser'
import SeriesDetailSidebar from '@/components/media/SeriesDetailSidebar'
import MetadataMatchModal, { type MetadataMatchSource } from '@/components/media/MetadataMatchModal'
import ConfirmDialog from '@/components/design-system/ConfirmDialog'
import { Button, EmptyState, Tag } from '@/components/design-system'
import { DetailTabs } from '@/ui'
import { formatErrMsg } from '@/utils/error'
import { parseDirectMatchId } from '@/utils/parseDirectMatchId'
import { invalidateMediaListCaches } from '@/utils/invalidateMediaCaches'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import type { Media, MediaPerson, SeasonInfo, Series, WatchHistory } from '@/types'
import { ChevronDown, ChevronLeft, ChevronUp, FileText, Pencil, RefreshCw } from 'lucide-react'

const HISTORY_PAGE_SIZE = 50
const HISTORY_FETCH_CONCURRENCY = 6

type SeriesDetailTab = 'episodes' | 'overview' | 'cast'

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

  const regularEpisodes = episodes.filter((episode) => episode.season_num > 0)
  const playbackOrder = regularEpisodes.length > 0 ? regularEpisodes : episodes

  const partial = playbackOrder
    .map((episode) => ({ episode, history: historyMap[episode.id] }))
    .filter(({ history }) => !!history && history.position > 0 && !isHistoryCompleted(history))
    .sort((left, right) => historyUpdatedAt(right.history) - historyUpdatedAt(left.history))[0]

  if (partial) return { episode: partial.episode, label: `继续播放 ${episodeCode(partial.episode)}` }

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
    if (firstUnwatched) return { episode: firstUnwatched, label: `继续播放 ${episodeCode(firstUnwatched)}` }

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
  const [activeTab, setActiveTab] = useState<SeriesDetailTab>('episodes')
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
    setActiveTab('episodes')

    Promise.all([seriesApi.detail(id), seriesApi.seasons(id)])
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
      <div className="nv-media-detail-page nv-series-detail-page relative -mx-4 -mt-6 sm:-mx-6 lg:-mx-8" aria-label="剧集详情加载中">
        <div className="skeleton h-[52px]" />
        <div className="skeleton h-[360px]" />
        <div className="mx-auto grid w-full max-w-[var(--nv-content-max)] gap-6 px-[var(--nv-page-gutter)] py-6 lg:grid-cols-[minmax(0,1fr)_328px]">
          <div className="space-y-4"><div className="skeleton h-12 rounded-[var(--nv-radius-card)]" /><div className="skeleton h-80 rounded-[var(--nv-radius-card)]" /></div>
          <div className="space-y-4"><div className="skeleton h-52 rounded-[var(--nv-radius-card)]" /><div className="skeleton h-44 rounded-[var(--nv-radius-card)]" /></div>
        </div>
      </div>
    )
  }

  const isLongOverview = (series.overview?.length || 0) > 240
  const genres = (series.genres || '').split(',').map((item) => item.trim()).filter(Boolean)

  return (
    <div className="nv-media-detail-page nv-series-detail-page relative -mx-4 -mt-6 sm:-mx-6 lg:-mx-8">
      <div className="nv-detail-local-toolbar">
        <Button type="button" variant="ghost" size="sm" className="nv-detail-back-button" onClick={handleBack} aria-label="返回">
          <ChevronLeft size={15} aria-hidden="true" />
          返回
        </Button>

        <div className="nv-detail-breadcrumb" title={`影视库 / 剧集 / ${series.title}`}>
          <span>影视库</span>
          <span aria-hidden="true">/</span>
          <span>剧集</span>
          <span aria-hidden="true">/</span>
          <strong>{series.title}</strong>
        </div>

        <div className="nv-detail-toolbar-spacer" />

        {isAdmin && (
          <div className="nv-detail-admin-actions">
            <Button type="button" variant="ghost" size="sm" onClick={handleEditMetadata}>
              <Pencil size={14} aria-hidden="true" />
              编辑元数据
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={handleRefreshMetadata} disabled={scraping}>
              <RefreshCw size={14} className={scraping ? 'animate-spin' : undefined} aria-hidden="true" />
              {scraping ? '重新刮削中' : '重新刮削'}
            </Button>
          </div>
        )}
      </div>

      <SeriesHero
        series={series}
        playEpisode={playbackChoice.episode}
        playLabel={playbackChoice.label}
        isFavorited={isFavorited}
        isAdmin={isAdmin}
        posterVersion={posterVersion}
        onFavorite={handleFavorite}
        onManualMatch={handleManualMatch}
        onUnmatch={() => setShowUnmatchConfirm(true)}
        onDelete={() => setShowDeleteConfirm(true)}
        onShare={handleShare}
      />

      <div className="nv-detail-content-shell mx-auto w-full max-w-[var(--nv-content-max)] px-[var(--nv-page-gutter)] py-6">
        <div className="nv-detail-body-grid">
          <main className="nv-detail-main-column min-w-0">
            <DetailTabs
              value={activeTab}
              onChange={setActiveTab}
              ariaLabel="剧集详情章节导航"
              className="nv-series-detail-tabs"
              items={[
                { value: 'episodes', label: '剧集', panelId: 'series-episodes', tabId: 'series-tab-episodes' },
                { value: 'overview', label: '简介', panelId: 'series-overview', tabId: 'series-tab-overview' },
                { value: 'cast', label: '演职人员', panelId: 'series-cast', tabId: 'series-tab-cast' },
              ]}
            />

            <section id="series-episodes" className="nv-detail-content-section nv-detail-tab-panel nv-series-episodes-panel" role="tabpanel" aria-labelledby="series-tab-episodes" hidden={activeTab !== 'episodes'}>
              <div className="nv-series-tab-heading">
                <div>
                  <span className="nv-series-tab-eyebrow">EPISODES</span>
                  <h2>选择剧集</h2>
                  <p>{series.season_count} 季 · {series.episode_count} 集，观看进度会自动同步到每一集。</p>
                </div>
              </div>
              <SeriesEpisodeBrowser
                key={series.id}
                seasons={seasons}
                seriesTitle={series.title}
                historyMap={historyMap}
                posterVersion={posterVersion}
                preferredSeason={playbackChoice.episode?.season_num}
              />
            </section>

            <section id="series-overview" className="nv-detail-content-section nv-detail-tab-panel nv-series-overview-panel" role="tabpanel" aria-labelledby="series-tab-overview" hidden={activeTab !== 'overview'}>
              {series.overview ? (
                <div className="nv-series-overview-copy">
                  <span className="nv-series-tab-eyebrow">OVERVIEW</span>
                  <h2>剧情简介</h2>
                  {series.orig_title && series.orig_title !== series.title && <p className="nv-series-overview-original">{series.orig_title}</p>}
                  <p className={!overviewExpanded && isLongOverview ? 'line-clamp-4' : undefined}>{series.overview}</p>
                  {isLongOverview && (
                    <Button type="button" variant="ghost" size="sm" className="nv-series-overview-toggle" onClick={() => setOverviewExpanded((expanded) => !expanded)}>
                      {overviewExpanded ? <ChevronUp size={14} aria-hidden="true" /> : <ChevronDown size={14} aria-hidden="true" />}
                      {overviewExpanded ? '收起' : '展开全部'}
                    </Button>
                  )}
                </div>
              ) : (
                <EmptyState className="nv-detail-tab-empty-state" icon={<FileText size={23} aria-hidden="true" />} title="暂无简介" description="当前剧集暂未提供剧情简介或相关文字信息。" />
              )}

              {genres.length > 0 && (
                <div className="nv-series-overview-genres">
                  <span>类型</span>
                  <div>
                    {genres.map((genre) => (
                      <Link key={genre} to={`/search?q=${encodeURIComponent(genre)}`} className="no-underline"><Tag>{genre}</Tag></Link>
                    ))}
                  </div>
                </div>
              )}
            </section>

            <section id="series-cast" className="nv-detail-content-section nv-detail-tab-panel" role="tabpanel" aria-labelledby="series-tab-cast" hidden={activeTab !== 'cast'}>
              <CastGrid persons={persons} />
            </section>
          </main>

          <SeriesDetailSidebar
            series={series}
            episodes={episodes}
            historyMap={historyMap}
            playEpisode={playbackChoice.episode}
            playLabel={playbackChoice.label}
          />
        </div>
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
