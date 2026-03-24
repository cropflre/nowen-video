import { useEffect, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { seriesApi } from '@/api'
import type { Series, SeasonInfo, Media } from '@/types'
import { Play, Calendar, Star, Film, Clock, ChevronRight, Tv } from 'lucide-react'
import clsx from 'clsx'

export default function SeriesDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [series, setSeries] = useState<Series | null>(null)
  const [seasons, setSeasons] = useState<SeasonInfo[]>([])
  const [activeSeason, setActiveSeason] = useState<number>(1)
  const [loading, setLoading] = useState(true)
  const [viewMode, setViewMode] = useState<'season' | 'all'>('season')

  useEffect(() => {
    if (!id) return
    setLoading(true)
    Promise.all([
      seriesApi.detail(id),
      seriesApi.seasons(id),
    ])
      .then(([seriesRes, seasonsRes]) => {
        setSeries(seriesRes.data.data)
        const seasonData = seasonsRes.data.data || []
        setSeasons(seasonData)
        if (seasonData.length > 0) {
          setActiveSeason(seasonData[0].season_num)
        }
      })
      .catch(() => navigate('/'))
      .finally(() => setLoading(false))
  }, [id, navigate])

  const activeSeasonData = seasons.find((s) => s.season_num === activeSeason)

  if (loading || !series) {
    return (
      <div className="space-y-6">
        <div className="skeleton h-64 rounded-2xl" />
        <div className="skeleton h-8 w-1/3 rounded-lg" />
        <div className="space-y-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="skeleton h-20 rounded-xl" />
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="animate-fade-in">
      {/* 头部 Banner */}
      <div className="relative -mx-4 -mt-6 mb-6 overflow-hidden sm:-mx-6 lg:-mx-8">
        <div className="h-64" style={{ background: 'var(--bg-surface)' }}>
          {series.backdrop_path ? (
            <img
              src={series.backdrop_path}
              alt={series.title}
              className="h-full w-full object-cover opacity-30"
            />
          ) : series.poster_path ? (
            <img
              src={series.poster_path}
              alt={series.title}
              className="h-full w-full object-cover opacity-15 blur-xl"
            />
          ) : null}
        </div>
        <div className="absolute inset-0" style={{ background: 'linear-gradient(to top, var(--bg-base), var(--bg-base)/70 50%, transparent)' }} />
        <div className="absolute inset-0" style={{ background: 'linear-gradient(to right, var(--bg-base)/40, transparent)' }} />

        <div className="absolute bottom-0 left-0 right-0 px-4 pb-6 sm:px-6 lg:px-8">
          <div className="mx-auto flex max-w-7xl gap-6">
            {/* 海报 */}
            <div className="hidden w-36 flex-shrink-0 overflow-hidden rounded-xl shadow-2xl sm:block"
              style={{
                border: '1px solid var(--border-strong)',
                boxShadow: 'var(--shadow-elevated), 0 0 1px rgba(0, 240, 255, 0.2)',
              }}
            >
              {series.poster_path ? (
                <img
                  src={series.poster_path}
                  alt={series.title}
                  className="h-full w-full object-cover"
                />
              ) : (
                <div className="flex aspect-[2/3] items-center justify-center text-surface-700" style={{ background: 'var(--bg-surface)' }}>
                  <Tv size={48} />
                </div>
              )}
            </div>

            {/* 信息 */}
            <div className="flex flex-col justify-end">
              <div className="mb-1 flex items-center gap-2">
                <span className="badge-accent">剧集</span>
              </div>
              <h1 className="font-display text-3xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>{series.title}</h1>
              {series.orig_title && series.orig_title !== series.title && (
                <p className="mt-1 text-base" style={{ color: 'var(--text-secondary)' }}>{series.orig_title}</p>
              )}
              <div className="mt-3 flex flex-wrap items-center gap-3 text-sm" style={{ color: 'var(--text-secondary)' }}>
                {series.year > 0 && (
                  <span className="flex items-center gap-1">
                    <Calendar size={14} />
                    {series.year}
                  </span>
                )}
                {series.rating > 0 && (
                  <span className="flex items-center gap-1 text-yellow-400">
                    <Star size={14} fill="currentColor" />
                    {series.rating.toFixed(1)}
                  </span>
                )}
                <span className="flex items-center gap-1">
                  <Film size={14} />
                  {series.season_count} 季 · {series.episode_count} 集
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="mx-auto max-w-7xl space-y-6">
        {/* 简介 */}
        {series.overview && (
          <section>
            <h3 className="mb-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>简介</h3>
            <p className="text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>{series.overview}</p>
          </section>
        )}

        {/* 类型标签 */}
        {series.genres && (
          <div className="flex flex-wrap gap-2">
            {series.genres.split(',').map((genre) => (
              <span
                key={genre}
                className="rounded-lg px-3 py-1 text-sm transition-colors hover:text-neon"
                style={{
                  background: 'var(--nav-hover-bg)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-secondary)',
                }}
              >
                {genre.trim()}
              </span>
            ))}
          </div>
        )}

        {/* 视图切换 */}
        <div className="flex items-center gap-2 pb-3" style={{ borderBottom: '1px solid var(--border-default)' }}>
          <button
            onClick={() => setViewMode('season')}
            className={clsx(
              'rounded-xl px-4 py-2 text-sm font-semibold transition-all duration-300',
              viewMode === 'season'
                ? 'text-neon'
                : 'hover:bg-[var(--nav-hover-bg)]'
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
              viewMode === 'all'
                ? 'text-neon'
                : 'hover:bg-[var(--nav-hover-bg)]'
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
                      activeSeason === season.season_num
                        ? ''
                        : 'hover:bg-[var(--nav-hover-bg)]'
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
                    {season.season_num === 0
                      ? '特别篇'
                      : `第 ${season.season_num} 季`}
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
                  {season.season_num === 0
                    ? '特别篇'
                    : `第 ${season.season_num} 季`}
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
