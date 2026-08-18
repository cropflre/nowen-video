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
  Globe,
  Grid3X3,
  LayoutGrid,
  LayoutList,
  Play,
  SlidersHorizontal,
  Star,
  Tag as TagIcon,
  Tv,
  X,
} from 'lucide-react'
import { Button, EmptyState, SearchField, Select, Surface, Tag } from '@/components/design-system'
import { MediaArtwork, MediaGrid as SharedMediaGrid } from '@/ui'

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

type LibraryViewMode = 'grid' | 'list' | 'poster'
type LibraryViewTab = 'all' | 'series'
const MAX_CLIENT_ITEMS = 2000

function parseViewMode(value: string | null): LibraryViewMode {
  return value === 'list' || value === 'poster' ? value : 'grid'
}

function FilterChip({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button type="button" onClick={onClick} aria-pressed={selected} className="nv-button !min-h-[30px] !rounded-[9px] !px-2.5 !text-[11px]" data-variant={selected ? 'secondary' : 'ghost'} data-size="sm">
      {children}
    </button>
  )
}

function FilterGroup({ icon, label, count, children }: { icon: ReactNode; label: string; count?: number; children: ReactNode }) {
  return (
    <div className="grid gap-2 sm:grid-cols-[92px_minmax(0,1fr)] sm:items-start">
      <div className="flex min-h-[30px] items-center gap-1.5 text-[11px] font-medium text-[var(--nv-text-tertiary)]">
        <span aria-hidden="true">{icon}</span><span>{label}</span>{!!count && <Tag tone="brand">{count}</Tag>}
      </div>
      <div className="flex flex-wrap gap-1">{children}</div>
    </div>
  )
}

function ContentTabButton({ active, icon, label, count, onClick }: { active: boolean; icon: ReactNode; label: string; count: number; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} aria-pressed={active} className="nv-button" data-variant={active ? 'secondary' : 'ghost'} data-size="sm">
      {icon}<span>{label}</span><span className="text-[10px] text-[var(--nv-text-tertiary)]">{count}</span>
    </button>
  )
}

function ViewButton({ active, title, onClick, children }: { active: boolean; title: string; onClick: () => void; children: ReactNode }) {
  return (
    <Button type="button" variant="ghost" size="sm" iconOnly onClick={onClick} title={title} aria-label={title} aria-pressed={active} className={active ? '!bg-[var(--nv-fill-active)] !text-[var(--nv-text-primary)]' : undefined}>
      {children}
    </Button>
  )
}

export default function LibraryPage() {
  const { id } = useParams<{ id: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const toast = useToast()

  const page = parseInt(searchParams.get('page') || '1', 10) || 1
  const size = parseInt(searchParams.get('limit') || '30', 10) || 30
  const viewTab: LibraryViewTab = searchParams.get('tab') === 'series' ? 'series' : 'all'
  const viewMode = parseViewMode(searchParams.get('view'))
  const searchQuery = searchParams.get('q') || ''
  const sortValue = searchParams.get('sort') || 'created_desc'
  const selectedGenres = useMemo(() => {
    const genres = searchParams.get('genres') || searchParams.get('genre') || ''
    return genres.split(',').map((value) => value.trim()).filter(Boolean)
  }, [searchParams])
  const selectedCountry = searchParams.get('country') || ''
  const yearRange = useMemo<{ min: number; max: number }>(() => ({
    min: parseInt(searchParams.get('year_min') || '0', 10) || 0,
    max: parseInt(searchParams.get('year_max') || '0', 10) || 0,
  }), [searchParams])
  const minRating = parseInt(searchParams.get('rating') || '0', 10) || 0

  const [mixedItems, setMixedItems] = useState<MixedItem[]>([])
  const [seriesList, setSeriesList] = useState<Series[]>([])
  const [total, setTotal] = useState(0)
  const [serverPaginated, setServerPaginated] = useState(false)
  const [loading, setLoading] = useState(true)
  const [showFilters, setShowFilters] = useState(false)

  const updateUrl = useCallback((changes: Record<string, string | null>) => {
    const next = new URLSearchParams(searchParams)
    for (const [key, value] of Object.entries(changes)) {
      if (value === null) next.delete(key)
      else next.set(key, value)
    }
    if ('genres' in changes) next.delete('genre')
    if (!('page' in changes)) next.delete('page')
    setSearchParams(next, { replace: true })
  }, [searchParams, setSearchParams])

  const setPage = useCallback((newPage: number) => updateUrl({ page: newPage <= 1 ? null : String(newPage) }), [updateUrl])
  const setSize = useCallback((newSize: number) => updateUrl({ limit: newSize === 30 ? null : String(newSize), page: null }), [updateUrl])
  const setViewTab = useCallback((nextTab: LibraryViewTab) => updateUrl({ tab: nextTab === 'series' ? 'series' : null, page: null }), [updateUrl])
  const setViewMode = useCallback((nextMode: LibraryViewMode) => updateUrl({ view: nextMode === 'grid' ? null : nextMode }), [updateUrl])

  const clearAllFilters = useCallback(() => {
    updateUrl({ q: null, genres: null, genre: null, country: null, year_min: null, year_max: null, rating: null })
  }, [updateUrl])

  const toggleGenre = useCallback((genre: string) => {
    const next = selectedGenres.includes(genre) ? selectedGenres.filter((value) => value !== genre) : [...selectedGenres, genre]
    updateUrl({ genres: next.length > 0 ? next.join(',') : null })
  }, [selectedGenres, updateUrl])

  useEffect(() => {
    if (!id) return
    let cancelled = false
    setLoading(true)
    const load = async () => {
      try {
        const probe = await mediaApi.listMixed({ page: 1, size: 1, library_id: id })
        const totalCount = probe.data.total || 0
        const shouldPaginateOnServer = totalCount > MAX_CLIENT_ITEMS
        const [mixedRes, seriesRes] = await Promise.all([
          mediaApi.listMixed({ page: shouldPaginateOnServer ? page : 1, size: shouldPaginateOnServer ? size : Math.max(totalCount, 1), library_id: id }),
          seriesApi.list({ library_id: id }),
        ])
        if (cancelled) return
        setMixedItems(mixedRes.data.data || [])
        setSeriesList(seriesRes.data.data || [])
        setTotal(totalCount)
        setServerPaginated(shouldPaginateOnServer)
      } catch {
        if (!cancelled) toast.error('加载媒体库内容失败')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    void load()
    return () => { cancelled = true }
  }, [id, page, size, toast])

  const { allGenres, allCountries } = useMemo(() => {
    const genres = new Set<string>()
    const countries = new Set<string>()
    const collect = (genreText?: string, countryText?: string) => {
      if (genreText) genreText.split(',').forEach((genre) => { const value = genre.trim(); if (value) genres.add(value) })
      if (countryText) countryText.split(',').forEach((country) => { const value = country.trim(); if (value) countries.add(value) })
    }
    mixedItems.forEach((item) => item.type === 'series' && item.series ? collect(item.series.genres, item.series.country) : item.media && collect(item.media.genres, item.media.country))
    seriesList.forEach((series) => collect(series.genres, series.country))
    return { allGenres: Array.from(genres).sort(), allCountries: Array.from(countries).sort() }
  }, [mixedItems, seriesList])

  const getItemTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.title || '') : (item.media?.title || '')
  const getItemOrigTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.orig_title || '') : (item.media?.orig_title || '')
  const getItemOverview = (item: MixedItem) => item.type === 'series' ? (item.series?.overview || '') : (item.media?.overview || '')
  const getItemGenres = (item: MixedItem) => item.type === 'series' ? (item.series?.genres || '') : (item.media?.genres || '')
  const getItemCountry = (item: MixedItem) => item.type === 'series' ? (item.series?.country || '') : (item.media?.country || '')
  const getItemYear = (item: MixedItem) => item.type === 'series' ? (item.series?.year || 0) : (item.media?.year || 0)
  const getItemRating = (item: MixedItem) => item.type === 'series' ? (item.series?.rating || 0) : (item.media?.rating || 0)
  const getItemTime = (item: MixedItem) => item.type === 'series' ? (item.series?.created_at || '') : (item.media?.created_at || '')

  const filteredMixed = useMemo(() => {
    let items = [...mixedItems]
    if (searchQuery.trim()) {
      const query = searchQuery.trim().toLowerCase()
      items = items.filter((item) => getItemTitle(item).toLowerCase().includes(query) || getItemOrigTitle(item).toLowerCase().includes(query) || getItemOverview(item).toLowerCase().includes(query))
    }
    if (selectedGenres.length > 0) items = items.filter((item) => selectedGenres.every((genre) => getItemGenres(item).includes(genre)))
    if (selectedCountry) items = items.filter((item) => getItemCountry(item).includes(selectedCountry))
    if (yearRange.min > 0 || yearRange.max > 0) items = items.filter((item) => {
      const year = getItemYear(item)
      if (year === 0) return false
      if (yearRange.min > 0 && year < yearRange.min) return false
      if (yearRange.max > 0 && year > yearRange.max) return false
      return true
    })
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
  }, [mixedItems, searchQuery, selectedGenres, selectedCountry, yearRange, minRating, sortValue])

  const deduplicatedSeries = useMemo(() => {
    const normalize = (title: string) => title
      .replace(/\s*S\d{1,2}\s*$/i, '')
      .replace(/\s*Season\s*\d{1,2}\s*$/i, '')
      .replace(/\s*第\s*[一二三四五六七八九十\d]+\s*季\s*$/, '')
      .replace(/\s*第\s*[一二三四五六七八九十\d]+\s*部\s*$/, '')
      .replace(/\s*[\(（]\s*Season\s*\d{1,2}\s*[\)）]\s*$/i, '')
      .replace(/\s*【\s*第?\s*[一二三四五六七八九十\d]+\s*季?\s*】\s*$/, '')
      .trim() || title
    const groups = new Map<string, { best: Series; totalSeasons: number; totalEps: number }>()
    const order: string[] = []
    for (const series of seriesList) {
      const key = `${series.library_id}:${normalize(series.title)}`
      const existing = groups.get(key)
      if (existing) {
        existing.totalSeasons += series.season_count
        existing.totalEps += series.episode_count
        const score = (candidate: Series) => (candidate.overview ? 3 : 0) + (candidate.poster_path ? 3 : 0) + (candidate.rating > 0 ? 2 : 0) + (candidate.tmdb_id > 0 ? 2 : 0) + candidate.episode_count
        if (score(series) > score(existing.best)) existing.best = series
      } else {
        groups.set(key, { best: series, totalSeasons: series.season_count, totalEps: series.episode_count })
        order.push(key)
      }
    }
    return order.map((key) => {
      const group = groups.get(key)!
      return { ...group.best, season_count: group.totalSeasons, episode_count: group.totalEps }
    })
  }, [seriesList])

  const filteredSeries = useMemo(() => {
    let items = [...deduplicatedSeries]
    if (searchQuery.trim()) {
      const query = searchQuery.trim().toLowerCase()
      items = items.filter((series) => series.title.toLowerCase().includes(query) || series.orig_title?.toLowerCase().includes(query) || series.overview?.toLowerCase().includes(query))
    }
    if (selectedGenres.length > 0) items = items.filter((series) => selectedGenres.every((genre) => (series.genres || '').includes(genre)))
    if (selectedCountry) items = items.filter((series) => (series.country || '').includes(selectedCountry))
    if (yearRange.min > 0 || yearRange.max > 0) items = items.filter((series) => {
      const year = series.year || 0
      if (year === 0) return false
      if (yearRange.min > 0 && year < yearRange.min) return false
      if (yearRange.max > 0 && year > yearRange.max) return false
      return true
    })
    if (minRating > 0) items = items.filter((series) => (series.rating || 0) >= minRating)
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
  }, [deduplicatedSeries, searchQuery, selectedGenres, selectedCountry, yearRange, minRating, sortValue])

  const pagedMixed = useMemo(() => serverPaginated ? filteredMixed : filteredMixed.slice((page - 1) * size, page * size), [serverPaginated, filteredMixed, page, size])
  const pagedSeries = useMemo(() => filteredSeries.slice((page - 1) * size, page * size), [filteredSeries, page, size])
  const activeFilterCount = [selectedGenres.length > 0, selectedCountry !== '', yearRange.min > 0 || yearRange.max > 0, minRating > 0].filter(Boolean).length
  const hasLocalFilter = Boolean(searchQuery) || activeFilterCount > 0
  const allTotal = serverPaginated && !hasLocalFilter ? total : filteredMixed.length
  const resultCount = viewTab === 'all' ? allTotal : filteredSeries.length
  const totalPages = Math.ceil(resultCount / size)
  const hasSeries = deduplicatedSeries.length > 0

  useEffect(() => {
    if (page <= 1 || totalPages <= 0 || page <= totalPages) return
    updateUrl({ page: totalPages > 1 ? String(totalPages) : null })
  }, [page, totalPages, updateUrl])

  return (
    <div className="nv-section-stack">
      <div className="flex flex-wrap items-center gap-1 border-b border-[var(--nv-border-subtle)] pb-3" role="group" aria-label="媒体库内容类型">
        <ContentTabButton active={viewTab === 'all'} onClick={() => setViewTab('all')} icon={<Film size={14} />} label="全部内容" count={total} />
        {hasSeries && <ContentTabButton active={viewTab === 'series'} onClick={() => setViewTab('series')} icon={<Tv size={14} />} label="剧集合集" count={deduplicatedSeries.length} />}
      </div>

      <div className="flex flex-wrap items-center gap-1.5">
        <SearchField value={searchQuery} onChange={(event) => updateUrl({ q: event.target.value || null })} placeholder="筛选此媒体库" wrapperClassName="min-w-[190px] flex-1 lg:max-w-sm" aria-label="筛选此媒体库" />
        <Button type="button" variant={showFilters || activeFilterCount > 0 ? 'secondary' : 'ghost'} size="sm" onClick={() => setShowFilters((value) => !value)} aria-expanded={showFilters}>
          <SlidersHorizontal size={14} />筛选{activeFilterCount > 0 && <Tag tone="brand">{activeFilterCount}</Tag>}
        </Button>
        <Select value={sortValue} onChange={(event) => updateUrl({ sort: event.target.value === 'created_desc' ? null : event.target.value })} aria-label="媒体库排序" className="!w-auto min-w-28">
          {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </Select>
        <div className="ml-auto flex items-center gap-0.5 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] p-0.5" role="group" aria-label="媒体库视图">
          <ViewButton active={viewMode === 'grid'} title="网格视图" onClick={() => setViewMode('grid')}><Grid3X3 size={14} /></ViewButton>
          <ViewButton active={viewMode === 'list'} title="列表视图" onClick={() => setViewMode('list')}><LayoutList size={14} /></ViewButton>
          <ViewButton active={viewMode === 'poster'} title="海报墙视图" onClick={() => setViewMode('poster')}><LayoutGrid size={14} /></ViewButton>
        </div>
      </div>

      {showFilters && (
        <Surface variant="glass" className="space-y-2.5 p-3 sm:p-4">
          {allGenres.length > 0 && <FilterGroup icon={<TagIcon size={12} />} label="类型" count={selectedGenres.length}>{allGenres.map((genre) => <FilterChip key={genre} selected={selectedGenres.includes(genre)} onClick={() => toggleGenre(genre)}>{genre}</FilterChip>)}</FilterGroup>}
          {allCountries.length > 0 && <FilterGroup icon={<Globe size={12} />} label="地区"><FilterChip selected={!selectedCountry} onClick={() => updateUrl({ country: null })}>全部</FilterChip>{allCountries.map((country) => <FilterChip key={country} selected={selectedCountry === country} onClick={() => updateUrl({ country: selectedCountry === country ? null : country })}>{country}</FilterChip>)}</FilterGroup>}
          <FilterGroup icon={<Calendar size={12} />} label="年份">{YEAR_RANGES.map((range) => <FilterChip key={range.label} selected={yearRange.min === range.min && yearRange.max === range.max} onClick={() => updateUrl({ year_min: range.min > 0 ? String(range.min) : null, year_max: range.max > 0 ? String(range.max) : null })}>{range.label}</FilterChip>)}</FilterGroup>
          <FilterGroup icon={<Star size={12} />} label="最低评分">{RATING_OPTIONS.map((option) => <FilterChip key={option.value} selected={minRating === option.value} onClick={() => updateUrl({ rating: option.value > 0 ? String(option.value) : null })}>{option.label}</FilterChip>)}</FilterGroup>
          {activeFilterCount > 0 && <div className="flex justify-end border-t border-[var(--nv-border-subtle)] pt-2.5"><Button type="button" variant="ghost" size="sm" onClick={clearAllFilters}><X size={13} />清除所有筛选</Button></div>}
        </Surface>
      )}

      {selectedGenres.length > 0 && <div className="flex flex-wrap items-center gap-1.5">{selectedGenres.map((genre) => <Tag key={genre} tone="brand">{genre}<button type="button" onClick={() => toggleGenre(genre)} className="ml-0.5 rounded p-0.5 hover:bg-[var(--nv-fill-hover)]" aria-label={`移除 ${genre} 标签`}><X size={10} /></button></Tag>)}</div>}
      {hasLocalFilter && <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]" aria-live="polite"><span>找到 <strong className="font-semibold text-[var(--nv-text-secondary)]">{resultCount}</strong> 个结果</span><Button type="button" variant="ghost" size="sm" onClick={clearAllFilters}>清除</Button></div>}

      {loading ? (
        <LibrarySkeleton viewMode={viewMode} />
      ) : viewTab === 'all' ? (
        pagedMixed.length === 0 ? (
          <EmptyState icon={<Film size={24} />} title={hasLocalFilter ? '没有找到匹配的内容' : '此媒体库暂无内容'} description={hasLocalFilter ? '尝试调整筛选条件或使用其他关键词。' : '扫描媒体文件后，内容会显示在这里。'} action={hasLocalFilter ? <Button variant="secondary" size="sm" onClick={clearAllFilters}>清除所有筛选</Button> : undefined} />
        ) : viewMode === 'grid' ? (
          <SharedMediaGrid>{pagedMixed.map((item) => item.type === 'series' && item.series ? <MediaCard key={`s-${item.series.id}`} series={item.series} /> : item.media ? <MediaCard key={`m-${item.media.id}`} media={item.media} /> : null)}</SharedMediaGrid>
        ) : viewMode === 'list' ? (
          <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">{pagedMixed.map((item) => <LibraryListItem key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`} item={item} />)}</div>
        ) : (
          <SharedMediaGrid variant="poster">{pagedMixed.map((item) => <LibraryPosterItem key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`} item={item} />)}</SharedMediaGrid>
        )
      ) : pagedSeries.length === 0 ? (
        <EmptyState icon={<Tv size={24} />} title={hasLocalFilter ? '没有找到匹配的剧集' : '此媒体库暂无剧集合集'} description={hasLocalFilter ? '尝试调整筛选条件或使用其他关键词。' : '识别到剧集后，合集会显示在这里。'} action={hasLocalFilter ? <Button variant="secondary" size="sm" onClick={clearAllFilters}>清除所有筛选</Button> : undefined} />
      ) : viewMode === 'grid' ? (
        <SharedMediaGrid>{pagedSeries.map((series) => <MediaCard key={series.id} series={series} />)}</SharedMediaGrid>
      ) : viewMode === 'list' ? (
        <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">{pagedSeries.map((series) => <LibraryListItem key={series.id} series={series} />)}</div>
      ) : (
        <SharedMediaGrid variant="poster">{pagedSeries.map((series) => <LibraryPosterItem key={series.id} series={series} />)}</SharedMediaGrid>
      )}

      <Pagination page={page} totalPages={totalPages} total={resultCount} pageSize={size} pageSizeOptions={[20, 30, 50, 100]} onPageChange={setPage} onPageSizeChange={setSize} />
    </div>
  )
}

function LibrarySkeleton({ viewMode }: { viewMode: LibraryViewMode }) {
  const list = viewMode === 'list'
  if (list) {
    return (
      <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]" aria-busy="true">
        {Array.from({ length: 8 }).map((_, index) => (
          <div key={index} className="flex items-center gap-3 py-2.5">
            <div className="skeleton h-16 w-11 shrink-0 rounded-[9px]" />
            <div className="flex-1 space-y-2"><div className="skeleton h-3 w-3/4" /><div className="skeleton h-2.5 w-1/2" /></div>
          </div>
        ))}
      </div>
    )
  }

  return (
    <SharedMediaGrid variant={viewMode === 'poster' ? 'poster' : 'standard'} aria-busy="true">
      {Array.from({ length: 12 }).map((_, index) => (
        <div key={index}>
          <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
          {viewMode !== 'poster' && <><div className="skeleton mt-2 h-3 w-3/4" /><div className="skeleton mt-1.5 h-2.5 w-1/2" /></>}
        </div>
      ))}
    </SharedMediaGrid>
  )
}

function formatDuration(seconds: number) {
  if (!seconds) return ''
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`
}

function LibraryListItem({ item, series: seriesProp }: { item?: MixedItem; series?: Series }) {
  const [tagsExpanded, setTagsExpanded] = useState(false)
  const isSeries = Boolean(seriesProp) || item?.type === 'series'
  const series = seriesProp || (item?.type === 'series' ? item.series : undefined)
  const media = isSeries ? undefined : item?.media
  const title = series?.title || media?.title || ''
  const year = series?.year || media?.year || 0
  const rating = series?.rating || media?.rating || 0
  const genres = series?.genres || media?.genres || ''
  const country = series?.country || media?.country || ''
  const overview = series?.overview || media?.overview || ''
  const duration = media?.duration || 0
  const genreList = genres ? genres.split(',').map((genre) => genre.trim()).filter(Boolean) : []
  const visibleTags = tagsExpanded ? genreList : genreList.slice(0, 3)
  const linkTo = series ? `/series/${series.id}` : media?.series_id ? `/series/${media.series_id}` : `/media/${media?.id}`
  const posterUrl = series ? streamApi.getSeriesPosterUrl(series.id) : media?.series_id ? streamApi.getSeriesPosterUrl(media.series_id) : streamApi.getPosterUrl(media?.id || '')
  const hasPoster = series ? !!series.poster_path : media?.series_id ? !!media.series?.poster_path || !!media.poster_path : !!media?.poster_path

  return (
    <Link to={linkTo} className="group flex items-center gap-3 px-1 py-2.5 transition-colors hover:bg-[var(--nv-fill-hover)]">
      <MediaArtwork
        src={hasPoster ? posterUrl : null}
        alt=""
        ratio="poster"
        className="h-16 w-11 shrink-0 rounded-[9px]"
        fallback={isSeries ? <Tv size={15} aria-hidden="true" /> : <Film size={15} aria-hidden="true" />}
      />
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2"><h3 className="truncate text-xs font-medium text-[var(--nv-text-primary)]">{title}</h3>{isSeries && <Tag>剧集</Tag>}</div>
        <div className="mt-1 flex flex-wrap items-center gap-x-2 text-[10px] text-[var(--nv-text-tertiary)]">{year > 0 && <span>{year}</span>}{country && <span>{country}</span>}{duration > 0 && <span>{formatDuration(duration)}</span>}{series && <span>{series.season_count} 季 · {series.episode_count} 集</span>}</div>
        {genreList.length > 0 && <div className="mt-1.5 flex flex-wrap items-center gap-1">{visibleTags.map((genre) => <Tag key={genre}>{genre}</Tag>)}{genreList.length > 3 && <button type="button" onClick={(event) => { event.preventDefault(); event.stopPropagation(); setTagsExpanded((value) => !value) }} className="text-[10px] text-[var(--nv-text-tertiary)] hover:text-[var(--nv-text-primary)]">{tagsExpanded ? '收起' : `+${genreList.length - 3}`}</button>}</div>}
        {overview && <p className="mt-1 line-clamp-1 text-[10px] text-[var(--nv-text-tertiary)]">{overview}</p>}
      </div>
      {rating > 0 && <Tag tone="rating" className="shrink-0"><Star size={10} fill="currentColor" />{rating.toFixed(1)}</Tag>}
    </Link>
  )
}

function LibraryPosterItem({ item, series: seriesProp }: { item?: MixedItem; series?: Series }) {
  const isSeries = Boolean(seriesProp) || item?.type === 'series'
  const series = seriesProp || (item?.type === 'series' ? item.series : undefined)
  const media: Media | undefined = isSeries ? undefined : item?.media
  const title = series?.title || media?.title || ''
  const rating = series?.rating || media?.rating || 0
  const linkTo = series ? `/series/${series.id}` : media?.series_id ? `/series/${media.series_id}` : `/media/${media?.id}`
  const posterUrl = series ? streamApi.getSeriesPosterUrl(series.id) : media?.series_id ? streamApi.getSeriesPosterUrl(media.series_id) : streamApi.getPosterUrl(media?.id || '')
  const hasPoster = series ? !!series.poster_path : media?.series_id ? !!media.series?.poster_path || !!media.poster_path : !!media?.poster_path

  return (
    <Link to={linkTo} className="group block min-w-0" aria-label={title}>
      <MediaArtwork
        src={hasPoster ? posterUrl : null}
        alt=""
        ratio="poster"
        className="nv-media-card-poster"
        imageClassName="transition-[filter,transform] duration-200 group-hover:brightness-[.82]"
        fallback={isSeries ? <Tv size={22} aria-hidden="true" /> : <Film size={22} aria-hidden="true" />}
      >
        <div className="absolute inset-0 z-20 grid place-items-center bg-black/20 opacity-0 transition-opacity duration-200 group-hover:opacity-100">
          <span className="grid h-8 w-8 place-items-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]"><Play size={12} fill="currentColor" /></span>
        </div>
        {rating > 0 && <Tag tone="quality" className="absolute left-1.5 top-1.5 z-30"><Star size={9} fill="currentColor" />{rating.toFixed(1)}</Tag>}
        {isSeries && <Tag tone="quality" className="absolute bottom-1.5 right-1.5 z-30">剧集</Tag>}
      </MediaArtwork>
      <p className="mt-1.5 truncate text-[11px] font-medium text-[var(--nv-text-primary)]">{title}</p>
    </Link>
  )
}
