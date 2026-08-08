import { useMemo, useState } from 'react'
import { Calendar, ChevronDown, Clock, Copy, Film, Grid3X3, LayoutList, Play, Star } from 'lucide-react'
import { Link, useSearchParams } from 'react-router-dom'
import { streamApi } from '@/api'
import type { Media } from '@/types'
import { groupByMovie, versionLabel, type GroupedMovieItem } from '@/utils/collectionGroup'
import { Button, Select, Tag } from '@/components/design-system'
import Pagination from '@/components/Pagination'
import { usePagination } from '@/hooks/usePagination'

type SortOption = 'premiered_asc' | 'premiered_desc' | 'title_asc' | 'rating_desc'

const SORT_OPTIONS: { value: SortOption; label: string }[] = [
  { value: 'premiered_asc', label: '首映日期 ↑' },
  { value: 'premiered_desc', label: '首映日期 ↓' },
  { value: 'title_asc', label: '标题 A-Z' },
  { value: 'rating_desc', label: '评分 ↓' },
]

interface CollectionMovieBrowserProps {
  media: Media[]
}

export default function CollectionMovieBrowser({ media }: CollectionMovieBrowserProps) {
  const [searchParams, setSearchParams] = useSearchParams()
  const viewMode = (searchParams.get('view') || 'grid') as 'grid' | 'list'
  const sortOption = (searchParams.get('sort') || 'premiered_asc') as SortOption
  const pagination = usePagination({ initialSize: 24, syncToUrl: true })

  const sortedMedia = useMemo(() => {
    const grouped = groupByMovie(media)
    const sorted = [...grouped]
    const byPremiered = (direction: 'asc' | 'desc') => (left: GroupedMovieItem, right: GroupedMovieItem) => {
      const a = left.primary
      const b = right.primary
      const dateA = a.premiered || ''
      const dateB = b.premiered || ''
      if (dateA && dateB) {
        const compared = direction === 'asc' ? dateA.localeCompare(dateB) : dateB.localeCompare(dateA)
        return compared || a.title.localeCompare(b.title)
      }
      if (dateA) return -1
      if (dateB) return 1
      const yearA = a.year || (direction === 'asc' ? 9999 : 0)
      const yearB = b.year || (direction === 'asc' ? 9999 : 0)
      const yearCompared = direction === 'asc' ? yearA - yearB : yearB - yearA
      return yearCompared || a.title.localeCompare(b.title)
    }

    if (sortOption === 'premiered_asc') sorted.sort(byPremiered('asc'))
    if (sortOption === 'premiered_desc') sorted.sort(byPremiered('desc'))
    if (sortOption === 'title_asc') sorted.sort((a, b) => a.primary.title.localeCompare(b.primary.title))
    if (sortOption === 'rating_desc') sorted.sort((a, b) => b.primary.rating - a.primary.rating || a.primary.title.localeCompare(b.primary.title))
    return sorted
  }, [media, sortOption])

  const pagedMedia = useMemo(() => {
    const start = (pagination.page - 1) * pagination.size
    return sortedMedia.slice(start, start + pagination.size)
  }, [pagination.page, pagination.size, sortedMedia])

  const setSort = (value: SortOption) => {
    const params = new URLSearchParams(searchParams)
    if (value === 'premiered_asc') params.delete('sort')
    else params.set('sort', value)
    params.delete('page')
    setSearchParams(params, { replace: true })
  }

  const setView = (view: 'grid' | 'list') => {
    const params = new URLSearchParams(searchParams)
    if (view === 'grid') params.delete('view')
    else params.set('view', view)
    setSearchParams(params, { replace: true })
  }

  return (
    <section className="space-y-5">
      <div className="flex flex-col gap-3 border-b border-[var(--nv-border-subtle)] pb-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-lg font-semibold text-[var(--nv-text-primary)]">系列电影</h2>
          <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{sortedMedia.length} 部电影 · 自动折叠同片多版本</p>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Select value={sortOption} onChange={(event) => setSort(event.target.value as SortOption)} className="h-9 text-xs" aria-label="合集电影排序">
            {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </Select>
          <div className="flex items-center gap-1 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-1">
            <Button type="button" variant={viewMode === 'grid' ? 'secondary' : 'ghost'} size="sm" iconOnly onClick={() => setView('grid')} aria-label="卡片视图" title="卡片视图"><Grid3X3 size={15} /></Button>
            <Button type="button" variant={viewMode === 'list' ? 'secondary' : 'ghost'} size="sm" iconOnly onClick={() => setView('list')} aria-label="列表视图" title="列表视图"><LayoutList size={15} /></Button>
          </div>
        </div>
      </div>

      {viewMode === 'grid' ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {pagedMedia.map((group, index) => (
            <MovieGridCard key={group.primary.id} group={group} index={(pagination.page - 1) * pagination.size + index + 1} />
          ))}
        </div>
      ) : (
        <div className="space-y-2">
          {pagedMedia.map((group, index) => (
            <MovieListCard key={group.primary.id} group={group} index={(pagination.page - 1) * pagination.size + index + 1} />
          ))}
        </div>
      )}

      <Pagination
        page={pagination.page}
        totalPages={pagination.totalPages(sortedMedia.length)}
        total={sortedMedia.length}
        pageSize={pagination.size}
        pageSizeOptions={[12, 24, 36, 48]}
        onPageChange={pagination.setPage}
        onPageSizeChange={pagination.setSize}
      />
    </section>
  )
}

function MovieGridCard({ group, index }: { group: GroupedMovieItem; index: number }) {
  const item = group.primary
  const [versionsOpen, setVersionsOpen] = useState(false)
  const versions = group.versions
  const genres = (item.genres || '').split(',').map((genre) => genre.trim()).filter(Boolean)

  return (
    <article className="relative overflow-visible rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] transition-[border-color,box-shadow,transform] hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:shadow-[var(--nv-shadow-card-hover)]">
      <Link to={`/media/${item.id}`} className="group block overflow-hidden rounded-[var(--nv-radius-card)]">
        <div className="relative aspect-[2/3] overflow-hidden bg-[var(--nv-bg-surface-soft)]">
          <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]"><Film size={32} /></div>
          <img src={streamApi.getPosterUrl(item.id)} alt={item.title} className="relative h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.025]" loading="lazy" onError={(event) => { event.currentTarget.style.display = 'none' }} />
          <div className="absolute inset-0 bg-gradient-to-t from-black/55 via-transparent to-transparent" />
          <div className="absolute left-2 top-2"><Tag>#{index}</Tag></div>
          {versions.length > 1 && <div className="absolute right-2 top-2"><Tag tone="brand"><Copy size={10} />{versions.length} 版</Tag></div>}
          {item.rating > 0 && <div className="absolute bottom-2 left-2"><Tag tone="rating"><Star size={10} fill="currentColor" />{item.rating.toFixed(1)}</Tag></div>}
          <div className="absolute inset-0 flex items-center justify-center bg-black/20 opacity-0 transition-opacity group-hover:opacity-100">
            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)] shadow-[var(--nv-shadow-card)]"><Play size={17} className="ml-0.5" fill="currentColor" /></div>
          </div>
        </div>
        <div className="p-3">
          <h3 className="truncate text-sm font-medium text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]" title={item.title}>{item.title}</h3>
          <div className="mt-1.5 flex flex-wrap items-center gap-2 text-[11px] text-[var(--nv-text-tertiary)]">
            {item.year > 0 && <span className="inline-flex items-center gap-1"><Calendar size={10} />{item.year}</span>}
            {item.runtime > 0 && <span className="inline-flex items-center gap-1"><Clock size={10} />{item.runtime}分钟</span>}
          </div>
        </div>
      </Link>

      {genres.length > 0 && (
        <div className="-mt-1 flex flex-wrap gap-1 px-3 pb-3">
          {genres.slice(0, 4).map((genre) => <Link key={genre} to={`/search?q=${encodeURIComponent(genre)}`}><Tag>{genre}</Tag></Link>)}
          {genres.length > 4 && <Tag>+{genres.length - 4}</Tag>}
        </div>
      )}

      {versions.length > 1 && (
        <Button type="button" variant={versionsOpen ? 'primary' : 'secondary'} size="sm" iconOnly onClick={() => setVersionsOpen((open) => !open)} className="absolute bottom-3 right-3" aria-label={versionsOpen ? '收起版本' : '查看所有版本'} title={versionsOpen ? '收起版本' : '查看所有版本'}>
          <ChevronDown size={13} className={versionsOpen ? 'rotate-180 transition-transform' : 'transition-transform'} />
        </Button>
      )}

      {versions.length > 1 && versionsOpen && <VersionMenu versions={versions} currentId={item.id} />}
    </article>
  )
}

function MovieListCard({ group, index }: { group: GroupedMovieItem; index: number }) {
  const item = group.primary
  const [versionsOpen, setVersionsOpen] = useState(false)
  const versions = group.versions
  const genres = (item.genres || '').split(',').map((genre) => genre.trim()).filter(Boolean)

  return (
    <article className="overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)]">
      <div className="flex items-center gap-3 p-3 transition-colors hover:bg-[var(--nv-bg-hover)]">
        <span className="w-7 shrink-0 text-center text-xs font-semibold text-[var(--nv-text-tertiary)]">{index}</span>
        <Link to={`/media/${item.id}`} className="relative h-20 w-14 shrink-0 overflow-hidden rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-surface-soft)]">
          <div className="absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)]"><Film size={18} /></div>
          <img src={streamApi.getPosterUrl(item.id)} alt={item.title} className="relative h-full w-full object-cover" loading="lazy" onError={(event) => { event.currentTarget.style.display = 'none' }} />
        </Link>

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <Link to={`/media/${item.id}`} className="min-w-0 truncate text-sm font-medium text-[var(--nv-text-primary)] hover:text-[var(--nv-action-primary)]">{item.title}</Link>
            {versions.length > 1 && <Tag tone="brand"><Copy size={10} />{versions.length} 个版本</Tag>}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
            {item.year > 0 && <span>{item.year}</span>}
            {item.runtime > 0 && <span>{formatDuration(item.runtime)}</span>}
            {genres.slice(0, 3).map((genre) => <Link key={genre} to={`/search?q=${encodeURIComponent(genre)}`}><Tag>{genre}</Tag></Link>)}
            {genres.length > 3 && <Tag>+{genres.length - 3}</Tag>}
          </div>
        </div>

        {item.rating > 0 && <Tag tone="rating"><Star size={11} fill="currentColor" />{item.rating.toFixed(1)}</Tag>}
        {versions.length > 1 && (
          <Button type="button" variant={versionsOpen ? 'primary' : 'secondary'} size="sm" iconOnly onClick={() => setVersionsOpen((open) => !open)} aria-label={versionsOpen ? '收起版本' : '展开版本'} title={versionsOpen ? '收起版本' : '展开版本'}>
            <ChevronDown size={14} className={versionsOpen ? 'rotate-180 transition-transform' : 'transition-transform'} />
          </Button>
        )}
        <Link to={`/media/${item.id}`} className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)] opacity-0 transition-opacity hover:opacity-90 group-hover:opacity-100" aria-label={`打开 ${item.title}`}><Play size={13} className="ml-0.5" fill="currentColor" /></Link>
      </div>

      {versions.length > 1 && versionsOpen && (
        <div className="border-t border-dashed border-[var(--nv-border-subtle)] px-3 py-2">
          <div className="space-y-1">{versions.map((version) => <VersionRow key={version.id} version={version} currentId={item.id} />)}</div>
        </div>
      )}
    </article>
  )
}

function VersionMenu({ versions, currentId }: { versions: Media[]; currentId: string }) {
  return (
    <div className="absolute left-0 right-0 top-full z-[var(--nv-z-dropdown)] mt-1 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] p-2 shadow-[var(--nv-shadow-elevated)]">
      <div className="mb-1 px-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--nv-text-tertiary)]">{versions.length} 个版本</div>
      <div className="space-y-1">{versions.map((version) => <VersionRow key={version.id} version={version} currentId={currentId} />)}</div>
    </div>
  )
}

function VersionRow({ version, currentId }: { version: Media; currentId: string }) {
  const current = version.id === currentId
  return (
    <Link to={`/media/${version.id}`} className={`flex items-center justify-between gap-2 rounded-[var(--nv-radius-control)] px-2.5 py-2 text-xs transition-colors ${current ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]'}`}>
      <span className="truncate">{versionLabel(version) || version.title || '默认版本'}</span>
      {current && <Tag tone="brand">当前</Tag>}
    </Link>
  )
}

function formatDuration(seconds: number) {
  if (!seconds) return ''
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`
}
