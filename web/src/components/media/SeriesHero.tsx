import { useState, type ReactNode } from 'react'
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
    <section className="relative border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
      <div className="absolute inset-x-0 top-0 h-[clamp(20rem,42vw,34rem)] overflow-hidden">
        {series.backdrop_path ? (
          <img
            key={`series-backdrop-${series.id}-${posterVersion}`}
            src={streamApi.getSeriesBackdropUrl(series.id, posterVersion)}
            alt=""
            className={`h-full w-full object-cover transition-opacity duration-200 ${imageLoaded ? 'opacity-55' : 'opacity-0'}`}
            onLoad={() => setImageLoaded(true)}
          />
        ) : series.poster_path ? (
          <img
            key={`series-backdrop-poster-${series.id}-${posterVersion}`}
            src={streamApi.getSeriesPosterUrl(series.id, posterVersion)}
            alt=""
            className="h-full w-full scale-105 object-cover opacity-20 blur-2xl"
          />
        ) : null}
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />
      </div>

      <div className="relative mx-auto grid min-h-[clamp(24rem,48vw,39rem)] w-full max-w-[var(--nv-content-max)] items-end gap-6 px-[var(--nv-page-gutter)] pb-8 pt-24 sm:grid-cols-[11rem_minmax(0,1fr)] lg:grid-cols-[12rem_minmax(0,1fr)] lg:gap-8">
        <div className="hidden sm:block">
          <div className="aspect-[2/3] overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-poster)] shadow-[var(--nv-shadow-card)]">
            {series.poster_path ? (
              <img
                key={`series-poster-${series.id}-${posterVersion}`}
                src={streamApi.getSeriesPosterUrl(series.id, posterVersion)}
                alt={series.title}
                className="h-full w-full object-cover"
                loading="eager"
              />
            ) : (
              <div className="flex h-full items-center justify-center text-[var(--nv-text-tertiary)]">
                <Tv size={32} aria-hidden="true" />
              </div>
            )}
          </div>
        </div>

        <div className="min-w-0 pb-1">
          <div className="mb-3 flex flex-wrap items-center gap-1.5">
            <Tag>剧集</Tag>
            {series.rating > 0 && <Tag tone="rating">★ {series.rating.toFixed(1)}</Tag>}
            {series.year > 0 && <Tag>{series.year}</Tag>}
            <Tag>{series.season_count} 季 · {series.episode_count} 集</Tag>
          </div>

          <h1 className="max-w-[24ch] text-[var(--nv-type-h1)] font-semibold leading-[var(--nv-line-tight)] tracking-[var(--nv-tracking-tight)] text-[var(--nv-text-primary)]">
            {series.title}
          </h1>
          {series.orig_title && series.orig_title !== series.title && (
            <p className="mt-2 max-w-3xl text-sm text-[var(--nv-text-secondary)]">{series.orig_title}</p>
          )}

          {genres.length > 0 && (
            <div className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1.5 text-xs text-[var(--nv-text-tertiary)]">
              {genres.slice(0, 4).map((genre, index) => (
                <span key={genre} className="inline-flex items-center gap-2">
                  {index > 0 && <span aria-hidden="true">·</span>}
                  <Link to={`/search?q=${encodeURIComponent(genre)}`} className="hover:text-[var(--nv-text-primary)]">{genre}</Link>
                </span>
              ))}
            </div>
          )}

          <div className="mt-5 flex flex-wrap items-center gap-2">
            {firstEpisode && (
              <Link to={`/play/${firstEpisode.id}`} className={buttonClassName({ variant: 'primary', size: 'lg' })} data-variant="primary" data-size="lg">
                <Play size={17} fill="currentColor" aria-hidden="true" />
                播放第一集
              </Link>
            )}

            <Button type="button" variant="secondary" size="lg" iconOnly onClick={onFavorite} title={isFavorited ? '取消收藏' : '收藏'} aria-label={isFavorited ? '取消收藏' : '收藏'} aria-pressed={isFavorited}>
              <Heart size={18} fill={isFavorited ? 'currentColor' : 'none'} aria-hidden="true" />
            </Button>

            <div className="relative">
              <Button type="button" variant="ghost" size="lg" iconOnly onClick={() => setMenuOpen((open) => !open)} aria-label="更多操作" aria-expanded={menuOpen}>
                <MoreHorizontal size={19} aria-hidden="true" />
              </Button>

              {menuOpen && (
                <div className="nv-menu absolute left-0 top-full z-[var(--nv-z-dropdown)] mt-2 w-56" role="menu">
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

      {menuOpen && <button type="button" className="fixed inset-0 z-[59] cursor-default" aria-label="关闭菜单" onClick={() => setMenuOpen(false)} />}
    </section>
  )
}

function MenuItem({ icon, label, onClick, disabled = false, danger = false }: { icon: ReactNode; label: string; onClick: () => void; disabled?: boolean; danger?: boolean }) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`nv-menu-item disabled:cursor-not-allowed disabled:opacity-50 ${danger ? '!text-[var(--nv-status-danger)]' : ''}`}
      role="menuitem"
    >
      {icon}
      <span>{label}</span>
    </button>
  )
}
