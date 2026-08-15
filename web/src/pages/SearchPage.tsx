import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { aiApi, mediaApi, personApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { Media, MixedItem, Person, SearchIntent, Series } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import Pagination from '@/components/Pagination'
import { Button, EmptyState, SearchField, Surface, Tag } from '@/components/design-system'
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

const SEARCH_BATCH_SIZE = 500
const SEARCH_FETCH_CONCURRENCY = 4
const SEARCH_CACHE_TTL = 30_000

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

type SearchUniverse = {
  media: Media[]
  series: Series[]
}

type SearchUniverseCacheEntry = SearchUniverse & {
  loadedAt: number
}

const searchUniverseCache = new Map<string, SearchUniverseCacheEntry>()
const searchUniverseInFlight = new Map<string, Promise<SearchUniverse>>()

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

function normalizeText(value?: string) {
  return (value || '').trim().toLocaleLowerCase()
}

function includesFold(value: string | undefined, query: string | undefined) {
  if (!query) return true
  return normalizeText(value).includes(normalizeText(query))
}

function getMixedTitle(item: MixedItem) {
  return item.type === 'series' ? item.series?.title || '' : item.media?.title || ''
}

function getMixedOrigTitle(item: MixedItem) {
  return item.type === 'series' ? item.series?.orig_title || '' : item.media?.orig_title || ''
}

function getMixedGenres(item: MixedItem) {
  return item.type === 'series' ? item.series?.genres || '' : item.media?.genres || ''
}

function getMixedYear(item: MixedItem) {
  return item.type === 'series' ? item.series?.year || 0 : item.media?.year || 0
}

function getMixedRating(item: MixedItem) {
  return item.type === 'series' ? item.series?.rating || 0 : item.media?.rating || 0
}

function getMixedCreatedAt(item: MixedItem) {
  return item.type === 'series' ? item.series?.created_at || '' : item.media?.created_at || ''
}

function relevanceScore(item: MixedItem, query: string) {
  const needle = normalizeText(query)
  const title = normalizeText(getMixedTitle(item))
  const origTitle = normalizeText(getMixedOrigTitle(item))
  const genres = normalizeText(getMixedGenres(item))

  if (title === needle) return 100
  if (origTitle === needle) return 95
  if (title.startsWith(needle)) return 85
  if (origTitle.startsWith(needle)) return 80
  if (title.includes(needle)) return 70
  if (origTitle.includes(needle)) return 65
  if (genres.includes(needle)) return 40
  return 0
}

function uniqueById<T extends { id: string }>(items: T[]) {
  const seen = new Set<string>()
  return items.filter((item) => {
    if (!item.id || seen.has(item.id)) return false
    seen.add(item.id)
    return true
  })
}

async function loadSearchUniverse(rawQuery: string): Promise<SearchUniverse> {
  const query = rawQuery.trim()
  const cacheKey = normalizeText(query)
  const cached = searchUniverseCache.get(cacheKey)
  if (cached && Date.now() - cached.loadedAt <= SEARCH_CACHE_TTL) {
    return { media: cached.media, series: cached.series }
  }

  const existing = searchUniverseInFlight.get(cacheKey)
  if (existing) return existing

  const promise = (async () => {
    const first = await mediaApi.searchMixed(query, 1, SEARCH_BATCH_SIZE)
    const media = [...(first.data.media || [])]
    const series = [...(first.data.series || [])]
    const mediaPages = Math.ceil((first.data.media_total || 0) / SEARCH_BATCH_SIZE)
    const seriesPages = Math.ceil((first.data.series_total || 0) / SEARCH_BATCH_SIZE)
    const totalPages = Math.max(1, mediaPages, seriesPages)

    if (totalPages > 1) {
      const remainingPages = Array.from({ length: totalPages - 1 }, (_, index) => index + 2)
      for (let index = 0; index < remainingPages.length; index += SEARCH_FETCH_CONCURRENCY) {
        const batch = remainingPages.slice(index, index + SEARCH_FETCH_CONCURRENCY)
        const responses = await Promise.all(batch.map((page) => mediaApi.searchMixed(query, page, SEARCH_BATCH_SIZE)))
        for (const response of responses) {
          media.push(...(response.data.media || []))
          series.push(...(response.data.series || []))
        }
      }
    }

    const result = {
      media: uniqueById(media.filter((item) => item.media_type !== 'episode')),
      series: uniqueById(series),
    }
    searchUniverseCache.set(cacheKey, { ...result, loadedAt: Date.now() })
    return result
  })().finally(() => {
    searchUniverseInFlight.delete(cacheKey)
  })

  searchUniverseInFlight.set(cacheKey, promise)
  return promise
}

function applySearchCriteria(universe: SearchUniverse, criteria: EffectiveSearch) {
  let items: MixedItem[] = [
    ...universe.media.map((media) => ({ type: 'movie' as const, media })),
    ...universe.series.map((series) => ({ type: 'series' as const, series })),
  ]

  if (criteria.type) items = items.filter((item) => item.type === criteria.type)
  if (criteria.genre) items = items.filter((item) => includesFold(getMixedGenres(item), criteria.genre))
  if (criteria.yearMin) items = items.filter((item) => getMixedYear(item) >= criteria.yearMin!)
  if (criteria.yearMax) items = items.filter((item) => getMixedYear(item) <= criteria.yearMax!)
  if (criteria.minRating > 0) items = items.filter((item) => getMixedRating(item) >= criteria.minRating)

  items.sort((left, right) => {
    if (criteria.sort === 'rating_desc') {
      return getMixedRating(right) - getMixedRating(left) || getMixedYear(right) - getMixedYear(left)
    }
    if (criteria.sort === 'year_desc') {
      return getMixedYear(right) - getMixedYear(left) || getMixedRating(right) - getMixedRating(left)
    }
    if (criteria.sort === 'year_asc') {
      return getMixedYear(left) - getMixedYear(right) || getMixedRating(right) - getMixedRating(left)
    }
    if (criteria.sort === 'title_asc') {
      return getMixedTitle(left).localeCompare(getMixedTitle(right), undefined, { numeric: true, sensitivity: 'base' })
    }

    const relevance = relevanceScore(right, criteria.query) - relevanceScore(left, criteria.query)
    if (relevance !== 0) return relevance
    const rating = getMixedRating(right) - getMixedRating(left)
    if (rating !== 0) return rating
    return new Date(getMixedCreatedAt(right)).getTime() - new Date(getMixedCreatedAt(left)).getTime()
  })

  return items
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

  const [draftQuery, setDraftQuery] = useState(query)
  const [results, setResults] = useState<MixedItem[]>([])
  const [people, setPeople] = useState<Person[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [showFilters, setShowFilters] = useState(false)
  const [aiState, setAiState] = useState<{ sourceQuery: string; intent: SearchIntent } | null>(null)
  const [aiLoadingQuery, setAiLoadingQuery] = useState<string | null>(null)
  const aiRequestRef = useRef(0)
  const searchRequestRef = useRef(0)
  const searchInputRef = useRef<HTMLInputElement>(null)

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

  useEffect(() => {
    setDraftQuery(query)
  }, [query])

  useEffect(() => {
    const handleShortcut = (event: KeyboardEvent) => {
      if (event.key !== '/' || event.metaKey || event.ctrlKey || event.altKey) return
      const target = event.target as HTMLElement | null
      if (target?.closest('input, textarea, select, [contenteditable="true"]')) return
      event.preventDefault()
      searchInputRef.current?.focus()
    }
    window.addEventListener('keydown', handleShortcut)
    return () => window.removeEventListener('keydown', handleShortcut)
  }, [])

  const submitSearch = (event: FormEvent) => {
    event.preventDefault()
    const nextQuery = draftQuery.trim()
    updateUrl({ q: nextQuery || null })
  }

  const clearSearch = () => {
    setDraftQuery('')
    updateUrl({ q: null })
    requestAnimationFrame(() => searchInputRef.current?.focus())
  }

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
      loadSearchUniverse(effectiveSearch.query),
      peoplePromise,
    ])
      .then(([universe, peopleResponse]) => {
        if (searchRequestRef.current !== requestId) return
        const filtered = applySearchCriteria(universe, effectiveSearch)
        const start = Math.max(0, (page - 1) * size)
        setResults(filtered.slice(start, start + size))
        setTotal(filtered.length)
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
  const draftChanged = draftQuery.trim() !== query
  const resultSummary = draftChanged && draftQuery.trim()
    ? `按 Enter 或点击搜索，查找“${draftQuery.trim()}”`
    : loading
      ? '正在搜索…'
      : searched
        ? `“${query}” · ${total} 个影视结果${people.length > 0 && page === 1 ? ` · ${people.length} 位人物` : ''}`
        : '搜索标题、原名、番号、题材、剧集或演职人员。'

  return (
    <div className="nv-section-stack nv-library-page nv-search-page">
      <header className="nv-search-workspace-header">
        <form className="nv-search-workspace-form" role="search" onSubmit={submitSearch}>
          <div className="nv-search-workspace-field-wrap">
            <SearchField
              ref={searchInputRef}
              value={draftQuery}
              onChange={(event) => setDraftQuery(event.target.value)}
              placeholder="搜索片名、原名、番号、剧集或演员"
              aria-label="搜索媒体库"
              enterKeyHint="search"
              autoComplete="off"
              wrapperClassName="nv-search-workspace-field"
            />
            <span className="nv-search-shortcut" aria-hidden="true">/</span>
          </div>

          {draftQuery.length > 0 && (
            <Button type="button" variant="ghost" size="md" iconOnly onClick={clearSearch} aria-label="清空搜索">
              <X size={16} aria-hidden="true" />
            </Button>
          )}

          <Button type="submit" variant="primary" size="md" disabled={!draftQuery.trim()}>
            <SearchIcon size={15} aria-hidden="true" />
            搜索
          </Button>

          <Button
            type="button"
            variant={showFilters || hasActiveFilters ? 'secondary' : 'ghost'}
            size="md"
            onClick={() => setShowFilters((value) => !value)}
            aria-expanded={showFilters}
            aria-controls="search-filter-panel"
          >
            <SlidersHorizontal size={15} aria-hidden="true" />
            筛选
            {hasActiveFilters && <span className="nv-search-filter-dot" aria-label="已启用筛选" />}
          </Button>
        </form>

        <div className="nv-search-workspace-summary" aria-live="polite">
          <span>{resultSummary}</span>
          {searched && !loading && hasAnyResults && (
            <span className="nv-search-result-context">第 {page} / {totalPages} 页</span>
          )}
        </div>
      </header>

      {showFilters && (
        <Surface id="search-filter-panel" className="nv-search-filter-panel space-y-3 p-3 sm:p-4">
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

      {!searched && (
        <EmptyState
          className="nv-search-empty-state"
          icon={<SearchIcon size={26} aria-hidden="true" />}
          title="搜索整个媒体库"
          description="输入片名、原名、番号、题材或演员后按 Enter 开始搜索。任何时候按 / 都可以快速回到搜索框。"
        />
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

      {(searched || loading) && <MediaGrid mixedItems={results} loading={loading} />}

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
