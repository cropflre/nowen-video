import { useState } from 'react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import { Button, Tag, buttonClassName } from '@/components/design-system'
import { useTranslation } from '@/i18n'
import { formatDuration, formatDurationShort } from '@/utils/format'
import type { Media, MediaPlayInfo, Playlist, WatchHistory } from '@/types'
import {
  Check,
  ChevronRight,
  Clapperboard,
  Copy,
  Film,
  Heart,
  Link2,
  ListPlus,
  MoreHorizontal,
  Pencil,
  Play,
  RefreshCw,
  Share2,
  Star,
  Trash2,
  Unlink,
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

const menuClassName = 'absolute left-0 top-full z-40 mt-2 min-w-[230px] overflow-hidden rounded-[var(--nv-radius-popover)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] py-1 shadow-[var(--nv-shadow-elevated)] backdrop-blur-xl'
const menuItemClassName = 'flex w-full items-center gap-2.5 px-3 py-2.5 text-left text-sm text-[var(--nv-text-secondary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:bg-[var(--nv-bg-hover)]'

function getBackdropUrl(media: Media, version?: number) {
  // Series has a dedicated public backdrop stream. Standalone Media currently
  // exposes poster only, while backdrop_path is a server-local filesystem path.
  // Keep this UI refactor frontend-only and do not invent a new backend API.
  if (media.series_id) {
    return media.backdrop_path
      ? streamApi.getSeriesBackdropUrl(media.series_id, version)
      : streamApi.getSeriesPosterUrl(media.series_id, version)
  }
  return streamApi.getPosterUrl(media.id, version)
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
  onAddToPlaylist,
  onShowTrailer,
  onManualMatch,
  onUnmatch,
  onRefreshMetadata,
  onEditMetadata,
  onDelete,
}: HeroSectionProps) {
  const toast = useToast()
  const { t } = useTranslation()
  const [imgLoaded, setImgLoaded] = useState(false)
  const [posterFailed, setPosterFailed] = useState(false)
  const [showPlaylistMenu, setShowPlaylistMenu] = useState(false)
  const [showMoreMenu, setShowMoreMenu] = useState(false)

  const copyFilePath = () => {
    if (!media.file_path) return
    navigator.clipboard.writeText(media.file_path)
      .then(() => toast.success(t('hero.filePathCopied')))
      .catch(() => {})
  }

  const handleAddToPlaylist = (playlistId: string) => {
    onAddToPlaylist(playlistId)
    setShowPlaylistMenu(false)
  }

  const title = media.media_type === 'episode'
    ? (media.episode_title || t('hero.episodeNum', { num: String(media.episode_num) }))
    : media.title

  const isResume = !!watchProgress && !watchProgress.completed && watchProgress.position > 0
  const playLabel = isResume
    ? t('hero.continuePlayAt', { time: formatDurationShort(watchProgress.position) })
    : t('media.play')

  const playStatus = playInfo
    ? playInfo.is_strm
      ? { label: 'STRM 远程流', tone: 'neutral' as const }
      : playInfo.can_direct_play
        ? { label: t('hero.directPlay'), tone: 'success' as const }
        : { label: t('hero.needTranscode'), tone: 'warning' as const }
    : null

  return (
    <>
      <section className="nv-detail-hero relative overflow-visible border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
        <div className="absolute inset-0 overflow-hidden">
          <div className="absolute inset-0 bg-[var(--nv-bg-surface-soft)]">
            <img
              src={getBackdropUrl(media, posterVersion)}
              alt=""
              className={clsx(
                'h-full w-full object-cover object-center transition-[opacity,transform] duration-500 ease-out',
                media.backdrop_path && media.series_id ? '' : 'scale-110 blur-2xl',
                imgLoaded ? (media.backdrop_path && media.series_id ? 'scale-100 opacity-70' : 'opacity-32') : 'scale-[1.025] opacity-0',
              )}
              onLoad={() => setImgLoaded(true)}
              onError={(event) => { event.currentTarget.style.display = 'none' }}
            />
          </div>
          <div className="absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
          <div className="absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />
          <div className="absolute inset-0 opacity-90" style={{ background: 'radial-gradient(circle at 78% 15%, var(--nv-ambient-purple-soft), transparent 32rem)' }} />
        </div>

        <div className="nv-detail-hero-inner relative mx-auto grid min-h-[clamp(28rem,48vw,42rem)] w-full max-w-[var(--nv-content-max)] items-end gap-6 px-[var(--nv-page-gutter)] pb-8 pt-24 sm:grid-cols-[12rem_minmax(0,1fr)] sm:pb-10 lg:grid-cols-[14rem_minmax(0,1fr)] lg:gap-8">
          <div className="hidden sm:block">
            <div className="nv-detail-poster relative aspect-[2/3] w-full overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] shadow-[var(--nv-shadow-card)] transition-[border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:shadow-[var(--nv-shadow-card-hover)]">
              <img
                src={streamApi.getPosterUrl(media.id, posterVersion)}
                alt={media.title}
                className={clsx('h-full w-full object-cover', posterFailed && 'hidden')}
                loading="eager"
                onError={() => setPosterFailed(true)}
              />
              {posterFailed && (
                <div className="flex h-full w-full flex-col items-center justify-center gap-2 text-[var(--nv-text-tertiary)]">
                  <Film size={34} aria-hidden="true" />
                  <span className="text-xs">暂无海报</span>
                </div>
              )}
              {media.rating > 0 && (
                <Tag tone="quality" className="absolute left-2 top-2">
                  <Star size={12} fill="currentColor" className="text-[var(--nv-status-rating)]" aria-hidden="true" />
                  {media.rating.toFixed(1)}
                </Tag>
              )}
            </div>
          </div>

          <div className="min-w-0 pb-1">
            {media.media_type === 'episode' && media.series_id && (
              <Link
                to={`/series/${media.series_id}`}
                className="mb-2 inline-flex items-center gap-1.5 text-sm font-medium text-[var(--nv-text-secondary)] transition-colors hover:text-[var(--nv-action-muted-hover)]"
              >
                <span className="truncate">{media.series?.title || media.title}</span>
                <ChevronRight size={14} aria-hidden="true" />
                <span className="shrink-0 text-[var(--nv-action-muted-hover)]">
                  S{String(media.season_num).padStart(2, '0')}E{String(media.episode_num).padStart(2, '0')}
                </span>
              </Link>
            )}

            <h1
              className="max-w-[28ch] text-balance font-bold text-[var(--nv-text-primary)]"
              style={{
                fontSize: 'var(--nv-type-h1)',
                lineHeight: 'var(--nv-line-tight)',
                letterSpacing: 'var(--nv-tracking-tight)',
              }}
            >
              {title}
            </h1>

            {media.orig_title && media.orig_title !== media.title && media.media_type !== 'episode' && (
              <p className="mt-2 max-w-3xl text-sm text-[var(--nv-text-secondary)] sm:text-base">{media.orig_title}</p>
            )}
            {media.tagline && (
              <p className="mt-1.5 max-w-3xl text-sm italic text-[var(--nv-text-tertiary)]">{media.tagline}</p>
            )}

            <div className="mt-4 flex flex-wrap items-center gap-x-2 gap-y-2 text-sm text-[var(--nv-text-secondary)]">
              {media.rating > 0 && (
                <span className="inline-flex items-center gap-1 font-semibold text-[var(--nv-status-rating)]">
                  <Star size={13} fill="currentColor" aria-hidden="true" />
                  {media.rating.toFixed(1)}
                </span>
              )}
              {media.year > 0 && <span>{media.year}</span>}
              {media.duration > 0 && <span>{formatDuration(media.duration)}</span>}
              {media.country && <span>{media.country}</span>}
              {media.genres && media.genres.split(',').slice(0, 3).map((genre) => (
                <Link key={genre} to={`/search?q=${encodeURIComponent(genre.trim())}`} className="transition-colors hover:text-[var(--nv-action-muted-hover)]">
                  {genre.trim()}
                </Link>
              ))}
            </div>

            <div className="mt-3 flex flex-wrap items-center gap-2">
              {media.resolution && <Tag tone="quality">{media.resolution}</Tag>}
              {media.video_codec && <Tag>{media.video_codec}</Tag>}
              {playStatus && <Tag tone={playStatus.tone}>{playStatus.label}</Tag>}
            </div>

            {media.overview && (
              <p className="mt-4 line-clamp-3 max-w-4xl text-sm leading-7 text-[var(--nv-text-secondary)]">{media.overview}</p>
            )}

            <div className="mt-5 flex flex-wrap items-center gap-2.5">
              <Link
                to={`/play/${media.id}`}
                className={buttonClassName({ variant: 'primary', size: 'lg' })}
                data-variant="primary"
                data-size="lg"
                aria-label={isResume ? t('hero.continuePlay', { title: media.title }) : t('hero.playTitle', { title: media.title })}
              >
                <Play size={18} fill="currentColor" aria-hidden="true" />
                {playLabel}
              </Link>

              {media.trailer_url && onShowTrailer && (
                <Button variant="secondary" size="lg" onClick={onShowTrailer}>
                  <Clapperboard size={17} aria-hidden="true" />
                  {t('media.trailer')}
                </Button>
              )}

              <Button
                variant="secondary"
                size="lg"
                iconOnly
                onClick={onFavorite}
                className={isFavorited ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-muted-hover)]' : undefined}
                title={isFavorited ? t('media.removeFavorite') : t('media.addFavorite')}
                aria-label={isFavorited ? t('media.removeFavorite') : t('media.addFavorite')}
                aria-pressed={isFavorited}
              >
                <Heart size={19} fill={isFavorited ? 'currentColor' : 'none'} aria-hidden="true" />
              </Button>

              <div className="relative">
                <Button
                  variant="secondary"
                  size="lg"
                  iconOnly
                  onClick={() => {
                    setShowPlaylistMenu((value) => !value)
                    setShowMoreMenu(false)
                  }}
                  title={t('hero.addToPlaylist')}
                  aria-label={t('hero.addToPlaylist')}
                  aria-expanded={showPlaylistMenu}
                  aria-haspopup="menu"
                >
                  <ListPlus size={19} aria-hidden="true" />
                </Button>

                {showPlaylistMenu && (
                  <div className={menuClassName} role="menu" aria-label={t('hero.playlists')}>
                    <div className="px-3 pb-1.5 pt-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--nv-text-tertiary)]">{t('hero.playlists')}</div>
                    {playlists.length === 0 ? (
                      <div className="px-3 py-3 text-sm text-[var(--nv-text-tertiary)]">{t('hero.noPlaylists')}</div>
                    ) : playlists.map((playlist) => (
                      <button key={playlist.id} onClick={() => handleAddToPlaylist(playlist.id)} className={menuItemClassName} role="menuitem">
                        <ListPlus size={14} aria-hidden="true" />
                        <span className="min-w-0 flex-1 truncate">{playlist.name}</span>
                        {playlist.items?.some((playlistItem) => playlistItem.media_id === media.id) && (
                          <Check size={14} className="text-[var(--nv-action-muted-hover)]" aria-hidden="true" />
                        )}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              <div className="relative">
                <Button
                  variant="secondary"
                  size="lg"
                  iconOnly
                  onClick={() => {
                    setShowMoreMenu((value) => !value)
                    setShowPlaylistMenu(false)
                  }}
                  title={t('hero.moreActions')}
                  aria-label={t('hero.moreActions')}
                  aria-haspopup="menu"
                  aria-expanded={showMoreMenu}
                >
                  <MoreHorizontal size={19} aria-hidden="true" />
                </Button>

                {showMoreMenu && (
                  <div className={menuClassName} role="menu" aria-label={t('hero.moreActions')}>
                    {isAdmin && (
                      <>
                        <div className="px-3 pb-1.5 pt-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--nv-text-tertiary)]">{t('hero.mediaManagement')}</div>
                        <button onClick={() => { onManualMatch?.(); setShowMoreMenu(false) }} className={menuItemClassName} role="menuitem">
                          <Link2 size={14} aria-hidden="true" /> {t('hero.manualMatch')}
                        </button>
                        <button onClick={() => { onUnmatch?.(); setShowMoreMenu(false) }} className={menuItemClassName} role="menuitem">
                          <Unlink size={14} aria-hidden="true" /> {t('hero.unmatch')}
                        </button>
                        <button onClick={() => { onRefreshMetadata?.(); setShowMoreMenu(false) }} disabled={scraping} className={clsx(menuItemClassName, 'disabled:cursor-not-allowed disabled:opacity-45')} role="menuitem">
                          <RefreshCw size={14} className={clsx(scraping && 'animate-spin')} aria-hidden="true" />
                          {scraping ? t('hero.refreshing') : t('hero.refreshMetadata')}
                        </button>
                        <button onClick={() => { onEditMetadata?.(); setShowMoreMenu(false) }} className={menuItemClassName} role="menuitem">
                          <Pencil size={14} aria-hidden="true" /> {t('hero.editMetadata')}
                        </button>
                        <button onClick={() => { onDelete?.(); setShowMoreMenu(false) }} className={clsx(menuItemClassName, 'text-[var(--nv-status-danger)] hover:text-[var(--nv-status-danger)]')} role="menuitem">
                          <Trash2 size={14} aria-hidden="true" /> {t('hero.deleteMedia')}
                        </button>
                        <div className="mx-3 my-1 h-px bg-[var(--nv-border-subtle)]" />
                      </>
                    )}
                    <button onClick={() => { copyFilePath(); setShowMoreMenu(false) }} className={menuItemClassName} role="menuitem">
                      <Copy size={14} aria-hidden="true" /> {t('hero.copyFilePath')}
                    </button>
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(window.location.href)
                          .then(() => toast.success(t('hero.linkCopied')))
                          .catch(() => {})
                        setShowMoreMenu(false)
                      }}
                      className={menuItemClassName}
                      role="menuitem"
                    >
                      <Share2 size={14} aria-hidden="true" /> {t('hero.shareLink')}
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </section>

      {(showPlaylistMenu || showMoreMenu) && (
        <button
          type="button"
          className="fixed inset-0 z-30 cursor-default"
          onClick={() => {
            setShowPlaylistMenu(false)
            setShowMoreMenu(false)
          }}
          aria-label="关闭菜单"
        />
      )}
    </>
  )
}
