import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { mediaApi, seriesApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { Media, MixedItem, Series } from '@/types'
import MediaCard from '@/components/MediaCard'
import Pagination from '@/components/Pagination'
import {
  Calendar,
  Film,
  Filter,
  Grid3X3,
  LayoutList,
  Search,
  Star,
  Tv,
  X,
} from 'lucide-react'
import {
  Button,
  EmptyState,
  Input,
  Select,
  Surface,
  Tag,
  type TagTone,
} from '@/components/design-system'

const SORT_OPTIONS = [
  { value: 'created_desc', label: '最近添加' },
  { value: 'created_asc', label: '最早添加' },
  { value: 'title_asc', label: '名称 A-Z' },
  { value: 'title_desc', label: '名称 Z-A' },
  { value: 'year_desc', label: '年份最新' },
  { value: 'year_asc', label: '年份最早' },
  { value: 'rating_desc', label: '评分最高' },
]

type LibraryViewMode = 'grid' | 'list'
type LibraryViewTab = 'all' | 'series'

const LIBRARY_VIEW_MODE_STORAGE_KEY = 'nowen:library-view-mode'

function parseLibraryViewMode(value: string | null): LibraryViewMode | null {
  return value === 'grid' || value === 'list' ? value : null
}

function FilterChip({
  selected,
  onClick,
  children,
}: {
  selected: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className="min-h-8 rounded-[var(--nv-radius-control)] border px-3 py-1.5 text-xs font-medium transition-[background-color,border-color,color] duration-200"
      style={{
        background: selected ? 'var(--nv-bg-active)' : 'var(--nv-bg-control)',
        borderColor: selected ? 'var(--nv-border-hover)' : 'var(--nv-border-default)',
        color: selected ? 'var(--nv-action-primary)' : 'var(--nv-text-secondary)',
      }}
    >
      {children}
    </button>
  )
}

function ViewTabButton({
  active,
  onClick,
  icon,
  children,
}: {
  active: boolean
  onClick: () => void
  icon: ReactNode
  children: ReactNode
}) {
  return (
    <Button
      type="button"
      variant={active ? 'secondary' : 'ghost'}
      size="sm"
      onClick={onClick}
      aria-pressed={active}
      className={active ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : undefined}
    >
      {icon}
      {children}
    </Button>
  )
}

export default function LibraryPage() {
  const { id } = useParams<{ id: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const [mixedItems, setMixedItems] = useState<MixedItem[]>([])
  const [seriesList, setSeriesList] = useState<Series[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [viewTab, setViewTab] = useState<LibraryViewTab>('all')

  const page = parseInt(searchParams.get('page') || '1', 10) || 1
  const size = parseInt(searchParams.get('limit') || '30', 10) || 30

  const [searchQuery, setSearchQuery] = useState('')
  const [sortValue, setSortValue] = useState('created_desc')
  const [viewMode, setViewMode] = useState<LibraryViewMode>(() => {
    const urlMode = parseLibraryViewMode(searchParams.get('view'))
    if (urlMode) return urlMode
    if (typeof window === 'undefined') return 'grid'
    return parseLibraryViewMode(window.localStorage.getItem(LIBRARY_VIEW_MODE_STORAGE_KEY)) || 'grid'
  })
  const [filterGenre, setFilterGenre] = useState<string | null>(null)
  const [showFilters, setShowFilters] = useState(false)
  const toast = useToast()

  const handleViewModeChange = useCallback((mode: LibraryViewMode) => {
    setViewMode(mode)
    window.localStorage.setItem(LIBRARY_VIEW_MODE_STORAGE_KEY, mode)
    const params = new URLSearchParams(searchParams)
    if (mode === 'grid') params.delete('view')
    else params.set('view', mode)
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

  useEffect(() => {
    const urlMode = parseLibraryViewMode(searchParams.get('view'))
    if (!urlMode || urlMode === viewMode) return
    setViewMode(urlMode)
    window.localStorage.setItem(LIBRARY_VIEW_MODE_STORAGE_KEY, urlMode)
  }, [searchParams, viewMode])

  const setPage = useCallback((newPage: number) => {
    const params = new URLSearchParams(searchParams)
    if (newPage <= 1) params.delete('page')
    else params.set('page', String(newPage))
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

  const setSize = useCallback((newSize: number) => {
    const params = new URLSearchParams(searchParams)
    if (newSize === 30) params.delete('limit')
    else params.set('limit', String(newSize))
    params.delete('page')
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

  useEffect(() => {
    const params = new URLSearchParams(searchParams)
    params.delete('page')
    setSearchParams(params, { replace: true })
    setLoading(true)
    setSearchQuery('')
    setFilterGenre(null)
  }, [id])

  useEffect(() => {
    if (!id) return
    setLoading(true)

    Promise.all([
      mediaApi.listMixed({ page, size, library_id: id }),
      seriesApi.list({ library_id: id }),
    ])
      .then(([mixedRes, seriesRes]) => {
        setMixedItems(mixedRes.data.data || [])
        setTotal(mixedRes.data.total)
        setSeriesList(seriesRes.data.data || [])
      })
      .catch(() => { toast.error('加载媒体库内容失败') })
      .finally(() => setLoading(false))
  }, [id, page, size])

  const totalPages = Math.ceil(total / size)
  const hasSeries = seriesList.length > 0

  const allGenres = useMemo(() => {
    const genres = new Set<string>()
    mixedItems.forEach((item) => {
      const value = item.type === 'series' ? item.series?.genres : item.media?.genres
      if (!value) return
      value.split(',').forEach((genre) => {
        const trimmed = genre.trim()
        if (trimmed) genres.add(trimmed)
      })
    })
    seriesList.forEach((series) => {
      if (!series.genres) return
      series.genres.split(',').forEach((genre) => {
        const trimmed = genre.trim()
        if (trimmed) genres.add(trimmed)
      })
    })
    return Array.from(genres).sort()
  }, [mixedItems, seriesList])

  const getItemTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.title || '') : (item.media?.title || '')
  const getItemOrigTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.orig_title || '') : (item.media?.orig_title || '')
  const getItemOverview = (item: MixedItem) => item.type === 'series' ? (item.series?.overview || '') : (item.media?.overview || '')
  const getItemGenres = (item: MixedItem) => item.type === 'series' ? (item.series?.genres || '') : (item.media?.genres || '')
  const getItemYear = (item: MixedItem) => item.type === 'series' ? (item.series?.year || 0) : (item.media?.year || 0)
  const getItemRating = (item: MixedItem) => item.type === 'series' ? (item.series?.rating || 0) : (item.media?.rating || 0)
  const getItemTime = (item: MixedItem) => item.type === 'series' ? (item.series?.created_at || '') : (item.media?.created_at || '')

  const filteredMixed = useMemo(() => {
    let items = [...mixedItems]

    if (searchQuery.trim()) {
      const query = searchQuery.trim().toLowerCase()
      items = items.filter((item) =>
        getItemTitle(item).toLowerCase().includes(query)
        || getItemOrigTitle(item).toLowerCase().includes(query)
        || getItemOverview(item).toLowerCase().includes(query)
      )
    }

    if (filterGenre) items = items.filter((item) => getItemGenres(item).includes(filterGenre))

    const [field, direction] = sortValue.split('_')
    items.sort((a, b) => {
      let comparison = 0
      if (field === 'title') comparison = getItemTitle(a).localeCompare(getItemTitle(b))
      else if (field === 'year') comparison = getItemYear(a) - getItemYear(b)
      else if (field === 'rating') comparison = getItemRating(a) - getItemRating(b)
      else comparison = new Date(getItemTime(a)).getTime() - new Date(getItemTime(b)).getTime()
      return direction === 'desc' ? -comparison : comparison
    })

    return items
  }, [mixedItems, searchQuery, filterGenre, sortValue])

  const deduplicatedSeries = useMemo(() => {
    const normalize = (title: string) => title
      .replace(/\s*S\d{1,2}\s*$/i, '')
      .replace(/\s*Season\s*\d{1,2}\s*$/i, '')
      .replace(/\s*第\s*[一二三四五六七八九十\d]+\s*季\s*$/, '')
      .replace(/\s*第\s*[一二三四五六七八九十\d]+\s*部\s*$/, '')
      .replace(/\s*[\(（]\s*Season\s*\d{1,2}\s*[\)）]\s*$/i, '')
      .replace(/\s*【\s*第?\s*[一二三四五六七八九十\d]+\s*季?\s*】\s*$/, '')
      .trim() || title

    const groups = new Map<string, { best: typeof seriesList[0]; totalSeasons: number; totalEps: number }>()
    const order: string[] = []

    for (const series of seriesList) {
      const key = `${series.library_id}:${normalize(series.title)}`
      const existing = groups.get(key)
      if (existing) {
        existing.totalSeasons += series.season_count
        existing.totalEps += series.episode_count
        const score = (candidate: typeof series) =>
          (candidate.overview ? 3 : 0)
          + (candidate.poster_path ? 3 : 0)
          + (candidate.rating > 0 ? 2 : 0)
          + (candidate.tmdb_id > 0 ? 2 : 0)
          + candidate.episode_count
        if (score(series) > score(existing.best)) existing.best = series
      } else {
        groups.set(key, {
          best: series,
          totalSeasons: series.season_count,
          totalEps: series.episode_count,
        })
        order.push(key)
      }
    }

    return order.map((key) => {
      const group = groups.get(key)!
      return {
        ...group.best,
        season_count: group.totalSeasons,
        episode_count: group.totalEps,
      }
    })
  }, [seriesList])

  const filteredSeries = useMemo(() => {
    let items = [...deduplicatedSeries]

    if (searchQuery.trim()) {
      const query = searchQuery.trim().toLowerCase()
      items = items.filter((series) =>
        series.title.toLowerCase().includes(query)
        || series.orig_title?.toLowerCase().includes(query)
        || series.overview?.toLowerCase().includes(query)
      )
    }

    if (filterGenre) items = items.filter((series) => (series.genres || '').includes(filterGenre))

    const [field, direction] = sortValue.split('_')
    items.sort((a, b) => {
      let comparison = 0
      if (field === 'title') comparison = a.title.localeCompare(b.title)
      else if (field === 'year') comparison = (a.year || 0) - (b.year || 0)
      else if (field === 'rating') comparison = (a.rating || 0) - (b.rating || 0)
      else comparison = new Date(a.created_at || '').getTime() - new Date(b.created_at || '').getTime()
      return direction === 'desc' ? -comparison : comparison
    })

    return items
  }, [deduplicatedSeries, searchQuery, filterGenre, sortValue])

  const resultCount = viewTab === 'all' ? filteredMixed.length : filteredSeries.length
  const hasLocalFilter = Boolean(searchQuery || filterGenre)

  return (
    <div className="nv-section-stack">
      <Surface className="space-y-4 p-4 sm:p-5">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-center">
          <div className="flex flex-wrap items-center gap-1" role="group" aria-label="媒体库内容类型">
            <ViewTabButton
              active={viewTab === 'all'}
              onClick={() => setViewTab('all')}
              icon={<Film size={14} aria-hidden="true" />}
            >
              全部内容
            </ViewTabButton>
            {hasSeries && (
              <ViewTabButton
                active={viewTab === 'series'}
                onClick={() => setViewTab('series')}
                icon={<Tv size={14} aria-hidden="true" />}
              >
                剧集合集
                <Tag tone="neutral">{seriesList.length}</Tag>
              </ViewTabButton>
            )}
          </div>

          <div className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row xl:justify-end">
            <div className="relative min-w-0 flex-1 sm:max-w-sm">
              <Search
                size={16}
                className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--nv-text-tertiary)]"
                aria-hidden="true"
              />
              <Input
                type="search"
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                className="pl-9 pr-9"
                placeholder="搜索此媒体库..."
                aria-label="搜索此媒体库"
              />
              {searchQuery && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  iconOnly
                  onClick={() => setSearchQuery('')}
                  className="absolute right-1.5 top-1/2 -translate-y-1/2"
                  aria-label="清除搜索"
                >
                  <X size={14} aria-hidden="true" />
                </Button>
              )}
            </div>

            <Select
              value={sortValue}
              onChange={(event) => setSortValue(event.target.value)}
              aria-label="媒体库排序"
              className="sm:w-36"
            >
              {SORT_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </Select>

            {allGenres.length > 0 && (
              <Button
                type="button"
                variant={showFilters || filterGenre ? 'secondary' : 'ghost'}
                size="sm"
                onClick={() => setShowFilters((value) => !value)}
                className={filterGenre ? 'text-[var(--nv-action-primary)]' : undefined}
                aria-expanded={showFilters}
              >
                <Filter size={14} aria-hidden="true" />
                筛选
                {filterGenre && <Tag tone="brand">1</Tag>}
              </Button>
            )}

            <div
              className="flex items-center overflow-hidden rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)]"
              role="group"
              aria-label="媒体库视图"
            >
              <Button
                type="button"
                variant="ghost"
                size="sm"
                iconOnly
                onClick={() => handleViewModeChange('grid')}
                className={viewMode === 'grid' ? 'rounded-none bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : 'rounded-none'}
                aria-pressed={viewMode === 'grid'}
                aria-label="网格视图"
              >
                <Grid3X3 size={16} aria-hidden="true" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                iconOnly
                onClick={() => handleViewModeChange('list')}
                className={viewMode === 'list' ? 'rounded-none border-l border-[var(--nv-border-subtle)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : 'rounded-none border-l border-[var(--nv-border-subtle)]'}
                aria-pressed={viewMode === 'list'}
                aria-label="列表视图"
              >
                <LayoutList size={16} aria-hidden="true" />
              </Button>
            </div>
          </div>
        </div>

        {showFilters && allGenres.length > 0 && (
          <div className="flex flex-wrap items-center gap-2 border-t border-[var(--nv-border-subtle)] pt-4">
            <span className="mr-1 text-xs font-medium text-[var(--nv-text-tertiary)]">类型</span>
            <FilterChip selected={!filterGenre} onClick={() => setFilterGenre(null)}>全部</FilterChip>
            {allGenres.map((genre) => (
              <FilterChip
                key={genre}
                selected={filterGenre === genre}
                onClick={() => setFilterGenre(filterGenre === genre ? null : genre)}
              >
                {genre}
              </FilterChip>
            ))}
          </div>
        )}

        {hasLocalFilter && (
          <div className="flex flex-wrap items-center gap-2 border-t border-[var(--nv-border-subtle)] pt-3 text-sm text-[var(--nv-text-secondary)]">
            <span>
              找到 <strong className="font-semibold text-[var(--nv-action-primary)]">{resultCount}</strong> 个结果
            </span>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setSearchQuery('')
                setFilterGenre(null)
              }}
            >
              <X size={13} aria-hidden="true" />
              清除筛选
            </Button>
          </div>
        )}
      </Surface>

      {viewTab === 'all' && (
        <>
          {viewMode === 'grid' ? (
            loading ? (
              <div
                className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
                aria-label="正在加载媒体库内容"
              >
                {Array.from({ length: 12 }).map((_, index) => (
                  <div key={index}>
                    <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
                    <div className="skeleton mt-2 h-4 w-3/4 rounded" />
                    <div className="skeleton mt-1 h-3 w-1/2 rounded" />
                  </div>
                ))}
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
                {filteredMixed.map((item) => {
                  if (item.type === 'series' && item.series) {
                    return <MediaCard key={`s-${item.series.id}`} series={item.series} />
                  }
                  if (item.media) {
                    return <MediaCard key={`m-${item.media.id}`} media={item.media} />
                  }
                  return null
                })}
              </div>
            )
          ) : loading ? (
            <div className="space-y-2" aria-label="正在加载媒体库内容">
              {Array.from({ length: 8 }).map((_, index) => (
                <div key={index} className="skeleton h-[90px] rounded-[var(--nv-radius-card)]" />
              ))}
            </div>
          ) : (
            <div className="space-y-2">
              {filteredMixed.map((item) => {
                if (item.type === 'series' && item.series) {
                  return <ListSeriesItem key={`s-${item.series.id}`} series={item.series} />
                }
                if (item.media) {
                  return <ListMediaItem key={`m-${item.media.id}`} media={item.media} />
                }
                return null
              })}
            </div>
          )}

          {!loading && filteredMixed.length === 0 && (
            <EmptyState
              icon={<Film size={26} aria-hidden="true" />}
              title={hasLocalFilter ? '没有找到匹配的内容' : '此媒体库暂无内容'}
              description={hasLocalFilter ? '尝试清除搜索词或类型筛选。' : '扫描媒体库后，内容会显示在这里。'}
              action={hasLocalFilter ? (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => {
                    setSearchQuery('')
                    setFilterGenre(null)
                  }}
                >
                  清除筛选
                </Button>
              ) : undefined}
            />
          )}

          <Pagination
            page={page}
            totalPages={totalPages}
            total={total}
            pageSize={size}
            pageSizeOptions={[20, 30, 50, 100]}
            onPageChange={setPage}
            onPageSizeChange={setSize}
          />
        </>
      )}

      {viewTab === 'series' && (
        <>
          {loading ? (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4" aria-label="正在加载剧集合集">
              {Array.from({ length: 8 }).map((_, index) => (
                <div key={index} className="skeleton aspect-[16/11] rounded-[var(--nv-radius-card)]" />
              ))}
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {filteredSeries.map((series) => <SeriesCard key={series.id} series={series} />)}
            </div>
          )}

          {!loading && filteredSeries.length === 0 && (
            <EmptyState
              icon={<Tv size={26} aria-hidden="true" />}
              title={hasLocalFilter ? '没有找到匹配的剧集' : '此媒体库暂无剧集合集'}
              description={hasLocalFilter ? '尝试清除搜索词或类型筛选。' : '识别到剧集后，合集会显示在这里。'}
            />
          )}
        </>
      )}
    </div>
  )
}

function formatDuration(seconds: number) {
  if (!seconds) return ''
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`
}

function ListMediaItem({ media }: { media: Media }) {
  return (
    <Link
      to={media.series_id ? `/series/${media.series_id}` : `/media/${media.id}`}
      className="group flex items-center gap-4 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] p-3 transition-[background-color,border-color,box-shadow] duration-200 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)] hover:shadow-[var(--nv-shadow-card)]"
    >
      <div className="h-16 w-12 shrink-0 overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]">
        <img
          src={streamApi.getPosterUrl(media.id)}
          alt={media.title}
          className="h-full w-full object-cover"
          loading="lazy"
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />
      </div>

      <div className="min-w-0 flex-1">
        <h3 className="truncate text-sm font-medium text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]">
          {media.title}
        </h3>
        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
          {media.year > 0 && <span>{media.year}</span>}
          {media.duration > 0 && <span>{formatDuration(media.duration)}</span>}
          {media.resolution && <Tag tone="brand">{media.resolution}</Tag>}
        </div>
      </div>

      {media.rating > 0 && (
        <div className="flex shrink-0 items-center gap-1 text-sm font-semibold text-[var(--nv-status-rating)]">
          <Star size={14} fill="currentColor" aria-hidden="true" />
          <span>{media.rating.toFixed(1)}</span>
        </div>
      )}
    </Link>
  )
}

function ListSeriesItem({ series }: { series: Series }) {
  return (
    <Link
      to={`/series/${series.id}`}
      className="group flex items-center gap-4 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] p-3 transition-[background-color,border-color,box-shadow] duration-200 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)] hover:shadow-[var(--nv-shadow-card)]"
    >
      <div className="h-16 w-12 shrink-0 overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]">
        {series.poster_path ? (
          <img
            src={streamApi.getSeriesPosterUrl(series.id)}
            alt={series.title}
            className="h-full w-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
            <Tv size={16} aria-hidden="true" />
          </div>
        )}
      </div>

      <div className="min-w-0 flex-1">
        <h3 className="truncate text-sm font-medium text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]">
          {series.title}
        </h3>
        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
          {series.year > 0 && <span>{series.year}</span>}
          <span>{series.season_count} 季 · {series.episode_count} 集</span>
        </div>
      </div>

      {series.rating > 0 && (
        <div className="flex shrink-0 items-center gap-1 text-sm font-semibold text-[var(--nv-status-rating)]">
          <Star size={14} fill="currentColor" aria-hidden="true" />
          <span>{series.rating.toFixed(1)}</span>
        </div>
      )}
    </Link>
  )
}

function scrapeTone(status?: string): TagTone {
  if (status === 'failed') return 'danger'
  if (status === 'partial') return 'warning'
  return 'neutral'
}

function scrapeLabel(status?: string) {
  if (status === 'failed') return '未识别'
  if (status === 'partial') return '部分识别'
  if (status === 'pending') return '待识别'
  return null
}

function SeriesCard({ series }: { series: Series }) {
  const statusLabel = scrapeLabel(series.scrape_status)

  return (
    <Link
      to={`/series/${series.id}`}
      className="group block overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] transition-[background-color,border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-elevated)] hover:shadow-[var(--nv-shadow-card)]"
    >
      <div className="relative aspect-video overflow-hidden bg-[var(--nv-bg-surface-soft)]">
        {series.poster_path ? (
          <img
            src={streamApi.getSeriesPosterUrl(series.id)}
            alt={series.title}
            className="h-full w-full object-cover transition-[transform,filter] duration-300 group-hover:scale-[1.025] group-hover:brightness-95"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
            <Tv size={42} aria-hidden="true" />
          </div>
        )}

        <div className="absolute bottom-2 right-2">
          <Tag tone="brand">{series.season_count} 季 · {series.episode_count} 集</Tag>
        </div>

        {statusLabel && (
          <div className="absolute left-2 top-2">
            <Tag tone={scrapeTone(series.scrape_status)}>{statusLabel}</Tag>
          </div>
        )}
      </div>

      <div className="p-3.5">
        <h3 className="truncate text-sm font-semibold text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]">
          {series.title}
        </h3>
        <div className="mt-1.5 flex flex-wrap items-center gap-3 text-xs text-[var(--nv-text-tertiary)]">
          {series.year > 0 && (
            <span className="flex items-center gap-1">
              <Calendar size={12} aria-hidden="true" />
              {series.year}
            </span>
          )}
          {series.rating > 0 && (
            <span className="flex items-center gap-1 text-[var(--nv-status-rating)]">
              <Star size={12} fill="currentColor" aria-hidden="true" />
              {series.rating.toFixed(1)}
            </span>
          )}
        </div>
        {series.overview && (
          <p className="mt-2 line-clamp-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{series.overview}</p>
        )}
      </div>
    </Link>
  )
}
