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

    // Collection metadata can aggregate hundreds of tags from many files.
    // Deduplicate case-insensitively so visually identical labels do not waste
    // precious hero space, while preserving the first source spelling.
    const genreMap = new Map<string, string>()
    media.forEach((item) => {
      ;(item.genres || '')
        .split(',')
        .map((genre) => genre.trim())
        .filter(Boolean)
        .forEach((genre) => {
          const key = genre.toLocaleLowerCase()
          if (!genreMap.has(key)) genreMap.set(key, genre)
        })
    })
    const genres = Array.from(genreMap.values()).sort((a, b) => a.localeCompare(b, 'zh-CN'))

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
              <div className="mt-4 max-w-4xl">
                <div className="mb-2 flex items-center gap-2">
                  <span className="text-xs font-medium text-[var(--nv-text-secondary)]">标签</span>
                  <span className="text-[11px] tabular-nums text-[var(--nv-text-tertiary)]">{stats.genres.length}</span>
                  {hiddenGenreCount > 0 && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="ml-auto"
                      onClick={() => setGenresExpanded((expanded) => !expanded)}
                      aria-expanded={genresExpanded}
                    >
                      {genresExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                      {genresExpanded ? '收起' : `查看全部 ${stats.genres.length}`}
                    </Button>
                  )}
                </div>

                <div
                  className={genresExpanded
                    ? 'max-h-40 overflow-y-auto rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[color-mix(in_srgb,var(--nv-bg-surface)_72%,transparent)] p-3'
                    : ''}
                >
                  <div className="flex flex-wrap items-center gap-2">
                    {visibleGenres.map((genre) => (
                      <Link
                        key={genre}
                        to={`/search?q=${encodeURIComponent(genre)}`}
                        className="min-w-0 max-w-[12rem] no-underline"
                        title={genre}
                      >
                        <Tag className="block max-w-full truncate">{genre}</Tag>
                      </Link>
                    ))}
                    {!genresExpanded && hiddenGenreCount > 0 && (
                      <button
                        type="button"
                        onClick={() => setGenresExpanded(true)}
                        className="inline-flex h-6 items-center rounded-full border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] px-2.5 text-[11px] font-medium text-[var(--nv-text-secondary)] transition-colors hover:border-[var(--nv-border-default)] hover:text-[var(--nv-text-primary)]"
                        aria-label={`查看其余 ${hiddenGenreCount} 个标签`}
                      >
                        +{hiddenGenreCount}
                      </button>
                    )}
                  </div>
                </div>
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
