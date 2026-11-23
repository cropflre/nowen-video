import { useAuthStore } from '@/stores/auth'
import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import { usePosterVersion } from '@/stores/mediaRefresh'
import { useToast } from '@/components/Toast'
import { Button, buttonClassName, Modal, ModalBody, ModalHeader } from '@/components/design-system'
import CommentSection from '@/components/CommentSection'
import { commentApi } from '@/api'
import { HeroContent, MediaArtwork } from '@/ui'
import { useTranslation } from '@/i18n'
import { episodeLabel, resumePosition } from '@/utils/viewing'
import { formatDuration, formatDurationShort, getDisplayDurationSeconds } from '@/utils/format'
import type { Media, MediaPlayInfo, Playlist, WatchHistory } from '@/types'
import {
  Captions,
  Cpu,
  ExternalLink,
  MessageSquare,
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
  ShieldCheck,
  Share2,
  Star,
  Trash2,
  Unlink,
} from 'lucide-react'
import clsx from 'clsx'
import DetailActionMenu from './DetailActionMenu'
import FamilyRatingPopover, { familyRatingLabel, useSavedFamilyRating } from './FamilyRatingMenu'

interface HeroSectionProps {
  media: Media
  playInfo: MediaPlayInfo | null
  isFavorited: boolean
  watchProgress: WatchHistory | null
  playlists: Playlist[]
  scraping: boolean
  isAdmin: boolean
  /** 字幕来源数；null=加载中（不显示），0=无字幕 */
  subtitleCount?: number | null
  onSubtitleClick?: () => void
  /** 技术信息入口：label 如 “1080p · H.264”，点击由页面打开详情弹层 */
  techLabel?: string
  onShowTech?: () => void
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

const menuItemClassName = 'flex w-full items-center gap-2.5 px-3 py-2.5 text-left text-sm text-[var(--nv-text-secondary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:bg-[var(--nv-bg-hover)]'

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
  isFavorited,
  watchProgress,
  playlists,
  scraping,
  isAdmin,
  subtitleCount = null,
  onSubtitleClick,
  techLabel,
  onShowTech,
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
  const { t, locale } = useTranslation()
  const childMode = useAuthStore((state) => state.user?.child_mode ?? false)
  const [imgLoaded, setImgLoaded] = useState(false)
  const [failedBackdropKey, setFailedBackdropKey] = useState<string | null>(null)
  const [showMoreMenu, setShowMoreMenu] = useState(false)
  const [showRating, setShowRating] = useState(false)
  const [showComments, setShowComments] = useState(false)
  const [commentSummary, setCommentSummary] = useState<{ total: number; avgRating: number; ratingCount: number } | null>(null)
  useEffect(() => {
    setCommentSummary(null)
    let cancelled = false
    void commentApi.listByMedia(media.id, 1, 1).then((res) => {
      if (!cancelled) setCommentSummary({ total: res.data.total, avgRating: res.data.avg_rating, ratingCount: res.data.rating_count })
    }).catch(() => {})
    return () => { cancelled = true }
  }, [media.id])
  const [savedRating, setSavedRating] = useSavedFamilyRating(media.id, isAdmin && !childMode)
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
    setShowMoreMenu(false)
  }

  const title = media.media_type === 'episode'
    ? (media.episode_title || t('hero.episodeNum', { num: String(media.episode_num) }))
    : media.title

  const isResume = !!watchProgress && resumePosition(watchProgress) > 0
  // 展示用时长：文件实际时长（秒）优先，缺失时用刮削片长（分钟）换算。
  // 详情区用同一值，不再各说各话。
  const displaySeconds = getDisplayDurationSeconds(media.duration, media.runtime)
  const playLabel = isResume
    ? t('hero.continuePlayAt', { time: formatDurationShort(watchProgress.position) })
    : t('media.play')

  const genreList = (media.genres || '').split(',').map((value) => value.trim()).filter(Boolean)

  const [overviewExpanded, setOverviewExpanded] = useState(false)
  const isLongOverview = (media.overview?.length || 0) > 200

  const episodeEyebrow = media.media_type === 'episode' && media.series_id ? (
    <Link
      to={`/series/${media.series_id}`}
      className="inline-flex min-w-0 items-center gap-1.5 text-sm font-medium text-[var(--nv-text-secondary)] transition-colors hover:text-[var(--nv-action-muted-hover)]"
    >
      <span className="truncate">{media.series?.title || media.title}</span>
      <ChevronRight size={14} aria-hidden="true" />
      <span className="shrink-0 text-[var(--nv-action-muted-hover)]">
        {episodeLabel(media.season_num, media.episode_num, locale)}
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


      <Button
        variant="secondary"
        size="lg"
        onClick={onFavorite}
        className={isFavorited ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-muted-hover)]' : undefined}
        title={isFavorited ? t('media.removeFavorite') : t('media.addFavorite')}
        aria-label={isFavorited ? t('media.removeFavorite') : t('media.addFavorite')}
        aria-pressed={isFavorited}
      >
        <Heart size={19} fill={isFavorited ? 'currentColor' : 'none'} aria-hidden="true" />
        {isFavorited ? t('media.removeFavorite') : t('nav.favorites')}
      </Button>

      <Button
        ref={moreButtonRef}
        variant="secondary"
        size="lg"
        iconOnly
        onClick={() => {
          setShowMoreMenu((value) => !value)
        }}
        title={t('hero.moreActions')}
        aria-label={t('hero.moreActions')}
        aria-haspopup="menu"
        aria-expanded={showMoreMenu}
      >
        <MoreHorizontal size={19} aria-hidden="true" />
      </Button>

      <DetailActionMenu
        open={showMoreMenu}
        anchorRef={moreButtonRef}
        ariaLabel={t('hero.moreActions')}
        onClose={() => setShowMoreMenu(false)}
      >
        <div className="px-3 pb-1.5 pt-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--nv-text-tertiary)]">{t('hero.viewingActions')}</div>
        {isResume && <Link to={`/play/${media.id}?start=0`} className={menuItemClassName} role="menuitem">从头播放</Link>}
        {media.trailer_url && onShowTrailer && <button onClick={() => { onShowTrailer(); setShowMoreMenu(false) }} className={menuItemClassName} role="menuitem"><Clapperboard size={14} aria-hidden="true" />{t('media.trailer')}</button>}
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
        {!childMode && (
          <>
            <div className="px-3 pb-1.5 pt-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--nv-text-tertiary)]">{t('hero.addToPlaylist')}</div>
            {playlists.length === 0 ? (
              <div className="px-3 py-2.5 text-sm text-[var(--nv-text-tertiary)]">{t('hero.noPlaylists')}</div>
            ) : playlists.map((playlist) => (
              <button key={playlist.id} onClick={() => handleAddToPlaylist(playlist.id)} className={menuItemClassName} role="menuitem">
                <ListPlus size={14} aria-hidden="true" />
                <span className="min-w-0 flex-1 truncate">{playlist.name}</span>
                {playlist.items?.some((playlistItem) => playlistItem.media_id === media.id) && (
                  <Check size={14} className="text-[var(--nv-action-muted-hover)]" aria-hidden="true" />
                )}
              </button>
            ))}
          </>
        )}
        {isAdmin && (
          <>
            <div className="px-3 pb-1.5 pt-2 text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--nv-text-tertiary)]">{t('hero.mediaManagement')}</div>
            {!childMode && (
              <button onClick={() => { setShowMoreMenu(false); setShowRating(true) }} className={menuItemClassName} role="menuitem">
                <ShieldCheck size={14} aria-hidden="true" />
                <span className="min-w-0 flex-1 truncate">{familyRatingLabel(savedRating)}</span>
                {savedRating === '' && <span aria-hidden="true" className="h-2 w-2 shrink-0 rounded-full" style={{ background: 'var(--nv-status-warning)' }} />}
              </button>
            )}
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
            <button onClick={() => { copyFilePath(); setShowMoreMenu(false) }} className={menuItemClassName} role="menuitem">
              <Copy size={14} aria-hidden="true" /> {t('hero.copyFilePath')}
            </button>
          </>
        )}
      </DetailActionMenu>

      <Modal open={showComments} onClose={() => setShowComments(false)} size="md" ariaLabel="评价">
        <ModalHeader
          title="评价"
          description={commentSummary && commentSummary.total > 0 ? `${commentSummary.total} 条评价${commentSummary.ratingCount > 0 ? ` · ${commentSummary.avgRating.toFixed(1)} 分` : ''}` : '暂无评价 · 可评分或留言'}
          onClose={() => setShowComments(false)}
        />
        <ModalBody>
          <CommentSection mediaId={media.id} embedded onSummaryChange={setCommentSummary} />
        </ModalBody>
      </Modal>

      {isAdmin && !childMode && (
        <FamilyRatingPopover
          id={media.id}
          mpaa={media.mpaa}
          tmdbRatingData={media.tmdb_rating_data}
          open={showRating}
          anchorRef={moreButtonRef}
          onClose={() => setShowRating(false)}
          onConfirmedChange={setSavedRating}
        />
      )}
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

      <div className="nv-detail-hero-inner relative mx-auto grid w-full max-w-[var(--nv-content-max)] items-end gap-6 px-[var(--nv-page-gutter)] pb-8 pt-24 sm:grid-cols-[12rem_minmax(0,1fr)] sm:pb-10 lg:grid-cols-[14rem_minmax(0,1fr)] lg:gap-8">
        <div className="nv-detail-poster-slot">
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
              {displaySeconds > 0 && <span>{formatDuration(displaySeconds)}</span>}
              {genreList.map((genre) => (
                <Link
                  key={genre}
                  to={`/search?q=${encodeURIComponent(genre)}`}
                  className="rounded-full border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)]/65 px-2.5 py-0.5 text-xs text-[var(--nv-text-primary)]/85 backdrop-blur-sm transition-colors hover:border-[var(--nv-border-hover)] hover:text-[var(--nv-text-primary)]"
                >
                  {genre}
                </Link>
              ))}
              {(subtitleCount !== null || onShowTech || commentSummary || media.tmdb_id > 0 || media.douban_id || media.bangumi_id > 0) && (
                <div className="flex w-full flex-wrap items-center gap-2">
                  {subtitleCount !== null && (() => {
                    const label = subtitleCount > 0 ? `字幕 ${subtitleCount}` : '无字幕'
                    const cls = 'inline-flex items-center gap-1 rounded-full border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)]/65 px-2.5 py-0.5 text-xs text-[var(--nv-text-primary)]/85 backdrop-blur-sm'
                    const body = (<><Captions size={12} aria-hidden="true" />{label}</>)
                    return onSubtitleClick ? (
                      <button
                        type="button"
                        onClick={onSubtitleClick}
                        title={subtitleCount > 0 ? '管理字幕' : '搜索字幕'}
                        className={clsx(cls, 'transition-colors hover:border-[var(--nv-border-hover)] hover:text-[var(--nv-text-primary)]')}
                      >
                        {body}
                      </button>
                    ) : (
                      <span className={cls} title="播放时可选择或关闭字幕">{body}</span>
                    )
                  })()}
                  {onShowTech && (
                    <button
                      type="button"
                      onClick={onShowTech}
                      title="查看技术信息"
                      className="inline-flex items-center gap-1 rounded-full border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)]/65 px-2.5 py-0.5 text-xs text-[var(--nv-text-primary)]/85 backdrop-blur-sm transition-colors hover:border-[var(--nv-border-hover)] hover:text-[var(--nv-text-primary)]"
                    >
                      <Cpu size={12} aria-hidden="true" />
                      {techLabel || '技术信息'}
                    </button>
                  )}
                  {commentSummary && (
                    <button
                      type="button"
                      onClick={() => setShowComments(true)}
                      title="查看或发表评价"
                      className="inline-flex items-center gap-1 rounded-full border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)]/65 px-2.5 py-0.5 text-xs text-[var(--nv-text-primary)]/85 backdrop-blur-sm transition-colors hover:border-[var(--nv-border-hover)] hover:text-[var(--nv-text-primary)]"
                    >
                      <MessageSquare size={12} aria-hidden="true" />
                      {commentSummary.total > 0 ? `评价 ${commentSummary.total}` : '写评价'}
                    </button>
                  )}
                  {media.tmdb_id > 0 && (
                    <a
                      href={`https://www.themoviedb.org/${media.media_type === 'episode' ? 'tv' : 'movie'}/${media.tmdb_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 rounded-full border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)]/65 px-2.5 py-0.5 text-xs text-[var(--nv-text-primary)]/85 backdrop-blur-sm transition-colors hover:border-[var(--nv-border-hover)] hover:text-[var(--nv-text-primary)]"
                    >
                      TMDb #{media.tmdb_id}
                      <ExternalLink size={10} aria-hidden="true" />
                    </a>
                  )}
                  {media.douban_id && (
                    <a
                      href={`https://movie.douban.com/subject/${media.douban_id}/`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 rounded-full border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)]/65 px-2.5 py-0.5 text-xs text-[var(--nv-text-primary)]/85 backdrop-blur-sm transition-colors hover:border-[var(--nv-border-hover)] hover:text-[var(--nv-text-primary)]"
                    >
                      豆瓣 #{media.douban_id}
                      <ExternalLink size={10} aria-hidden="true" />
                    </a>
                  )}
                  {media.bangumi_id > 0 && (
                    <a
                      href={`https://bgm.tv/subject/${media.bangumi_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 rounded-full border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)]/65 px-2.5 py-0.5 text-xs text-[var(--nv-text-primary)]/85 backdrop-blur-sm transition-colors hover:border-[var(--nv-border-hover)] hover:text-[var(--nv-text-primary)]"
                    >
                      Bangumi #{media.bangumi_id}
                      <ExternalLink size={10} aria-hidden="true" />
                    </a>
                  )}
                </div>
              )}
            </>
          )}
          overview={media.overview || undefined}
          overviewToggle={isLongOverview ? {
            expanded: overviewExpanded,
            onToggle: () => setOverviewExpanded((value) => !value),
            expandLabel: t('mediaInfo.expandAll'),
            collapseLabel: t('mediaInfo.collapse'),
          } : undefined}
          actions={heroActions}
        />
      </div>
    </section>
  )
}
