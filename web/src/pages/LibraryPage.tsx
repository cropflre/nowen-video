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
  Search,
  SlidersHorizontal,
  Star,
  Tag as TagIcon,
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
} from '@/components/design-system'
import { AnimatePresence, motion } from 'framer-motion'
import {
  easeSmooth,
  staggerContainerVariants,
  staggerItemVariants,
} from '@/lib/motion'
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

type LibraryViewMode = 'grid' | 'list' | 'poster'
type LibraryViewTab = 'all' | 'series'

const MAX_CLIENT_ITEMS = 2000

function parseViewMode(value: string | null): LibraryViewMode {
  return value === 'list' || value === 'poster' ? value : 'grid'
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
        {!!count && <Tag tone="brand">{count}</Tag>}
      </div>
      <div className="flex flex-wrap gap-2">{children}</div>
    </div>
  )
}

function ContentTabButton({
  active,
  icon,
  label,
  count,
  onClick,
}: {
  active: boolean
  icon: ReactNode
  label: string
  count: number
  onClick: () => void
}) {
  return (
    <motion.button
      type="button"
      variants={staggerItemVariants}
      onClick={onClick}
      aria-pressed={active}
      className="inline-flex min-h-9 items-center gap-2 rounded-[var(--nv-radius-control)] border px-3 text-sm transition-[background-color,border-color,color] duration-200"
      style={{
        background: active ? 'var(--nv-bg-active)' : 'var(--nv-bg-surface-soft)',
        borderColor: active ? 'var(--nv-border-hover)' : 'var(--nv-border-default)',
        color: active ? 'var(--nv-action-primary)' : 'var(--nv-text-secondary)',
      }}
    >
      {icon}
      <span>{label}</span>
      <strong className="font-semibold text-[var(--nv-text-primary)]">{count}</strong>
    </motion.button>
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

  const setPage = useCallback((newPage: number) => {
    updateUrl({ page: newPage <= 1 ? null : String(newPage) })
  }, [updateUrl])

  const setSize = useCallback((newSize: number) => {
    updateUrl({ limit: newSize === 30 ? null : String(newSize), page: null })
  }, [updateUrl])

  const setViewTab = useCallback((nextTab: LibraryViewTab) => {
    updateUrl({ tab: nextTab === 'series' ? 'series' : null, page: null })
  }, [updateUrl])

  const setViewMode = useCallback((nextMode: LibraryViewMode) => {
    updateUrl({ view: nextMode === 'grid' ? null : nextMode })
  }, [updateUrl])

  const clearAllFilters = useCallback(() => {
    updateUrl({
      q: null,
      genres: null,
      genre: null,
      country: null,
      year_min: null,
      year_max: null,
      rating: null,
    })
  }, [updateUrl])

  const toggleGenre = useCallback((genre: string) => {
    const next = selectedGenres.includes(genre)
      ? selectedGenres.filter((value) => value !== genre)
      : [...selectedGenres, genre]
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
          mediaApi.listMixed({
            page: shouldPaginateOnServer ? page : 1,
            size: shouldPaginateOnServer ? size : Math.max(totalCount, 1),
            library_id: id,
          }),
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
      if (genreText) {
        genreText.split(',').forEach((genre) => {
          const trimmed = genre.trim()
          if (trimmed) genres.add(trimmed)
        })
      }
      if (countryText) {
        countryText.split(',').forEach((country) => {
          const trimmed = country.trim()
          if (trimmed) countries.add(trimmed)
        })
      }
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
      items = items.filter((item) =>
        getItemTitle(item).toLowerCase().includes(query)
        || getItemOrigTitle(item).toLowerCase().includes(query)
        || getItemOverview(item).toLowerCase().includes(query)
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
        const score = (candidate: Series) =>
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

    if (selectedGenres.length > 0) {
      items = items.filter((series) => {
        const genres = series.genres || ''
        return selectedGenres.every((genre) => genres.includes(genre))
      })
    }

    if (selectedCountry) {
      items = items.filter((series) => (series.country || '').includes(selectedCountry))
    }

    if (yearRange.min > 0 || yearRange.max > 0) {
      items = items.filter((series) => {
        const year = series.year || 0
        if (year === 0) return false
        if (yearRange.min > 0 && year < yearRange.min) return false
        if (yearRange.max > 0 && year > yearRange.max) return false
        return true
      })
    }

    if (minRating > 0) {
      items = items.filter((series) => (series.rating || 0) >= minRating)
    }

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

  const pagedMixed = useMemo(() => {
    if (serverPaginated) return filteredMixed
    const start = (page - 1) * size
    return filteredMixed.slice(start, start + size)
  }, [serverPaginated, filteredMixed, page, size])

  const pagedSeries = useMemo(() => {
    const start = (page - 1) * size
    return filteredSeries.slice(start, start + size)
  }, [filteredSeries, page, size])

  const activeFilterCount = [
    selectedGenres.length > 0,
    selectedCountry !== '',
    yearRange.min > 0 || yearRange.max > 0,
    minRating > 0,
  ].filter(Boolean).length

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
      <motion.div
        className="flex flex-wrap items-center gap-2"
        variants={staggerContainerVariants}
        initial="hidden"
        animate="visible"
        role="group"
        aria-label="媒体库内容类型"
      >
        <ContentTabButton
          active={viewTab === 'all'}
          onClick={() => setViewTab('all')}
          icon={<Film size={14} aria-hidden="true" />}
          label="全部内容"
          count={total}
        />
        {hasSeries && (
          <ContentTabButton
            active={viewTab === 'series'}
            onClick={() => setViewTab('series')}
            icon={<Tv size={14} aria-hidden="true" />}
            label="剧集合集"
            count={deduplicatedSeries.length}
          />
        )}
      </motion.div>

      <Surface className="space-y-3 p-3 sm:p-4">
        <div className="flex flex-col gap-2.5 lg:flex-row lg:items-center">
          <div className="relative min-w-0 flex-1 lg:max-w-xl">
            <Search
              size={16}
              className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--nv-text-tertiary)]"
              aria-hidden="true"
            />
            <Input
              type="search"
              value={searchQuery}
              onChange={(event) => updateUrl({ q: event.target.value || null })}
              className="pl-9 pr-10"
              placeholder="搜索此媒体库..."
              aria-label="搜索此媒体库"
            />
            {searchQuery && (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                iconOnly
                onClick={() => updateUrl({ q: null })}
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
              {activeFilterCount > 0 && <Tag tone="brand">{activeFilterCount}</Tag>}
            </Button>

            <Select
              value={sortValue}
              onChange={(event) => updateUrl({ sort: event.target.value === 'created_desc' ? null : event.target.value })}
              aria-label="媒体库排序"
              className="h-9 min-w-32"
            >
              {SORT_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </Select>

            <div
              className="flex items-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] p-0.5"
              role="group"
              aria-label="媒体库视图"
            >
              <ViewButton active={viewMode === 'grid'} title="网格视图" onClick={() => setViewMode('grid')}>
                <Grid3X3 size={15} aria-hidden="true" />
              </ViewButton>
              <ViewButton active={viewMode === 'list'} title="列表视图" onClick={() => setViewMode('list')}>
                <LayoutList size={15} aria-hidden="true" />
              </ViewButton>
              <ViewButton active={viewMode === 'poster'} title="海报墙视图" onClick={() => setViewMode('poster')}>
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
            <Tag key={genre} tone="brand">
              {genre}
              <button
                type="button"
                onClick={() => toggleGenre(genre)}
                aria-label={`移除 ${genre} 标签`}
                className="ml-0.5 inline-flex rounded-full p-0.5 hover:bg-[var(--nv-bg-hover)]"
              >
                <X size={10} aria-hidden="true" />
              </button>
            </Tag>
          ))}
          <Button type="button" variant="ghost" size="sm" onClick={() => updateUrl({ genres: null })}>清除</Button>
        </div>
      )}

      {hasLocalFilter && (
        <div className="flex flex-wrap items-center gap-2 text-sm text-[var(--nv-text-secondary)]" aria-live="polite">
          <span>找到 <strong className="font-semibold text-[var(--nv-text-primary)]">{resultCount}</strong> 个结果</span>
          <Button type="button" variant="ghost" size="sm" onClick={clearAllFilters}>
            <X size={13} aria-hidden="true" />
            清除筛选
          </Button>
        </div>
      )}

      <AnimatePresence mode="wait">
        <motion.div
          key={`${viewTab}-${viewMode}-${sortValue}-${page}`}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          {loading ? (
            <LibrarySkeleton viewMode={viewMode} />
          ) : viewTab === 'all' ? (
            pagedMixed.length === 0 ? (
              <EmptyState
                icon={<Film size={26} aria-hidden="true" />}
                title={hasLocalFilter ? '没有找到匹配的内容' : '此媒体库暂无内容'}
                description={hasLocalFilter ? '尝试调整筛选条件或使用其他关键词。' : '扫描媒体库后，内容会显示在这里。'}
                action={hasLocalFilter ? (
                  <Button type="button" variant="secondary" size="sm" onClick={clearAllFilters}>
                    清除所有筛选
                  </Button>
                ) : undefined}
              />
            ) : viewMode === 'grid' ? (
              <motion.div
                className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
                variants={staggerContainerVariants}
                initial="hidden"
                animate="visible"
              >
                {pagedMixed.map((item) => (
                  <motion.div
                    key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`}
                    variants={staggerItemVariants}
                    className="min-w-0"
                  >
                    {item.type === 'series' && item.series
                      ? <MediaCard series={item.series} />
                      : item.media
                        ? <MediaCard media={item.media} />
                        : null}
                  </motion.div>
                ))}
              </motion.div>
            ) : viewMode === 'list' ? (
              <motion.div className="space-y-2" variants={staggerContainerVariants} initial="hidden" animate="visible">
                {pagedMixed.map((item) => (
                  <motion.div
                    key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`}
                    variants={staggerItemVariants}
                  >
                    <LibraryListItem item={item} />
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
                {pagedMixed.map((item) => (
                  <motion.div
                    key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`}
                    variants={staggerItemVariants}
                  >
                    <LibraryPosterItem item={item} />
                  </motion.div>
                ))}
              </motion.div>
            )
          ) : pagedSeries.length === 0 ? (
            <EmptyState
              icon={<Tv size={26} aria-hidden="true" />}
              title={hasLocalFilter ? '没有找到匹配的剧集' : '此媒体库暂无剧集合集'}
              description={hasLocalFilter ? '尝试调整筛选条件或使用其他关键词。' : '识别到剧集后，合集会显示在这里。'}
              action={hasLocalFilter ? (
                <Button type="button" variant="secondary" size="sm" onClick={clearAllFilters}>
                  清除所有筛选
                </Button>
              ) : undefined}
            />
          ) : viewMode === 'grid' ? (
            <motion.div
              className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
              variants={staggerContainerVariants}
              initial="hidden"
              animate="visible"
            >
              {pagedSeries.map((series) => (
                <motion.div key={series.id} variants={staggerItemVariants} className="min-w-0">
                  <MediaCard series={series} />
                </motion.div>
              ))}
            </motion.div>
          ) : viewMode === 'list' ? (
            <motion.div className="space-y-2" variants={staggerContainerVariants} initial="hidden" animate="visible">
              {pagedSeries.map((series) => (
                <motion.div key={series.id} variants={staggerItemVariants}>
                  <LibraryListItem series={series} />
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
              {pagedSeries.map((series) => (
                <motion.div key={series.id} variants={staggerItemVariants}>
                  <LibraryPosterItem series={series} />
                </motion.div>
              ))}
            </motion.div>
          )}
        </motion.div>
      </AnimatePresence>

      <Pagination
        page={page}
        totalPages={totalPages}
        total={resultCount}
        pageSize={size}
        pageSizeOptions={[20, 30, 50, 100]}
        onPageChange={setPage}
        onPageSizeChange={setSize}
      />
    </div>
  )
}

function LibrarySkeleton({ viewMode }: { viewMode: LibraryViewMode }) {
  const list = viewMode === 'list'
  return (
    <div className={clsx(
      viewMode === 'poster'
        ? 'grid grid-cols-3 gap-x-2.5 gap-y-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8'
        : list
          ? 'space-y-2'
          : 'grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6',
    )}>
      {Array.from({ length: list ? 8 : 12 }).map((_, index) => (
        list ? (
          <Surface key={index} className="flex items-center gap-4 p-3">
            <div className="skeleton h-20 w-14 shrink-0 rounded-[var(--nv-radius-control)]" />
            <div className="flex-1 space-y-2">
              <div className="skeleton h-4 w-3/4 rounded" />
              <div className="skeleton h-3 w-1/2 rounded" />
            </div>
          </Surface>
        ) : (
          <div key={index}>
            <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
            {viewMode !== 'poster' && (
              <>
                <div className="skeleton mt-2 h-4 w-3/4 rounded" />
                <div className="skeleton mt-1 h-3 w-1/2 rounded" />
              </>
            )}
          </div>
        )
      ))}
    </div>
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
          {isSeries && <Tag>剧集</Tag>}
        </div>
        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-[var(--nv-text-tertiary)]">
          {year > 0 && <span>{year}</span>}
          {country && <span>{country}</span>}
          {duration > 0 && <span>{formatDuration(duration)}</span>}
          {series && <span>{series.season_count} 季 · {series.episode_count} 集</span>}
        </div>

        {genreList.length > 0 && (
          <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
            {visibleTags.map((genre) => <Tag key={genre}>{genre}</Tag>)}
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
        <Tag tone="rating" className="shrink-0">
          <Star size={11} fill="currentColor" aria-hidden="true" />
          {rating.toFixed(1)}
        </Tag>
      )}
    </Link>
  )
}

function LibraryPosterItem({ item, series: seriesProp }: { item?: MixedItem; series?: Series }) {
  const isSeries = Boolean(seriesProp) || item?.type === 'series'
  const series = seriesProp || (item?.type === 'series' ? item.series : undefined)
  const media: Media | undefined = isSeries ? undefined : item?.media
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

        <div className="pointer-events-none absolute inset-0 -z-10 flex items-center justify-center text-[var(--nv-text-tertiary)]">
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
          <Tag tone="rating" className="absolute left-1.5 top-1.5">
            <Star size={10} fill="currentColor" aria-hidden="true" />
            {rating.toFixed(1)}
          </Tag>
        )}
      </div>
    </Link>
  )
}
