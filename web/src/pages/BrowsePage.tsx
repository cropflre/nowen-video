import { useState, useEffect, useCallback, useMemo, useRef, type ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { mediaApi, seriesApi, libraryApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { usePageCache, invalidatePageCachePrefix } from '@/hooks/usePageCache'
import type { Series, MixedItem, Library } from '@/types'
import MediaCard from '@/components/MediaCard'
import Pagination from '@/components/Pagination'
import { Button, EmptyState, Input, Select, Surface, Tag as SemanticTag } from '@/components/design-system'
import { motion, AnimatePresence } from 'framer-motion'
import {
  pageVariants,
  staggerContainerVariants,
  staggerItemVariants,
  easeSmooth,
} from '@/lib/motion'
import {
  Search,
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

const getItemTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.title || '') : (item.media?.title || '')
const getItemOrigTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.orig_title || '') : (item.media?.orig_title || '')
const getItemOverview = (item: MixedItem) => item.type === 'series' ? (item.series?.overview || '') : (item.media?.overview || '')
const getItemGenres = (item: MixedItem) => item.type === 'series' ? (item.series?.genres || '') : (item.media?.genres || '')
const getItemCountry = (item: MixedItem) => item.type === 'series' ? (item.series?.country || '') : (item.media?.country || '')
const getItemYear = (item: MixedItem) => item.type === 'series' ? (item.series?.year || 0) : (item.media?.year || 0)
const getItemRating = (item: MixedItem) => item.type === 'series' ? (item.series?.rating || 0) : (item.media?.rating || 0)
const getItemTime = (item: MixedItem) => item.type === 'series' ? (item.series?.created_at || '') : (item.media?.created_at || '')

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

function FilterGroup({
  icon,
  label,
  count,
  children,
}: {
  icon: ReactNode
  label: string
  count?: number
  children: ReactNode
}) {
  return (
    <div className="space-y-2.5">
      <div className="flex items-center gap-2 text-xs font-semibold text-[var(--nv-text-secondary)]">
        <span className="text-[var(--nv-text-tertiary)]" aria-hidden="true">{icon}</span>
        <span>{label}</span>
        {!!count && <SemanticTag tone="brand">{count}</SemanticTag>}
      </div>
      <div className="flex flex-wrap gap-2">{children}</div>
    </div>
  )
}

function ViewButton({
  active,
  title,
  onClick,
  children,
}: {
  active: boolean
  title: string
  onClick: () => void
  children: ReactNode
}) {
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
      className={active ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : undefined}
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

  const setPage = useCallback((newPage: number) => {
    updateUrl({ page: newPage <= 1 ? null : String(newPage) })
  }, [updateUrl])

  const setPageSize = useCallback((newSize: number) => {
    updateUrl({ size: newSize === 30 ? null : String(newSize) })
  }, [updateUrl])

  useEffect(() => {
    libraryApi.list().then((res) => {
      setLibraries(res.data.data || [])
    }).catch(() => {})
  }, [])

  useEffect(() => {
    setSearchInput(searchQuery)
  }, [searchQuery])

  const MAX_CLIENT_ITEMS = 2000

  interface BrowseData {
    mixedItems: MixedItem[]
    seriesList: Series[]
    totalCount: number
    movieCount: number
    seriesCount: number
    serverPaginated: boolean
  }

  const { data: browseData, loading, refetch } = usePageCache<BrowseData>(
    `browse:lib=${selectedLibrary || 'all'}:page=${page}:size=${size}`,
    async () => {
      const libraryId = selectedLibrary || undefined
      const probe = await mediaApi.listMixed({ page: 1, size: 1, library_id: libraryId })
      const total = probe.data.total || 0
      const movieCount = probe.data.movie_count || 0
      const seriesCount = probe.data.series_count || 0

      if (total <= MAX_CLIENT_ITEMS) {
        const [mixedRes, seriesRes] = await Promise.all([
          mediaApi.listMixed({ page: 1, size: MAX_CLIENT_ITEMS, library_id: libraryId }),
          seriesApi.list({ library_id: libraryId }),
        ])
        return {
          mixedItems: mixedRes.data.data || [],
          seriesList: seriesRes.data.data || [],
          totalCount: total,
          movieCount,
          seriesCount,
          serverPaginated: false,
        }
      }

      const [mixedRes, seriesRes] = await Promise.all([
        mediaApi.listMixed({ page, size, library_id: libraryId }),
        seriesApi.list({ library_id: libraryId }),
      ])
      return {
        mixedItems: mixedRes.data.data || [],
        seriesList: seriesRes.data.data || [],
        totalCount: total,
        movieCount,
        seriesCount,
        serverPaginated: true,
      }
    },
    { ttl: 20_000 },
  )

  const mixedItems = browseData?.mixedItems ?? []
  const seriesList = browseData?.seriesList ?? []
  const totalCount = browseData?.totalCount ?? 0
  const serverMovieCount = browseData?.movieCount ?? 0
  const serverSeriesCount = browseData?.seriesCount ?? 0
  const serverPaginated = browseData?.serverPaginated ?? false

  const toastRef = useRef(toast)
  useEffect(() => { toastRef.current = toast }, [toast])
  const hasDataRef = useRef(false)
  useEffect(() => {
    if (browseData) hasDataRef.current = true
  }, [browseData])
  useEffect(() => {
    if (!loading && !browseData && hasDataRef.current) {
      toastRef.current.error('加载影视库内容失败')
    }
  }, [loading, browseData])

  useEffect(() => {
    const debouncedRefresh = () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(() => {
        invalidatePageCachePrefix('browse:')
        refetch(true)
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
  }, [on, off, refetch])

  const { allGenres, allCountries } = useMemo(() => {
    const genres = new Set<string>()
    const countries = new Set<string>()
    const collect = (genreText?: string, countryText?: string) => {
      if (genreText) genreText.split(',').forEach((value) => {
        const genre = value.trim()
        if (genre) genres.add(genre)
      })
      if (countryText) countryText.split(',').forEach((value) => {
        const country = value.trim()
        if (country) countries.add(country)
      })
    }

    mixedItems.forEach((item) => {
      if (item.type === 'series' && item.series) collect(item.series.genres, item.series.country)
      else if (item.media) collect(item.media.genres, item.media.country)
    })
    seriesList.forEach((series) => collect(series.genres, series.country))

    return {
      allGenres: Array.from(genres).sort(),
      allCountries: Array.from(countries).sort(),
    }
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
        getItemOverview(item).toLowerCase().includes(query)
      )
    }

    if (selectedGenres.length > 0) {
      items = items.filter((item) => {
        const genres = getItemGenres(item)
        return selectedGenres.every((genre) => genres.includes(genre))
      })
    }

    if (selectedCountry) {
      items = items.filter((item) => getItemCountry(item).includes(selectedCountry))
    }

    if (yearRange.min > 0 || yearRange.max > 0) {
      items = items.filter((item) => {
        const year = getItemYear(item)
        if (year === 0) return false
        if (yearRange.min > 0 && year < yearRange.min) return false
        if (yearRange.max > 0 && year > yearRange.max) return false
        return true
      })
    }

    if (minRating > 0) {
      items = items.filter((item) => getItemRating(item) >= minRating)
    }

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

  const totalPages = serverPaginated
    ? Math.ceil(totalCount / size)
    : Math.ceil(filteredItems.length / size)

  const pagedItems = useMemo(() => {
    if (serverPaginated) return filteredItems
    const start = (page - 1) * size
    return filteredItems.slice(start, start + size)
  }, [serverPaginated, filteredItems, page, size])

  const activeFilterCount = [
    selectedGenres.length > 0,
    selectedCountry !== '',
    yearRange.min > 0 || yearRange.max > 0,
    minRating > 0,
  ].filter(Boolean).length

  const clearAllFilters = () => {
    setSearchInput('')
    const next = new URLSearchParams()
    if (size !== 30) next.set('size', String(size))
    if (viewMode !== 'grid') next.set('view', viewMode)
    setSearchParams(next, { replace: true })
  }

  const toggleGenre = (genre: string) => {
    const next = selectedGenres.includes(genre)
      ? selectedGenres.filter((value) => value !== genre)
      : [...selectedGenres, genre]
    updateUrl({ genres: next.length > 0 ? next.join(',') : null })
  }

  const stats = useMemo(() => {
    if (serverPaginated) {
      return { movieCount: serverMovieCount, seriesCount: serverSeriesCount, total: totalCount }
    }
    let movieCount = 0
    let seriesCount = 0
    mixedItems.forEach((item) => {
      if (item.type === 'movie') movieCount++
      else if (item.type === 'series') seriesCount++
    })
    return { movieCount, seriesCount, total: mixedItems.length }
  }, [mixedItems, serverPaginated, totalCount, serverMovieCount, serverSeriesCount])

  const hasSearchOrFilters = !!searchQuery || activeFilterCount > 0

  return (
    <motion.div variants={pageVariants} initial="initial" animate="enter" className="nv-section-stack">
      <header>
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex h-9 w-9 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
            <Layers size={18} aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <h1 className="text-2xl font-bold tracking-[-0.02em] text-[var(--nv-text-primary)]">影视库</h1>
            <p className="mt-1 text-sm text-[var(--nv-text-tertiary)]">浏览和发现你的影视收藏 · {stats.total} 部作品</p>
          </div>
        </div>
      </header>

      <motion.div
        className="flex flex-wrap items-center gap-2"
        variants={staggerContainerVariants}
        initial="hidden"
        animate="visible"
        aria-label="媒体类型"
      >
        {[
          { key: '' as const, label: '全部', icon: Layers, value: stats.total },
          { key: 'movie' as const, label: '电影', icon: Film, value: stats.movieCount },
          { key: 'series' as const, label: '剧集', icon: Tv, value: stats.seriesCount },
        ].map(({ key, label, icon: Icon, value }) => {
          const selected = mediaType === key
          return (
            <motion.button
              key={key || 'all'}
              type="button"
              variants={staggerItemVariants}
              onClick={() => updateUrl({ type: selected || key === '' ? null : key })}
              aria-pressed={selected}
              className="inline-flex min-h-9 items-center gap-2 rounded-[var(--nv-radius-control)] border px-3 text-sm transition-[background-color,border-color,color] duration-200"
              style={{
                background: selected ? 'var(--nv-bg-active)' : 'var(--nv-bg-surface-soft)',
                borderColor: selected ? 'var(--nv-border-hover)' : 'var(--nv-border-default)',
                color: selected ? 'var(--nv-action-primary)' : 'var(--nv-text-secondary)',
              }}
            >
              <Icon size={14} aria-hidden="true" />
              <span>{label}</span>
              <strong className="font-semibold text-[var(--nv-text-primary)]">{value}</strong>
            </motion.button>
          )
        })}
        <motion.div variants={staggerItemVariants}>
          <SemanticTag>
            <TagIcon size={11} aria-hidden="true" />
            {serverPaginated ? '类型统计不可用' : `${allGenres.length} 个类型`}
          </SemanticTag>
        </motion.div>
      </motion.div>

      <Surface className="space-y-3 p-3 sm:p-4">
        {libraries.length > 1 && (
          <div className="flex flex-wrap items-center gap-2" aria-label="媒体库">
            <span className="mr-1 text-xs font-medium text-[var(--nv-text-tertiary)]">媒体库</span>
            <FilterChip selected={!selectedLibrary} onClick={() => updateUrl({ lib: null })}>全部库</FilterChip>
            {libraries.map((library) => (
              <FilterChip
                key={library.id}
                selected={selectedLibrary === library.id}
                onClick={() => updateUrl({ lib: library.id })}
              >
                {library.name}
              </FilterChip>
            ))}
          </div>
        )}

        <div className="flex flex-col gap-2.5 lg:flex-row lg:items-center">
          <div className="relative min-w-0 flex-1 lg:max-w-xl">
            <Search
              size={16}
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--nv-text-tertiary)]"
              aria-hidden="true"
            />
            <Input
              type="search"
              value={searchInput}
              onChange={(event) => {
                setSearchInput(event.target.value)
                updateUrl({ q: event.target.value || null })
              }}
              className="pl-9 pr-10"
              placeholder="搜索影视作品..."
              aria-label="搜索影视作品"
            />
            {searchInput && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                iconOnly
                onClick={() => {
                  setSearchInput('')
                  updateUrl({ q: null })
                }}
                className="absolute right-1 top-1/2 -translate-y-1/2"
                aria-label="清空搜索"
              >
                <X size={14} aria-hidden="true" />
              </Button>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => setShowFilters((value) => !value)}
              aria-expanded={showFilters}
              className={activeFilterCount > 0 ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : undefined}
            >
              <SlidersHorizontal size={14} aria-hidden="true" />
              筛选
              {activeFilterCount > 0 && <SemanticTag tone="brand">{activeFilterCount}</SemanticTag>}
            </Button>

            <Select
              value={sortValue}
              onChange={(event) => updateUrl({ sort: event.target.value === 'created_desc' ? null : event.target.value })}
              aria-label="排序方式"
              className="h-9 min-w-32"
            >
              {SORT_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </Select>

            <div className="flex items-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] p-0.5" role="group" aria-label="视图模式">
              <ViewButton active={viewMode === 'grid'} title="网格视图" onClick={() => updateUrl({ view: null })}>
                <Grid3X3 size={15} aria-hidden="true" />
              </ViewButton>
              <ViewButton active={viewMode === 'list'} title="列表视图" onClick={() => updateUrl({ view: 'list' })}>
                <LayoutList size={15} aria-hidden="true" />
              </ViewButton>
              <ViewButton active={viewMode === 'poster'} title="海报墙视图" onClick={() => updateUrl({ view: 'poster' })}>
                <LayoutGrid size={15} aria-hidden="true" />
              </ViewButton>
            </div>
          </div>
        </div>
      </Surface>

      <AnimatePresence initial={false}>
        {showFilters && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.2, ease: easeSmooth as unknown as [number, number, number, number] }}
            className="overflow-hidden"
          >
            <Surface className="space-y-5 p-4 sm:p-5">
              {allGenres.length > 0 && (
                <FilterGroup icon={<TagIcon size={13} />} label="类型标签" count={selectedGenres.length}>
                  {allGenres.map((genre) => (
                    <FilterChip key={genre} selected={selectedGenres.includes(genre)} onClick={() => toggleGenre(genre)}>
                      {genre}
                    </FilterChip>
                  ))}
                </FilterGroup>
              )}

              {allCountries.length > 0 && (
                <FilterGroup icon={<Globe size={13} />} label="地区">
                  <FilterChip selected={!selectedCountry} onClick={() => updateUrl({ country: null })}>全部</FilterChip>
                  {allCountries.map((country) => (
                    <FilterChip
                      key={country}
                      selected={selectedCountry === country}
                      onClick={() => updateUrl({ country: selectedCountry === country ? null : country })}
                    >
                      {country}
                    </FilterChip>
                  ))}
                </FilterGroup>
              )}

              <FilterGroup icon={<Calendar size={13} />} label="年份">
                {YEAR_RANGES.map((range) => (
                  <FilterChip
                    key={range.label}
                    selected={yearRange.min === range.min && yearRange.max === range.max}
                    onClick={() => updateUrl({
                      year_min: range.min > 0 ? String(range.min) : null,
                      year_max: range.max > 0 ? String(range.max) : null,
                    })}
                  >
                    {range.label}
                  </FilterChip>
                ))}
              </FilterGroup>

              <FilterGroup icon={<Star size={13} />} label="最低评分">
                {RATING_OPTIONS.map((option) => (
                  <FilterChip
                    key={option.value}
                    selected={minRating === option.value}
                    onClick={() => updateUrl({ rating: option.value > 0 ? String(option.value) : null })}
                  >
                    {option.label}
                  </FilterChip>
                ))}
              </FilterGroup>

              {activeFilterCount > 0 && (
                <div className="flex flex-col gap-3 border-t border-[var(--nv-border-subtle)] pt-4 sm:flex-row sm:items-center sm:justify-between">
                  <span className="text-xs text-[var(--nv-text-tertiary)]">已选择 {activeFilterCount} 个筛选条件</span>
                  <Button type="button" variant="ghost" size="sm" onClick={clearAllFilters}>
                    <X size={13} aria-hidden="true" />
                    清除所有筛选
                  </Button>
                </div>
              )}
            </Surface>
          </motion.div>
        )}
      </AnimatePresence>

      {selectedGenres.length > 0 && (
        <div className="flex flex-wrap items-center gap-2" aria-label="已选类型标签">
          <span className="text-xs text-[var(--nv-text-tertiary)]">已选标签</span>
          {selectedGenres.map((genre) => (
            <SemanticTag key={genre} tone="brand">
              {genre}
              <button type="button" onClick={() => toggleGenre(genre)} aria-label={`移除 ${genre} 标签`} className="ml-0.5 inline-flex rounded-full p-0.5 hover:bg-[var(--nv-bg-hover)]">
                <X size={10} aria-hidden="true" />
              </button>
            </SemanticTag>
          ))}
          <Button type="button" variant="ghost" size="sm" onClick={() => updateUrl({ genres: null })}>清除</Button>
        </div>
      )}

      {hasSearchOrFilters && !serverPaginated && (
        <div className="flex flex-wrap items-center gap-2 text-sm text-[var(--nv-text-secondary)]" aria-live="polite">
          <span>找到 <strong className="font-semibold text-[var(--nv-text-primary)]">{filteredItems.length}</strong> 个结果</span>
          <Button type="button" variant="ghost" size="sm" onClick={clearAllFilters}>
            <X size={13} aria-hidden="true" />
            清除筛选
          </Button>
        </div>
      )}

      {serverPaginated && (
        <Surface className="flex items-start gap-2.5 p-3 text-xs leading-5 text-[var(--nv-text-tertiary)]" role="status">
          <Info size={15} className="mt-0.5 shrink-0 text-[var(--nv-action-primary)]" aria-hidden="true" />
          <span>影视库较大（共 {totalCount} 部），暂仅支持基础分页浏览。如需使用高级筛选和排序，请先选择单个媒体库缩小范围。</span>
        </Surface>
      )}

      <AnimatePresence mode="wait">
        <motion.div
          key={`${viewMode}-${mediaType}-${sortValue}-${page}`}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          {loading ? (
            <BrowseSkeleton viewMode={viewMode} />
          ) : pagedItems.length === 0 ? (
            <EmptyState
              icon={<Film size={26} aria-hidden="true" />}
              title={hasSearchOrFilters ? '没有找到匹配的内容' : '影视库暂无内容'}
              description={hasSearchOrFilters ? '尝试调整筛选条件或使用其他关键词。' : '前往管理页面添加媒体库并扫描文件。'}
              action={hasSearchOrFilters ? (
                <Button type="button" variant="secondary" size="sm" onClick={clearAllFilters}>清除所有筛选</Button>
              ) : undefined}
            />
          ) : viewMode === 'grid' ? (
            <motion.div
              className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
              variants={staggerContainerVariants}
              initial="hidden"
              animate="visible"
            >
              {pagedItems.map((item) => {
                if (item.type === 'series' && item.series) {
                  return (
                    <motion.div key={`s-${item.series.id}`} variants={staggerItemVariants} className="min-w-0">
                      <MediaCard series={item.series} />
                    </motion.div>
                  )
                }
                if (item.media) {
                  return (
                    <motion.div key={`m-${item.media.id}`} variants={staggerItemVariants} className="min-w-0">
                      <MediaCard media={item.media} />
                    </motion.div>
                  )
                }
                return null
              })}
            </motion.div>
          ) : viewMode === 'list' ? (
            <motion.div className="space-y-2" variants={staggerContainerVariants} initial="hidden" animate="visible">
              {pagedItems.map((item) => (
                <motion.div key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`} variants={staggerItemVariants}>
                  <BrowseListItem item={item} />
                </motion.div>
              ))}
            </motion.div>
          ) : (
            <motion.div
              className="grid grid-cols-3 gap-x-2.5 gap-y-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8"
              variants={staggerContainerVariants}
              initial="hidden"
              animate="visible"
            >
              {pagedItems.map((item) => (
                <motion.div key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`} variants={staggerItemVariants}>
                  <PosterWallItem item={item} />
                </motion.div>
              ))}
            </motion.div>
          )}
        </motion.div>
      </AnimatePresence>

      <Pagination
        page={page}
        totalPages={totalPages}
        total={serverPaginated ? totalCount : filteredItems.length}
        pageSize={size}
        pageSizeOptions={[20, 30, 50, 100]}
        onPageSizeChange={setPageSize}
        onPageChange={setPage}
      />
    </motion.div>
  )
}

function BrowseSkeleton({ viewMode }: { viewMode: ViewMode }) {
  const list = viewMode === 'list'
  return (
    <div className={clsx(
      viewMode === 'poster'
        ? 'grid grid-cols-3 gap-x-2.5 gap-y-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8'
        : list
          ? 'space-y-2'
          : 'grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6'
    )}>
      {Array.from({ length: list ? 8 : 12 }).map((_, index) => (
        list ? (
          <Surface key={index} className="flex items-center gap-4 p-3">
            <div className="skeleton h-16 w-12 shrink-0 rounded-[var(--nv-radius-control)]" />
            <div className="flex-1 space-y-2">
              <div className="skeleton h-4 w-3/4 rounded" />
              <div className="skeleton h-3 w-1/2 rounded" />
            </div>
          </Surface>
        ) : (
          <div key={index}>
            <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
            <div className="skeleton mt-2 h-4 w-3/4 rounded" />
            <div className="skeleton mt-1 h-3 w-1/2 rounded" />
          </div>
        )
      ))}
    </div>
  )
}

function BrowseListItem({ item }: { item: MixedItem }) {
  const [tagsExpanded, setTagsExpanded] = useState(false)
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
  const maxVisibleTags = 3
  const hasMoreTags = genreList.length > maxVisibleTags
  const visibleTags = tagsExpanded ? genreList : genreList.slice(0, maxVisibleTags)

  const linkTo = series
    ? `/series/${series.id}`
    : media?.series_id
      ? `/series/${media.series_id}`
      : `/media/${media?.id}`

  const posterUrl = series
    ? streamApi.getSeriesPosterUrl(series.id)
    : media?.series_id
      ? streamApi.getSeriesPosterUrl(media.series_id)
      : streamApi.getPosterUrl(media?.id || '')

  const formatDuration = (seconds: number) => {
    if (!seconds) return ''
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`
  }

  return (
    <Link
      to={linkTo}
      className="group flex items-center gap-4 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] p-3 transition-[background-color,border-color,box-shadow] duration-200 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)] hover:shadow-[var(--nv-shadow-card)]"
    >
      <div className="h-20 w-14 shrink-0 overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]">
        <img
          src={posterUrl}
          alt={title}
          className="h-full w-full object-cover"
          loading="lazy"
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />
      </div>

      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <h3 className="truncate text-sm font-semibold text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]">{title}</h3>
          {isSeries && <SemanticTag>剧集</SemanticTag>}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-[var(--nv-text-tertiary)]">
          {year > 0 && <span>{year}</span>}
          {country && <span>{country}</span>}
          {duration > 0 && <span>{formatDuration(duration)}</span>}
          {isSeries && series && <span>{series.season_count} 季 · {series.episode_count} 集</span>}
        </div>

        {genreList.length > 0 && (
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
            {visibleTags.map((genre) => <SemanticTag key={genre}>{genre}</SemanticTag>)}
            {hasMoreTags && (
              <button
                type="button"
                onClick={(event) => {
                  event.preventDefault()
                  event.stopPropagation()
                  setTagsExpanded((value) => !value)
                }}
                className="inline-flex items-center gap-0.5 rounded-[var(--nv-radius-pill)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] px-2 py-0.5 text-[10px] font-medium text-[var(--nv-action-primary)] transition-colors hover:bg-[var(--nv-bg-hover)]"
                title={tagsExpanded ? '收起标签' : `展开全部 ${genreList.length} 个标签`}
              >
                {tagsExpanded ? '收起' : `+${genreList.length - maxVisibleTags}`}
              </button>
            )}
          </div>
        )}

        {overview && <p className="mt-1.5 line-clamp-1 text-xs text-[var(--nv-text-tertiary)]">{overview}</p>}
      </div>

      {rating > 0 && (
        <SemanticTag tone="rating" className="shrink-0">
          <Star size={11} fill="currentColor" aria-hidden="true" />
          {rating.toFixed(1)}
        </SemanticTag>
      )}
    </Link>
  )
}

function PosterWallItem({ item }: { item: MixedItem }) {
  const isSeries = item.type === 'series'
  const media = isSeries ? undefined : item.media
  const series = isSeries ? item.series : undefined
  const title = series?.title || media?.title || ''
  const rating = series?.rating || media?.rating || 0

  const linkTo = series
    ? `/series/${series.id}`
    : media?.series_id
      ? `/series/${media.series_id}`
      : `/media/${media?.id}`

  const posterUrl = series
    ? streamApi.getSeriesPosterUrl(series.id)
    : media?.series_id
      ? streamApi.getSeriesPosterUrl(media.series_id)
      : streamApi.getPosterUrl(media?.id || '')

  return (
    <Link
      to={linkTo}
      className="group block overflow-hidden rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] transition-[border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:shadow-[var(--nv-shadow-card)]"
      aria-label={title}
    >
      <div className="relative aspect-[2/3] overflow-hidden bg-[var(--nv-bg-surface-soft)]">
        <img
          src={posterUrl}
          alt={title}
          className="h-full w-full object-cover transition-transform duration-300 ease-out group-hover:scale-[1.025]"
          loading="lazy"
          onError={(event) => { event.currentTarget.style.display = 'none' }}
        />

        <div className="pointer-events-none absolute inset-0 flex items-center justify-center text-[var(--nv-text-tertiary)] -z-10">
          {isSeries ? <Tv size={20} aria-hidden="true" /> : <Film size={20} aria-hidden="true" />}
        </div>

        <div className="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-black/80 via-black/25 to-transparent opacity-0 transition-opacity duration-200 group-hover:opacity-100" />
        <div className="absolute inset-x-2 bottom-2 translate-y-1 opacity-0 transition-[opacity,transform] duration-200 group-hover:translate-y-0 group-hover:opacity-100">
          <p className="truncate text-xs font-semibold text-white">{title}</p>
        </div>
        <div className="absolute right-2 top-2 flex h-7 w-7 scale-95 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-action)] opacity-0 shadow-[var(--nv-shadow-card)] transition-[opacity,transform] duration-200 group-hover:scale-100 group-hover:opacity-100">
          <Play size={12} className="ml-0.5" fill="currentColor" aria-hidden="true" />
        </div>

        {rating > 0 && (
          <SemanticTag tone="rating" className="absolute left-1.5 top-1.5 bg-black/65 text-white backdrop-blur-sm">
            <Star size={9} fill="currentColor" className="text-[var(--nv-status-rating)]" aria-hidden="true" />
            {rating.toFixed(1)}
          </SemanticTag>
        )}
        {isSeries && <SemanticTag className="absolute bottom-1.5 right-1.5 bg-black/60 text-white backdrop-blur-sm">剧集</SemanticTag>}
      </div>
    </Link>
  )
}
