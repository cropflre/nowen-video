import { useEffect, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { Heart, Link2, MoreHorizontal, Play, Share2, Trash2, Tv, Unlink } from 'lucide-react'
import type { Media, Series } from '@/types'
import { streamApi } from '@/api'
import { Button, Tag, buttonClassName } from '@/components/design-system'
import { HeroContent, MediaArtwork } from '@/ui'

interface SeriesHeroProps {
  series: Series
  playEpisode: Media | null
  playLabel: string
  isFavorited: boolean
  isAdmin: boolean
  posterVersion: number
  onFavorite: () => void
  onManualMatch: () => void
  onUnmatch: () => void
  onDelete: () => void
  onShare: () => void
}

export default function SeriesHero({
  series,
  playEpisode,
  playLabel,
  isFavorited,
  isAdmin,
  posterVersion,
  onFavorite,
  onManualMatch,
  onUnmatch,
  onDelete,
  onShare,
}: SeriesHeroProps) {
  const [imageLoaded, setImageLoaded] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const genres = (series.genres || '').split(',').map((item) => item.trim()).filter(Boolean)

  useEffect(() => {
    setImageLoaded(false)
  }, [posterVersion, series.backdrop_path, series.id])

  const closeAndRun = (action: () => void) => {
    setMenuOpen(false)
    action()
  }

  const actions = (
    <>
      {playEpisode && (
        <Link to={`/play/${playEpisode.id}`} className={buttonClassName({ variant: 'primary', size: 'lg' })} data-variant="primary" data-size="lg">
          <Play size={17} fill="currentColor" aria-hidden="true" />
          {playLabel}
        </Link>
      )}

      <Button type="button" variant="secondary" size="lg" iconOnly onClick={onFavorite} disabled={!playEpisode} title={isFavorited ? '取消收藏' : '收藏'} aria-label={isFavorited ? '取消收藏' : '收藏'} aria-pressed={isFavorited}>
        <Heart size={18} fill={isFavorited ? 'currentColor' : 'none'} aria-hidden="true" />
      </Button>

      <div className="relative">
        <Button type="button" variant="secondary" size="lg" iconOnly onClick={() => setMenuOpen((open) => !open)} aria-label="更多操作" aria-expanded={menuOpen}>
          <MoreHorizontal size={19} aria-hidden="true" />
        </Button>

        {menuOpen && (
          <div className="nv-menu absolute left-0 top-full z-[var(--nv-z-dropdown)] mt-2 w-56" role="menu">
            {isAdmin && (
              <>
                <div className="px-2.5 py-1.5 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--nv-text-tertiary)]">剧集管理</div>
                <MenuItem icon={<Link2 size={14} />} label="手动匹配剧集" onClick={() => closeAndRun(onManualMatch)} />
                <MenuItem icon={<Unlink size={14} />} label="解除匹配剧集" onClick={() => closeAndRun(onUnmatch)} />
                <MenuItem icon={<Trash2 size={14} />} label="删除剧集" onClick={() => closeAndRun(onDelete)} danger />
                <div className="my-1 h-px bg-[var(--nv-border-subtle)]" />
              </>
            )}
            <MenuItem icon={<Share2 size={14} />} label="分享链接" onClick={() => closeAndRun(onShare)} />
          </div>
        )}
      </div>
    </>
  )

  return (
    <section className="nv-detail-hero nv-series-hero relative border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
      <div className="nv-series-backdrop absolute inset-0 overflow-hidden" aria-hidden="true">
        {series.backdrop_path ? (
          <img
            key={`series-backdrop-${series.id}-${posterVersion}`}
            src={streamApi.getSeriesBackdropUrl(series.id, posterVersion)}
            alt=""
            className={`h-full w-full object-cover object-center transition-opacity duration-300 ${imageLoaded ? 'opacity-100' : 'opacity-0'}`}
            onLoad={() => setImageLoaded(true)}
          />
        ) : series.poster_path ? (
          <img
            key={`series-backdrop-poster-${series.id}-${posterVersion}`}
            src={streamApi.getSeriesPosterUrl(series.id, posterVersion)}
            alt=""
            className="h-full w-full scale-110 object-cover opacity-30 blur-2xl"
          />
        ) : null}
        <div className="nv-series-hero-reading-scrim absolute inset-0" />
        <div className="nv-series-hero-edge-scrim absolute inset-0" />
      </div>

      <div className="nv-detail-hero-inner nv-series-hero-inner relative mx-auto grid w-full max-w-[var(--nv-content-max)] items-center gap-6 px-[var(--nv-page-gutter)] sm:grid-cols-[11rem_minmax(0,1fr)] lg:grid-cols-[12rem_minmax(0,1fr)] lg:gap-8">
        <div className="hidden sm:block">
          <MediaArtwork
            src={series.poster_path ? streamApi.getSeriesPosterUrl(series.id, posterVersion) : null}
            alt={series.title}
            ratio="poster"
            loading="eager"
            fallback={<Tv size={32} aria-hidden="true" />}
            className="nv-detail-poster nv-series-poster shadow-[var(--nv-shadow-card)]"
          />
        </div>

        <HeroContent
          compact
          className="nv-series-hero-content"
          badges={(
            <>
              <Tag>剧集</Tag>
              {series.rating > 0 && <Tag tone="rating">★ {series.rating.toFixed(1)}</Tag>}
              {series.year > 0 && <Tag>{series.year}</Tag>}
              <Tag>{series.season_count} 季 · {series.episode_count} 集</Tag>
            </>
          )}
          title={series.title}
          subtitle={series.orig_title && series.orig_title !== series.title ? series.orig_title : undefined}
          meta={genres.slice(0, 4).map((genre) => (
            <Link key={genre} to={`/search?q=${encodeURIComponent(genre)}`} className="hover:text-[var(--nv-text-primary)]">
              {genre}
            </Link>
          ))}
          overview={series.overview || undefined}
          actions={actions}
        />
      </div>

      {menuOpen && <button type="button" className="fixed inset-0 z-[59] cursor-default" aria-label="关闭菜单" onClick={() => setMenuOpen(false)} />}
    </section>
  )
}

function MenuItem({ icon, label, onClick, danger = false }: { icon: ReactNode; label: string; onClick: () => void; danger?: boolean }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`nv-menu-item ${danger ? '!text-[var(--nv-status-danger)]' : ''}`}
      role="menuitem"
    >
      {icon}
      <span>{label}</span>
    </button>
  )
}
