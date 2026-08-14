import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { aiApi, mediaApi, personApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { MixedItem, Person, SearchIntent } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import Pagination from '@/components/Pagination'
import { Button, EmptyState, Surface, Tag } from '@/components/design-system'
import {
  ArrowUpDown,
  Calendar,
  Film,
  Search as SearchIcon,
  SlidersHorizontal,
  Sparkles,
  Star,
  User,
  X,
} from 'lucide-react'
import { t as translate, useTranslation } from '@/i18n'

const SORT_OPTIONS = [
  { value: 'relevance', labelKey: 'search.sortRelevance' },
  { value: 'rating_desc', labelKey: 'search.sortRatingDesc' },
  { value: 'year_desc', labelKey: 'search.sortYearDesc' },
  { value: 'year_asc', labelKey: 'search.sortYearAsc' },
  { value: 'title_asc', labelKey: 'search.sortTitleAsc' },
] as const

const YEAR_RANGES = [
  { labelKey: 'search.yearAll', min: 0, max: 0 },
  { labelKey: '', min: 2024, max: 2026 },
  { labelKey: '', min: 2020, max: 2023 },
  { labelKey: '', min: 2010, max: 2019 },
  { labelKey: '', min: 2000, max: 2009 },
  { labelKey: 'search.yearEarlier', min: 0, max: 1999 },
]

const MIXED_BULK_SIZE = 2000
const MIXED_FETCH_CONCURRENCY = 3

type SearchType = '' | 'movie' | 'series'
type SearchSort = typeof SORT_OPTIONS[number]['value']

type EffectiveSearch = {
  query: string
  type?: 'movie' | 'series'
  genre?: string
  yearMin?: number
  yearMax?: number
  minRating: number
  sort: SearchSort
}

function FilterChip({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="nv-button nv-search-filter-chip"
      data-variant={selected ? 'secondary' : 'ghost'}
      data-size="sm"
      aria-pressed={selected}
    >
      {children}
    </button>
  )
}

function FilterRow({ icon, label, children }: { icon: ReactNode; label: string; children: ReactNode }) {
  return (
    <div className="nv-search-filter-row flex flex-wrap items-center gap-1.5">
      <span className="mr-1 inline-flex min-w-20 items-center gap-1.5 text-xs font-medium text-[var(--nv-text-tertiary)]">
        {icon}
        {label}
      </span>
      {children}
    </div>
  )
}

function normalizeIntentType(value?: string): 'movie' | 'series' | undefined {
  const normalized = (value || '').trim().toLowerCase()
  if (normalized === 'movie') return 'movie'
  if (normalized === 'episode' || normalized === 'series' || normalized === 'tv' || normalized === 'tvshow') return 'series'
  return undefined
}

function normalizeIntentSort(value?: string): SearchSort {
  return SORT_OPTIONS.some((option) => option.value === value) ? value as SearchSort : 'relevance'
}

function parseSearchSort(value: SearchSort) {
  switch (value) {
    case 'rating_desc':
      return { sort: 'rating' as const, order: 'desc' as const }
    case 'year_desc':
      return { sort: 'year' as const, order: 'desc' as const }
    case 'year_asc':
      return { sort: 'year' as const, order: 'asc' as const }
    case 'title_asc':
      return { sort: 'title' as const, order: 'asc' as const }
    default:
      // /media/mixed already filters the query before pagination. Its stable
      // default order is used for "relevance" until the API exposes a dedicated score.
      return { sort: 'added' as const, order: 'desc' as const }
  }
}

function getMixedRating(item: MixedItem) {
  if (item.type === 'series') return item.series?.rating || 0
  return item.media?.rating || 0
}

async function fetchMixedSearchPage(criteria: EffectiveSearch, page: number, size: number) {
  const sort = parseSearchSort(criteria.sort)
  const baseParams = {
    q: criteria.query,
    type: criteria.type,
    genre: criteria.genre,
    year_from: criteria.yearMin,
    year_to: criteria.yearMax,
    sort: sort.sort,
    order: sort.order,
  }

  if (criteria.minRating <= 0) {
    const response = await mediaApi.listMixed({ ...baseParams, page, size })
    return {
      items: response.data.data || [],
      total: response.data.total || 0,
    }
  }

  // The current mixed-list contract does not expose min_rating. Keep the
  // existing filter accurate without changing the backend contract: collect
  // the already-filtered mixed result set in 2000-item pages, apply rating,
  // then paginate locally. This path only runs when a rating filter is active.
  const first = await mediaApi.listMixed({ ...baseParams, page: 1, size: MIXED_BULK_SIZE })
  const allItems = [...(first.data.data || [])]
  const unfilteredTotal = first.data.total || allItems.length
  const totalPages = Math.max(1, Math.ceil(unfilteredTotal / MIXED_BULK_SIZE))

  if (totalPages > 1) {
    const remainingPages = Array.from({ length: totalPages - 1 }, (_, index) => index + 2)
    for (let index = 0; index < remainingPages.length; index += MIXED_FETCH_CONCURRENCY) {
      const batch = remainingPages.slice(index, index + MIXED_FETCH_CONCURRENCY)
      const responses = await Promise.all(batch.map((nextPage) => mediaApi.listMixed({
        ...baseParams,
        page: nextPage,
        size: MIXED_BULK_SIZE,
      })))
      for (const response of responses) allItems.push(...(response.data.data || []))
    }
  }

  const filtered = allItems.filter((item) => getMixedRating(item) >= criteria.minRating)
  const start = Math.max(0, (page - 1) * size)
  return {
    items: filtered.slice(start, start + size),
    total: filtered.length,
  }
}

export default function SearchPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const toast = useToast()
  const toastRef = useRef(toast)
  const { t } = useTranslation()

  useEffect(() => { toastRef.current = toast }, [toast])

  const query = (searchParams.get('q') || '').trim()
  const page = Math.max(1, parseInt(searchParams.get('page') || '1', 10) || 1)
  const rawSize = parseInt(searchParams.get('size') || '30', 10) || 30
  const size = rawSize > 0 && rawSize <= 100 ? rawSize : 30
  const typeParam = searchParams.get('type')
  const filterType: SearchType = typeParam === 'movie' || typeParam === 'series' ? typeParam : ''
  const sortParam = searchParams.get('sort')
  const sortBy: SearchSort = SORT_OPTIONS.some((option) => option.value === sortParam) ? sortParam as SearchSort : 'relevance'
  const yearRange = {
    min: Math.max(0, parseInt(searchParams.get('year_min') || '0', 10) || 0),
    max: Math.max(0, parseInt(searchParams.get('year_max') || '0', 10) || 0),
  }
  const parsedRating = Number(searchParams.get('rating') || 0)
  const minRating = Number.isFinite(parsedRating) && parsedRating >= 0 && parsedRating <= 10 ? parsedRating : 0

  const [results, setResults] = useState<MixedItem[]>([])
  const [people, setPeople] = useState<Person[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [showFilters, setShowFilters] = useState(false)
  const [aiState, setAiState] = useState<{ sourceQuery: string; intent: SearchIntent } | null>(null)
  const [aiLoadingQuery, setAiLoadingQuery] = useState<string | null>(null)
  const aiRequestRef = useRef(0)
  const searchRequestRef = useRef(0)

  const updateUrl = useCallback((changes: Record<string, string | null>, resetPage = true) => {
    setSearchParams((currentParams) => {
      const params = new URLSearchParams(currentParams)
      for (const [key, value] of Object.entries(changes)) {
        if (value === null || value === '') params.delete(key)
        else params.set(key, value)
      }
      if (resetPage) params.delete('page')
      return params
    }, { replace: true })
  }, [setSearchParams])

  const setPage = useCallback((newPage: number) => {
    setSearchParams((currentParams) => {
      const params = new URLSearchParams(currentParams)
      if (newPage <= 1) params.delete('page')
      else params.set('page', String(newPage))
      return params
    }, { replace: true })
  }, [setSearchParams])

  const setSize = useCallback((newSize: number) => {
    setSearchParams((currentParams) => {
      const params = new URLSearchParams(currentParams)
      if (newSize === 30) params.delete('size')
      else params.set('size', String(newSize))
      params.delete('page')
      return params
    }, { replace: true })
  }, [setSearchParams])

  const hasActiveFilters = filterType !== '' || sortBy !== 'relevance' || yearRange.min > 0 || yearRange.max > 0 || minRating > 0
  const validAiIntent = aiState?.sourceQuery === query && aiState.intent.parsed ? aiState.intent : null
  const aiLoading = aiLoadingQuery === query

  useEffect(() => {
    const requestId = ++aiRequestRef.current
    setAiState(null)

    if (!query || Array.from(query).length <= 4) {
      setAiLoadingQuery(null)
      return
    }

    setAiLoadingQuery(query)
    aiApi.smartSearch(query)
      .then((response) => {
        if (aiRequestRef.current !== requestId) return
        const intent = response.data.data
        setAiState(intent.parsed ? { sourceQuery: query, intent } : null)
      })
      .catch(() => {
        if (aiRequestRef.current === requestId) setAiState(null)
      })
      .finally(() => {
        if (aiRequestRef.current === requestId) setAiLoadingQuery(null)
      })

    return () => {
      if (aiRequestRef.current === requestId) aiRequestRef.current += 1
    }
  }, [query])

  const effectiveSearch = useMemo<EffectiveSearch>(() => {
    const useAiStructure = !!validAiIntent && !hasActiveFilters
    const aiQuery = validAiIntent?.query?.trim()
    return {
      query: aiQuery || query,
      type: filterType || (useAiStructure ? normalizeIntentType(validAiIntent?.media_type) : undefined),
      genre: useAiStructure ? validAiIntent?.genre?.trim() || undefined : undefined,
      yearMin: yearRange.min || (useAiStructure && validAiIntent?.year_min ? validAiIntent.year_min : undefined),
      yearMax: yearRange.max || (useAiStructure && validAiIntent?.year_max ? validAiIntent.year_max : undefined),
      minRating: minRating || (useAiStructure && validAiIntent?.min_rating ? validAiIntent.min_rating : 0),
      sort: sortBy !== 'relevance' ? sortBy : (useAiStructure ? normalizeIntentSort(validAiIntent?.sort_by) : 'relevance'),
    }
  }, [filterType, hasActiveFilters, minRating, query, sortBy, validAiIntent, yearRange.max, yearRange.min])

  useEffect(() => {
    const requestId = ++searchRequestRef.current
    if (!query || !effectiveSearch.query) {
      setResults([])
      setPeople([])
      setTotal(0)
      setLoading(false)
      return
    }

    setLoading(true)
    setResults([])
    setPeople([])
    setTotal(0)

    const peoplePromise = page === 1
      ? personApi.search(effectiveSearch.query, 10).catch(() => null)
      : Promise.resolve(null)

    Promise.all([
      fetchMixedSearchPage(effectiveSearch, page, size),
      peoplePromise,
    ])
      .then(([mediaResult, peopleResponse]) => {
        if (searchRequestRef.current !== requestId) return
        setResults(mediaResult.items)
        setTotal(mediaResult.total)
        setPeople(peopleResponse?.data.data || [])
      })
      .catch(() => {
        if (searchRequestRef.current !== requestId) return
        setResults([])
        setPeople([])
        setTotal(0)
        toastRef.current.error(translate('search.searchFailed'))
      })
      .finally(() => {
        if (searchRequestRef.current === requestId) setLoading(false)
      })

    return () => {
      if (searchRequestRef.current === requestId) searchRequestRef.current += 1
    }
  }, [effectiveSearch, page, query, size])

  const totalPages = Math.max(1, Math.ceil(total / size))
  useEffect(() => {
    if (!loading && total > 0 && page > totalPages) setPage(totalPages)
  }, [loading, page, setPage, total, totalPages])

  const clearFilters = () => {
    updateUrl({ type: null, sort: null, year_min: null, year_max: null, rating: null })
  }

  const searched = query.length > 0
  const hasAnyResults = total > 0 || people.length > 0
  const resultSummary = loading
    ? '正在搜索…'
    : searched
      ? `“${query}” · ${total} 个影视结果${people.length > 0 && page === 1 ? ` · ${people.length} 位人物` : ''}`
      : '搜索标题、原名、剧集或演职人员，并使用筛选快速缩小范围。'

  return (
    <div className="nv-section-stack nv-library-page nv-search-page">
      <header className="nv-page-hero-header nv-search-page-toolbar">
        <div className="nv-page-title-lockup">
          <div className="nv-page-title-icon" aria-hidden="true">
            <SearchIcon size={20} />
          </div>
          <div className="min-w-0">
            <h1 className="nv-page-title">搜索</h1>
            <p className="nv-page-subtitle" aria-live="polite">{resultSummary}</p>
          </div>
        </div>
        <Button
          variant={showFilters || hasActiveFilters ? 'secondary' : 'ghost'}
          size="sm"
          onClick={() => setShowFilters((value) => !value)}
          aria-expanded={showFilters}
        >
          <SlidersHorizontal size={14} aria-hidden="true" />
          {t('search.filterAndSort')}
          {hasActiveFilters && <Tag tone="brand">已筛选</Tag>}
        </Button>
      </header>

      {showFilters && (
        <Surface className="nv-search-filter-panel space-y-3 p-3 sm:p-4">
          <FilterRow icon={<Film size={13} aria-hidden="true" />} label={`${t('search.type')}:`}>
            {[
              { value: '' as SearchType, label: t('search.typeAll') },
              { value: 'movie' as SearchType, label: t('search.typeMovie') },
              { value: 'series' as SearchType, label: t('search.typeEpisode') },
            ].map((option) => (
              <FilterChip key={option.value} selected={filterType === option.value} onClick={() => updateUrl({ type: option.value || null })}>
                {option.label}
              </FilterChip>
            ))}
          </FilterRow>

          <FilterRow icon={<Calendar size={13} aria-hidden="true" />} label={`${t('search.year')}:`}>
            {YEAR_RANGES.map((range) => (
              <FilterChip
                key={range.labelKey || `${range.min}-${range.max}`}
                selected={yearRange.min === range.min && yearRange.max === range.max}
                onClick={() => updateUrl({
                  year_min: range.min > 0 ? String(range.min) : null,
                  year_max: range.max > 0 ? String(range.max) : null,
                })}
              >
                {range.labelKey ? t(range.labelKey) : `${range.min}-${range.max}`}
              </FilterChip>
            ))}
          </FilterRow>

          <FilterRow icon={<Star size={13} aria-hidden="true" />} label={`${t('search.minRating')}:`}>
            {[0, 6, 7, 8, 9].map((rating) => (
              <FilterChip key={rating} selected={minRating === rating} onClick={() => updateUrl({ rating: rating > 0 ? String(rating) : null })}>
                {rating === 0 ? t('search.ratingAll') : `≥${rating}分`}
              </FilterChip>
            ))}
          </FilterRow>

          <FilterRow icon={<ArrowUpDown size={13} aria-hidden="true" />} label={`${t('search.sort')}:`}>
            {SORT_OPTIONS.map((option) => (
              <FilterChip key={option.value} selected={sortBy === option.value} onClick={() => updateUrl({ sort: option.value === 'relevance' ? null : option.value })}>
                {t(option.labelKey)}
              </FilterChip>
            ))}
          </FilterRow>

          {hasActiveFilters && (
            <Button variant="ghost" size="sm" onClick={clearFilters}>
              <X size={14} aria-hidden="true" />
              {t('search.clearFilters')}
            </Button>
          )}
        </Surface>
      )}

      {(validAiIntent || aiLoading) && (
        <div className="nv-search-ai-row flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
          {validAiIntent && (
            <Tag tone="brand">
              <Sparkles size={11} aria-hidden="true" />
              {t('search.aiUnderstand')}: “{validAiIntent.query}”
              {validAiIntent.genre && ` · ${validAiIntent.genre}`}
              {validAiIntent.min_rating ? ` · ≥${validAiIntent.min_rating}分` : ''}
            </Tag>
          )}
          {aiLoading && <Tag><Sparkles size={11} aria-hidden="true" />{t('search.aiAnalyzing')}</Tag>}
          {validAiIntent && hasActiveFilters && <Tag>手动筛选优先</Tag>}
        </div>
      )}

      {people.length > 0 && page === 1 && !loading && (
        <section className="nv-search-person-section" aria-labelledby="search-person-title">
          <div className="mb-3 flex items-baseline gap-2">
            <h2 id="search-person-title" className="nv-section-title">人物</h2>
            <span className="text-[11px] text-[var(--nv-text-tertiary)]">{people.length}</span>
          </div>
          <div className="scrollbar-hide flex gap-3 overflow-x-auto pb-2" role="list" aria-label="人物搜索结果">
            {people.map((person) => <PersonSearchCard key={person.id} person={person} />)}
          </div>
        </section>
      )}

      <MediaGrid mixedItems={results} loading={loading} />

      {searched && !loading && !aiLoading && !hasAnyResults && (
        <EmptyState
          icon={<SearchIcon size={22} aria-hidden="true" />}
          title={t('search.noMatch')}
          description={hasActiveFilters ? t('search.noMatchHintFiltered') : t('search.noMatchHint')}
          action={hasActiveFilters ? <Button variant="secondary" size="sm" onClick={clearFilters}>{t('search.clearFilterConditions')}</Button> : undefined}
        />
      )}

      {total > 0 && !loading && (
        <Pagination
          page={page}
          totalPages={totalPages}
          total={total}
          pageSize={size}
          pageSizeOptions={[20, 30, 50, 100]}
          onPageChange={setPage}
          onPageSizeChange={setSize}
        />
      )}
    </div>
  )
}

function PersonSearchCard({ person }: { person: Person }) {
  const [imageFailed, setImageFailed] = useState(false)
  const profileUrl = streamApi.getPersonProfileUrl(person.id)

  return (
    <Link
      to={`/person/${person.id}`}
      className="group w-[92px] shrink-0 no-underline sm:w-[104px]"
      role="listitem"
      aria-label={`查看人物 ${person.name}`}
    >
      <div className="relative aspect-[4/5] overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-poster)] shadow-[var(--nv-shadow-card)] transition-[transform,border-color,box-shadow] duration-200 group-hover:-translate-y-[3px] group-hover:border-[var(--nv-border-default)] group-hover:shadow-[var(--nv-shadow-card-hover)]">
        {!imageFailed ? (
          <img
            src={profileUrl}
            alt=""
            className="h-full w-full object-cover transition-[filter] duration-200 group-hover:brightness-[.88]"
            loading="lazy"
            onError={() => setImageFailed(true)}
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
            <User size={28} strokeWidth={1.4} aria-hidden="true" />
          </div>
        )}
      </div>
      <p className="mt-1.5 truncate text-xs font-medium text-[var(--nv-text-primary)]" title={person.name}>{person.name}</p>
      {person.orig_name && person.orig_name !== person.name && (
        <p className="mt-0.5 truncate text-[10px] text-[var(--nv-text-tertiary)]" title={person.orig_name}>{person.orig_name}</p>
      )}
    </Link>
  )
}
