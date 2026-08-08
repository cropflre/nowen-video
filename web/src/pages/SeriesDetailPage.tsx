import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { adminApi, playlistApi, seriesApi, streamApi, userApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import EditMetadataModal from '@/components/EditMetadataModal'
import { CastGrid } from '@/components/media'
import SeriesHero from '@/components/media/SeriesHero'
import SeriesEpisodeBrowser from '@/components/media/SeriesEpisodeBrowser'
import MetadataMatchModal, { type MetadataMatchSource } from '@/components/media/MetadataMatchModal'
import ConfirmDialog from '@/components/design-system/ConfirmDialog'
import { Button, Section, Surface, Tag } from '@/components/design-system'
import { formatErrMsg } from '@/utils/error'
import { parseDirectMatchId } from '@/utils/parseDirectMatchId'
import { invalidateMediaListCaches } from '@/utils/invalidateMediaCaches'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import type { MediaPerson, Playlist, SeasonInfo, Series, WatchHistory } from '@/types'
import { AnimatePresence, motion } from 'framer-motion'
import { durations, easeSmooth } from '@/lib/motion'
import { ArrowLeft, ChevronDown, ChevronUp, Database } from 'lucide-react'

export default function SeriesDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const toast = useToast()

  const [series, setSeries] = useState<Series | null>(null)
  const [seasons, setSeasons] = useState<SeasonInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [isFavorited, setIsFavorited] = useState(false)
  const [, setPlaylists] = useState<Playlist[]>([])
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

    Promise.all([
      seriesApi.detail(id),
      seriesApi.seasons(id),
      playlistApi.list(),
    ])
      .then(([seriesRes, seasonsRes, playlistRes]) => {
        if (abortController.signal.aborted) return
        setSeries(seriesRes.data.data)
        setSeasons(seasonsRes.data.data || [])
        setPlaylists(playlistRes.data.data || [])
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        toast.error('加载剧集详情失败')
        navigate('/')
      })
      .finally(() => {
        if (!abortController.signal.aborted) setLoading(false)
      })

    userApi.history(1, 200)
      .then((res) => {
        if (abortController.signal.aborted) return
        const map: Record<string, WatchHistory> = {}
        for (const history of (res.data.data || [])) {
          map[history.media_id] = history
        }
        setHistoryMap(map)
      })
      .catch(() => {})

    seriesApi.getPersons(id)
      .then((res) => {
        if (!abortController.signal.aborted) setPersons(res.data.data || [])
      })
      .catch(() => {
        if (!abortController.signal.aborted) setPersons([])
      })

    return () => abortController.abort()
  }, [id, navigate])

  const firstEpisode = seasons.length > 0 && seasons[0].episodes?.length > 0
    ? seasons[0].episodes[0]
    : null

  const handleFavorite = async () => {
    if (!firstEpisode) return
    try {
      if (isFavorited) {
        await userApi.removeFavorite(firstEpisode.id)
        setIsFavorited(false)
      } else {
        await userApi.addFavorite(firstEpisode.id)
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
      <AnimatePresence mode="wait">
        <motion.div
          key="series-skeleton"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: durations.fast }}
          className="space-y-6"
        >
          <div className="skeleton h-[420px] rounded-2xl" />
          <div className="flex gap-6 pt-4">
            <div className="skeleton hidden h-72 w-48 rounded-xl sm:block" />
            <div className="flex-1 space-y-4">
              <div className="skeleton h-10 w-2/3 rounded-lg" />
              <div className="skeleton h-5 w-1/3 rounded-lg" />
              <div className="flex gap-3">
                <div className="skeleton h-12 w-28 rounded-xl" />
                <div className="skeleton h-12 w-24 rounded-xl" />
              </div>
              <div className="skeleton h-20 w-full rounded-xl" />
            </div>
          </div>
        </motion.div>
      </AnimatePresence>
    )
  }

  const isLongOverview = (series.overview?.length || 0) > 200
  const genres = (series.genres || '').split(',').map((item) => item.trim()).filter(Boolean)
  const hasSources = series.tmdb_id > 0 || Boolean(series.douban_id) || series.bangumi_id > 0

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: durations.page, ease: easeSmooth as unknown as [number, number, number, number] }}
      className="relative -mx-4 -mt-6 sm:-mx-6 lg:-mx-8"
    >
      <Button
        type="button"
        variant="secondary"
        size="sm"
        iconOnly
        onClick={handleBack}
        className="absolute left-4 top-4 z-30 bg-[color-mix(in_srgb,var(--nv-bg-elevated)_80%,transparent)] backdrop-blur-md"
        aria-label="返回"
        title="返回"
      >
        <ArrowLeft size={18} aria-hidden="true" />
      </Button>

      <SeriesHero
        series={series}
        firstEpisode={firstEpisode}
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

      <div className="mx-auto max-w-7xl space-y-8 px-4 py-8 sm:px-6 lg:px-8">
        {series.overview && (
          <Section title="剧情简介">
            <Surface className="p-5 sm:p-6">
              <div className="relative">
                <p className={`text-sm leading-7 text-[var(--nv-text-secondary)] ${!overviewExpanded && isLongOverview ? 'line-clamp-3' : ''}`}>
                  {series.overview}
                </p>
              </div>
              {isLongOverview && (
                <Button type="button" variant="ghost" size="sm" className="mt-3" onClick={() => setOverviewExpanded((expanded) => !expanded)}>
                  {overviewExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                  {overviewExpanded ? '收起' : '展开全部'}
                </Button>
              )}
            </Surface>
          </Section>
        )}

        {(genres.length > 0 || hasSources) && (
          <Section title="类型与来源">
            <Surface className="space-y-5 p-5 sm:p-6">
              {genres.length > 0 && (
                <div>
                  <div className="mb-2 text-xs font-medium uppercase tracking-[0.08em] text-[var(--nv-text-tertiary)]">类型</div>
                  <div className="flex flex-wrap gap-2">
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
                  <div className="mb-2 flex items-center gap-1.5 text-xs font-medium uppercase tracking-[0.08em] text-[var(--nv-text-tertiary)]">
                    <Database size={13} aria-hidden="true" />
                    数据来源
                  </div>
                  <div className="flex flex-wrap gap-2">
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
            </Surface>
          </Section>
        )}

        <CastGrid persons={persons} />

        <Section
          title="剧集"
          description={`${series.season_count} 季 · ${series.episode_count} 集`}
        >
          <SeriesEpisodeBrowser
            seasons={seasons}
            seriesTitle={series.title}
            historyMap={historyMap}
            posterVersion={posterVersion}
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
    </motion.div>
  )
}
