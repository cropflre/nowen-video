import { useMemo, useState } from 'react'
import { Calendar, ChevronDown, ChevronUp, Layers, Star } from 'lucide-react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import type { CollectionWithMedia } from '@/types'
import { Button, Tag } from '@/components/design-system'

interface CollectionDetailHeroProps {
  data: CollectionWithMedia
  movieCount: number
  fileCount: number
  onBack: () => void
}

const COLLAPSED_GENRE_COUNT = 8

export default function CollectionDetailHero({ data, movieCount, fileCount, onBack }: CollectionDetailHeroProps) {
  const { collection, media } = data
  const [genresExpanded, setGenresExpanded] = useState(false)

  const stats = useMemo(() => {
    const years = media.filter((item) => item.year > 0).map((item) => item.year)
    const ratings = media.filter((item) => item.rating > 0).map((item) => item.rating)
    const genres = Array.from(new Set(media.flatMap((item) => (item.genres || '').split(',').map((genre) => genre.trim()).filter(Boolean)))).sort()
    return {
      yearRange: years.length > 0 ? `${Math.min(...years)}${Math.min(...years) === Math.max(...years) ? '' : ` - ${Math.max(...years)}`}` : '',
      averageRating: ratings.length > 0 ? ratings.reduce((sum, value) => sum + value, 0) / ratings.length : 0,
      genres,
    }
  }, [media])

  const visibleGenres = genresExpanded ? stats.genres : stats.genres.slice(0, COLLAPSED_GENRE_COUNT)
  const hiddenGenreCount = Math.max(0, stats.genres.length - COLLAPSED_GENRE_COUNT)

  return (
    <section className="relative overflow-hidden border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
      <div className="absolute inset-0 overflow-hidden">
        <img
          src={streamApi.getCollectionPosterUrl(collection.id)}
          alt=""
          className="h-full w-full scale-110 object-cover opacity-25 blur-3xl"
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />
      </div>

      <div className="relative mx-auto max-w-7xl px-4 pb-8 pt-6 sm:px-6 lg:px-8">
        <Button type="button" variant="secondary" size="sm" onClick={onBack} className="mb-7">
          <span aria-hidden="true">←</span>
          返回
        </Button>

        <div className="flex flex-col gap-6 sm:flex-row sm:items-end">
          <div className="relative h-56 w-40 shrink-0 overflow-hidden rounded-[var(--nv-radius-hero)] border border-[var(--nv-border-strong)] bg-[var(--nv-bg-surface)] shadow-[var(--nv-shadow-elevated)] sm:h-64 sm:w-44">
            <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]"><Layers size={42} /></div>
            <img
              src={streamApi.getCollectionPosterUrl(collection.id)}
              alt={collection.name}
              className="relative h-full w-full object-cover"
              onError={(event) => { event.currentTarget.style.display = 'none' }}
            />
          </div>

          <div className="min-w-0 flex-1 pb-1">
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <Tag tone="brand">系列合集</Tag>
              <Tag>{movieCount} 部电影</Tag>
              {fileCount > movieCount && <Tag>{fileCount} 个文件</Tag>}
              {stats.yearRange && <Tag><Calendar size={11} />{stats.yearRange}</Tag>}
              {stats.averageRating > 0 && <Tag tone="rating"><Star size={11} fill="currentColor" />均分 {stats.averageRating.toFixed(1)}</Tag>}
            </div>

            <h1 className="max-w-4xl text-3xl font-bold leading-tight tracking-[-0.025em] text-[var(--nv-text-primary)] sm:text-4xl lg:text-5xl">
              {collection.name}
            </h1>

            {stats.genres.length > 0 && (
              <div className="mt-4 flex flex-wrap items-center gap-2">
                {visibleGenres.map((genre) => (
                  <Link key={genre} to={`/search?q=${encodeURIComponent(genre)}`} className="no-underline">
                    <Tag>{genre}</Tag>
                  </Link>
                ))}
                {hiddenGenreCount > 0 && (
                  <Button type="button" variant="ghost" size="sm" onClick={() => setGenresExpanded((expanded) => !expanded)}>
                    {genresExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                    {genresExpanded ? '收起' : `更多 ${hiddenGenreCount} 个`}
                  </Button>
                )}
              </div>
            )}

            {collection.overview && (
              <p className="mt-5 max-w-3xl text-sm leading-6 text-[var(--nv-text-secondary)]">{collection.overview}</p>
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
