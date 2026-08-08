import { Film, Play } from 'lucide-react'
import { Link } from 'react-router-dom'
import { streamApi } from '@/api'
import type { MovieCollection } from '@/types'
import { Tag } from '@/components/design-system'

interface CollectionCardProps {
  collection: MovieCollection
  variant?: 'grid' | 'list'
}

export default function CollectionCard({ collection, variant = 'grid' }: CollectionCardProps) {
  if (variant === 'list') {
    return <CollectionListCard collection={collection} />
  }

  return (
    <Link
      to={`/collections/${collection.id}`}
      className="group block overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] transition-[background-color,border-color,box-shadow,transform] hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:shadow-[var(--nv-shadow-card-hover)]"
    >
      <div className="relative aspect-[2/3] overflow-hidden bg-[var(--nv-bg-surface-soft)]">
        <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]">
          <Film size={34} aria-hidden="true" />
        </div>
        <img
          src={streamApi.getCollectionPosterUrl(collection.id)}
          alt={collection.name}
          className="relative h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.025]"
          loading="lazy"
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />

        <div className="absolute inset-0 bg-gradient-to-t from-black/55 via-transparent to-transparent opacity-70" />
        <div className="absolute left-2 top-2 flex flex-wrap gap-1.5">
          {collection.auto_matched && <Tag>自动匹配</Tag>}
        </div>
        <div className="absolute bottom-2 right-2 flex flex-col items-end gap-1">
          <span className="rounded-[var(--nv-radius-sm)] bg-black/70 px-2 py-1 text-xs font-semibold text-white backdrop-blur-sm">
            {collection.media_count} 部
          </span>
          {collection.file_count != null && collection.file_count > collection.media_count && (
            <span className="rounded-[var(--nv-radius-sm)] bg-black/65 px-1.5 py-0.5 text-[10px] font-medium text-white/80 backdrop-blur-sm">
              {collection.file_count} 文件
            </span>
          )}
        </div>

        <div className="absolute inset-0 flex items-center justify-center bg-black/20 opacity-0 transition-opacity group-hover:opacity-100">
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)] shadow-[var(--nv-shadow-card)]">
            <Play size={17} className="ml-0.5" fill="currentColor" aria-hidden="true" />
          </div>
        </div>
      </div>

      <div className="p-3.5">
        <h3 className="truncate text-sm font-semibold text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]">
          {collection.name}
        </h3>
        <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs text-[var(--nv-text-tertiary)]">
          {collection.year_range && <span>{collection.year_range}</span>}
          {collection.year_range && <span aria-hidden="true">·</span>}
          <span>{collection.media_count} 部电影</span>
        </div>
      </div>
    </Link>
  )
}

function CollectionListCard({ collection }: { collection: MovieCollection }) {
  return (
    <Link
      to={`/collections/${collection.id}`}
      className="group flex items-center gap-4 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] p-3 transition-[background-color,border-color,box-shadow,transform] hover:-translate-y-px hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)] hover:shadow-[var(--nv-shadow-card)]"
    >
      <div className="relative h-16 w-12 shrink-0 overflow-hidden rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-surface-soft)]">
        <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]"><Film size={18} /></div>
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
          <h3 className="min-w-0 flex-1 truncate text-sm font-medium text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]">
            {collection.name}
          </h3>
          <Tag tone={collection.auto_matched ? 'brand' : 'neutral'}>{collection.auto_matched ? '自动匹配' : '手动创建'}</Tag>
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
          {collection.year_range && <span>{collection.year_range}</span>}
          <span>{collection.media_count} 部电影</span>
          {collection.file_count != null && collection.file_count > collection.media_count && <span>{collection.file_count} 文件</span>}
        </div>
      </div>

      <Film size={16} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
    </Link>
  )
}
