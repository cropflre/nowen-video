import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { seriesApi, userApi, streamApi, playlistApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import type { Series, SeasonInfo, Media, Playlist } from '@/types'
import {
  Play,
  Calendar,
  Star,
  Film,
  Clock,
  ChevronRight,
  Tv,
  Heart,
  ListPlus,
  Check,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  MoreHorizontal,
  Share2,
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
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [showPlaylistMenu, setShowPlaylistMenu] = useState(false)
  const [showMoreMenu, setShowMoreMenu] = useState(false)
  const [overviewExpanded, setOverviewExpanded] = useState(false)
  const [imgLoaded, setImgLoaded] = useState(false)

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
          英雄区 — 全宽背景图 + 海报 + 信息（与电影详情页一致）
          ============================================================ */}
      <div className="relative">
        {/* 背景图 */}
        <div className="relative h-[360px] overflow-hidden sm:h-[420px]">
          <div className="absolute inset-0" style={{ background: 'var(--bg-surface)' }}>
            {series.backdrop_path ? (
              <img
                src={series.backdrop_path}
                alt=""
                className={clsx(
                  'h-full w-full object-cover transition-all duration-1000',
                  imgLoaded ? 'opacity-40 scale-100' : 'opacity-0 scale-105'
                )}
                onLoad={() => setImgLoaded(true)}
              />
            ) : series.poster_path ? (
              <img
                src={series.poster_path}
                alt=""
                className="h-full w-full object-cover opacity-15 blur-2xl scale-110"
                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
              />
            ) : null}
          </div>
          {/* 多层渐变遮罩 */}
          <div className="absolute inset-0" style={{ background: 'linear-gradient(to top, var(--bg-base), var(--bg-base)/80 30%, transparent)' }} />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(to right, var(--bg-base)/60, transparent, var(--bg-base)/30)' }} />
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
                  boxShadow: 'var(--shadow-elevated), 0 0 1px rgba(0, 240, 255, 0.3)',
                }}
              >
                {series.poster_path ? (
                  <img
                    src={series.poster_path}
                    alt={series.title}
                    className="w-full object-cover"
                    style={{ aspectRatio: '2/3' }}
                    loading="eager"
                  />
                ) : (
                  <div className="flex items-center justify-center bg-surface-900 text-surface-600" style={{ aspectRatio: '2/3' }}>
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
                boxShadow: '0 0 8px rgba(0, 240, 255, 0.3)',
              }} />

              {/* 操作按钮行 */}
              <div className="mb-4 flex flex-wrap items-center gap-3">
                {/* 播放按钮 */}
                {firstEpisode && (
                  <Link
                    to={`/play/${firstEpisode.id}`}
                    className="group relative inline-flex items-center gap-2.5 rounded-2xl px-8 py-3.5 text-base font-bold transition-all duration-300 hover:-translate-y-0.5"
                    style={{
                      background: 'linear-gradient(135deg, var(--neon-blue), rgba(0, 180, 220, 0.95))',
                      boxShadow: 'var(--shadow-neon), 0 4px 15px rgba(0, 240, 255, 0.15)',
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
                    isFavorited ? 'text-pink-400' : 'text-surface-300 hover:text-white'
                  )}
                  style={{
                    background: isFavorited ? 'rgba(236, 72, 153, 0.12)' : 'var(--nav-hover-bg)',
                    border: `1px solid ${isFavorited ? 'rgba(236, 72, 153, 0.2)' : 'var(--border-default)'}`,
                    backdropFilter: 'blur(12px)',
                  }}
                  title={isFavorited ? '取消收藏' : '收藏'}
                >
                  {isFavorited ? <Heart size={20} fill="currentColor" /> : <Heart size={20} />}
                </button>

                {/* 更多操作 */}
                <div className="relative">
                  <button
                    onClick={() => { setShowMoreMenu(!showMoreMenu); setShowPlaylistMenu(false) }}
                    className="flex h-12 w-12 items-center justify-center rounded-2xl text-surface-300 transition-all duration-300 hover:scale-105 hover:text-white"
                    style={{
                      background: 'var(--nav-hover-bg)',
                      border: '1px solid var(--border-default)',
                      backdropFilter: 'blur(12px)',
                    }}
                  >
                    <MoreHorizontal size={20} />
                  </button>

                  {showMoreMenu && (
                    <div className="absolute left-0 top-full z-20 mt-2 min-w-[180px] rounded-xl py-1 shadow-2xl"
                      style={{
                        background: 'var(--bg-elevated)',
                        border: '1px solid var(--glass-border)',
                        backdropFilter: 'blur(20px)',
                      }}
                    >
                      <button
                        onClick={() => { navigator.clipboard.writeText(window.location.href); setShowMoreMenu(false) }}
                        className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm text-surface-300 transition-colors hover:bg-neon-blue/5 hover:text-white"
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
                    <span className="rounded-lg px-2.5 py-1 text-sm text-surface-300"
                      style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.06)' }}
                    >
                      {series.year}
                    </span>
                  )}
                  <span className="rounded-lg px-2.5 py-1 text-sm text-surface-300"
                    style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.06)' }}
                  >
                    {series.season_count} 季 · {series.episode_count} 集
                  </span>
                  {series.genres && series.genres.split(',').slice(0, 3).map((g) => (
                    <span key={g} className="rounded-lg px-2.5 py-1 text-sm text-surface-300"
                      style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.06)' }}
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
                  <span className="text-sm text-surface-400">{series.year}</span>
                )}
                <span className="text-sm text-surface-400">
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

        {/* 剧情简介 — 可展开/收起 */}
        {series.overview && (
          <section>
            <div className="relative">
              <p className={clsx(
                'text-sm leading-relaxed text-surface-300 transition-all duration-500',
                !overviewExpanded && isLongOverview && 'line-clamp-3'
              )}>
                {series.overview}
              </p>
              {isLongOverview && !overviewExpanded && (
                <div className="absolute bottom-0 left-0 right-0 h-8 bg-gradient-to-t from-surface-950 to-transparent" />
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
                className="rounded-xl px-4 py-1.5 text-sm text-surface-300 transition-all duration-300 hover:text-white hover:scale-[1.04]"
                style={{
                  background: 'rgba(0, 240, 255, 0.04)',
                  border: '1px solid rgba(0, 240, 255, 0.08)',
                }}
              >
                {genre.trim()}
              </Link>
            ))}
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
                  <EpisodeCard key={ep.id} episode={ep} seriesTitle={series.title} />
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
                      <EpisodeCard key={ep.id} episode={ep} seriesTitle={series.title} />
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
    </div>
  )
}

// 剧集卡片组件
function EpisodeCard({ episode: ep, seriesTitle }: { episode: Media; seriesTitle: string }) {
  const formatDuration = (seconds: number) => {
    if (!seconds) return ''
    const m = Math.floor(seconds / 60)
    return `${m}分钟`
  }

  return (
    <Link
      to={`/play/${ep.id}`}
      className="glass-panel-subtle group flex items-center gap-4 rounded-xl p-3 transition-all duration-300 hover:border-neon-blue/20 hover:shadow-card-hover"
    >
      {/* 缩略图区域 */}
      <div className="relative h-16 w-28 flex-shrink-0 overflow-hidden rounded-lg" style={{ background: 'var(--bg-surface)' }}>
        {ep.poster_path ? (
          <img
            src={ep.poster_path}
            alt={ep.title}
            className="h-full w-full object-cover"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-surface-700">
            <Play size={20} />
          </div>
        )}
        <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
          <Play size={20} className="text-white" fill="white" />
        </div>
      </div>

      {/* 信息 */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="badge-neon text-[10px]">
            S{String(ep.season_num).padStart(2, '0')}E{String(ep.episode_num).padStart(2, '0')}
          </span>
          <h4 className="truncate text-sm font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
            {ep.episode_title || seriesTitle}
          </h4>
        </div>
        <div className="mt-1 flex items-center gap-3 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {ep.duration > 0 && (
            <span className="flex items-center gap-1">
              <Clock size={12} />
              {formatDuration(ep.duration)}
            </span>
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
