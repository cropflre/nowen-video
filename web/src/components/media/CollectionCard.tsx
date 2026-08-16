import { useState } from 'react'
import { ChevronRight, Layers } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import clsx from 'clsx'
import { streamApi } from '@/api'
import type { MovieCollection } from '@/types'
import { Button, Tag } from '@/components/design-system'

interface CollectionCardProps {
  collection: MovieCollection
  variant?: 'grid' | 'list'
  className?: string
}

export default function CollectionCard({ collection, variant = 'grid', className }: CollectionCardProps) {
  if (variant === 'list') return <CollectionListCard collection={collection} />
  return <CollectionGridCard collection={collection} className={className} />
}

/** 网格卡片：复用 nv-media-card 契约，悬停物理与角标显露由 streaming-os 层统一决定 */
function CollectionGridCard({ collection, className }: { collection: MovieCollection; className?: string }) {
  const navigate = useNavigate()
  const [posterFailed, setPosterFailed] = useState(false)
  const detailTo = `/collections/${collection.id}`
  const hasPoster = !!collection.poster_path && !posterFailed

  return (
    <article className={clsx('nv-media-card group', className)}>
      <div className="nv-media-card-poster isolate">
        {hasPoster ? (
          <img
            src={streamApi.getCollectionPosterUrl(collection.id)}
            alt=""
            loading="lazy"
            onError={() => setPosterFailed(true)}
          />
        ) : (
          <div className="nv-media-card-placeholder absolute inset-0 flex flex-col items-center justify-center gap-2 bg-[var(--nv-bg-poster)] text-[var(--nv-text-tertiary)]">
            <Layers size={24} aria-hidden="true" />
            <span className="text-[10px]">暂无海报</span>
          </div>
        )}

        <Link
          to={detailTo}
          className="absolute inset-0 z-10 rounded-[inherit]"
          aria-label={`查看合集 ${collection.name}`}
        />

        <div className="nv-media-card-overlay z-20 pointer-events-none">
          <Button
            variant="primary"
            size="sm"
            iconOnly
            className="nv-media-card-play pointer-events-auto"
            onClick={() => navigate(detailTo)}
            aria-label={`查看合集 ${collection.name}`}
            title="查看合集"
          >
            <Layers size={16} aria-hidden="true" />
          </Button>
        </div>

        {collection.auto_matched && (
          <Tag tone="quality" className="nv-media-card-badge absolute left-2 top-2 z-30">
            自动匹配
          </Tag>
        )}
        <Tag tone="quality" className="nv-media-card-badge absolute right-2 top-2 z-30">
          {collection.media_count} 部
        </Tag>
      </div>

      <div className="pb-1 pt-2">
        <Link to={detailTo} className="nv-media-card-title" title={collection.name}>
          {collection.name}
        </Link>
        <div className="nv-media-card-meta mt-1 flex min-w-0 items-center gap-1.5 overflow-hidden">
          {collection.year_range && <span className="shrink-0">{collection.year_range}</span>}
          {collection.year_range && <span aria-hidden="true">·</span>}
          <span className="shrink-0">{collection.media_count} 部电影</span>
          {collection.file_count != null && collection.file_count > collection.media_count && (
            <>
              <span aria-hidden="true">·</span>
              <span className="shrink-0">{collection.file_count} 文件</span>
            </>
          )}
        </div>
      </div>
    </article>
  )
}

/** 列表行：复用 nv-browse-list-item 契约，与影视库列表视图保持同一密度 */
function CollectionListCard({ collection }: { collection: MovieCollection }) {
  const [posterFailed, setPosterFailed] = useState(false)
  const hasPoster = !!collection.poster_path && !posterFailed

  return (
    <Link
      to={`/collections/${collection.id}`}
      className="nv-browse-list-item group flex items-center gap-3 px-1 py-2.5 transition-colors hover:bg-[var(--nv-fill-hover)]"
    >
      <div className="relative h-16 w-11 shrink-0 overflow-hidden rounded-[9px] bg-[var(--nv-bg-poster)]">
        {hasPoster ? (
          <img
            src={streamApi.getCollectionPosterUrl(collection.id)}
            alt=""
            className="h-full w-full object-cover"
            loading="lazy"
            onError={() => setPosterFailed(true)}
          />
        ) : (
          <div className="nv-media-card-placeholder absolute inset-0 grid place-items-center text-[var(--nv-text-tertiary)]">
            <Layers size={15} aria-hidden="true" />
          </div>
        )}
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <h3 className="truncate text-xs font-medium text-[var(--nv-text-primary)]">{collection.name}</h3>
          <Tag>{collection.auto_matched ? '自动匹配' : '手动创建'}</Tag>
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-x-2 text-[10px] text-[var(--nv-text-tertiary)]">
          {collection.year_range && <span>{collection.year_range}</span>}
          <span>{collection.media_count} 部电影</span>
          {collection.file_count != null && collection.file_count > collection.media_count && (
            <span>{collection.file_count} 文件</span>
          )}
        </div>
      </div>

      <ChevronRight
        size={15}
        className="shrink-0 text-[var(--nv-text-tertiary)] transition-transform duration-150 group-hover:translate-x-0.5"
        aria-hidden="true"
      />
    </Link>
  )
}
