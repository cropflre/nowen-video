import { useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from 'react'
import { createPortal } from 'react-dom'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import { usePosterVersion } from '@/stores/mediaRefresh'
import { useToast } from '@/components/Toast'
import { Button, Tag, buttonClassName } from '@/components/design-system'
import { HeroContent, MediaArtwork } from '@/ui'
import { useTranslation } from '@/i18n'
import { formatDuration, formatDurationShort, formatTime } from '@/utils/format'
import type { Media, MediaPlayInfo, Playlist, WatchHistory } from '@/types'
import {
  Check,
  ChevronRight,
  Clapperboard,
  Copy,
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

const menuClassName = 'fixed z-[calc(var(--nv-z-dropdown)+1)] min-w-[230px] max-w-[calc(100vw-24px)] max-h-[calc(100vh-24px)] overflow-y-auto rounded-[var(--nv-radius-popover)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] py-1 shadow-[var(--nv-shadow-elevated)] backdrop-blur-xl'
const menuItemClassName = 'flex w-full items-center gap-2.5 px-3 py-2.5 text-left text-sm text-[var(--nv-text-secondary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:bg-[var(--nv-bg-hover)]'

interface AnchoredMenuProps {
  open: boolean
  anchorRef: RefObject<HTMLElement>
  ariaLabel: string
  onClose: () => void
  children: ReactNode
}

function AnchoredMenu({ open, anchorRef, ariaLabel, onClose, children }: AnchoredMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null)
  const [position, setPosition] = useState({ top: 0, left: 0, ready: false })

  useLayoutEffect(() => {
    if (!open) {
      setPosition((current) => current.ready ? { ...current, ready: false } : current)
      return
    }

    const updatePosition = () => {
      const anchor = anchorRef.current
      if (!anchor) return

      const rect = anchor.getBoundingClientRect()
      const menuWidth = menuRef.current?.offsetWidth || 230
      const menuHeight = menuRef.current?.offsetHeight || 300
      const viewportWidth = window.innerWidth
      const viewportHeight = window.innerHeight
      const edge = 12
      const gap = 8

      let left = rect.left
      if (left + menuWidth > viewportWidth - edge) left = rect.right - menuWidth
      left = Math.max(edge, Math.min(left, viewportWidth - menuWidth - edge))

      let top = rect.bottom + gap
      const roomBelow = viewportHeight - rect.bottom - edge
      const roomAbove = rect.top - edge
      if (roomBelow < menuHeight + gap && roomAbove > roomBelow) {
        top = rect.top - menuHeight - gap
      }
      top = Math.max(edge, Math.min(top, viewportHeight - menuHeight - edge))

      setPosition({ top, left, ready: true })
    }

    updatePosition()
    const frame = window.requestAnimationFrame(updatePosition)
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }

    window.addEventListener('resize', updatePosition)
    window.addEventListener('scroll', updatePosition, true)
    window.addEventListener('keydown', handleKeyDown)

    return () => {
      window.cancelAnimationFrame(frame)
      window.removeEventListener('resize', updatePosition)
      window.removeEventListener('scroll', updatePosition, true)
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [anchorRef, onClose, open])

  if (!open || typeof document === 'undefined') return null

  return createPortal(
    <>
      <button
        type="button"
        className="fixed inset-0 z-[var(--nv-z-dropdown)] cursor-default bg-transparent"
        onClick={onClose}
        aria-label="关闭菜单"
      />
      <div
        ref={menuRef}
        className={menuClassName}
        role="menu"
        aria-label={ariaLabel}
        style={{
          top: position.top,
          left: position.left,
          visibility: position.ready ? 'visible' : 'hidden',
        }}
      >
        {children}
      </div>
    </>,
    document.body,
  )
}

function getBackdropUrl(media: Media, version?: number) {
  if (media.series_id) return streamApi.getSeriesBackdropUrl(media.series_id, version)
  const suffix = version ? `?v=${version}` : ''
  return streamApi.withTokenUrl(`/api/media/${media.id}/backdrop${suffix}`)
}

function getBackdropFallbackUrl(media: Media, version?: number) {
  return media.series_id
    ? streamApi.getSeriesPosterUrl(media.series_id, version)
    : streamApi.getPosterUrl(media.id, version)
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
  const assetRefreshVersion = usePosterVersion()
  const toast = useToast()
  const { t } = useTranslation()
  const [imgLoaded, setImgLoaded] = useState(false)
  const [failedBackdropKey, setFailedBackdropKey] = useState<string | null>(null)
  const [showPlaylistMenu, setShowPlaylistMenu] = useState(false)
  const [showMoreMenu, setShowMoreMenu] = useState(false)
  const playlistButtonRef = useRef<HTMLButtonElement>(null)
  const moreButtonRef = useRef<HTMLButtonElement>(null)
  const effectivePosterVersion = posterVersion === undefined
    ? (assetRefreshVersion || undefined)
    : posterVersion + assetRefreshVersion
  const backdropAttemptKey = `${media.id}:${media.series_id || ''}:${effectivePosterVersion || 0}`
  const backdropFailed = failedBackdropKey === backdropAttemptKey
  const backdropUrl = backdropFailed
    ? getBackdropFallbackUrl(media, effectivePosterVersion)
    : getBackdropUrl(media, effectivePosterVersion)

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
  const watchDuration = watchProgress?.duration || media.duration || 0
  const watchPercent = watchProgress && watchDuration > 0
    ? Math.min(100, Math.max(0, Math.round((watchProgress.position / watchDuration) * 100)))
    : 0
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

  const episodeEyebrow = media.media_type === 'episode' && media.series_id ? (
    <Link
      to={`/series/${media.series_id}`}
      className="inline-flex min-w-0 items-center gap-1.5 text-sm font-medium text-[var(--nv-text-secondary)] transition-colors hover:text-[var(--nv-action-muted-hover)]"
    >
      <span className="truncate">{media.series?.title || media.title}</span>
      <ChevronRight size={14} aria-hidden="true" />
      <span className="shrink-0 text-[var(--nv-action-muted-hover)]">
        S{String(media.season_num).padStart(2, '0')}E{String(media.episode_num).padStart(2, '0')}
      </span>
    </Link>
  ) : undefined

  const subtitle = media.media_type !== 'episode' && (media.orig_title || media.tagline) ? (
    <div className="space-y-1">
      {media.orig_title && media.orig_title !== media.title && <div>{media.orig_title}</div>}
      {media.tagline && <div className="text-[var(--nv-text-tertiary)] italic">{media.tagline}</div>}
    </div>
  ) : undefined

  const heroActions = (
    <>
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

      <Button
        ref={playlistButtonRef}
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

      <AnchoredMenu
        open={showPlaylistMenu}
        anchorRef={playlistButtonRef}
        ariaLabel={t('hero.playlists')}
        onClose={() => setShowPlaylistMenu(false)}
      >
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
      </AnchoredMenu>

      <Button
        ref={moreButtonRef}
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

      <AnchoredMenu
        open={showMoreMenu}
        anchorRef={moreButtonRef}
        ariaLabel={t('hero.moreActions')}
        onClose={() => setShowMoreMenu(false)}
      >
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
      </AnchoredMenu>
    </>
  )

  return (
    <section className="nv-detail-hero relative overflow-visible border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
      <div className="absolute inset-0 overflow-hidden">
        <div className="absolute inset-0 bg-[var(--nv-bg-surface-soft)]">
          <img
            key={`${backdropAttemptKey}:${backdropFailed ? 'poster' : 'backdrop'}`}
            src={backdropUrl}
            alt=""
            className={clsx(
              'h-full w-full object-center transition-[opacity,transform] duration-500 ease-out',
              backdropFailed ? 'object-cover scale-110 blur-2xl' : 'object-contain',
              imgLoaded ? (backdropFailed ? 'opacity-32' : 'scale-100 opacity-70') : 'scale-[1.025] opacity-0',
            )}
            onLoad={() => setImgLoaded(true)}
            onError={(event) => {
              if (!backdropFailed) {
                setImgLoaded(false)
                setFailedBackdropKey(backdropAttemptKey)
                return
              }
              event.currentTarget.style.display = 'none'
            }}
          />
        </div>
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />
        <div className="absolute inset-0 opacity-90" style={{ background: 'radial-gradient(circle at 78% 15%, var(--nv-ambient-purple-soft), transparent 32rem)' }} />
      </div>

      <div className="nv-detail-hero-inner relative mx-auto grid min-h-[clamp(28rem,48vw,42rem)] w-full max-w-[var(--nv-content-max)] items-end gap-6 px-[var(--nv-page-gutter)] pb-8 pt-24 sm:grid-cols-[12rem_minmax(0,1fr)] sm:pb-10 lg:grid-cols-[14rem_minmax(0,1fr)] lg:gap-8">
        <div className="hidden sm:block">
          <MediaArtwork
            src={streamApi.getPosterUrl(media.id, effectivePosterVersion)}
            alt={media.title}
            ratio="poster"
            loading="eager"
            className="nv-detail-poster w-full shadow-[var(--nv-shadow-card)] transition-[border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:shadow-[var(--nv-shadow-card-hover)]"
          />
        </div>

        <HeroContent
          className="pb-1"
          eyebrow={episodeEyebrow}
          title={title}
          subtitle={subtitle}
          meta={(
            <>
              {media.rating > 0 && (
                <span className="inline-flex items-center gap-1 font-semibold text-[var(--nv-status-rating)]">
                  <Star size={13} fill="currentColor" aria-hidden="true" />
                  {media.rating.toFixed(1)}
                </span>
              )}
              {media.year > 0 && <span>{media.year}</span>}
              {media.duration > 0 && <span>{formatDuration(media.duration)}</span>}
              {media.resolution && <Tag tone="quality">{media.resolution}</Tag>}
              {media.genres && media.genres.split(',').slice(0, 3).map((genre) => (
                <Link
                  key={genre}
                  to={`/search?q=${encodeURIComponent(genre.trim())}`}
                  className="transition-opacity hover:opacity-80"
                >
                  <Tag>{genre.trim()}</Tag>
                </Link>
              ))}
              {media.video_codec && <Tag>{media.video_codec}</Tag>}
              {playStatus && <Tag tone={playStatus.tone}>{playStatus.label}</Tag>}
              {media.country && <span>{media.country}</span>}
            </>
          )}
          overview={media.overview || undefined}
          supplemental={watchProgress && watchDuration > 0 ? (
            <div className="nv-detail-hero-watch-progress" aria-label={`已观看 ${watchPercent}%`}>
              <div className="nv-detail-hero-watch-progress-label">
                上次观看至 {formatTime(watchProgress.position)} / {formatTime(watchDuration)}
              </div>
              <div className="nv-detail-hero-watch-progress-track">
                <span style={{ width: `${watchPercent}%` }} />
              </div>
            </div>
          ) : undefined}
          actions={heroActions}
        />
      </div>
    </section>
  )
}
