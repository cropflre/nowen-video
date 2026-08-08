import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Heart, Link2, MoreHorizontal, Pencil, Play, RefreshCw, Share2, Trash2, Tv, Unlink } from 'lucide-react'
import type { Media, Series } from '@/types'
import { streamApi } from '@/api'
import { Button, Tag, buttonClassName } from '@/components/design-system'

interface SeriesHeroProps {
  series: Series
  firstEpisode: Media | null
  isFavorited: boolean
  isAdmin: boolean
  scraping: boolean
  posterVersion: number
  onFavorite: () => void
  onManualMatch: () => void
  onUnmatch: () => void
  onRefreshMetadata: () => void
  onEditMetadata: () => void
  onDelete: () => void
  onShare: () => void
}

export default function SeriesHero({
  series,
  firstEpisode,
  isFavorited,
  isAdmin,
  scraping,
  posterVersion,
  onFavorite,
  onManualMatch,
  onUnmatch,
  onRefreshMetadata,
  onEditMetadata,
  onDelete,
  onShare,
}: SeriesHeroProps) {
  const [imageLoaded, setImageLoaded] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)

  const genres = (series.genres || '').split(',').map((item) => item.trim()).filter(Boolean)

  const closeAndRun = (action: () => void) => {
    setMenuOpen(false)
    action()
  }

  return (
    <section className="relative overflow-hidden border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
      <div className="absolute inset-0 overflow-hidden">
        {series.backdrop_path ? (
          <img
            key={`series-backdrop-${series.id}-${posterVersion}`}
            src={streamApi.getSeriesBackdropUrl(series.id, posterVersion)}
            alt=""
            className={`h-full w-full object-cover transition-[opacity,transform] duration-700 ${imageLoaded ? 'scale-100 opacity-55' : 'scale-[1.02] opacity-0'}`}
            onLoad={() => setImageLoaded(true)}
          />
        ) : series.poster_path ? (
          <img
            key={`series-backdrop-poster-${series.id}-${posterVersion}`}
            src={streamApi.getSeriesPosterUrl(series.id, posterVersion)}
            alt=""
            className="h-full w-full scale-110 object-cover opacity-20 blur-2xl"
          />
        ) : null}
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />
      </div>

      <div className="relative mx-auto flex min-h-[30rem] max-w-7xl items-end gap-6 px-4 pb-8 pt-24 sm:px-6 lg:px-8">
        <div className="hidden w-52 shrink-0 overflow-hidden rounded-[var(--nv-radius-hero)] border border-[var(--nv-border-strong)] bg-[var(--nv-bg-surface)] shadow-[var(--nv-shadow-elevated)] sm:block">
          {series.poster_path ? (
            <img
              key={`series-poster-${series.id}-${posterVersion}`}
              src={streamApi.getSeriesPosterUrl(series.id, posterVersion)}
              alt={series.title}
              className="aspect-[2/3] w-full object-cover"
              loading="eager"
            />
          ) : (
            <div className="flex aspect-[2/3] items-center justify-center text-[var(--nv-text-tertiary)]">
              <Tv size={48} aria-hidden="true" />
            </div>
          )}
        </div>

        <div className="min-w-0 flex-1 pb-1">
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <Tag tone="brand">剧集</Tag>
            {series.rating > 0 && <Tag tone="rating">★ {series.rating.toFixed(1)}</Tag>}
            {series.year > 0 && <Tag>{series.year}</Tag>}
            <Tag>{series.season_count} 季 · {series.episode_count} 集</Tag>
          </div>

          <h1 className="max-w-4xl text-3xl font-bold leading-tight tracking-[-0.025em] text-[var(--nv-text-primary)] sm:text-4xl lg:text-5xl">
            {series.title}
          </h1>
          {series.orig_title && series.orig_title !== series.title && (
            <p className="mt-2 max-w-3xl text-sm text-[var(--nv-text-secondary)] sm:text-base">{series.orig_title}</p>
          )}

          {genres.length > 0 && (
            <div className="mt-4 flex flex-wrap gap-2">
              {genres.slice(0, 4).map((genre) => (
                <Link key={genre} to={`/search?q=${encodeURIComponent(genre)}`} className="no-underline">
                  <Tag>{genre}</Tag>
                </Link>
              ))}
            </div>
          )}

          <div className="mt-6 flex flex-wrap items-center gap-2.5">
            {firstEpisode && (
              <Link to={`/play/${firstEpisode.id}`} className={buttonClassName({ variant: 'primary', size: 'lg' })}>
                <Play size={19} fill="currentColor" aria-hidden="true" />
                播放第一集
              </Link>
            )}

            <Button
              type="button"
              variant={isFavorited ? 'primary' : 'secondary'}
              size="lg"
              iconOnly
              onClick={onFavorite}
              title={isFavorited ? '取消收藏' : '收藏'}
              aria-label={isFavorited ? '取消收藏' : '收藏'}
            >
              <Heart size={19} fill={isFavorited ? 'currentColor' : 'none'} aria-hidden="true" />
            </Button>

            <div className="relative">
              <Button
                type="button"
                variant="secondary"
                size="lg"
                iconOnly
                onClick={() => setMenuOpen((open) => !open)}
                aria-label="更多操作"
                aria-expanded={menuOpen}
              >
                <MoreHorizontal size={20} aria-hidden="true" />
              </Button>

              {menuOpen && (
                <div className="absolute left-0 top-full z-[var(--nv-z-dropdown)] mt-2 min-w-56 overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] p-1.5 shadow-[var(--nv-shadow-elevated)]">
                  {isAdmin && (
                    <>
                      <div className="px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--nv-text-tertiary)]">剧集管理</div>
                      <MenuItem icon={<Link2 size={14} />} label="手动匹配剧集" onClick={() => closeAndRun(onManualMatch)} />
                      <MenuItem icon={<Unlink size={14} />} label="解除匹配剧集" onClick={() => closeAndRun(onUnmatch)} />
                      <MenuItem icon={<RefreshCw size={14} className={scraping ? 'animate-spin' : undefined} />} label={scraping ? '刷新中...' : '刷新元数据'} onClick={() => closeAndRun(onRefreshMetadata)} disabled={scraping} />
                      <MenuItem icon={<Pencil size={14} />} label="编辑元数据" onClick={() => closeAndRun(onEditMetadata)} />
                      <MenuItem icon={<Trash2 size={14} />} label="删除剧集" onClick={() => closeAndRun(onDelete)} danger />
                      <div className="my-1 h-px bg-[var(--nv-border-subtle)]" />
                    </>
                  )}
                  <MenuItem icon={<Share2 size={14} />} label="分享链接" onClick={() => closeAndRun(onShare)} />
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {menuOpen && <button type="button" className="fixed inset-0 z-[calc(var(--nv-z-dropdown)-1)] cursor-default" aria-label="关闭菜单" onClick={() => setMenuOpen(false)} />}
    </section>
  )
}

function MenuItem({
  icon,
  label,
  onClick,
  disabled = false,
  danger = false,
}: {
  icon: React.ReactNode
  label: string
  onClick: () => void
  disabled?: boolean
  danger?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`flex w-full items-center gap-2.5 rounded-[var(--nv-radius-control)] px-2.5 py-2 text-left text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${danger ? 'text-[var(--nv-status-danger)] hover:bg-[color-mix(in_srgb,var(--nv-status-danger)_10%,transparent)]' : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]'}`}
    >
      {icon}
      <span>{label}</span>
    </button>
  )
}
