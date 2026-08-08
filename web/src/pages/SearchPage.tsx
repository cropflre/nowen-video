import { useState, useEffect, useCallback, useRef, type ReactNode } from 'react'
import { useSearchParams } from 'react-router-dom'
import { mediaApi, aiApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { Media, SearchIntent } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import Pagination from '@/components/Pagination'
import { Button, EmptyState, Input, Surface, Tag } from '@/components/design-system'
import {
  ArrowUpDown,
  Calendar,
  Film,
  Search as SearchIcon,
  SlidersHorizontal,
  Sparkles,
  Star,
  Tv,
  X,
} from 'lucide-react'
import { useTranslation } from '@/i18n'

const SORT_OPTIONS = [
  { value: 'relevance', labelKey: 'search.sortRelevance' },
  { value: 'rating_desc', labelKey: 'search.sortRatingDesc' },
  { value: 'year_desc', labelKey: 'search.sortYearDesc' },
  { value: 'year_asc', labelKey: 'search.sortYearAsc' },
  { value: 'title_asc', labelKey: 'search.sortTitleAsc' },
]

const YEAR_RANGES = [
  { labelKey: 'search.yearAll', min: 0, max: 0 },
  { labelKey: '', min: 2024, max: 2026 },
  { labelKey: '', min: 2020, max: 2023 },
  { labelKey: '', min: 2010, max: 2019 },
  { labelKey: '', min: 2000, max: 2009 },
  { labelKey: 'search.yearEarlier', min: 0, max: 1999 },
]

function FilterChip({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="min-h-8 rounded-[var(--nv-radius-control)] border px-3 py-1.5 text-xs font-medium transition-[background-color,border-color,color] duration-200"
      style={{
        background: selected ? 'var(--nv-bg-active)' : 'var(--nv-bg-control)',
        borderColor: selected ? 'var(--nv-border-hover)' : 'var(--nv-border-default)',
        color: selected ? 'var(--nv-action-primary)' : 'var(--nv-text-secondary)',
      }}
      aria-pressed={selected}
    >
      {children}
    </button>
  )
}

function FilterRow({ icon, label, children }: { icon: ReactNode; label: string; children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center gap-2.5">
      <span className="mr-1 inline-flex min-w-20 items-center gap-1.5 text-xs font-medium text-[var(--nv-text-tertiary)]">
        {icon}
        {label}
      </span>
      {children}
    </div>
  )
}

export default function SearchPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [query, setQuery] = useState(searchParams.get('q') || '')
  const [results, setResults] = useState<Media[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)
  const toast = useToast()
  const { t } = useTranslation()

  const page = parseInt(searchParams.get('page') || '1', 10) || 1
  const size = parseInt(searchParams.get('size') || '30', 10) || 30

  const setPage = useCallback((newPage: number) => {
    const params = new URLSearchParams(searchParams)
    if (newPage <= 1) params.delete('page')
    else params.set('page', String(newPage))
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

  const setSize = useCallback((newSize: number) => {
    const params = new URLSearchParams(searchParams)
    if (newSize === 30) params.delete('size')
    else params.set('size', String(newSize))
    params.delete('page')
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

  const [showFilters, setShowFilters] = useState(false)
  const [filterType, setFilterType] = useState<'' | 'movie' | 'episode'>('')
  const [sortBy, setSortBy] = useState('relevance')
  const [yearRange, setYearRange] = useState<{ min: number; max: number }>({ min: 0, max: 0 })
  const [minRating, setMinRating] = useState(0)
  const [aiParsed, setAiParsed] = useState<SearchIntent | null>(null)
  const [aiLoading, setAiLoading] = useState(false)
  const aiAbortRef = useRef<AbortController | null>(null)

  const doSearch = useCallback(async (q: string, p: number, aiIntent?: SearchIntent | null) => {
    if (!q.trim()) return
    setLoading(true)
    setSearched(true)
    try {
      const intent = aiIntent || aiParsed
      const searchQuery = intent?.parsed ? intent.query : q.trim()
      const searchType = intent?.parsed && intent.media_type ? intent.media_type : (filterType || undefined)
      const searchGenre = intent?.parsed && intent.genre ? intent.genre : undefined
      const searchYearMin = intent?.parsed && intent.year_min ? intent.year_min : (yearRange.min || undefined)
      const searchYearMax = intent?.parsed && intent.year_max ? intent.year_max : (yearRange.max || undefined)
      const searchMinRating = intent?.parsed && intent.min_rating ? intent.min_rating : (minRating || undefined)
      const searchSortBy = intent?.parsed && intent.sort_by && intent.sort_by !== 'relevance' ? intent.sort_by : sortBy

      let sort_by = 'created_at'
      let sort_order = 'desc'
      if (searchSortBy === 'rating_desc') {
        sort_by = 'rating'
      } else if (searchSortBy === 'year_desc') {
        sort_by = 'year'
      } else if (searchSortBy === 'year_asc') {
        sort_by = 'year'
        sort_order = 'asc'
      } else if (searchSortBy === 'title_asc') {
        sort_by = 'title'
        sort_order = 'asc'
      }

      const res = await mediaApi.searchAdvanced({
        q: searchQuery,
        type: searchType || undefined,
        genre: searchGenre,
        year_min: searchYearMin,
        year_max: searchYearMax,
        min_rating: searchMinRating,
        sort_by,
        sort_order,
        page: p,
        size,
      })

      setResults(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      toast.error(t('search.searchFailed'))
    } finally {
      setLoading(false)
    }
  }, [filterType, sortBy, yearRange, minRating, size])

  useEffect(() => {
    if (!query.trim()) {
      setResults([])
      setTotal(0)
      setSearched(false)
      return
    }

    const timer = setTimeout(() => {
      setPage(1)
      const params = new URLSearchParams(searchParams)
      params.set('q', query.trim())
      params.delete('page')
      setSearchParams(params, { replace: true })

      if (query.trim().length > 4) {
        aiAbortRef.current?.abort()
        const controller = new AbortController()
        aiAbortRef.current = controller
        setAiLoading(true)
        aiApi.smartSearch(query.trim())
          .then((res) => {
            if (!controller.signal.aborted) {
              const intent = res.data.data
              if (intent.parsed) {
                setAiParsed(intent)
                doSearch(query, 1, intent)
              } else {
                setAiParsed(null)
                doSearch(query, 1)
              }
            }
          })
          .catch(() => {
            if (!controller.signal.aborted) {
              setAiParsed(null)
              doSearch(query, 1)
            }
          })
          .finally(() => {
            if (!controller.signal.aborted) setAiLoading(false)
          })
      } else {
        setAiParsed(null)
        doSearch(query, 1)
      }
    }, 400)

    return () => clearTimeout(timer)
  }, [query, doSearch])

  useEffect(() => {
    if (page > 1 && query.trim()) doSearch(query, page)
  }, [page, query, doSearch])

  useEffect(() => {
    if (query.trim() && searched) doSearch(query, 1)
  }, [filterType, sortBy, yearRange, minRating, size])

  const totalPages = Math.ceil(total / size)
  const hasActiveFilters = filterType !== '' || sortBy !== 'relevance' || yearRange.min > 0 || yearRange.max > 0 || minRating > 0

  const clearFilters = () => {
    setFilterType('')
    setSortBy('relevance')
    setYearRange({ min: 0, max: 0 })
    setMinRating(0)
  }

  return (
    <div className="nv-section-stack">
      <section aria-labelledby="search-page-title">
        <div className="mb-5">
          <h1 id="search-page-title" className="text-2xl font-bold tracking-[-0.02em] text-[var(--nv-text-primary)]">
            {t('nav.search')}
          </h1>
          <p className="mt-1 text-sm text-[var(--nv-text-tertiary)]">{t('search.searchPlaceholder')}</p>
        </div>

        <div className="relative">
          <SearchIcon
            size={19}
            className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-[var(--nv-text-tertiary)]"
            aria-hidden="true"
          />
          <Input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="h-12 pl-11 pr-24 text-base"
            placeholder={t('search.searchPlaceholder')}
            autoFocus
            aria-label={t('nav.search')}
          />
          <div className="absolute right-2 top-1/2 flex -translate-y-1/2 items-center gap-1">
            {query && (
              <Button variant="ghost" size="sm" iconOnly onClick={() => setQuery('')} aria-label="清空搜索">
                <X size={16} aria-hidden="true" />
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              iconOnly
              onClick={() => setShowFilters((value) => !value)}
              className={showFilters || hasActiveFilters ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : undefined}
              title={t('search.filterAndSort')}
              aria-label={t('search.filterAndSort')}
              aria-expanded={showFilters}
            >
              <SlidersHorizontal size={16} aria-hidden="true" />
            </Button>
          </div>
        </div>
      </section>

      {showFilters && (
        <Surface className="space-y-4 p-4 sm:p-5">
          <FilterRow icon={<Film size={13} aria-hidden="true" />} label={`${t('search.type')}:`}>
            {[
              { value: '', label: t('search.typeAll') },
              { value: 'movie', label: t('search.typeMovie') },
              { value: 'episode', label: t('search.typeEpisode') },
            ].map((option) => (
              <FilterChip
                key={option.value}
                selected={filterType === option.value}
                onClick={() => setFilterType(option.value as '' | 'movie' | 'episode')}
              >
                {option.label}
              </FilterChip>
            ))}
          </FilterRow>

          <FilterRow icon={<Calendar size={13} aria-hidden="true" />} label={`${t('search.year')}:`}>
            {YEAR_RANGES.map((range) => (
              <FilterChip
                key={range.labelKey || `${range.min}-${range.max}`}
                selected={yearRange.min === range.min && yearRange.max === range.max}
                onClick={() => setYearRange({ min: range.min, max: range.max })}
              >
                {range.labelKey ? t(range.labelKey) : `${range.min}-${range.max}`}
              </FilterChip>
            ))}
          </FilterRow>

          <FilterRow icon={<Star size={13} aria-hidden="true" />} label={`${t('search.minRating')}:`}>
            {[0, 6, 7, 8, 9].map((rating) => (
              <FilterChip key={rating} selected={minRating === rating} onClick={() => setMinRating(rating)}>
                {rating === 0 ? t('search.ratingAll') : `≥${rating}分`}
              </FilterChip>
            ))}
          </FilterRow>

          <FilterRow icon={<ArrowUpDown size={13} aria-hidden="true" />} label={`${t('search.sort')}:`}>
            {SORT_OPTIONS.map((option) => (
              <FilterChip key={option.value} selected={sortBy === option.value} onClick={() => setSortBy(option.value)}>
                {t(option.labelKey)}
              </FilterChip>
            ))}
          </FilterRow>

          {hasActiveFilters && (
            <Button variant="danger" size="sm" onClick={clearFilters}>
              <X size={14} aria-hidden="true" />
              {t('search.clearFilters')}
            </Button>
          )}
        </Surface>
      )}

      {searched && (
        <div className="flex flex-wrap items-center gap-2 text-sm text-[var(--nv-text-secondary)]" aria-live="polite">
          <span>
            {t('search.found')} <strong className="font-semibold text-[var(--nv-text-primary)]">{total}</strong> {t('search.results2')}
          </span>
          {aiParsed?.parsed && (
            <Tag>
              <Sparkles size={11} aria-hidden="true" />
              {t('search.aiUnderstand')}: “{aiParsed.query}”
              {aiParsed.genre && ` · ${aiParsed.genre}`}
              {aiParsed.year_min && aiParsed.year_max ? ` · ${aiParsed.year_min}-${aiParsed.year_max}` : ''}
            </Tag>
          )}
          {aiLoading && (
            <Tag>
              <Sparkles size={11} className="animate-pulse" aria-hidden="true" />
              {t('search.aiAnalyzing')}
            </Tag>
          )}
          {hasActiveFilters && <Tag tone="brand">{t('search.filtered')}</Tag>}
        </div>
      )}

      <MediaGrid items={results} loading={loading} />

      {searched && !loading && results.length === 0 && (
        <EmptyState
          icon={<SearchIcon size={26} aria-hidden="true" />}
          title={t('search.noMatch')}
          description={hasActiveFilters ? t('search.noMatchHintFiltered') : t('search.noMatchHint')}
          action={hasActiveFilters ? (
            <Button variant="secondary" size="sm" onClick={clearFilters}>
              {t('search.clearFilterConditions')}
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
    </div>
  )
}
