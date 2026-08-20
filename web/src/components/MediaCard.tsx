import { Link, useNavigate } from 'react-router-dom'
import { Film, Play, Star, Tv } from 'lucide-react'
import { streamApi } from '@/api'
import type { Media, Series } from '@/types'
import clsx from 'clsx'
import { usePosterVersion } from '@/stores/mediaRefresh'
import { Button, Tag } from '@/components/design-system'
import { MediaArtwork, type MediaArtworkRatio } from '@/ui'

export type MediaCardVariant = 'poster' | 'landscape' | 'compact' | 'recommendation'

interface MediaCardProps {
  media?: Media
  series?: Series
  eyebrow?: string
  className?: string
  variant?: MediaCardVariant
  showBadges?: boolean
}

export default function MediaCard({
  media,
  series,
  eyebrow,
  className,
  variant = 'poster',
  showBadges = true,
}: MediaCardProps) {
  const navigate = useNavigate()
  const posterVersion = usePosterVersion()

  const isSeries = !!series || !!media?.series_id
  const seriesData = series || media?.series
  const isLandscape = variant === 'landscape' || variant === 'compact'
  const detailTo = series
    ? `/series/${series.id}`
    : media!.series_id
      ? `/series/${media!.series_id}`
      : `/media/${media!.id}`
  const playTo = series
    ? `/series/${series.id}`
    : media!.series_id
      ? `/series/${media!.series_id}`
      : `/play/${media!.id}`
  const title = series ? series.title : media!.title
  const year = series ? series.year : media!.year
  const rating = series ? series.rating : media!.rating
  const posterUrl = series
    ? streamApi.getSeriesPosterUrl(series.id, posterVersion)
    : media!.series_id
      ? streamApi.getSeriesPosterUrl(media!.series_id, posterVersion)
      : streamApi.getPosterUrl(media!.id, posterVersion)
  const hasPoster = series
    ? !!series.poster_path
    : media!.series_id
      ? !!media!.series?.poster_path || !!media!.poster_path
      : !!media!.poster_path
  const backdropUrl = series
    ? streamApi.getSeriesBackdropUrl(series.id)
    : media!.series_id
      ? streamApi.getSeriesBackdropUrl(media!.series_id)
      : streamApi.getBackdropUrl(media!.id)
  const hasBackdrop = series
    ? !!series.backdrop_path
    : media!.series_id
      ? !!media!.series?.backdrop_path || !!media!.backdrop_path
      : !!media!.backdrop_path
  const artworkUrl = isLandscape && hasBackdrop ? backdropUrl : posterUrl
  const hasArtwork = isLandscape && hasBackdrop ? true : hasPoster

  const artworkRatio: MediaArtworkRatio = isLandscape ? 'landscape' : 'poster'

  const formatDuration = (seconds: number) => {
    if (!seconds) return ''
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    return h > 0 ? `${h}h ${m}m` : `${m}m`
  }

  return (
    <article className={clsx('nv-media-card group', className)} data-variant={variant}>
      <MediaArtwork
        src={hasArtwork ? artworkUrl : null}
        fallbackSrc={isLandscape && hasBackdrop && hasPoster ? posterUrl : undefined}
        ratio={artworkRatio}
        className="nv-media-card-poster"
        imageClassName="nv-media-card-image"
        fallback={(
          <div className="flex flex-col items-center justify-center gap-2 text-[var(--nv-text-tertiary)]">
            {isSeries ? <Tv size={24} aria-hidden="true" /> : <Film size={24} aria-hidden="true" />}
            <span className="text-[10px]">暂无海报</span>
          </div>
        )}
      >
        <Link
          to={detailTo}
          className="absolute inset-0 z-10 rounded-[inherit]"
          aria-label={`查看 ${title} 详情`}
        />

        <div className="nv-media-card-overlay z-20 pointer-events-none">
          <Button
            variant="primary"
            size="sm"
            iconOnly
            className="nv-media-card-play pointer-events-auto"
            onClick={() => navigate(playTo)}
            aria-label={isSeries ? `查看系列 ${title}` : `播放 ${title}`}
            title={isSeries ? '查看系列' : '立即播放'}
          >
            {isSeries
              ? <Tv size={16} aria-hidden="true" />
              : <Play size={16} fill="currentColor" aria-hidden="true" />}
          </Button>
        </div>

        {showBadges && eyebrow && (
          <Tag
            tone="quality"
            className="nv-media-card-badge absolute left-2 top-2 z-30 max-w-[calc(100%-4rem)] truncate"
            title={eyebrow}
          >
            {eyebrow}
          </Tag>
        )}

        {showBadges && !isSeries && media!.resolution && (
          <Tag tone="quality" className="nv-media-card-badge absolute right-2 top-2 z-30">
            {media!.resolution}
          </Tag>
        )}
      </MediaArtwork>

      <div className="pb-1 pt-2">
        <Link to={detailTo} className="nv-media-card-title" title={title}>
          {title}
        </Link>
        <div className="nv-media-card-meta mt-1 flex min-w-0 items-center gap-1.5 overflow-hidden">
          {year > 0 && <span className="shrink-0">{year}</span>}
          {rating > 0 && (
            <>
              {year > 0 && <span aria-hidden="true">·</span>}
              <span className="flex shrink-0 items-center gap-1">
                <Star size={10} fill="currentColor" aria-hidden="true" />
                {rating.toFixed(1)}
              </span>
            </>
          )}
          {!isSeries && media!.duration > 0 && (
            <>
              {(year > 0 || rating > 0) && <span aria-hidden="true">·</span>}
              <span className="shrink-0">{formatDuration(media!.duration)}</span>
            </>
          )}
          {isSeries && seriesData?.episode_count ? (
            <>
              {(year > 0 || rating > 0) && <span aria-hidden="true">·</span>}
              <span className="shrink-0">{seriesData.episode_count} 集</span>
            </>
          ) : null}
        </div>
      </div>
    </article>
  )
}
