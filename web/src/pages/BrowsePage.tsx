import { useState, useEffect, useCallback, useMemo, useRef, type ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { mediaApi, seriesApi, libraryApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { usePageCache, invalidatePageCachePrefix } from '@/hooks/usePageCache'
import { usePosterVersion } from '@/stores/mediaRefresh'
import type { Series, MixedItem, Library } from '@/types'
import MediaCard from '@/components/MediaCard'
import Pagination from '@/components/Pagination'
import { Button, EmptyState, SearchField, Select, Surface, Tag as SemanticTag } from '@/components/design-system'
import { MediaArtwork, MediaGrid } from '@/ui'
import {
  X,
  Grid3X3,
  LayoutList,
  LayoutGrid,
  Film,
  Tv,
  Star,
  Calendar,
  Globe,
  Tag as TagIcon,
  Layers,
  SlidersHorizontal,
  Play,
  Info,
} from 'lucide-react'
import clsx from 'clsx'

const MAX_CLIENT_ITEMS = 2000

const SORT_OPTIONS = [
  { value: 'created_desc', label: '最近添加' },
  { value: 'rating_desc', label: '评分最高' },
  { value: 'year_desc', label: '年份最新' },
  { value: 'year_asc', label: '年份最早' },
  { value: 'title_asc', label: '名称 A-Z' },
  { value: 'title_desc', label: '名称 Z-A' },
]

const YEAR_RANGES = [
  { label: '全部', min: 0, max: 0 },
  { label: '2024-2026', min: 2024, max: 2026 },
  { label: '2020-2023', min: 2020, max: 2023 },
  { label: '2010-2019', min: 2010, max: 2019 },
  { label: '2000-2009', min: 2000, max: 2009 },
  { label: '更早', min: 0, max: 1999 },
]

const RATING_OPTIONS = [
  { label: '不限', value: 0 },
  { label: '≥6分', value: 6 },
  { label: '≥7分', value: 7 },
  { label: '≥8分', value: 8 },
  { label: '≥9分', value: 9 },
]

type ViewMode = 'grid' | 'list' | 'poster'
type MixedSort = 'added' | 'title' | 'year' | 'rating'
type SortOrder = 'asc' | 'desc'

interface BrowseProbe {
  libraryKey: string
  total: number
  movieCount: number
  seriesCount: number
}

interface BrowseData {
  scopeKey: string
  mixedItems: MixedItem[]
  seriesList: Series[]
  totalCount: number
  serverPaginated: boolean
}

const getItemTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.title || '') : (item.media?.title || '')
const getItemOrigTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.orig_title || '') : (item.media?.orig_title || '')
const getItemOverview = (item: MixedItem) => item.type === 'series' ? (item.series?.overview || '') : (item.media?.overview || '')
const getItemGenres = (item: MixedItem) => item.type === 'series' ? (item.series?.genres || '') : (item.media?.genres || '')
const getItemCountry = (item: MixedItem) => item.type === 'series' ? (item.series?.country || '') : (item.media?.country || '')
const getItemYear = (item: MixedItem) => item.type === 'series' ? (item.series?.year || 0) : (item.media?.year || 0)
const getItemRating = (item: MixedItem) => item.type === 'series' ? (item.series?.rating || 0) : (item.media?.rating || 0)
const getItemTime = (item: MixedItem) => item.type === 'series' ? (item.series?.created_at || '') : (item.media?.created_at || '')

function parseServerSort(value: string): { sort: MixedSort; order: SortOrder } {
  const [field, direction] = value.split('_')
  const sort: MixedSort = field === 'title' || field === 'year' || field === 'rating' ? field : 'added'
  return { sort, order: direction === 'asc' ? 'asc' : 'desc' }
}

function hasItemPoster(item: MixedItem): boolean {
  if (item.type === 'series') return !!item.series?.poster_path
  const media = item.media
  if (!media) return false
  if (media.series_id) return !!media.series?.poster_path || !!media.poster_path
  return !!media.poster_path
}

function getItemPosterUrl(item: MixedItem, version: number): string {
  if (item.type === 'series' && item.series) return streamApi.getSeriesPosterUrl(item.series.id, version)
  if (item.media?.series_id) return streamApi.getSeriesPosterUrl(item.media.series_id, version)
  return streamApi.getPosterUrl(item.media?.id || '', version)
}

function FilterChip({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className="nv-button !min-h-[30px] !rounded-[9px] !px-2.5 !text-[11px]"
      data-variant={selected ? 'secondary' : 'ghost'}
      data-size="sm"
    >
      {children}
    </button>
  )
}

function FilterGroup({ icon, label, count, children }: { icon: ReactNode; label: string; count?: number; children: ReactNode }) {
  return (
    <div className="grid gap-2 sm:grid-cols-[92px_minmax(0,1fr)] sm:items-start">
      <div className="flex min-h-[30px] items-center gap-1.5 text-[11px] font-medium text-[var(--nv-text-tertiary)]">
        <span aria-hidden="true">{icon}</span>
        <span>{label}</span>
        {!!count && <SemanticTag tone="brand">{count}</SemanticTag>}
      </div>
      <div className="flex flex-wrap gap-1">{children}</div>
    </div>
  )
}

function ViewButton({ active, title, onClick, children }: { active: boolean; title: string; onClick: () => void; children: ReactNode }) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      iconOnly
      onClick={onClick}
      title={title}
      aria-label={title}
      aria-pressed={active}
      className={active ? '!bg-[var(--nv-fill-active)] !text-[var(--nv-text-primary)]' : undefined}
    >
      {children}
    </Button>
  )
}

export default function BrowsePage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const toast = useToast()
  const { on, off } = useWebSocket()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])

  const page = parseInt(searchParams.get('page') || '1', 10) || 1
  const size = parseInt(searchParams.get('size') || '30', 10) || 30
  const searchQuery = searchParams.get('q') || ''
  const selectedLibrary = searchParams.get('lib') || ''
  const mediaType = (searchParams.get('type') || '') as '' | 'movie' | 'series'
  const selectedGenres = useMemo(() => {
    const genres = searchParams.get('genres')
    return genres ? genres.split(',').filter(Boolean) : []
  }, [searchParams])
  const selectedCountry = searchParams.get('country') || ''
  const yearRange = useMemo<{ min: number; max: number }>(() => ({
    min: parseInt(searchParams.get('year_min') || '0', 10) || 0,
    max: parseInt(searchParams.get('year_max') || '0', 10) || 0,
  }), [searchParams])
  const minRating = parseInt(searchParams.get('rating') || '0', 10) || 0
  const sortValue = searchParams.get('sort') || 'created_desc'
  const viewMode = (searchParams.get('view') || 'grid') as ViewMode
  const [showFilters, setShowFilters] = useState(false)
  const [searchInput, setSearchInput] = useState(searchQuery)

  const updateUrl = useCallback((changes: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams)
    for (const [key, value] of Object.entries(changes)) {
      if (value === null) next.delete(key)
      else next.set(key, value)
    }
    if (!('page' in changes)) next.delete('page')
    setSearchParams(next, { replace: true })
  }, [searchParams, setSearchParams])

  const setPage = useCallback((newPage: number) => updateUrl({ page: newPage <= 1 ? null : String(newPage) }), [updateUrl])
  const setPageSize = useCallback((newSize: number) => updateUrl({ size: newSize === 30 ? null : String(newSize) }), [updateUrl])

  useEffect(() => {
    libraryApi.list().then((res) => setLibraries(res.data.data || [])).catch(() => {})
  }, [])

  useEffect(() => { setSearchInput(searchQuery) }, [searchQuery])

  const libraryKey = selectedLibrary || 'all'
  const { data: probeData, loading: probeLoading, refetch: refetchProbe } = usePageCache<BrowseProbe>(
    `browse:probe:lib=${libraryKey}`,
    async () => {
      const probe = await mediaApi.listMixed({ page: 1, size: 1, library_id: selectedLibrary || undefined })
      return {
        libraryKey,
        total: probe.data.total || 0,
        movieCount: probe.data.movie_count || 0,
        seriesCount: probe.data.series_count || 0,
      }
    },
    { ttl: 20_000 },
  )

  const probeReady = probeData?.libraryKey === libraryKey
  const serverPaginated = !!probeReady && (probeData?.total || 0) > MAX_CLIENT_ITEMS
  const serverGenre = selectedGenres[0] || ''
  const serverSort = useMemo(() => parseServerSort(sortValue), [sortValue])

  const browseKey = useMemo(() => {
    if (!probeReady) return null
    if (!serverPaginated) return `browse:client:lib=${libraryKey}`
    return [
      'browse:server',
      `lib=${libraryKey}`,
      `page=${page}`,
      `size=${size}`,
      `type=${mediaType || 'all'}`,
      `q=${encodeURIComponent(searchQuery.trim())}`,
      `genre=${encodeURIComponent(serverGenre)}`,
      `year=${yearRange.min}-${yearRange.max}`,
      `sort=${serverSort.sort}-${serverSort.order}`,
    ].join(':')
  }, [probeReady, serverPaginated, libraryKey, page, size, mediaType, searchQuery, serverGenre, yearRange.min, yearRange.max, serverSort])

  const { data: rawBrowseData, loading: browseLoading, refetch } = usePageCache<BrowseData>(
    browseKey,
    async () => {
      if (!browseKey || !probeData) throw new Error('Browse probe is not ready')
      const libraryId = selectedLibrary || undefined

      if (!serverPaginated) {
        const [mixedRes, seriesRes] = await Promise.all([
          mediaApi.listMixed({ page: 1, size: MAX_CLIENT_ITEMS, library_id: libraryId }),
          seriesApi.list({ library_id: libraryId }),
        ])
        return {
          scopeKey: browseKey,
          mixedItems: mixedRes.data.data || [],
          seriesList: seriesRes.data.data || [],
          totalCount: mixedRes.data.total || probeData.total,
          serverPaginated: false,
        }
      }

      const mixedRes = await mediaApi.listMixed({
        page,
        size,
        library_id: libraryId,
        type: mediaType || undefined,
        q: searchQuery.trim() || undefined,
        genre: serverGenre || undefined,
        year_from: yearRange.min || undefined,
        year_to: yearRange.max || undefined,
        sort: serverSort.sort,
        order: serverSort.order,
      })
      return {
        scopeKey: browseKey,
        mixedItems: mixedRes.data.data || [],
        seriesList: [],
        totalCount: mixedRes.data.total || 0,
        serverPaginated: true,
      }
    },
    { ttl: 20_000 },
  )

  const browseData = rawBrowseData?.scopeKey === browseKey ? rawBrowseData : undefined
  const loading = probeLoading || browseLoading || !probeReady
  const mixedItems = browseData?.mixedItems ?? []
  const seriesList = browseData?.seriesList ?? []
  const totalCount = browseData?.totalCount ?? 0

  const toastRef = useRef(toast)
  useEffect(() => { toastRef.current = toast }, [toast])

  useEffect(() => {
    if (!serverPaginated) return
    const changes: Record<string, string | null> = {}
    if (selectedCountry) changes.country = null
    if (minRating > 0) changes.rating = null
    if (selectedGenres.length > 1) changes.genres = selectedGenres[0] || null
    if (Object.keys(changes).length === 0) return
    toastRef.current.info('大型影视库使用服务端分页，已移除当前接口无法准确执行的组合筛选条件')
    updateUrl(changes)
  }, [serverPaginated, selectedCountry, minRating, selectedGenres, updateUrl])

  useEffect(() => {
    if (searchInput === searchQuery) return
    const timer = setTimeout(() => updateUrl({ q: searchInput || null }), serverPaginated ? 280 : 120)
    return () => clearTimeout(timer)
  }, [searchInput, searchQuery, serverPaginated, updateUrl])

  const hasDataRef = useRef(false)
  useEffect(() => { if (browseData) hasDataRef.current = true }, [browseData])
  useEffect(() => {
    if (!loading && !browseData && hasDataRef.current) toastRef.current.error('加载影视库内容失败')
  }, [loading, browseData])

  useEffect(() => {
    const debouncedRefresh = () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(() => {
        invalidatePageCachePrefix('browse:')
        void refetchProbe(true)
        void refetch(true)
      }, 1000)
    }
    on(WS_EVENTS.SCAN_COMPLETED, debouncedRefresh)
    on(WS_EVENTS.SCRAPE_COMPLETED, debouncedRefresh)
    on(WS_EVENTS.LIBRARY_UPDATED, debouncedRefresh)
    return () => {
      off(WS_EVENTS.SCAN_COMPLETED, debouncedRefresh)
      off(WS_EVENTS.SCRAPE_COMPLETED, debouncedRefresh)
      off(WS_EVENTS.LIBRARY_UPDATED, debouncedRefresh)
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
    }
  }, [on, off, refetch, refetchProbe])

  const { allGenres, allCountries } = useMemo(() => {
    const genres = new Set<string>()
    const countries = new Set<string>()
    const collect = (genreText?: string, countryText?: string) => {
      if (genreText) genreText.split(',').forEach((value) => { const genre = value.trim(); if (genre) genres.add(genre) })
      if (countryText) countryText.split(',').forEach((value) => { const country = value.trim(); if (country) countries.add(country) })
    }
    mixedItems.forEach((item) => {
      if (item.type === 'series' && item.series) collect(item.series.genres, item.series.country)
      else if (item.media) collect(item.media.genres, item.media.country)
    })
    seriesList.forEach((series) => collect(series.genres, series.country))
    return { allGenres: Array.from(genres).sort(), allCountries: Array.from(countries).sort() }
  }, [mixedItems, seriesList])

  const filteredItems = useMemo(() => {
    if (serverPaginated) return mixedItems
    let items = [...mixedItems]
    if (mediaType === 'movie') items = items.filter((item) => item.type === 'movie')
    else if (mediaType === 'series') items = items.filter((item) => item.type === 'series')

    if (searchQuery.trim()) {
      const query = searchQuery.trim().toLowerCase()
      items = items.filter((item) =>
        getItemTitle(item).toLowerCase().includes(query) ||
        getItemOrigTitle(item).toLowerCase().includes(query) ||
        getItemOverview(item).toLowerCase().includes(query),
      )
    }
    if (selectedGenres.length > 0) items = items.filter((item) => selectedGenres.every((genre) => getItemGenres(item).includes(genre)))
    if (selectedCountry) items = items.filter((item) => getItemCountry(item).includes(selectedCountry))
    if (yearRange.min > 0 || yearRange.max > 0) {
      items = items.filter((item) => {
        const year = getItemYear(item)
        if (year === 0) return false
        if (yearRange.min > 0 && year < yearRange.min) return false
        if (yearRange.max > 0 && year > yearRange.max) return false
        return true
      })
    }
    if (minRating > 0) items = items.filter((item) => getItemRating(item) >= minRating)

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
  }, [serverPaginated, mixedItems, mediaType, searchQuery, selectedGenres, selectedCountry, yearRange, minRating, sortValue])

  const totalPages = serverPaginated ? Math.ceil(totalCount / size) : Math.ceil(filteredItems.length / size)
  const pagedItems = useMemo(() => {
    if (serverPaginated) return filteredItems
    return filteredItems.slice((page - 1) * size, page * size)
  }, [serverPaginated, filteredItems, page, size])

  const activeFilterCount = [selectedGenres.length > 0, selectedCountry !== '', yearRange.min > 0 || yearRange.max > 0, minRating > 0].filter(Boolean).length

  const clearAllFilters = () => {
    setSearchInput('')
    const next = new URLSearchParams()
    if (selectedLibrary) next.set('lib', selectedLibrary)
    if (size !== 30) next.set('size', String(size))
    if (viewMode !== 'grid') next.set('view', viewMode)
    setSearchParams(next, { replace: true })
  }

  const toggleGenre = (genre: string) => {
    if (serverPaginated) {
      updateUrl({ genres: selectedGenres.includes(genre) ? null : genre })
      return
    }
    const next = selectedGenres.includes(genre) ? selectedGenres.filter((value) => value !== genre) : [...selectedGenres, genre]
    updateUrl({ genres: next.length > 0 ? next.join(',') : null })
  }

  const stats = useMemo(() => {
    if (serverPaginated && probeData) {
      return { movieCount: probeData.movieCount, seriesCount: probeData.seriesCount, total: probeData.total }
    }
    let movieCount = 0
    let seriesCount = 0
    mixedItems.forEach((item) => { if (item.type === 'movie') movieCount++; else if (item.type === 'series') seriesCount++ })
    return { movieCount, seriesCount, total: mixedItems.length }
  }, [mixedItems, serverPaginated, probeData])

  const hasSearchOrFilters = mediaType !== '' || !!searchQuery || activeFilterCount > 0

  return (
    <div className={clsx('nv-section-stack')}>
      <div className="nv-browse-type-tabs flex flex-wrap items-center gap-1 border-b border-[var(--nv-border-subtle)] pb-3" aria-label="媒体类型">
        {[
          { key: '' as const, label: '全部', icon: Layers, value: stats.total },
          { key: 'movie' as const, label: '电影', icon: Film, value: stats.movieCount },
          { key: 'series' as const, label: '剧集', icon: Tv, value: stats.seriesCount },
        ].map(({ key, label, icon: Icon, value }) => {
          const selected = mediaType === key
          return (
            <button
              key={key || 'all'}
              type="button"
              onClick={() => updateUrl({ type: selected || key === '' ? null : key })}
              aria-pressed={selected}
              className="nv-button"
              data-variant={selected ? 'secondary' : 'ghost'}
              data-size="sm"
            >
              <Icon size={14} aria-hidden="true" />
              <span>{label}</span>
              <span className="text-[10px] text-[var(--nv-text-tertiary)]">{value}</span>
            </button>
          )
        })}
        {!serverPaginated && <SemanticTag className="ml-1"><TagIcon size={10} aria-hidden="true" />{allGenres.length} 类型</SemanticTag>}
      </div>

      <div className="nv-browse-toolbar flex flex-wrap items-center gap-1.5">
        {libraries.length > 1 && (
          <Select
            value={selectedLibrary}
            onChange={(event) => updateUrl({ lib: event.target.value || null })}
            aria-label="媒体库"
            className="!w-auto min-w-28"
          >
            <option value="">全部媒体库</option>
            {libraries.map((library) => <option key={library.id} value={library.id}>{library.name}</option>)}
          </Select>
        )}

        <SearchField
          value={searchInput}
          onChange={(event) => setSearchInput(event.target.value)}
          placeholder={serverPaginated ? '服务端搜索标题' : '筛选当前影视库'}
          wrapperClassName="min-w-[190px] flex-1 lg:max-w-sm"
          aria-label="筛选当前影视库"
        />

        <Button type="button" variant={showFilters || activeFilterCount > 0 ? 'secondary' : 'ghost'} size="sm" onClick={() => setShowFilters((value) => !value)} aria-expanded={showFilters}>
          <SlidersHorizontal size={14} aria-hidden="true" />
          筛选
          {activeFilterCount > 0 && <SemanticTag tone="brand">{activeFilterCount}</SemanticTag>}
        </Button>

        <Select value={sortValue} onChange={(event) => updateUrl({ sort: event.target.value === 'created_desc' ? null : event.target.value })} aria-label="排序方式" className="!w-auto min-w-28">
          {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </Select>

        <div className="flex items-center gap-0.5 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] p-0.5" role="group" aria-label="视图模式">
          <ViewButton active={viewMode === 'grid'} title="网格视图" onClick={() => updateUrl({ view: null })}><Grid3X3 size={14} /></ViewButton>
          <ViewButton active={viewMode === 'list'} title="列表视图" onClick={() => updateUrl({ view: 'list' })}><LayoutList size={14} /></ViewButton>
          <ViewButton active={viewMode === 'poster'} title="海报墙视图" onClick={() => updateUrl({ view: 'poster' })}><LayoutGrid size={14} /></ViewButton>
        </div>
      </div>

      {showFilters && (
        <Surface variant="glass" className="nv-browse-filter-panel space-y-2.5 p-3 sm:p-4">
          {serverPaginated ? (
            <div className="flex items-start gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3 py-2 text-[11px] leading-5 text-[var(--nv-text-tertiary)]">
              <Info size={14} className="mt-0.5 shrink-0" aria-hidden="true" />
              <span>大型库模式下，媒体类型、关键词、年份与排序由服务端准确执行；题材、地区和最低评分组合筛选在当前库不超过 {MAX_CLIENT_ITEMS} 部时开放。</span>
            </div>
          ) : (
            <>
              {allGenres.length > 0 && (
                <FilterGroup icon={<TagIcon size={12} />} label="类型" count={selectedGenres.length}>
                  {allGenres.map((genre) => <FilterChip key={genre} selected={selectedGenres.includes(genre)} onClick={() => toggleGenre(genre)}>{genre}</FilterChip>)}
                </FilterGroup>
              )}
              {allCountries.length > 0 && (
                <FilterGroup icon={<Globe size={12} />} label="地区">
                  <FilterChip selected={!selectedCountry} onClick={() => updateUrl({ country: null })}>全部</FilterChip>
                  {allCountries.map((country) => <FilterChip key={country} selected={selectedCountry === country} onClick={() => updateUrl({ country: selectedCountry === country ? null : country })}>{country}</FilterChip>)}
                </FilterGroup>
              )}
            </>
          )}
          <FilterGroup icon={<Calendar size={12} />} label="年份">
            {YEAR_RANGES.map((range) => (
              <FilterChip key={range.label} selected={yearRange.min === range.min && yearRange.max === range.max} onClick={() => updateUrl({ year_min: range.min > 0 ? String(range.min) : null, year_max: range.max > 0 ? String(range.max) : null })}>{range.label}</FilterChip>
            ))}
          </FilterGroup>
          {!serverPaginated && (
            <FilterGroup icon={<Star size={12} />} label="最低评分">
              {RATING_OPTIONS.map((option) => <FilterChip key={option.value} selected={minRating === option.value} onClick={() => updateUrl({ rating: option.value > 0 ? String(option.value) : null })}>{option.label}</FilterChip>)}
            </FilterGroup>
          )}
          {activeFilterCount > 0 && (
            <div className="flex items-center justify-end border-t border-[var(--nv-border-subtle)] pt-2.5">
              <Button type="button" variant="ghost" size="sm" onClick={clearAllFilters}><X size={13} />清除所有筛选</Button>
            </div>
          )}
        </Surface>
      )}

      {selectedGenres.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5" aria-label="已选类型标签">
          {selectedGenres.map((genre) => (
            <SemanticTag key={genre} tone="brand">
              {genre}
              <button type="button" onClick={() => toggleGenre(genre)} aria-label={`移除 ${genre} 标签`} className="ml-0.5 inline-flex rounded p-0.5 hover:bg-[var(--nv-fill-hover)]"><X size={10} /></button>
            </SemanticTag>
          ))}
        </div>
      )}

      {hasSearchOrFilters && !serverPaginated && (
        <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]" aria-live="polite">
          <span>找到 <strong className="font-semibold text-[var(--nv-text-secondary)]">{filteredItems.length}</strong> 个结果</span>
          <Button type="button" variant="ghost" size="sm" onClick={clearAllFilters}>清除</Button>
        </div>
      )}

      {serverPaginated && (
        <div className="flex items-start gap-2 border-y border-[var(--nv-border-subtle)] py-2.5 text-[11px] leading-5 text-[var(--nv-text-tertiary)]" role="status">
          <Info size={14} className="mt-0.5 shrink-0" aria-hidden="true" />
          <span>大型影视库模式（库内共 {probeData?.total || 0} 部），当前条件匹配 {totalCount} 部；筛选与排序先在服务端执行，再进行稳定分页。</span>
        </div>
      )}

      {loading ? (
        <BrowseSkeleton viewMode={viewMode} />
      ) : pagedItems.length === 0 ? (
        <EmptyState
          icon={<Film size={24} aria-hidden="true" />}
          title={hasSearchOrFilters ? '没有找到匹配的内容' : '影视库暂无内容'}
          description={hasSearchOrFilters ? '尝试调整筛选条件或使用其他关键词。' : '前往管理页面添加媒体库并扫描文件。'}
          action={hasSearchOrFilters ? <Button type="button" variant="secondary" size="sm" onClick={clearAllFilters}>清除所有筛选</Button> : undefined}
        />
      ) : viewMode === 'grid' ? (
        <MediaGrid>
          {pagedItems.map((item) => item.type === 'series' && item.series
            ? <MediaCard key={`s-${item.series.id}`} series={item.series} />
            : item.media ? <MediaCard key={`m-${item.media.id}`} media={item.media} /> : null)}
        </MediaGrid>
      ) : viewMode === 'list' ? (
        <div className="nv-browse-list divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
          {pagedItems.map((item) => <BrowseListItem key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`} item={item} />)}
        </div>
      ) : (
        <MediaGrid variant="poster">
          {pagedItems.map((item) => <PosterWallItem key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`} item={item} />)}
        </MediaGrid>
      )}

      <Pagination page={page} totalPages={totalPages} total={serverPaginated ? totalCount : filteredItems.length} pageSize={size} pageSizeOptions={[20, 30, 50, 100]} onPageSizeChange={setPageSize} onPageChange={setPage} />
    </div>
  )
}

function BrowseSkeleton({ viewMode }: { viewMode: ViewMode }) {
  const list = viewMode === 'list'
  if (list) {
    return (
      <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
        {Array.from({ length: 8 }).map((_, index) => (
          <div key={index} className="flex items-center gap-3 py-2.5">
            <div className="skeleton h-16 w-11 shrink-0 rounded-[var(--nv-radius-control)]" />
            <div className="flex-1 space-y-2"><div className="skeleton h-3 w-3/4" /><div className="skeleton h-2.5 w-1/2" /></div>
          </div>
        ))}
      </div>
    )
  }

  return (
    <MediaGrid variant={viewMode === 'poster' ? 'poster' : 'standard'}>
      {Array.from({ length: 12 }).map((_, index) => (
        <div key={index}><div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" /><div className="skeleton mt-2 h-3 w-3/4" /><div className="skeleton mt-1.5 h-2.5 w-1/2" /></div>
      ))}
    </MediaGrid>
  )
}

function BrowseListItem({ item }: { item: MixedItem }) {
  const [tagsExpanded, setTagsExpanded] = useState(false)
  const posterVersion = usePosterVersion()
  const isSeries = item.type === 'series'
  const media = isSeries ? undefined : item.media
  const series = isSeries ? item.series : undefined
  const title = series?.title || media?.title || ''
  const year = series?.year || media?.year || 0
  const rating = series?.rating || media?.rating || 0
  const genres = series?.genres || media?.genres || ''
  const country = series?.country || media?.country || ''
  const overview = series?.overview || media?.overview || ''
  const duration = media?.duration || 0
  const genreList = genres ? genres.split(',').map((genre: string) => genre.trim()).filter(Boolean) : []
  const visibleTags = tagsExpanded ? genreList : genreList.slice(0, 3)
  const linkTo = series ? `/series/${series.id}` : media?.series_id ? `/series/${media.series_id}` : `/media/${media?.id}`
  const posterUrl = getItemPosterUrl(item, posterVersion)
  const hasPoster = hasItemPoster(item)
  const durationLabel = duration ? `${Math.floor(duration / 3600) ? `${Math.floor(duration / 3600)}h ` : ''}${Math.floor((duration % 3600) / 60)}m` : ''

  return (
    <Link to={linkTo} className="nv-browse-list-item group flex items-center gap-3 px-1 py-2.5 transition-colors hover:bg-[var(--nv-fill-hover)]">
      <MediaArtwork
        src={hasPoster ? posterUrl : null}
        alt=""
        ratio="poster"
        className="h-16 w-11 shrink-0 !rounded-[9px]"
        fallback={isSeries ? <Tv size={15} aria-hidden="true" /> : <Film size={15} aria-hidden="true" />}
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2"><h3 className="truncate text-xs font-medium text-[var(--nv-text-primary)]">{title}</h3>{isSeries && <SemanticTag>剧集</SemanticTag>}</div>
        <div className="mt-1 flex flex-wrap items-center gap-x-2 text-[10px] text-[var(--nv-text-tertiary)]">
          {year > 0 && <span>{year}</span>}{country && <span>{country}</span>}{durationLabel && <span>{durationLabel}</span>}{isSeries && series && <span>{series.season_count} 季 · {series.episode_count} 集</span>}
        </div>
        {genreList.length > 0 && (
          <div className="mt-1.5 flex flex-wrap items-center gap-1">
            {visibleTags.map((genre) => <SemanticTag key={genre}>{genre}</SemanticTag>)}
            {genreList.length > 3 && <button type="button" onClick={(event) => { event.preventDefault(); event.stopPropagation(); setTagsExpanded((value) => !value) }} className="text-[10px] text-[var(--nv-text-tertiary)] hover:text-[var(--nv-text-primary)]">{tagsExpanded ? '收起' : `+${genreList.length - 3}`}</button>}
          </div>
        )}
        {overview && <p className="mt-1 line-clamp-1 text-[10px] text-[var(--nv-text-tertiary)]">{overview}</p>}
      </div>
      {rating > 0 && <SemanticTag tone="rating" className="shrink-0"><Star size={10} fill="currentColor" />{rating.toFixed(1)}</SemanticTag>}
    </Link>
  )
}

function PosterWallItem({ item }: { item: MixedItem }) {
  const posterVersion = usePosterVersion()
  const isSeries = item.type === 'series'
  const media = isSeries ? undefined : item.media
  const series = isSeries ? item.series : undefined
  const title = series?.title || media?.title || ''
  const rating = series?.rating || media?.rating || 0
  const linkTo = series ? `/series/${series.id}` : media?.series_id ? `/series/${media.series_id}` : `/media/${media?.id}`
  const posterUrl = getItemPosterUrl(item, posterVersion)
  const hasPoster = hasItemPoster(item)

  return (
    <Link to={linkTo} className="nv-browse-poster-card group block min-w-0" aria-label={title}>
      <MediaArtwork
        src={hasPoster ? posterUrl : null}
        alt=""
        ratio="poster"
        className="shadow-[0_5px_16px_rgba(0,0,0,.12)] transition-[transform,box-shadow,border-color] duration-200 group-hover:-translate-y-[3px] group-hover:border-[var(--nv-border-default)] group-hover:shadow-[var(--nv-shadow-card-hover)]"
        imageClassName="transition-[filter] duration-200 group-hover:brightness-[.82]"
        fallback={(
          <div className="flex flex-col items-center justify-center gap-2 text-[var(--nv-text-tertiary)]">
            {isSeries ? <Tv size={22} aria-hidden="true" /> : <Film size={22} aria-hidden="true" />}
            <span className="text-[9px]">暂无海报</span>
          </div>
        )}
      >
        <div className="absolute inset-0 z-10 grid place-items-center bg-[var(--nv-bg-overlay)] opacity-0 transition-opacity duration-200 group-hover:opacity-100">
          <span className="grid h-8 w-8 place-items-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]"><Play size={12} fill="currentColor" /></span>
        </div>
        {rating > 0 && <SemanticTag tone="quality" className="absolute left-1.5 top-1.5 z-20"><Star size={9} fill="currentColor" />{rating.toFixed(1)}</SemanticTag>}
        {isSeries && <SemanticTag tone="quality" className="absolute bottom-1.5 right-1.5 z-20">剧集</SemanticTag>}
      </MediaArtwork>
      <p className="mt-1.5 truncate text-[11px] font-medium text-[var(--nv-text-primary)]">{title}</p>
    </Link>
  )
}
