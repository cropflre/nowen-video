import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { seriesApi, userApi, streamApi, playlistApi, adminApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import EditMetadataModal from '@/components/EditMetadataModal'
import type { Series, SeasonInfo, Media, Playlist, WatchHistory } from '@/types'
import {
  Play,
  Star,
  Clock,
  ChevronRight,
  Tv,
  Heart,
  Check,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  MoreHorizontal,
  Share2,
  Link2,
  Unlink,
  Pencil,
  Trash2,
} from 'lucide-react'
import clsx from 'clsx'

export default function SeriesDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const toast = useToast()
  const [series, setSeries] = useState<Series | null>(null)
  const [seasons, setSeasons] = useState<SeasonInfo[]>([])
  const [activeSeason, setActiveSeason] = useState<number>(1)
  const [loading, setLoading] = useState(true)
  const [viewMode, setViewMode] = useState<'season' | 'all'>('season')
  const [isFavorited, setIsFavorited] = useState(false)
  const [, setPlaylists] = useState<Playlist[]>([])
  const [showPlaylistMenu, setShowPlaylistMenu] = useState(false)
  const [showMoreMenu, setShowMoreMenu] = useState(false)
  const [overviewExpanded, setOverviewExpanded] = useState(false)
  const [imgLoaded, setImgLoaded] = useState(false)
  // 观看历史：在父组件一次性获取，避免每个 EpisodeCard 重复请求
  const [historyMap, setHistoryMap] = useState<Record<string, WatchHistory>>({})

  // 管理功能状态
  const [scraping, setScraping] = useState(false)
  const [showMatchModal, setShowMatchModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showUnmatchConfirm, setShowUnmatchConfirm] = useState(false)
  const [matchQuery, setMatchQuery] = useState('')
  const [matchResults, setMatchResults] = useState<any[]>([])
  const [matchSearching, setMatchSearching] = useState(false)
  const [matchSource, setMatchSource] = useState<'tmdb' | 'bangumi'>('tmdb')
  const [editForm, setEditForm] = useState<{
    title: string; orig_title: string; year: number; overview: string;
    rating: number; genres: string; country: string; language: string; studio: string
  }>({ title: '', orig_title: '', year: 0, overview: '', rating: 0, genres: '', country: '', language: '', studio: '' })
  const isAdmin = user?.role === 'admin'

  useEffect(() => {
    if (!id) return
    setLoading(true)
    setImgLoaded(false)
    Promise.all([
      seriesApi.detail(id),
      seriesApi.seasons(id),
      playlistApi.list(),
    ])
      .then(([seriesRes, seasonsRes, playlistRes]) => {
        setSeries(seriesRes.data.data)
        const seasonData = seasonsRes.data.data || []
        setSeasons(seasonData)
        if (seasonData.length > 0) {
          setActiveSeason(seasonData[0].season_num)
        }
        setPlaylists(playlistRes.data.data || [])
      })
      .catch(() => {
        toast.error('加载剧集详情失败')
        navigate('/')
      })
      .finally(() => setLoading(false))

    // 一次性获取观看历史，构建 mediaId -> history 的映射
    userApi.history(1, 200).then((res) => {
      const map: Record<string, WatchHistory> = {}
      for (const h of (res.data.data || [])) {
        map[h.media_id] = h
      }
      setHistoryMap(map)
    }).catch(() => {})
  }, [id, navigate])

  const activeSeasonData = seasons.find((s) => s.season_num === activeSeason)

// 获取第一集用于播放
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

  const isLongOverview = (series?.overview?.length || 0) > 200

  // ==================== 管理功能事件处理 ====================
  const handleManualMatch = () => {
    if (!series) return
    setMatchQuery(series.title)
    setMatchResults([])
    setMatchSource('tmdb')
    setShowMatchModal(true)
  }

  const handleMatchSearch = async () => {
    if (!matchQuery.trim()) return
    setMatchSearching(true)
    try {
      if (matchSource === 'tmdb') {
        const res = await adminApi.searchMetadata(matchQuery, 'tv')
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) {
          toast.info('TMDb 未找到匹配结果，请尝试其他关键词或切换到 Bangumi 数据源')
        }
      } else {
        // Bangumi 搜索：2=动画, 6=三次元
        const subjectType = (series?.genres || '').includes('动画') ? 2 : 6
        const res = await adminApi.searchBangumi(matchQuery, subjectType, series?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) {
          toast.info('Bangumi 未找到匹配结果，可尝试切换类型（动画/三次元）或更换关键词')
        }
      }
    } catch {
      toast.error(matchSource === 'tmdb' ? '搜索失败，请检查 TMDb API Key 配置' : 'Bangumi 搜索失败')
    } finally {
      setMatchSearching(false)
    }
  }

  const handleMatchSelect = async (resultId: number) => {
    if (!id) return
    try {
      if (matchSource === 'tmdb') {
        await adminApi.matchSeriesMetadata(id, resultId)
      } else {
        await adminApi.matchSeriesBangumi(id, resultId)
      }
      const res = await seriesApi.detail(id)
      setSeries(res.data.data)
      setShowMatchModal(false)
      toast.success(`剧集匹配成功（来源：${matchSource === 'tmdb' ? 'TMDb' : 'Bangumi'}）`)
    } catch {
      toast.error('匹配失败')
    }
  }

  const handleUnmatch = async () => {
    if (!id) return
    try {
      await adminApi.unmatchSeriesMetadata(id)
      const res = await seriesApi.detail(id)
      setSeries(res.data.data)
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
      const res = await seriesApi.detail(id)
      setSeries(res.data.data)
      toast.success('元数据刷新成功')
    } catch {
      toast.error('元数据刷新失败')
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
      const res = await seriesApi.detail(id)
      setSeries(res.data.data)
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
      toast.success('剧集已删除')
      navigate(-1)
    } catch {
      toast.error('删除剧集失败')
    }
  }

  if (loading || !series) {
    return (
      <div className="space-y-6 animate-fade-in">
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
      </div>
    )
  }

  return (
    <div className="animate-fade-in -mx-4 -mt-6 sm:-mx-6 lg:-mx-8">
      {/* ============================================================
          英雄区 —— 全宽背景图 + 海报 + 信息（与电影详情页一致）
          ============================================================ */}
      <div className="relative">
        {/* 背景图 */}
        <div className="relative h-[360px] overflow-hidden sm:h-[420px]">
          <div className="absolute inset-0" style={{ background: 'var(--bg-surface)' }}>
            {series.backdrop_path ? (
              <img
                src={streamApi.getSeriesBackdropUrl(series.id)}
                alt=""
                className={clsx(
                  'h-full w-full object-cover transition-all duration-1000',
                  imgLoaded ? 'opacity-40 scale-100' : 'opacity-0 scale-105'
                )}
                onLoad={() => setImgLoaded(true)}
              />
            ) : series.poster_path ? (
              <img
                src={streamApi.getSeriesPosterUrl(series.id)}
                alt=""
                className="h-full w-full object-cover opacity-15 blur-2xl scale-110"
                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
              />
            ) : null}
          </div>
          {/* 多层渐变遮罩 */}
          <div className="absolute inset-0" style={{ background: `linear-gradient(to top, var(--bg-base) 0%, color-mix(in srgb, var(--bg-base) 80%, transparent) 30%, transparent 100%)` }} />
          <div className="absolute inset-0" style={{ background: `linear-gradient(to right, color-mix(in srgb, var(--bg-base) 60%, transparent), transparent, color-mix(in srgb, var(--bg-base) 30%, transparent))` }} />
          <div className="absolute bottom-0 left-0 right-0 h-32" style={{ background: 'linear-gradient(to top, var(--bg-base), transparent)' }} />
        </div>

        {/* 信息叠加层 */}
        <div className="relative -mt-48 px-4 pb-2 sm:px-6 lg:px-8">
          <div className="mx-auto flex max-w-7xl gap-6">
            {/* 海报 */}
            <div className="hidden flex-shrink-0 sm:block">
              <div
                className="relative w-52 overflow-hidden rounded-xl shadow-2xl transition-transform duration-500 hover:scale-[1.02]"
                style={{
                  border: '1px solid var(--border-strong)',
                  boxShadow: 'var(--shadow-elevated), 0 0 1px var(--neon-blue-30)',
                }}
              >
                {series.poster_path ? (
                  <img
                    src={streamApi.getSeriesPosterUrl(series.id)}
                    alt={series.title}
                    className="w-full object-cover"
                    style={{ aspectRatio: '2/3' }}
                    loading="eager"
                  />
                ) : (
                  <div className="flex items-center justify-center" style={{ aspectRatio: '2/3', background: 'var(--bg-surface)', color: 'var(--text-muted)' }}>
                    <Tv size={48} />
                  </div>
                )}
                {/* 评分标签 */}
                {series.rating > 0 && (
                  <div
                    className="absolute left-3 top-3 flex items-center gap-1 rounded-lg px-2 py-1 text-sm font-bold"
                    style={{
                      background: 'rgba(0, 0, 0, 0.7)',
                      backdropFilter: 'blur(8px)',
                      border: '1px solid rgba(234, 179, 8, 0.3)',
                      color: '#FBBF24',
                    }}
                  >
                    <Star size={13} fill="currentColor" />
                    {series.rating.toFixed(1)}
                  </div>
                )}
              </div>
            </div>

            {/* 右侧信息 */}
            <div className="flex min-w-0 flex-1 flex-col justify-end">
              {/* 类型标签 */}
              <div className="mb-1 flex items-center gap-2">
                <span className="badge-accent">剧集</span>
              </div>

              {/* 标题 */}
              <h1 className="font-display text-3xl font-bold tracking-wide drop-shadow-lg sm:text-4xl" style={{ color: 'var(--text-primary)' }}>
                {series.title}
              </h1>
              {series.orig_title && series.orig_title !== series.title && (
                <p className="mt-1.5 text-base" style={{ color: 'var(--text-secondary)' }}>{series.orig_title}</p>
              )}

              {/* 霓虹分隔线 */}
              <div className="my-3 h-[2px] w-24 rounded-full" style={{
                background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple), transparent)',
                boxShadow: '0 0 8px var(--neon-blue-30)',
              }} />

              {/* 操作按钮区 */}
              <div className="mb-4 flex flex-wrap items-center gap-3">
                {/* 播放按钮 */}
                {firstEpisode && (
                  <Link
                    to={`/play/${firstEpisode.id}`}
                    className="group relative inline-flex items-center gap-2.5 rounded-2xl px-8 py-3.5 text-base font-bold transition-all duration-300 hover:-translate-y-0.5"
                    style={{
                      background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-blue-mid))',
                      boxShadow: 'var(--shadow-neon), 0 4px 15px var(--neon-blue-15)',
                      color: 'var(--text-on-neon)',
                    }}
                  >
                    <Play size={22} fill="currentColor" />
                    播放
                  </Link>
                )}

                {/* 收藏 */}
                <button
                  onClick={handleFavorite}
                  className={clsx(
                    'flex h-12 w-12 items-center justify-center rounded-2xl transition-all duration-300 hover:scale-105',
                    isFavorited ? 'text-pink-400' : ''
                  )}
                  style={{
                    background: isFavorited ? 'rgba(236, 72, 153, 0.12)' : 'var(--nav-hover-bg)',
                    border: `1px solid ${isFavorited ? 'rgba(236, 72, 153, 0.2)' : 'var(--border-default)'}`,
                    backdropFilter: 'blur(12px)',
                    color: isFavorited ? undefined : 'var(--text-secondary)',
                  }}
                  title={isFavorited ? '取消收藏' : '收藏'}
                >
                  {isFavorited ? <Heart size={20} fill="currentColor" /> : <Heart size={20} />}
                </button>

                {/* 更多操作 */}
                <div className="relative">
                  <button
                    onClick={() => { setShowMoreMenu(!showMoreMenu); setShowPlaylistMenu(false) }}
                    className="flex h-12 w-12 items-center justify-center rounded-2xl transition-all duration-300 hover:scale-105"
                    style={{
                      background: 'var(--nav-hover-bg)',
                      border: '1px solid var(--border-default)',
                      backdropFilter: 'blur(12px)',
                      color: 'var(--text-secondary)',
                    }}
                  >
                    <MoreHorizontal size={20} />
                  </button>

                  {showMoreMenu && (
                    <div className="absolute left-0 top-full z-20 mt-2 min-w-[200px] rounded-xl py-1 shadow-2xl"
                      style={{
                        background: 'var(--bg-elevated)',
                        border: '1px solid var(--glass-border)',
                        backdropFilter: 'blur(20px)',
                      }}
                    >
                      {/* 管理操作（仅管理员可见） */}
                      {isAdmin && (
                        <>
                          <div className="px-4 py-1.5 text-[10px] font-bold uppercase tracking-widest" style={{ color: 'var(--text-muted)' }}>剧集管理</div>
                          <button
                            onClick={() => { handleManualMatch(); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <Link2 size={14} />
                            手动匹配剧集
                          </button>
                          <button
                            onClick={() => { setShowUnmatchConfirm(true); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <Unlink size={14} />
                            解除匹配剧集
                          </button>
                          <button
                            onClick={() => { handleRefreshMetadata(); setShowMoreMenu(false) }}
                            disabled={scraping}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5 disabled:opacity-50"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <RefreshCw size={14} className={clsx(scraping && 'animate-spin')} />
                            {scraping ? '刷新中...' : '刷新元数据'}
                          </button>
                          <button
                            onClick={() => { handleEditMetadata(); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <Pencil size={14} />
                            编辑元数据
                          </button>
                          <button
                            onClick={() => { setShowDeleteConfirm(true); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm text-red-400 transition-colors hover:bg-red-500/10 hover:text-red-300"
                          >
                            <Trash2 size={14} />
                            删除剧集
                          </button>
                          <div className="my-1 mx-3 h-px" style={{ background: 'var(--border-default)' }} />
                        </>
                      )}
                      {/* 通用操作 */}
                      <button
                        onClick={() => { navigator.clipboard.writeText(window.location.href).then(() => toast.success('链接已复制')).catch(() => {}); setShowMoreMenu(false) }}
                        className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        <Share2 size={14} />
                        分享链接
                      </button>
                    </div>
                  )}
                </div>

                {/* 右侧元数据标签 */}
                <div className="ml-auto hidden flex-wrap items-center gap-2 lg:flex">
                  {series.year > 0 && (
                    <span className="rounded-lg px-2.5 py-1 text-sm"
                      style={{ background: 'var(--neon-blue-4)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                    >
                      {series.year}
                    </span>
                  )}
                  <span className="rounded-lg px-2.5 py-1 text-sm"
                    style={{ background: 'var(--neon-blue-4)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                  >
                    {series.season_count} 季 · {series.episode_count} 集
                  </span>
                  {series.genres && series.genres.split(',').slice(0, 3).map((g) => (
                    <span key={g} className="rounded-lg px-2.5 py-1 text-sm"
                      style={{ background: 'var(--neon-blue-4)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                    >
                      {g.trim()}
                    </span>
                  ))}
                </div>
              </div>

              {/* 移动端元数据标签 */}
              <div className="mb-3 flex flex-wrap items-center gap-2 lg:hidden">
                {series.rating > 0 && (
                  <span className="flex items-center gap-1 text-sm font-bold text-yellow-400">
                    <Star size={14} fill="currentColor" />
                    {series.rating.toFixed(1)}
                  </span>
                )}
                {series.year > 0 && (
                  <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{series.year}</span>
                )}
                <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                  {series.season_count} 季 · {series.episode_count} 集
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* ============================================================
          内容区
          ============================================================ */}
      <div className="mx-auto max-w-7xl space-y-8 px-4 pt-6 sm:px-6 lg:px-8">

        {/* 剧情简介 —— 可展开/收起 */}
        {series.overview && (
          <section>
            <div className="relative">
              <p className={clsx(
                'text-sm leading-relaxed transition-all duration-500',
                !overviewExpanded && isLongOverview && 'line-clamp-3'
              )} style={{ color: 'var(--text-secondary)' }}>
                {series.overview}
              </p>
              {isLongOverview && !overviewExpanded && (
                <div className="absolute bottom-0 left-0 right-0 h-8" style={{ background: `linear-gradient(to top, var(--bg-base), transparent)` }} />
              )}
            </div>
            {isLongOverview && (
              <button
                onClick={() => setOverviewExpanded(!overviewExpanded)}
                className="mt-2 flex items-center gap-1 text-xs font-medium text-neon transition-colors hover:text-neon-blue"
              >
                {overviewExpanded ? (
                  <><ChevronUp size={14} />收起</>
                ) : (
                  <><ChevronDown size={14} />展开全部</>
                )}
              </button>
            )}
          </section>
        )}

        {/* 类型标签 */}
        {series.genres && (
          <section className="flex flex-wrap gap-2">
            {series.genres.split(',').map((genre) => (
              <Link
                key={genre}
                to={`/search?q=${encodeURIComponent(genre.trim())}`}
                className="rounded-xl px-4 py-1.5 text-sm transition-all duration-300 hover:scale-[1.04]"
                style={{
                  background: 'var(--neon-blue-4)',
                  border: '1px solid var(--neon-blue-8)',
                  color: 'var(--text-secondary)',
                }}
              >
                {genre.trim()}
              </Link>
            ))}
          </section>
        )}

        {/* 数据来源标识 */}
        {(series.tmdb_id > 0 || series.douban_id || series.bangumi_id > 0) && (
          <section className="flex items-center gap-2 text-sm">
            <span className="shrink-0" style={{ color: 'var(--text-muted)' }}>数据来源：</span>
            <div className="flex flex-wrap gap-1.5">
              {series.tmdb_id > 0 && (
                <a
                  href={`https://www.themoviedb.org/tv/${series.tmdb_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium transition-opacity hover:opacity-80"
                  style={{ background: 'rgba(1,180,228,0.12)', color: '#01b4e4' }}
                >
                  🎬 TMDb #{series.tmdb_id}
                </a>
              )}
              {series.douban_id && (
                <a
                  href={`https://movie.douban.com/subject/${series.douban_id}/`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium transition-opacity hover:opacity-80"
                  style={{ background: 'rgba(0,180,20,0.12)', color: '#00b414' }}
                >
                  🎯 豆瓣 #{series.douban_id}
                </a>
              )}
              {series.bangumi_id > 0 && (
                <a
                  href={`https://bgm.tv/subject/${series.bangumi_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-medium transition-opacity hover:opacity-80"
                  style={{ background: 'rgba(240,145,153,0.12)', color: '#f09199' }}
                >
                  📺 Bangumi #{series.bangumi_id}
                </a>
              )}
            </div>
          </section>
        )}

        {/* 视图切换 + 季标签 */}
        <div className="space-y-4">
          <div className="flex items-center gap-2 pb-3" style={{ borderBottom: '1px solid var(--border-default)' }}>
            <button
              onClick={() => setViewMode('season')}
              className={clsx(
                'rounded-xl px-4 py-2 text-sm font-semibold transition-all duration-300',
                viewMode === 'season' ? 'text-neon' : 'hover:bg-[var(--nav-hover-bg)]'
              )}
              style={viewMode === 'season' ? {
                background: 'var(--nav-active-bg)',
                border: '1px solid var(--border-hover)',
                color: 'var(--neon-blue)',
              } : { border: '1px solid transparent', color: 'var(--text-secondary)' }}
            >
              季视图
            </button>
            <button
              onClick={() => setViewMode('all')}
              className={clsx(
                'rounded-xl px-4 py-2 text-sm font-semibold transition-all duration-300',
                viewMode === 'all' ? 'text-neon' : 'hover:bg-[var(--nav-hover-bg)]'
              )}
              style={viewMode === 'all' ? {
                background: 'var(--nav-active-bg)',
                border: '1px solid var(--border-hover)',
                color: 'var(--neon-blue)',
              } : { border: '1px solid transparent', color: 'var(--text-secondary)' }}
            >
              全部剧集
            </button>
          </div>

          {/* 季视图 */}
          {viewMode === 'season' && (
            <div>
              {/* 季标签 */}
              {seasons.length > 1 && (
                <div className="mb-4 flex flex-wrap gap-2">
                  {seasons.map((season) => (
                    <button
                      key={season.season_num}
                      onClick={() => setActiveSeason(season.season_num)}
                      className={clsx(
                        'rounded-xl px-4 py-2 text-sm font-semibold transition-all duration-300',
                        activeSeason === season.season_num ? '' : 'hover:bg-[var(--nav-hover-bg)]'
                      )}
                      style={activeSeason === season.season_num ? {
                        background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                        boxShadow: 'var(--shadow-neon)',
                        color: 'var(--text-on-neon)',
                      } : {
                        background: 'var(--bg-card)',
                        border: '1px solid var(--border-default)',
                        color: 'var(--text-secondary)',
                      }}
                    >
                      {season.season_num === 0 ? '特别篇' : `第 ${season.season_num} 季`}
                      <span className="ml-1.5 text-xs opacity-60">({season.episode_count}集)</span>
                    </button>
                  ))}
                </div>
              )}

              {/* 当前季的剧集列表 */}
              <div className="space-y-2">
                {activeSeasonData?.episodes.map((ep) => (
                  <EpisodeCard key={ep.id} episode={ep} seriesTitle={series.title} historyRecord={historyMap[ep.id]} />
                ))}
              </div>
            </div>
          )}

          {/* 全部剧集视图 */}
          {viewMode === 'all' && (
            <div className="space-y-6">
              {seasons.map((season) => (
                <div key={season.season_num}>
                  <h4 className="mb-3 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                    {season.season_num === 0 ? '特别篇' : `第 ${season.season_num} 季`}
                    <span className="ml-2 text-xs font-normal" style={{ color: 'var(--text-tertiary)' }}>
                      {season.episode_count} 集
                    </span>
                  </h4>
                  <div className="space-y-2">
                    {season.episodes.map((ep) => (
                      <EpisodeCard key={ep.id} episode={ep} seriesTitle={series.title} historyRecord={historyMap[ep.id]} />
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* 点击空白关闭弹出菜单 */}
      {(showPlaylistMenu || showMoreMenu) && (
        <div className="fixed inset-0 z-10" onClick={() => { setShowPlaylistMenu(false); setShowMoreMenu(false) }} />
      )}

      {/* ==================== 手动匹配弹窗 ==================== */}
      {showMatchModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
          <div className="w-full max-w-2xl rounded-2xl p-6 shadow-2xl" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--glass-border)' }}>
            <h3 className="mb-4 text-lg font-bold" style={{ color: 'var(--text-primary)' }}>手动匹配剧集</h3>
            {/* 数据源切换 */}
            <div className="mb-4 flex gap-2">
              <button
                onClick={() => { setMatchSource('tmdb'); setMatchResults([]) }}
                className="rounded-lg px-4 py-1.5 text-sm font-medium transition-all"
                style={{
                  background: matchSource === 'tmdb' ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-blue-mid))' : 'var(--bg-surface)',
                  color: matchSource === 'tmdb' ? '#fff' : 'var(--text-secondary)',
                  border: matchSource === 'tmdb' ? 'none' : '1px solid var(--border-default)',
                }}
              >
                🎬 TMDb
              </button>
              <button
                onClick={() => { setMatchSource('bangumi'); setMatchResults([]) }}
                className="rounded-lg px-4 py-1.5 text-sm font-medium transition-all"
                style={{
                  background: matchSource === 'bangumi' ? 'linear-gradient(135deg, #f09199, #e8788a)' : 'var(--bg-surface)',
                  color: matchSource === 'bangumi' ? '#fff' : 'var(--text-secondary)',
                  border: matchSource === 'bangumi' ? 'none' : '1px solid var(--border-default)',
                }}
              >
                📺 Bangumi
              </button>
            </div>
            <p className="mb-3 text-xs" style={{ color: 'var(--text-muted)' }}>
              {matchSource === 'tmdb'
                ? '搜索 TMDb 数据库，适合欧美电视剧。'
                : '搜索 Bangumi (bgm.tv) 数据库，适合日本动画/日剧。'
              }
            </p>
            <div className="mb-4 flex gap-2">
              <input
                value={matchQuery}
                onChange={(e) => setMatchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleMatchSearch()}
                placeholder="输入剧集名称搜索..."
                className="flex-1 rounded-xl px-4 py-2.5 text-sm outline-none"
                style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
                autoFocus
              />
              <button
                onClick={handleMatchSearch}
                disabled={matchSearching}
                className="rounded-xl px-5 py-2.5 text-sm font-semibold text-white transition-all hover:opacity-90 disabled:opacity-50"
                style={{ background: matchSource === 'tmdb' ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-blue-mid))' : 'linear-gradient(135deg, #f09199, #e8788a)' }}
              >
                {matchSearching ? '搜索中...' : '搜索'}
              </button>
            </div>
            <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
              {matchResults.map((result: any) => {
                const isBangumi = matchSource === 'bangumi'
                const displayTitle = isBangumi ? (result.name_cn || result.name) : (result.name || result.title)
                const displayOrigTitle = isBangumi ? result.name : (result.original_name || result.original_title)
                const displayYear = isBangumi ? result.air_date?.split('-')[0] : (result.first_air_date || result.release_date)?.split('-')[0]
                const displayRating = isBangumi ? result.rating?.score : result.vote_average
                const displayOverview = isBangumi ? result.summary : result.overview
                const posterUrl = isBangumi
                  ? result.images?.common || result.images?.medium
                  : (result.poster_path ? `https://image.tmdb.org/t/p/w92${result.poster_path}` : null)

                return (
                  <button
                    key={result.id}
                    onClick={() => handleMatchSelect(result.id)}
                    className="flex w-full items-center gap-3 rounded-xl p-3 text-left transition-all hover:scale-[1.01]"
                    style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
                  >
                    {posterUrl ? (
                      <img src={posterUrl} alt="" className="h-16 w-11 rounded-lg object-cover" />
                    ) : (
                      <div className="flex h-16 w-11 items-center justify-center rounded-lg" style={{ background: 'var(--bg-card)', color: 'var(--text-muted)' }}>
                        <span className="text-xs">N/A</span>
                      </div>
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <div className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                          {displayTitle}
                        </div>
                        {isBangumi && (
                          <span className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'rgba(240,145,153,0.15)', color: '#f09199' }}>
                            {result.type === 2 ? '动画' : result.type === 6 ? '三次元' : 'BGM'}
                          </span>
                        )}
                      </div>
                      {displayOrigTitle && displayOrigTitle !== displayTitle && (
                        <div className="truncate text-xs" style={{ color: 'var(--text-tertiary)' }}>{displayOrigTitle}</div>
                      )}
                      <div className="mt-0.5 flex items-center gap-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                        {displayYear && <span>{displayYear}</span>}
                        {displayRating > 0 && (
                          <span className="text-yellow-400">★ {displayRating.toFixed(1)}</span>
                        )}
                        {isBangumi && result.eps > 0 && (
                          <span>{result.eps}话</span>
                        )}
                      </div>
                      {displayOverview && (
                        <p className="mt-1 line-clamp-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>{displayOverview}</p>
                      )}
                    </div>
                  </button>
                )
              })}
              {matchResults.length === 0 && !matchSearching && (
                <div className="py-8 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
                  输入关键词搜索{matchSource === 'tmdb' ? ' TMDb' : ' Bangumi'} 数据库
                </div>
              )}
            </div>
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => setShowMatchModal(false)}
                className="rounded-xl px-5 py-2 text-sm font-medium transition-colors hover:opacity-80"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ==================== 解除匹配确认弹窗 ==================== */}
      {showUnmatchConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
          <div className="w-full max-w-md rounded-2xl p-6 shadow-2xl" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--glass-border)' }}>
            <h3 className="mb-2 text-lg font-bold" style={{ color: 'var(--text-primary)' }}>解除匹配剧集</h3>
            <p className="mb-6 text-sm" style={{ color: 'var(--text-secondary)' }}>
              确定要解除此剧集的元数据匹配吗？这将清除所有从 TMDb/豆瓣获取的信息（简介、海报、评分等），但保留原始的剧集名称。
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowUnmatchConfirm(false)}
                className="rounded-xl px-5 py-2.5 text-sm font-medium transition-colors"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button
                onClick={handleUnmatch}
                className="rounded-xl bg-orange-600 px-5 py-2.5 text-sm font-semibold text-white transition-all hover:bg-orange-500"
              >
                确认解除
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ==================== 编辑元数据弹窗 ==================== */}
      {showEditModal && (
        <EditMetadataModal
          type="series"
          id={id!}
          tmdbId={series.tmdb_id}
          mediaType="tv"
          editForm={editForm}
          setEditForm={setEditForm}
          currentPoster={streamApi.getSeriesPosterUrl(series.id)}
          hasPoster={!!series.poster_path}
          hasBackdrop={!!series.backdrop_path}
          onSave={handleEditSave}
          onClose={() => setShowEditModal(false)}
        />
      )}

      {/* ==================== 删除确认弹窗 ==================== */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
          <div className="w-full max-w-md rounded-2xl p-6 shadow-2xl" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--glass-border)' }}>
            <h3 className="mb-2 text-lg font-bold text-red-400">删除剧集</h3>
            <p className="mb-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
              确定要删除此剧集合集及其所有剧集记录吗？
            </p>
            <p className="mb-6 text-xs" style={{ color: 'var(--text-muted)' }}>
              此操作仅从数据库中移除记录，不会删除磁盘上的视频文件。重新扫描媒体库后可恢复。
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowDeleteConfirm(false)}
                className="rounded-xl px-5 py-2.5 text-sm font-medium transition-colors"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button
                onClick={handleDelete}
                className="rounded-xl bg-red-600 px-5 py-2.5 text-sm font-semibold text-white transition-all hover:bg-red-500"
              >
                确认删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

// 剧集卡片组件
function EpisodeCard({ episode: ep, seriesTitle, historyRecord }: { episode: Media; seriesTitle: string; historyRecord?: WatchHistory }) {
  const watchStatus = (() => {
    if (!historyRecord) return { watched: false, progress: 0 }
    return {
      watched: historyRecord.completed || (historyRecord.duration > 0 && historyRecord.position / historyRecord.duration > 0.9),
      progress: historyRecord.duration > 0 ? Math.round((historyRecord.position / historyRecord.duration) * 100) : 0,
    }
  })()

  const formatDuration = (seconds: number) => {
    if (!seconds) return ''
    const m = Math.floor(seconds / 60)
    return `${m}分钟`
  }

  return (
    <Link
      to={`/media/${ep.id}`}
      className="glass-panel-subtle group flex items-center gap-4 rounded-xl p-3 transition-all duration-300 hover:border-neon-blue/20 hover:shadow-card-hover"
    >
      {/* 缩略图区域 */}
      <div className="relative h-16 w-28 flex-shrink-0 overflow-hidden rounded-lg" style={{ background: 'var(--bg-surface)' }}>
        {ep.poster_path ? (
          <img
            src={streamApi.getPosterUrl(ep.id)}
            alt={ep.title}
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center" style={{ color: 'var(--text-muted)' }}>
            <Play size={20} />
          </div>
        )}
        <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
          <Play size={20} className="text-white" fill="white" />
        </div>
        {/* 已观看覆盖层 */}
        {watchStatus.watched && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/50">
            <div className="flex h-7 w-7 items-center justify-center rounded-full" style={{ background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))' }}>
              <Check size={14} className="text-white" />
            </div>
          </div>
        )}
        {/* 观看进度条（未看完时显示） */}
        {!watchStatus.watched && watchStatus.progress > 0 && (
          <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-white/10">
            <div
              className="h-full"
              style={{
                width: `${watchStatus.progress}%`,
                background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                boxShadow: '0 0 6px var(--neon-blue-30)',
              }}
            />
          </div>
        )}
      </div>

      {/* 信息 */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="badge-neon text-[10px]">
            S{String(ep.season_num).padStart(2, '0')}E{String(ep.episode_num).padStart(2, '0')}
          </span>
          <h4 className={clsx(
            'truncate text-sm font-medium transition-colors group-hover:text-neon'
          )} style={watchStatus.watched ? { color: 'var(--text-muted)' } : { color: 'var(--text-primary)' }}>
            {ep.episode_title || (ep.episode_num > 0 ? `第 ${ep.episode_num} 集` : seriesTitle)}
          </h4>
          {watchStatus.watched && (
            <span className="flex-shrink-0 text-[10px] text-green-400/70">✓ 已看</span>
          )}
        </div>
        <div className="mt-1 flex items-center gap-3 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {ep.duration > 0 && (
            <span className="flex items-center gap-1">
              <Clock size={12} />
              {formatDuration(ep.duration)}
            </span>
          )}
          {!watchStatus.watched && watchStatus.progress > 0 && (
            <span className="text-neon/60">{watchStatus.progress}%</span>
          )}
          {ep.resolution && (
            <span className="badge-neon text-[10px] !py-0">
              {ep.resolution}
            </span>
          )}
          {ep.video_codec && (
            <span className="badge-neon text-[10px] !py-0">
              {ep.video_codec}
            </span>
          )}
        </div>
      </div>

      {/* 箭头 */}
      <ChevronRight size={16} className="flex-shrink-0 transition-colors group-hover:text-neon" style={{ color: 'var(--text-muted)' }} />
    </Link>
  )
}
