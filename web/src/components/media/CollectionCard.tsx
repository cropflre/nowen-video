import { ChevronRight, Film, Play } from 'lucide-react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import type { MovieCollection } from '@/types'
import { Tag } from '@/components/design-system'

interface CollectionCardProps {
  collection: MovieCollection
  variant?: 'grid' | 'list'
}

export default function CollectionCard({ collection, variant = 'grid' }: CollectionCardProps) {
  if (variant === 'list') return <CollectionListCard collection={collection} />

  return (
    <Link
      to={`/collections/${collection.id}`}
      className="group block min-w-0 transition-transform duration-150 hover:-translate-y-0.5"
    >
      <div className="relative aspect-[2/3] overflow-hidden rounded-[var(--nv-radius-card)] bg-[var(--nv-bg-surface-soft)] shadow-[var(--nv-shadow-card)] transition-shadow duration-150 group-hover:shadow-[var(--nv-shadow-card-hover)]">
        <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]">
          <Film size={34} aria-hidden="true" />
        </div>
        <img
          src={streamApi.getCollectionPosterUrl(collection.id)}
          alt={collection.name}
          className="relative h-full w-full object-cover"
          loading="lazy"
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />

        {collection.auto_matched && <Tag className="absolute left-2 top-2">自动匹配</Tag>}
        <div className="absolute bottom-2 right-2 rounded-[var(--nv-radius-sm)] bg-black/65 px-2 py-1 text-[11px] font-medium text-white/90">
          {collection.media_count} 部
        </div>

        <div className="absolute inset-0 flex items-center justify-center bg-black/25 opacity-0 transition-opacity duration-150 group-hover:opacity-100">
          <span className="grid h-10 w-10 place-items-center rounded-full bg-white/90 text-black shadow-[var(--nv-shadow-card)]">
            <Play size={16} className="ml-0.5" fill="currentColor" aria-hidden="true" />
          </span>
        </div>
      </div>

      <div className="px-0.5 pt-2.5">
        <h3 className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{collection.name}</h3>
        <div className="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-[var(--nv-text-tertiary)]">
          {collection.year_range && <span>{collection.year_range}</span>}
          {collection.year_range && <span aria-hidden="true">·</span>}
          <span>{collection.media_count} 部电影</span>
          {collection.file_count != null && collection.file_count > collection.media_count && (
            <><span aria-hidden="true">·</span><span>{collection.file_count} 文件</span></>
          )}
        </div>
      </div>
    </Link>
  )
}

function CollectionListCard({ collection }: { collection: MovieCollection }) {
  return (
    <Link
      to={`/collections/${collection.id}`}
      className="group flex items-center gap-3 border-b border-[var(--nv-border-subtle)] px-1 py-3 transition-colors duration-150 hover:bg-[var(--nv-fill-hover)]"
    >
      <div className="relative h-16 w-12 shrink-0 overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]">
        <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]"><Film size={18} aria-hidden="true" /></div>
        <img
          src={streamApi.getCollectionPosterUrl(collection.id)}
          alt={collection.name}
          className="relative h-full w-full object-cover"
          loading="lazy"
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="min-w-0 flex-1 truncate text-sm font-medium text-[var(--nv-text-primary)]">{collection.name}</h3>
          <Tag>{collection.auto_matched ? '自动匹配' : '手动创建'}</Tag>
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
          {collection.year_range && <span>{collection.year_range}</span>}
          <span>{collection.media_count} 部电影</span>
          {collection.file_count != null && collection.file_count > collection.media_count && <span>{collection.file_count} 文件</span>}
        </div>
      </div>

      <ChevronRight size={15} className="shrink-0 text-[var(--nv-text-tertiary)] transition-transform duration-150 group-hover:translate-x-0.5" aria-hidden="true" />
    </Link>
  )
}
