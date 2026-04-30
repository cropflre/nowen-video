import { useState } from 'react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { formatDuration, formatDurationShort } from '@/utils/format'
import type { Media, MediaPlayInfo, Playlist, WatchHistory } from '@/types'
import {
  Play,
  Heart,
  Clock,
  Film,
  Star,
  RefreshCw,
  ListPlus,
  Check,
  MoreHorizontal,
  Copy,
  Share2,
  Clapperboard,
  ChevronRight,
  Pencil,
  Link2,
  Unlink,
  Trash2,
  Zap,
} from 'lucide-react'
import clsx from 'clsx'

interface HeroSectionProps {
  media: Media
  playInfo: MediaPlayInfo | null
  isFavorited: boolean
  watchProgress: WatchHistory | null
  playlists: Playlist[]
  scraping: boolean
  isAdmin: boolean
  /** 海报/背景图版本号：元数据更新后递增此值可强制刷新图片 */
  posterVersion?: number
  onFavorite: () => void
  onScrape?: () => void
  onAddToPlaylist: (playlistId: string) => void
  onShowTrailer?: () => void
  onManualMatch?: () => void
  onUnmatch?: () => void
  onRefreshMetadata?: () => void
  onEditMetadata?: () => void
  onDelete?: () => void
  onPreprocess?: () => void
  onTranscode?: () => void
}

export default function HeroSection({
  media,
  playInfo,
  isFavorited,
  watchProgress,
  playlists,
  scraping,
  isAdmin,
  posterVersion,
  onFavorite,
  onScrape: _onScrape,
  onAddToPlaylist,
  onShowTrailer,
  onManualMatch,
  onUnmatch,
  onRefreshMetadata,
  onEditMetadata,
  onDelete,
  onPreprocess,
  onTranscode,
}: HeroSectionProps) {
  const toast = useToast()
  const { t } = useTranslation()
  const [imgLoaded, setImgLoaded] = useState(false)
  const [posterFailed, setPosterFailed] = useState(false)
  const [showPlaylistMenu, setShowPlaylistMenu] = useState(false)
  const [showMoreMenu, setShowMoreMenu] = useState(false)

  const copyFilePath = () => {
    if (media.file_path) {
      navigator.clipboard.writeText(media.file_path)
        .then(() => toast.success(t('hero.filePathCopied')))
        .catch(() => {})
    }
  }

  const handleAddToPlaylist = (playlistId: string) => {
    onAddToPlaylist(playlistId)
    setShowPlaylistMenu(false)
  }

  return (
    <>
      <div className="relative">
        {/* 背景图 */}
        <div className="relative h-[360px] overflow-hidden sm:h-[420px]">
          <div className="absolute inset-0" style={{ background: 'var(--bg-surface)' }}>
            {media.backdrop_path ? (
              <img
                src={streamApi.getPosterUrl(media.id, posterVersion)}
                alt=""
                className={clsx(
                  'h-full w-full object-cover transition-all duration-1000',
                  imgLoaded ? 'opacity-40 scale-100' : 'opacity-0 scale-105'
                )}
                onLoad={() => setImgLoaded(true)}
              />
            ) : (
              <img
                src={streamApi.getPosterUrl(media.id, posterVersion)}
                alt=""
                className="h-full w-full object-cover opacity-15 blur-2xl scale-110"
                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
              />
            )}
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
                <img
                  src={streamApi.getPosterUrl(media.id, posterVersion)}
                  alt={media.title}
                  className={clsx('w-full object-cover', posterFailed && 'hidden')}
                  style={{ aspectRatio: '2/3' }}
                  loading="eager"
                  onError={() => setPosterFailed(true)}
                />
                {posterFailed && (
                  <div className="flex items-center justify-center" style={{ aspectRatio: '2/3', background: 'var(--bg-surface)', color: 'var(--text-muted)' }}>
                    <Film size={48} />
                  </div>
                )}
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
              {/* 剧集所属系列面包屑导航 */}
              {media.media_type === 'episode' && media.series_id && (
                <Link
                  to={`/series/${media.series_id}`}
                  className="mb-2 inline-flex items-center gap-1 text-sm font-medium transition-colors hover:text-neon"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {media.series?.title || media.title}
                  <ChevronRight size={14} />
                  <span style={{ color: 'var(--neon-blue)' }}>
                    S{String(media.season_num).padStart(2, '0')}E{String(media.episode_num).padStart(2, '0')}
                  </span>
                </Link>
              )}

              {/* 标题 */}
              <h1 className="font-display text-3xl font-bold tracking-wide drop-shadow-lg sm:text-4xl" style={{ color: 'var(--text-primary)' }}>
                {media.media_type === 'episode'
                  ? (media.episode_title || t('hero.episodeNum', { num: String(media.episode_num) }))
                  : media.title
                }
              </h1>
              {media.orig_title && media.orig_title !== media.title && media.media_type !== 'episode' && (
                <p className="mt-1.5 text-base" style={{ color: 'var(--text-secondary)' }}>{media.orig_title}</p>
              )}
              {media.tagline && (
                <p className="mt-1 text-sm italic" style={{ color: 'var(--text-tertiary)' }}>{media.tagline}</p>
              )}

              {/* 霓虹分隔线 */}
              <div className="my-3 h-[2px] w-24 rounded-full" style={{
                background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple), transparent)',
                boxShadow: '0 0 8px var(--neon-blue-30)',
              }} />

              {/* 操作按钮组 */}
              <div className="mb-4 flex flex-wrap items-center gap-3">
                {/* 播放按钮 */}
                <Link
                  to={`/play/${media.id}`}
                  className="group relative inline-flex items-center gap-2.5 rounded-2xl px-8 py-3.5 text-base font-bold transition-all duration-300 hover:-translate-y-0.5"
                  style={{
                    background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-blue-mid))',
                    boxShadow: 'var(--shadow-neon), 0 4px 15px var(--neon-blue-15)',
                    color: 'var(--text-on-neon)',
                  }}
                  aria-label={watchProgress && !watchProgress.completed && watchProgress.position > 0 ? t('hero.continuePlay', { title: media.title }) : t('hero.playTitle', { title: media.title })}
                >
                  <Play size={22} fill="currentColor" />
                  {watchProgress && !watchProgress.completed && watchProgress.position > 0
                    ? t('hero.continuePlayAt', { time: formatDurationShort(watchProgress.position) })
                    : t('media.play')}
                </Link>

                {/* 预告片按钮 */}
                {media.trailer_url && onShowTrailer && (
                  <button
                    onClick={onShowTrailer}
                    className="btn-secondary inline-flex items-center gap-2 rounded-2xl px-5 py-3.5 text-sm font-semibold"
                    aria-label={t('media.trailer')}
                  >
                    <Clapperboard size={18} />
                    {t('media.trailer')}
                  </button>
                )}

                {/* 收藏 */}
                <button
                  onClick={onFavorite}
                  className={clsx(
                    'btn-icon',
                    isFavorited && 'text-pink-400 !bg-pink-500/[0.12] !border-pink-500/20'
                  )}
                  title={isFavorited ? t('media.removeFavorite') : t('media.addFavorite')}
                  aria-label={isFavorited ? t('media.removeFavorite') : t('media.addFavorite')}
                  aria-pressed={isFavorited}
                >
                  {isFavorited ? <Heart size={20} fill="currentColor" /> : <Heart size={20} />}
                </button>

                {/* 添加到列表 */}
                <div className="relative">
                  <button
                    onClick={() => { setShowPlaylistMenu(!showPlaylistMenu); setShowMoreMenu(false) }}
                    className="btn-icon"
                    title={t('hero.addToPlaylist')}
                    aria-label={t('hero.addToPlaylist')}
                    aria-expanded={showPlaylistMenu}
                    aria-haspopup="true"
                  >
                    <ListPlus size={20} />
                  </button>

                  {showPlaylistMenu && (
                    <div className="absolute left-0 top-full z-20 mt-2 min-w-[220px] rounded-xl py-1 shadow-2xl animate-scale-in"
                      style={{
                        background: 'var(--bg-elevated)',
                        border: '1px solid var(--glass-border)',
                        backdropFilter: 'blur(20px)',
                      }}
                      role="menu"
                      aria-label={t('hero.playlists')}
                    >
                    <div className="px-4 py-2 text-[10px] font-bold uppercase tracking-widest" style={{ color: 'var(--text-muted)' }}>{t('hero.playlists')}</div>
                      {playlists.length === 0 ? (
                        <div className="px-4 py-3 text-sm" style={{ color: 'var(--text-muted)' }}>{t('hero.noPlaylists')}</div>
                      ) : (
                        playlists.map((pl) => (
                          <button
                            key={pl.id}
                            onClick={() => handleAddToPlaylist(pl.id)}
                            className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <ListPlus size={14} />
                            {pl.name}
                            {pl.items?.some(item => item.media_id === media.id) && (
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
                    className="btn-icon"
                    aria-haspopup="true"
                    aria-expanded={showMoreMenu}
                  >
                    <MoreHorizontal size={20} />
                  </button>

                  {showMoreMenu && (
                    <div className="absolute left-0 top-full z-20 mt-2 min-w-[200px] rounded-xl py-1 shadow-2xl animate-scale-in"
                      style={{
                        background: 'var(--bg-elevated)',
                        border: '1px solid var(--glass-border)',
                        backdropFilter: 'blur(20px)',
                      }}
                      role="menu"
                      aria-label={t('hero.moreActions')}
                    >
                      {/* 管理操作（仅管理员可见） */}
                      {isAdmin && (
                        <>
                          <div className="px-4 py-1.5 text-[10px] font-bold uppercase tracking-widest" style={{ color: 'var(--text-muted)' }}>{t('hero.mediaManagement')}</div>
                          <button
                            onClick={() => { onManualMatch?.(); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <Link2 size={14} />
                            {t('hero.manualMatch')}
                          </button>
                          <button
                            onClick={() => { onUnmatch?.(); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <Unlink size={14} />
                            {t('hero.unmatch')}
                          </button>
                          <button
                            onClick={() => { onRefreshMetadata?.(); setShowMoreMenu(false) }}
                            disabled={scraping}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5 disabled:opacity-50"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <RefreshCw size={14} className={clsx(scraping && 'animate-spin')} />
                            {scraping ? t('hero.refreshing') : t('hero.refreshMetadata')}
                          </button>
                          <button
                            onClick={() => { onEditMetadata?.(); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <Pencil size={14} />
                            {t('hero.editMetadata')}
                          </button>
                          <button
                            onClick={() => { onDelete?.(); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm text-red-400 transition-colors hover:bg-red-500/10 hover:text-red-300"
                          >
                            <Trash2 size={14} />
                            {t('hero.deleteMedia')}
                          </button>
                          <div className="my-1 mx-3 h-px" style={{ background: 'var(--border-default)' }} />
                          <div className="px-4 py-1.5 text-[10px] font-bold uppercase tracking-widest" style={{ color: 'var(--text-muted)' }}>预处理与转码</div>
                          <button
                            onClick={() => { onPreprocess?.(); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <Zap size={14} />
                            手动预处理
                          </button>
                          <button
                            onClick={() => { onTranscode?.(); setShowMoreMenu(false) }}
                            className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                            style={{ color: 'var(--text-secondary)' }}
                          >
                            <Clapperboard size={14} />
                            强制重新转码
                          </button>
                          <div className="my-1 mx-3 h-px" style={{ background: 'var(--border-default)' }} />
                        </>
                      )}
                      {/* 通用操作 */}
                      <button
                        onClick={() => { copyFilePath(); setShowMoreMenu(false) }}
                        className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        <Copy size={14} />
                        {t('hero.copyFilePath')}
                      </button>
                      <button
                        onClick={() => { navigator.clipboard.writeText(window.location.href).then(() => toast.success(t('hero.linkCopied'))).catch(() => {}); setShowMoreMenu(false) }}
                        className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-neon-blue/5"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        <Share2 size={14} />
                        {t('hero.shareLink')}
                      </button>
                    </div>
                  )}
                </div>

                {/* 右侧元数据标签 */}
                <div className="ml-auto hidden flex-wrap items-center gap-2 lg:flex">
                  {media.rating > 0 && (
                    <span className="flex items-center gap-1 rounded-lg px-2.5 py-1 text-sm font-bold text-yellow-400"
                      style={{ background: 'rgba(234, 179, 8, 0.1)', border: '1px solid rgba(234, 179, 8, 0.15)' }}
                    >
                      <Star size={13} fill="currentColor" />
                      {media.rating.toFixed(1)}
                    </span>
                  )}
                  {media.year > 0 && (
                    <span className="rounded-lg px-2.5 py-1 text-sm"
                      style={{ background: 'var(--neon-blue-4)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                    >
                      {media.year}
                    </span>
                  )}
                  {media.duration > 0 && (
                    <span className="rounded-lg px-2.5 py-1 text-sm"
                      style={{ background: 'var(--neon-blue-4)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                    >
                      {formatDuration(media.duration)}
                    </span>
                  )}
                  {media.genres && media.genres.split(',').slice(0, 3).map((g) => (
                    <Link key={g} to={`/search?q=${encodeURIComponent(g.trim())}`}
                      className="rounded-lg px-2.5 py-1 text-sm transition-all duration-200 hover:scale-[1.04] hover:brightness-125 cursor-pointer"
                      style={{ background: 'var(--neon-blue-4)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
                    >
                      {g.trim()}
                    </Link>
                  ))}
                  {media.resolution && <span className="badge-neon font-bold">{media.resolution}</span>}
                  {media.video_codec && <span className="badge-neon">{media.video_codec}</span>}
                  {playInfo && (
                    <span className={clsx(
                      'rounded-lg px-2.5 py-1 text-xs font-semibold',
                      playInfo.is_strm
                        ? 'bg-purple-500/10 text-purple-400 border border-purple-500/20'
                        : playInfo.can_direct_play
                          ? 'bg-green-500/10 text-green-400 border border-green-500/20'
                          : 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/20'
                    )}>
                      {playInfo.is_strm ? 'STRM 远程流' : playInfo.can_direct_play ? t('hero.directPlay') : t('hero.needTranscode')}
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
                  <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>{media.year}</span>
                )}
                {media.duration > 0 && (
                  <span className="flex items-center gap-1 text-sm" style={{ color: 'var(--text-secondary)' }}>
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

      {/* 点击空白关闭弹出菜单 */}
      {(showPlaylistMenu || showMoreMenu) && (
        <div className="fixed inset-0 z-10" onClick={() => { setShowPlaylistMenu(false); setShowMoreMenu(false) }} aria-hidden="true" />
      )}
    </>
  )
}
