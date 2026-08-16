import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ChevronUp,
  Clock,
  Copy,
  Film,
  Layers,
  Play,
  Star,
} from 'lucide-react'
import { collectionApi, streamApi } from '@/api'
import { Button, Surface, Tag } from '@/components/design-system'
import type { CollectionMediaItem, CollectionWithMedia } from '@/types'
import { groupByMovie, versionLabel, type GroupedMovieItem } from '@/utils/collectionGroup'

interface CollectionCarouselProps {
  mediaId: string
}

export default function CollectionCarousel({ mediaId }: CollectionCarouselProps) {
  const [data, setData] = useState<CollectionWithMedia | null>(null)
  const [loading, setLoading] = useState(true)
  const [expanded, setExpanded] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  useEffect(() => {
    let cancelled = false
    setLoading(true)

    collectionApi.getMediaCollection(mediaId)
      .then((response) => {
        if (!cancelled) setData(response.data.data)
      })
      .catch(() => {
        if (!cancelled) setData(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })

    return () => {
      cancelled = true
    }
  }, [mediaId])

  const scroll = (direction: 'left' | 'right') => {
    const element = scrollRef.current
    if (!element) return
    element.scrollBy({ left: direction === 'left' ? -320 : 320, behavior: 'smooth' })
  }

  // 同片多版本先折叠，避免一个合集里重复铺满相同电影。
  const groupedMovies = useMemo(() => {
    if (!data?.media) return []
    return groupByMovie(data.media)
  }, [data?.media])

  if (loading || !data || groupedMovies.length <= 1) return null

  const { collection, media } = data
  const currentIndex = groupedMovies.findIndex((group) => group.versions.some((version) => version.is_current))
  const movieCount = groupedMovies.length
  const fileCount = media.length

  return (
    <section className="mt-6" aria-labelledby="collection-carousel-title">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h3 id="collection-carousel-title" className="inline-flex items-center gap-2 text-base font-semibold text-[var(--nv-text-primary)]">
            <Layers size={17} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
            系列合集
          </h3>

          <Link
            to={`/collections/${collection.id}`}
            className="inline-flex min-h-7 max-w-full items-center gap-1 rounded-[var(--nv-radius-pill)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-active)] px-2.5 py-1 text-[10px] font-semibold text-[var(--nv-action-primary)] transition-[background-color,border-color,color] duration-200 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)]"
            title="查看合集详情"
          >
            <span className="truncate">{collection.name} · {movieCount}部</span>
            {fileCount > movieCount && <span className="shrink-0 text-[var(--nv-text-tertiary)]">/{fileCount}个文件</span>}
            <ChevronRight size={11} className="shrink-0" aria-hidden="true" />
          </Link>

          <Button
            type="button"
            variant="ghost"
            size="sm"
            iconOnly
            onClick={() => setExpanded((value) => !value)}
            title={expanded ? '收起列表' : '展开列表'}
            aria-label={expanded ? '收起合集列表' : '展开合集列表'}
            aria-expanded={expanded}
          >
            {expanded ? <ChevronUp size={16} aria-hidden="true" /> : <ChevronDown size={16} aria-hidden="true" />}
          </Button>
        </div>

        {!expanded && (
          <div className="flex items-center gap-2">
            {currentIndex >= 0 && (
              <Tag tone="neutral">第 {currentIndex + 1}/{movieCount} 部</Tag>
            )}
            <div className="flex gap-1" role="group" aria-label="合集横向滚动控制">
              <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => scroll('left')} aria-label="向左滚动">
                <ChevronLeft size={18} aria-hidden="true" />
              </Button>
              <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => scroll('right')} aria-label="向右滚动">
                <ChevronRight size={18} aria-hidden="true" />
              </Button>
            </div>
          </div>
        )}
      </div>

      {!expanded && (
        <div
          ref={scrollRef}
          className="scrollbar-hide flex gap-4 overflow-x-auto pb-2"
          style={{ scrollbarWidth: 'none' }}
          role="list"
          aria-label="系列合集电影列表"
        >
          {groupedMovies.map((group) => {
            const item = group.primary
            const isCurrent = group.versions.some((version) => version.is_current)
            return (
              <CollectionCard
                key={item.id}
                item={item}
                versionCount={group.versions.length}
                isCurrent={isCurrent}
                onClick={() => {
                  if (!isCurrent) navigate(`/media/${item.id}`)
                }}
              />
            )
          })}
        </div>
      )}

      {expanded && (
        <div className="space-y-2" role="list" aria-label="系列合集电影列表">
          {groupedMovies.map((group, index) => (
            <CollectionListItem
              key={group.primary.id}
              group={group}
              index={index + 1}
              isCurrent={group.versions.some((version) => version.is_current)}
            />
          ))}
        </div>
      )}
    </section>
  )
}

function CollectionCard({
  item,
  versionCount,
  isCurrent,
  onClick,
}: {
  item: CollectionMediaItem
  versionCount: number
  isCurrent: boolean
  onClick: () => void
}) {
  const hasMultipleVersions = versionCount > 1

  return (
    <Surface
      as="article"
      onClick={onClick}
      role="listitem"
      aria-current={isCurrent ? 'true' : undefined}
      className={`nv-collection-carousel-card group w-36 flex-shrink-0 overflow-hidden p-0 ${
        isCurrent
          ? 'is-current border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] shadow-[var(--nv-shadow-card-hover)]'
          : 'cursor-pointer'
      }`}
    >
      <div className="relative aspect-[2/3] overflow-hidden bg-[var(--nv-bg-surface-soft)]">
        <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]">
          <Film size={30} aria-hidden="true" />
        </div>
        <img
          src={streamApi.getPosterUrl(item.id)}
          alt={item.title}
          className="relative z-10 h-full w-full object-cover transition-[transform,filter] duration-300 ease-out group-hover:scale-[1.025] group-hover:brightness-90"
          loading="lazy"
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />

        <div className="absolute inset-0 z-20 bg-gradient-to-t from-black/75 via-black/10 to-transparent opacity-0 transition-opacity duration-200 group-hover:opacity-100" />

        {isCurrent && (
          <Tag tone="brand" className="absolute left-1.5 top-1.5 z-30">
            当前
          </Tag>
        )}

        {hasMultipleVersions && (
          <Tag tone="neutral" className="absolute right-1.5 top-1.5 z-30" title={`共有 ${versionCount} 个版本`}>
            <Copy size={9} aria-hidden="true" />
            {versionCount}版
          </Tag>
        )}

        {!isCurrent && (
          <div className="nv-collection-carousel-play pointer-events-none absolute bottom-2 left-2 z-30 flex h-8 w-8 items-center justify-center rounded-full">
            <Play size={14} className="ml-0.5" fill="currentColor" aria-hidden="true" />
          </div>
        )}
      </div>

      <div className="p-2.5">
        <h4 className={`truncate text-xs font-semibold transition-colors ${isCurrent ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-primary)] group-hover:text-[var(--nv-action-primary)]'}`}>
          {item.title}
        </h4>
        <div className="mt-1 flex items-center gap-1.5 text-[10px] text-[var(--nv-text-tertiary)]">
          {item.year > 0 && <span>{item.year}</span>}
          {item.rating > 0 && (
            <>
              {item.year > 0 && <span aria-hidden="true">·</span>}
              <span className="inline-flex items-center gap-0.5 text-[var(--nv-status-rating)]">
                <Star size={9} fill="currentColor" aria-hidden="true" />
                {item.rating.toFixed(1)}
              </span>
            </>
          )}
        </div>
      </div>
    </Surface>
  )
}

function CollectionListItem({
  group,
  index,
  isCurrent,
}: {
  group: GroupedMovieItem
  index: number
  isCurrent: boolean
}) {
  const item = group.primary
  const hasMultipleVersions = group.versions.length > 1
  const [showVersions, setShowVersions] = useState(false)

  return (
    <Surface
      role="listitem"
      aria-current={isCurrent ? 'true' : undefined}
      className={`overflow-hidden p-0 transition-[background-color,border-color] duration-200 ${
        isCurrent ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)]' : ''
      }`}
    >
      <Link
        to={isCurrent ? '#' : `/media/${item.id}`}
        className="group flex items-center gap-3 p-3 transition-colors hover:bg-[var(--nv-bg-hover)] sm:gap-4"
        onClick={(event) => {
          if (isCurrent) event.preventDefault()
        }}
      >
        <div className={`flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] text-sm font-bold ${
          isCurrent
            ? 'bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]'
            : 'bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-tertiary)]'
        }`}>
          {index}
        </div>

        <div className="relative h-16 w-11 flex-shrink-0 overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]">
          <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]">
            <Film size={16} aria-hidden="true" />
          </div>
          <img
            src={streamApi.getPosterUrl(item.id)}
            alt={item.title}
            className="relative z-10 h-full w-full object-cover"
            loading="lazy"
            onError={(event) => { event.currentTarget.style.display = 'none' }}
          />
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <h4 className={`truncate text-sm font-semibold ${isCurrent ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-primary)] group-hover:text-[var(--nv-action-primary)]'}`}>
              {item.title}
            </h4>
            {isCurrent && <Tag tone="brand">当前</Tag>}
            {hasMultipleVersions && (
              <Tag tone="neutral" title={`共有 ${group.versions.length} 个版本`}>
                <Copy size={9} aria-hidden="true" />
                {group.versions.length}版
              </Tag>
            )}
          </div>

          <div className="mt-1 flex flex-wrap items-center gap-3 text-xs text-[var(--nv-text-tertiary)]">
            {item.year > 0 && <span>{item.year}</span>}
            {item.rating > 0 && (
              <span className="inline-flex items-center gap-1 text-[var(--nv-status-rating)]">
                <Star size={10} fill="currentColor" aria-hidden="true" />
                {item.rating.toFixed(1)}
              </span>
            )}
            {item.runtime > 0 && (
              <span className="inline-flex items-center gap-1">
                <Clock size={10} aria-hidden="true" />
                {item.runtime}分钟
              </span>
            )}
          </div>

          {item.overview && (
            <p className="mt-1 line-clamp-1 text-[11px] text-[var(--nv-text-tertiary)]">
              {item.overview}
            </p>
          )}
        </div>

        {hasMultipleVersions && (
          <Button
            type="button"
            variant={showVersions ? 'secondary' : 'ghost'}
            size="sm"
            iconOnly
            onClick={(event) => {
              event.preventDefault()
              event.stopPropagation()
              setShowVersions((value) => !value)
            }}
            className={showVersions ? 'text-[var(--nv-action-primary)]' : undefined}
            title={showVersions ? '收起版本' : '展开版本'}
            aria-label={showVersions ? '收起版本' : '展开版本'}
            aria-expanded={showVersions}
          >
            <ChevronDown size={13} className={`transition-transform ${showVersions ? 'rotate-180' : ''}`} aria-hidden="true" />
          </Button>
        )}

        {!isCurrent && (
          <div className="nv-collection-carousel-play pointer-events-none flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full">
            <Play size={12} className="ml-0.5" fill="currentColor" aria-hidden="true" />
          </div>
        )}
      </Link>

      {hasMultipleVersions && showVersions && (
        <div className="border-t border-dashed border-[var(--nv-border-default)] px-3 pb-3">
          <div className="mt-2 space-y-1">
            {group.versions.map((version) => {
              const label = versionLabel(version) || '默认版本'
              const versionIsCurrent = version.is_current

              return (
                <Link
                  key={version.id}
                  to={versionIsCurrent ? '#' : `/media/${version.id}`}
                  onClick={(event) => {
                    if (versionIsCurrent) event.preventDefault()
                  }}
                  className={`flex items-center justify-between gap-2 rounded-[var(--nv-radius-control)] px-3 py-2 text-xs transition-colors ${
                    versionIsCurrent
                      ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]'
                      : 'bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]'
                  }`}
                >
                  <span className="truncate">{label}</span>
                  {versionIsCurrent && <Tag tone="brand">当前</Tag>}
                </Link>
              )
            })}
          </div>
        </div>
      )}
    </Surface>
  )
}
