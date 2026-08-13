import { useState, useEffect, useCallback, useRef, type ReactNode } from 'react'
import { useSearchParams } from 'react-router-dom'
import { mediaApi, aiApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { Media, SearchIntent } from '@/types'
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
      className="nv-button"
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
    <div className="flex flex-wrap items-center gap-1.5">
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
  const paramQuery = searchParams.get('q') || ''
  const [query, setQuery] = useState(paramQuery)
  const [results, setResults] = useState<Media[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)
  const toast = useToast()
  const { t } = useTranslation()

  const page = parseInt(searchParams.get('page') || '1', 10) || 1
  const size = parseInt(searchParams.get('size') || '30', 10) || 30

  useEffect(() => {
    setQuery(paramQuery)
  }, [paramQuery])

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
      if (searchSortBy === 'rating_desc') sort_by = 'rating'
      else if (searchSortBy === 'year_desc') sort_by = 'year'
      else if (searchSortBy === 'year_asc') { sort_by = 'year'; sort_order = 'asc' }
      else if (searchSortBy === 'title_asc') { sort_by = 'title'; sort_order = 'asc' }

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
  }, [aiParsed, filterType, sortBy, yearRange, minRating, size, toast, t])

  useEffect(() => {
    if (!query.trim()) {
      setResults([])
      setTotal(0)
      setSearched(false)
      return
    }

    const timer = setTimeout(() => {
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
            if (controller.signal.aborted) return
            const intent = res.data.data
            if (intent.parsed) {
              setAiParsed(intent)
              doSearch(query, 1, intent)
            } else {
              setAiParsed(null)
              doSearch(query, 1, null)
            }
          })
          .catch(() => {
            if (!controller.signal.aborted) {
              setAiParsed(null)
              doSearch(query, 1, null)
            }
          })
          .finally(() => {
            if (!controller.signal.aborted) setAiLoading(false)
          })
      } else {
        setAiParsed(null)
        doSearch(query, 1, null)
      }
    }, 320)

    return () => clearTimeout(timer)
  }, [query, doSearch, searchParams, setSearchParams])

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
      <div className="flex min-h-8 flex-wrap items-center gap-2">
        <div className="min-w-0 flex-1 text-xs text-[var(--nv-text-tertiary)]" aria-live="polite">
          {searched && (
            <>
              “<span className="text-[var(--nv-text-secondary)]">{query}</span>” · {total} 个结果
            </>
          )}
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
      </div>

      {showFilters && (
        <Surface className="space-y-3 p-3 sm:p-4">
          <FilterRow icon={<Film size={13} aria-hidden="true" />} label={`${t('search.type')}:`}>
            {[
              { value: '', label: t('search.typeAll') },
              { value: 'movie', label: t('search.typeMovie') },
              { value: 'episode', label: t('search.typeEpisode') },
            ].map((option) => (
              <FilterChip key={option.value} selected={filterType === option.value} onClick={() => setFilterType(option.value as '' | 'movie' | 'episode')}>
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
            <Button variant="ghost" size="sm" onClick={clearFilters}>
              <X size={14} aria-hidden="true" />
              {t('search.clearFilters')}
            </Button>
          )}
        </Surface>
      )}

      {(aiParsed?.parsed || aiLoading) && (
        <div className="flex flex-wrap items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
          {aiParsed?.parsed && (
            <Tag>
              <Sparkles size={11} aria-hidden="true" />
              {t('search.aiUnderstand')}: “{aiParsed.query}”
              {aiParsed.genre && ` · ${aiParsed.genre}`}
            </Tag>
          )}
          {aiLoading && <Tag><Sparkles size={11} aria-hidden="true" />{t('search.aiAnalyzing')}</Tag>}
        </div>
      )}

      <MediaGrid items={results} loading={loading} />

      {searched && !loading && results.length === 0 && (
        <EmptyState
          icon={<SearchIcon size={22} aria-hidden="true" />}
          title={t('search.noMatch')}
          description={hasActiveFilters ? t('search.noMatchHintFiltered') : t('search.noMatchHint')}
          action={hasActiveFilters ? <Button variant="secondary" size="sm" onClick={clearFilters}>{t('search.clearFilterConditions')}</Button> : undefined}
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
