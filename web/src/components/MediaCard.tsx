import { Link, useNavigate } from 'react-router-dom'
import { Film, Play, Star, Tv } from 'lucide-react'
import { streamApi } from '@/api'
import type { Media, Series } from '@/types'
import { useState } from 'react'
import clsx from 'clsx'
import { usePosterVersion } from '@/stores/mediaRefresh'
import { Button, Tag } from '@/components/design-system'

interface MediaCardProps {
  media?: Media
  series?: Series
  eyebrow?: string
  className?: string
}

export default function MediaCard({ media, series, eyebrow, className }: MediaCardProps) {
  const navigate = useNavigate()
  const posterVersion = usePosterVersion()
  const [posterFailed, setPosterFailed] = useState(false)

  const isSeries = !!series || !!media?.series_id
  const seriesData = series || media?.series
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

  const formatDuration = (seconds: number) => {
    if (!seconds) return ''
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    return h > 0 ? `${h}h ${m}m` : `${m}m`
  }

  return (
    <article className={clsx('nv-media-card group', className)}>
      <div className="nv-media-card-poster isolate">
        {hasPoster && !posterFailed ? (
          <img
            src={posterUrl}
            alt=""
            loading="lazy"
            onLoad={() => setPosterFailed(false)}
            onError={() => setPosterFailed(true)}
          />
        ) : (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 bg-[var(--nv-bg-poster)] text-[var(--nv-text-tertiary)]">
            {isSeries ? <Tv size={24} aria-hidden="true" /> : <Film size={24} aria-hidden="true" />}
            <span className="text-[10px]">暂无海报</span>
          </div>
        )}

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

        {eyebrow && (
          <Tag
            tone="quality"
            className="nv-media-card-badge absolute left-2 top-2 z-30 max-w-[calc(100%-4rem)] truncate"
            title={eyebrow}
          >
            {eyebrow}
          </Tag>
        )}

        {!isSeries && media!.resolution && (
          <Tag tone="quality" className="nv-media-card-badge absolute right-2 top-2 z-30">
            {media!.resolution}
          </Tag>
        )}
      </div>

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
