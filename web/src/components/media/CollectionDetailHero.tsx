import { useMemo, useState } from 'react'
import { ArrowLeft, Calendar, ChevronDown, ChevronUp, Layers, Star } from 'lucide-react'
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
    <section className="nv-collection-hero relative overflow-hidden border-b border-[var(--nv-border-subtle)]">
      <div className="nv-collection-hero-backdrop absolute inset-0 overflow-hidden" aria-hidden="true">
        <img
          src={streamApi.getCollectionPosterUrl(collection.id)}
          alt=""
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
        <div className="absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />
      </div>

      <div className="nv-collection-hero-inner relative">
        <Button type="button" variant="secondary" size="sm" onClick={onBack} className="nv-collection-hero-back">
          <ArrowLeft size={14} aria-hidden="true" />
          返回
        </Button>

        <div className="flex flex-col gap-6 sm:flex-row sm:items-end">
          <div className="nv-collection-hero-poster relative shrink-0 overflow-hidden">
            <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]"><Layers size={42} aria-hidden="true" /></div>
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

            <h1 className="nv-collection-hero-title font-display text-[var(--nv-text-primary)]">
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
