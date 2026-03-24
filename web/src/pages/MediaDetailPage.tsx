import { useEffect, useState, useRef } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { mediaApi, userApi, streamApi, playlistApi, recommendApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import type { Media, MediaPlayInfo, Playlist, RecommendedMedia } from '@/types'
import {
  Play,
  Heart,
  HeartOff,
  Clock,
  Film,
  HardDrive,
  Calendar,
  Star,
  RefreshCw,
  ListPlus,
  Check,
  ChevronDown,
  ChevronUp,
  ExternalLink,
  Monitor,
  Music,
  Subtitles,
  FileText,
  Copy,
  Share2,
  MoreHorizontal,
  ChevronLeft,
  ChevronRight,
  Tv,
  Clapperboard,
} from 'lucide-react'
import clsx from 'clsx'
import CommentSection from '@/components/CommentSection'

export default function MediaDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [isFavorited, setIsFavorited] = useState(false)
  const [loading, setLoading] = useState(true)
  const [scraping, setScraping] = useState(false)
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [showPlaylistMenu, setShowPlaylistMenu] = useState(false)
  const [showMoreMenu, setShowMoreMenu] = useState(false)
  const [overviewExpanded, setOverviewExpanded] = useState(false)
  const [recommendations, setRecommendations] = useState<RecommendedMedia[]>([])
  const [imgLoaded, setImgLoaded] = useState(false)

  // 推荐栏横向滚动
  const recoScrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    setImgLoaded(false)
    Promise.all([
      mediaApi.detail(id),
      streamApi.getPlayInfo(id),
      playlistApi.list(),
      recommendApi.getRecommendations(12),
    ])
      .then(([mediaRes, playInfoRes, playlistRes, recoRes]) => {
        setMedia(mediaRes.data.data)
        setPlayInfo(playInfoRes.data.data)
        setPlaylists(playlistRes.data.data || [])
        setRecommendations(recoRes.data.data || [])
      })
      .catch(() => navigate('/'))
      .finally(() => setLoading(false))
  }, [id, navigate])

  const handleFavorite = async () => {
    if (!id) return
    try {
      if (isFavorited) {
        await userApi.removeFavorite(id)
        setIsFavorited(false)
      } else {
        await userApi.addFavorite(id)
        setIsFavorited(true)
      }
    } catch {
      // 静默处理
    }
  }

  const handleScrape = async () => {
    if (!id) return
    setScraping(true)
    try {
      await mediaApi.scrape(id)
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
    } catch {
      alert('元数据刮削失败，请检查TMDb API Key配置')
    } finally {
      setScraping(false)
    }
  }

  const handleAddToPlaylist = async (playlistId: string) => {
    if (!id) return
    try {
      await playlistApi.addItem(playlistId, id)
      setShowPlaylistMenu(false)
    } catch {
      // 静默处理
    }
  }

  const copyFilePath = () => {
    if (media?.file_path) {
      navigator.clipboard.writeText(media.file_path).catch(() => {})
    }
  }

  const formatSize = (bytes: number) => {
    if (!bytes) return '-'
    const gb = bytes / (1024 * 1024 * 1024)
    if (gb >= 1) return `${gb.toFixed(2)} GB`
    const mb = bytes / (1024 * 1024)
    return `${mb.toFixed(0)} MB`
  }

  const formatDuration = (seconds: number) => {
    if (!seconds) return '-'
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    if (h > 0) return `${h} 小时 ${m} 分钟`
    return `${m} 分钟`
  }

  const formatDurationShort = (seconds: number) => {
    if (!seconds) return '-'
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    if (h > 0) return `${h}h${m}m`
    return `${m}min`
  }

  const formatDate = (dateStr: string) => {
    if (!dateStr) return '-'
    const d = new Date(dateStr)
    return d.toLocaleDateString('zh-CN')
  }

  const scrollReco = (direction: 'left' | 'right') => {
    const el = recoScrollRef.current
    if (!el) return
    const scrollAmount = 300
    el.scrollBy({ left: direction === 'left' ? -scrollAmount : scrollAmount, behavior: 'smooth' })
  }

  if (loading || !media) {
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
              <div className="skeleton h-12 w-28 rounded-xl" />
            </div>
            <div className="skeleton h-20 w-full rounded-xl" />
          </div>
        </div>
      </div>
    )
  }

  const isLongOverview = (media.overview?.length || 0) > 200

  return (
    <div className="animate-fade-in -mx-4 -mt-6 sm:-mx-6 lg:-mx-8">
      {/* ============================================================
          英雄区 — 全宽背景图 + 海报 + 信息
          ============================================================ */}
      <div className="relative">
        {/* 背景图 */}
        <div className="relative h-[360px] overflow-hidden sm:h-[420px]">
          <div className="absolute inset-0" style={{ background: 'var(--bg-surface)' }}>
            {media.backdrop_path ? (
              <img
                src={media.backdrop_path}
                alt=""
                className={clsx(
                  'h-full w-full object-cover transition-all duration-1000',
                  imgLoaded ? 'opacity-40 scale-100' : 'opacity-0 scale-105'
                )}
                onLoad={() => setImgLoaded(true)}
              />
            ) : (
              <img
                src={streamApi.getPosterUrl(media.id)}
                alt=""
                className="h-full w-full object-cover opacity-15 blur-2xl scale-110"
                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
              />
            )}
          </div>
          {/* 多层渐变遮罩 */}
          <div className="absolute inset-0" style={{ background: 'linear-gradient(to top, var(--bg-base), var(--bg-base)/80 30%, transparent)' }} />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(to right, var(--bg-base)/60, transparent, var(--bg-base)/30)' }} />
          {/* 底部额外渐变 */}
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
                <img
                  src={streamApi.getPosterUrl(media.id)}
                  alt={media.title}
                  className="w-full object-cover"
                  style={{ aspectRatio: '2/3' }}
                  loading="eager"
                  onError={(e) => {
                    const el = e.target as HTMLImageElement
                    el.style.display = 'none'
                    el.parentElement!.innerHTML = `
                      <div class="flex items-center justify-center bg-surface-900 text-surface-600" style="aspect-ratio:2/3">
                        <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                          <rect x="2" y="2" width="20" height="20" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="M21 15l-5-5L5 21"/>
                        </svg>
                      </div>`
                  }}
                />
                {/* 海报上的评分标签 */}
                {media.rating > 0 && (
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
                    {media.rating.toFixed(1)}
                  </div>
                )}
              </div>
            </div>

            {/* 右侧信息 */}
            <div className="flex min-w-0 flex-1 flex-col justify-end">
              {/* 标题 */}
              <h1 className="font-display text-3xl font-bold tracking-wide drop-shadow-lg sm:text-4xl" style={{ color: 'var(--text-primary)' }}>
                {media.title}
              </h1>
              {media.orig_title && media.orig_title !== media.title && (
                <p className="mt-1.5 text-base" style={{ color: 'var(--text-secondary)' }}>{media.orig_title}</p>
              )}

              {/* 霓虹分隔线 */}
              <div className="my-3 h-[2px] w-24 rounded-full" style={{
                background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple), transparent)',
                boxShadow: '0 0 8px rgba(0, 240, 255, 0.3)',
              }} />

              {/* 操作按钮行 */}
              <div className="mb-4 flex flex-wrap items-center gap-3">
                {/* 播放按钮 — 大型主操作 */}
                <Link
                  to={`/play/${media.id}`}
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

                {/* 添加到列表 */}
                <div className="relative">
                  <button
                    onClick={() => { setShowPlaylistMenu(!showPlaylistMenu); setShowMoreMenu(false) }}
                    className="flex h-12 w-12 items-center justify-center rounded-2xl text-surface-300 transition-all duration-300 hover:scale-105 hover:text-white"
                    style={{
                      background: 'var(--nav-hover-bg)',
                      border: '1px solid var(--border-default)',
                      backdropFilter: 'blur(12px)',
                    }}
                    title="添加到播放列表"
                  >
                    <ListPlus size={20} />
                  </button>

                  {showPlaylistMenu && (
                    <div className="absolute left-0 top-full z-20 mt-2 min-w-[220px] rounded-xl py-1 shadow-2xl"
                      style={{
                        background: 'var(--bg-elevated)',
                        border: '1px solid var(--glass-border)',
                        backdropFilter: 'blur(20px)',
                      }}
                    >
                      <div className="px-4 py-2 text-[10px] font-bold uppercase tracking-widest text-neon-blue/40">播放列表</div>
                      {playlists.length === 0 ? (
                        <div className="px-4 py-3 text-sm text-surface-500">暂无播放列表</div>
                      ) : (
                        playlists.map((pl) => (
                          <button
                            key={pl.id}
                            onClick={() => handleAddToPlaylist(pl.id)}
                            className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm text-surface-300 transition-colors hover:bg-neon-blue/5 hover:text-white"
                          >
                            <ListPlus size={14} />
                            {pl.name}
                            {pl.items?.some(item => item.media_id === id) && (
                              <Check size={14} className="ml-auto text-neon" />
                            )}
                          </button>
                        ))
                      )}
                    </div>
                  )}
                </div>

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
                      {user?.role === 'admin' && (
                        <button
                          onClick={() => { handleScrape(); setShowMoreMenu(false) }}
                          disabled={scraping}
                          className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm text-surface-300 transition-colors hover:bg-neon-blue/5 hover:text-white disabled:opacity-50"
                        >
                          <RefreshCw size={14} className={clsx(scraping && 'animate-spin')} />
                          {scraping ? '刮削中...' : '刮削元数据'}
                        </button>
                      )}
                      <button
                        onClick={() => { copyFilePath(); setShowMoreMenu(false) }}
                        className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm text-surface-300 transition-colors hover:bg-neon-blue/5 hover:text-white"
                      >
                        <Copy size={14} />
                        复制文件路径
                      </button>
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

                {/* 右侧元数据标签 — 飞牛影视风格 */}
                <div className="ml-auto hidden flex-wrap items-center gap-2 lg:flex">
                  {media.rating > 0 && (
                    <span className="flex items-center gap-1 rounded-lg px-2.5 py-1 text-sm font-bold text-yellow-400 sm:hidden"
                      style={{ background: 'rgba(234, 179, 8, 0.1)', border: '1px solid rgba(234, 179, 8, 0.15)' }}
                    >
                      <Star size={13} fill="currentColor" />
                      {media.rating.toFixed(1)}
                    </span>
                  )}
                  {media.year > 0 && (
                    <span className="rounded-lg px-2.5 py-1 text-sm text-surface-300"
                      style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.06)' }}
                    >
                      {media.year}
                    </span>
                  )}
                  {media.duration > 0 && (
                    <span className="rounded-lg px-2.5 py-1 text-sm text-surface-300"
                      style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.06)' }}
                    >
                      {formatDuration(media.duration)}
                    </span>
                  )}
                  {media.genres && media.genres.split(',').slice(0, 3).map((g) => (
                    <span key={g} className="rounded-lg px-2.5 py-1 text-sm text-surface-300"
                      style={{ background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.06)' }}
                    >
                      {g.trim()}
                    </span>
                  ))}
                  {/* 技术标签 */}
                  {media.resolution && (
                    <span className="badge-neon font-bold">{media.resolution}</span>
                  )}
                  {media.video_codec && (
                    <span className="badge-neon">{media.video_codec}</span>
                  )}
                  {playInfo && (
                    <span className={clsx(
                      'rounded-lg px-2.5 py-1 text-xs font-semibold',
                      playInfo.can_direct_play
                        ? 'bg-green-500/10 text-green-400 border border-green-500/20'
                        : 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/20'
                    )}>
                      {playInfo.can_direct_play ? '直接播放' : '需转码'}
                    </span>
                  )}
                </div>
              </div>

              {/* 移动端元数据标签 */}
              <div className="mb-3 flex flex-wrap items-center gap-2 lg:hidden">
                {media.rating > 0 && (
                  <span className="flex items-center gap-1 text-sm font-bold text-yellow-400">
                    <Star size={14} fill="currentColor" />
                    {media.rating.toFixed(1)}
                  </span>
                )}
                {media.year > 0 && (
                  <span className="text-sm text-surface-400">{media.year}</span>
                )}
                {media.duration > 0 && (
                  <span className="flex items-center gap-1 text-sm text-surface-400">
                    <Clock size={13} />
                    {formatDurationShort(media.duration)}
                  </span>
                )}
                {media.resolution && <span className="badge-neon text-[10px]">{media.resolution}</span>}
                {media.video_codec && <span className="badge-neon text-[10px]">{media.video_codec}</span>}
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
        {media.overview && (
          <section>
            <div className="relative">
              <p className={clsx(
                'text-sm leading-relaxed text-surface-300 transition-all duration-500',
                !overviewExpanded && isLongOverview && 'line-clamp-3'
              )}>
                {media.overview}
              </p>
              {/* 收起时底部渐变遮罩 */}
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
        {media.genres && (
          <section className="flex flex-wrap gap-2">
            {media.genres.split(',').map((genre) => (
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

        {/* 文件信息卡片 — 飞牛影视风格 */}
        <section>
          <h3 className="mb-3 flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <FileText size={16} className="text-neon/60" />
            文件信息
          </h3>
          <div className="glass-panel rounded-xl p-5">
            {/* 文件路径 */}
            {media.file_path && (
              <div className="mb-4 flex items-start gap-3">
                <span className="shrink-0 text-xs font-medium text-surface-500">文件位置：</span>
                <div className="flex min-w-0 flex-1 items-center gap-2">
                  <code className="flex-1 truncate rounded-lg px-3 py-1.5 text-xs"
                    style={{ background: 'var(--bg-subtle)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                  >
                    {media.file_path}
                  </code>
                  <button
                    onClick={copyFilePath}
                    className="shrink-0 rounded-lg p-1.5 text-surface-500 transition-colors hover:text-neon hover:bg-neon-blue/5"
                    title="复制路径"
                  >
                    <Copy size={14} />
                  </button>
                </div>
              </div>
            )}
            {/* 文件属性行 */}
            <div className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
              <div>
                <span className="text-surface-500">文件大小：</span>
                <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{formatSize(media.file_size)}</span>
              </div>
              <div>
                <span className="text-surface-500">添加日期：</span>
                <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{formatDate(media.created_at)}</span>
              </div>
              {media.duration > 0 && (
                <div>
                  <span className="text-surface-500">总时长：</span>
                  <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{formatDuration(media.duration)}</span>
                </div>
              )}
            </div>
          </div>
        </section>

        {/* 视频信息 — 三栏卡片（视频/音频/字幕）飞牛影视风格 */}
        <section>
          <h3 className="mb-3 flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Clapperboard size={16} className="text-neon/60" />
            视频信息
          </h3>
          <div className="grid gap-4 sm:grid-cols-3">
            {/* 视频 */}
            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="mb-2 flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg"
                  style={{ background: 'rgba(0, 240, 255, 0.08)' }}
                >
                  <Monitor size={14} className="text-neon" />
                </div>
                <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>视频</span>
              </div>
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                {[media.resolution, media.video_codec].filter(Boolean).join(' ')}
                {media.duration > 0 && ` · ${formatDurationShort(media.duration)}`}
              </p>
            </div>
            {/* 音频 */}
            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="mb-2 flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg"
                  style={{ background: 'rgba(138, 43, 226, 0.08)' }}
                >
                  <Music size={14} className="text-purple-400" />
                </div>
                <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>音频</span>
              </div>
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                {media.audio_codec || '未知编码'}
              </p>
            </div>
            {/* 字幕 */}
            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="mb-2 flex items-center gap-2">
                <div className="flex h-7 w-7 items-center justify-center rounded-lg"
                  style={{ background: 'rgba(0, 255, 136, 0.08)' }}
                >
                  <Subtitles size={14} style={{ color: 'var(--neon-green)' }} />
                </div>
                <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>字幕</span>
              </div>
              <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                {media.subtitle_paths ? '有外挂字幕' : '无外挂字幕'}
              </p>
            </div>
          </div>
        </section>

        {/* 相关推荐 — 横向滚动 */}
        {recommendations.length > 0 && (
          <section>
            <div className="mb-3 flex items-center justify-between">
              <h3 className="flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                <Film size={16} className="text-neon/60" />
                相关推荐
              </h3>
              <div className="flex gap-1">
                <button
                  onClick={() => scrollReco('left')}
                  className="rounded-lg p-1.5 text-surface-400 transition-colors hover:text-white hover:bg-neon-blue/5"
                >
                  <ChevronLeft size={18} />
                </button>
                <button
                  onClick={() => scrollReco('right')}
                  className="rounded-lg p-1.5 text-surface-400 transition-colors hover:text-white hover:bg-neon-blue/5"
                >
                  <ChevronRight size={18} />
                </button>
              </div>
            </div>
            <div
              ref={recoScrollRef}
              className="flex gap-4 overflow-x-auto pb-2 scrollbar-hide"
              style={{ scrollbarWidth: 'none' }}
            >
              {recommendations.map((item) => (
                <Link
                  key={item.media.id}
                  to={`/media/${item.media.id}`}
                  className="media-card group w-36 flex-shrink-0"
                >
                  <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl bg-surface-900">
                    <img
                      src={streamApi.getPosterUrl(item.media.id)}
                      alt={item.media.title}
                      className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110"
                      loading="lazy"
                      onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                    />
                    <div className="absolute inset-0 -z-10 flex items-center justify-center text-surface-700">
                      <Film size={32} />
                    </div>
                    {/* 悬停播放图标 */}
                    <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
                      <div className="absolute bottom-2 left-2">
                        <div className="flex h-8 w-8 items-center justify-center rounded-full"
                          style={{
                            background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                            boxShadow: '0 0 12px rgba(0, 240, 255, 0.4)',
                          }}
                        >
                          <Play size={14} className="ml-0.5 text-white" fill="white" />
                        </div>
                      </div>
                    </div>
                    {/* 推荐理由 */}
                    <div className="absolute right-1.5 top-1.5">
                      <span className="badge-accent text-[9px]">{item.reason}</span>
                    </div>
                  </div>
                  <div className="p-2.5">
                    <h4 className="truncate text-xs font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
                      {item.media.title}
                    </h4>
                    <div className="mt-0.5 flex items-center gap-1 text-[10px] text-surface-500">
                      {item.media.year > 0 && <span>{item.media.year}</span>}
                      {item.media.rating > 0 && (
                        <>
                          <span className="text-neon-blue/20">·</span>
                          <span className="text-yellow-400">★ {item.media.rating.toFixed(1)}</span>
                        </>
                      )}
                    </div>
                  </div>
                </Link>
              ))}
            </div>
          </section>
        )}

        {/* 评论区 */}
        {id && <CommentSection mediaId={id} />}
      </div>

      {/* 点击空白关闭弹出菜单 */}
      {(showPlaylistMenu || showMoreMenu) && (
        <div className="fixed inset-0 z-10" onClick={() => { setShowPlaylistMenu(false); setShowMoreMenu(false) }} />
      )}
    </div>
  )
}
