import { useState, useEffect, useCallback, useRef } from 'react'
import { useSearchParams } from 'react-router-dom'
import { mediaApi, aiApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { Media, SearchIntent } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import Pagination from '@/components/Pagination'
import {
  Search as SearchIcon,
  X,
  SlidersHorizontal,
  ArrowUpDown,
  Film,
  Tv,
  Calendar,
  Star,
  Sparkles,
} from 'lucide-react'
import clsx from 'clsx'
import { useTranslation } from '@/i18n'

// 排序选项
const SORT_OPTIONS = [
{ value: 'relevance', labelKey: 'search.sortRelevance' },
{ value: 'rating_desc', labelKey: 'search.sortRatingDesc' },
{ value: 'year_desc', labelKey: 'search.sortYearDesc' },
{ value: 'year_asc', labelKey: 'search.sortYearAsc' },
  { value: 'title_asc', labelKey: 'search.sortTitleAsc' },
]

// 年份范围快捷选项
const YEAR_RANGES = [
  { labelKey: 'search.yearAll', min: 0, max: 0 },
  { labelKey: '', min: 2024, max: 2026 },
  { labelKey: '', min: 2020, max: 2023 },
  { labelKey: '', min: 2010, max: 2019 },
  { labelKey: '', min: 2000, max: 2009 },
  { labelKey: 'search.yearEarlier', min: 0, max: 1999 },
]

export default function SearchPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [query, setQuery] = useState(searchParams.get('q') || '')
  const [results, setResults] = useState<Media[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)
  const toast = useToast()
  const { t } = useTranslation()

  // 从 URL 参数读取分页状态
  const page = parseInt(searchParams.get('page') || '1', 10) || 1
  const size = parseInt(searchParams.get('size') || '30', 10) || 30

  // 分页变化时同步到 URL
  const setPage = useCallback((newPage: number) => {
    const params = new URLSearchParams(searchParams)
    if (newPage <= 1) {
      params.delete('page')
    } else {
      params.set('page', String(newPage))
    }
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

  // 每页数量变化时同步到 URL，并重置到第一页
  const setSize = useCallback((newSize: number) => {
    const params = new URLSearchParams(searchParams)
    if (newSize === 30) {
      params.delete('size')
    } else {
      params.set('size', String(newSize))
    }
    params.delete('page')
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

// 筛选状态
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
      // 如果有 AI 解析结果，使用 AI 解析的参数
      const intent = aiIntent || aiParsed
      const searchQuery = intent?.parsed ? intent.query : q.trim()
      const searchType = intent?.parsed && intent.media_type ? intent.media_type : (filterType || undefined)
      const searchGenre = intent?.parsed && intent.genre ? intent.genre : undefined
      const searchYearMin = intent?.parsed && intent.year_min ? intent.year_min : (yearRange.min || undefined)
      const searchYearMax = intent?.parsed && intent.year_max ? intent.year_max : (yearRange.max || undefined)
      const searchMinRating = intent?.parsed && intent.min_rating ? intent.min_rating : (minRating || undefined)
      const searchSortBy = intent?.parsed && intent.sort_by && intent.sort_by !== 'relevance' ? intent.sort_by : sortBy

// 使用服务端高级搜索API，所有筛选和排序都在服务端完成
      let sort_by = 'created_at'
      let sort_order = 'desc'
      if (searchSortBy === 'rating_desc') {
        sort_by = 'rating'
        sort_order = 'desc'
      } else if (searchSortBy === 'year_desc') {
        sort_by = 'year'
        sort_order = 'desc'
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

  // 防抖搜索
  useEffect(() => {
    if (!query.trim()) {
      setResults([])
      setTotal(0)
      setSearched(false)
      return
    }

    const timer = setTimeout(() => {
      setPage(1)
      // 同步搜索关键词到 URL
      const params = new URLSearchParams(searchParams)
      params.set('q', query.trim())
      params.delete('page')
      setSearchParams(params, { replace: true })
      // 先尝试 AI 智能搜索
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
            if (!controller.signal.aborted) {
              setAiLoading(false)
            }
          })
      } else {
        setAiParsed(null)
        doSearch(query, 1)
      }
    }, 400)

    return () => clearTimeout(timer)
  }, [query, doSearch])

// 翻页时搜索
  useEffect(() => {
    if (page > 1 && query.trim()) {
      doSearch(query, page)
    }
  }, [page, query, doSearch])

// 筛选/排序/每页数量变化时重新搜索（回到第 1 页）
  useEffect(() => {
    if (query.trim() && searched) {
      doSearch(query, 1)
    }
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
    <div>
{/* 搜索栏 - 霓虹风格 */}
      <div className="relative mb-4">
        <SearchIcon
          size={20}
          className="absolute left-4 top-1/2 -translate-y-1/2 text-neon/40"
        />
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="input pl-12 pr-24 py-3.5 text-base"
placeholder={t('search.searchPlaceholder')}
          autoFocus
        />
        <div className="absolute right-3 top-1/2 -translate-y-1/2 flex items-center gap-1">
          {query && (
            <button
              onClick={() => setQuery('')}
              className="rounded-lg p-1.5 text-surface-500 transition-colors hover:text-neon"
            >
              <X size={16} />
            </button>
          )}
          <button
            onClick={() => setShowFilters(!showFilters)}
            className={clsx(
              'rounded-lg p-1.5 transition-colors',
              showFilters || hasActiveFilters ? 'text-neon' : 'text-surface-500 hover:text-neon'
            )}
            title={t('search.filterAndSort')}
          >
            <SlidersHorizontal size={16} />
          </button>
        </div>
      </div>

      {/* 筛选栏 */}
      {showFilters && (
        <div className="animate-slide-up mb-6 glass-panel rounded-xl p-4 space-y-4">
{/* 类型筛选 */}
          <div className="flex flex-wrap items-center gap-3">
            <span className="flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <Film size={12} /> {t('search.type')}:
            </span>
            {[
              { value: '', label: t('search.typeAll'), icon: null },
              { value: 'movie', label: t('search.typeMovie'), icon: Film },
              { value: 'episode', label: t('search.typeEpisode'), icon: Tv },
            ].map((opt) => (
              <button
                key={opt.value}
                onClick={() => setFilterType(opt.value as '' | 'movie' | 'episode')}
                className={clsx(
                  'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                  filterType === opt.value
                    ? 'bg-neon-blue/15 text-neon border border-neon-blue/30'
                    : 'text-surface-400 hover:text-surface-300'
                )}
                style={filterType !== opt.value ? { border: '1px solid var(--border-default)' } : {}}
              >
                {opt.label}
              </button>
            ))}
          </div>

{/* 年份筛选 */}
          <div className="flex flex-wrap items-center gap-3">
            <span className="flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <Calendar size={12} /> {t('search.year')}:
            </span>
            {YEAR_RANGES.map((yr) => (
              <button
                key={yr.labelKey || `${yr.min}-${yr.max}`}
                onClick={() => setYearRange({ min: yr.min, max: yr.max })}
                className={clsx(
                  'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                  yearRange.min === yr.min && yearRange.max === yr.max
                    ? 'bg-neon-blue/15 text-neon border border-neon-blue/30'
                    : 'text-surface-400 hover:text-surface-300'
                )}
                style={!(yearRange.min === yr.min && yearRange.max === yr.max) ? { border: '1px solid var(--border-default)' } : {}}
              >
                {yr.labelKey ? t(yr.labelKey) : `${yr.min}-${yr.max}`}
              </button>
            ))}
          </div>

{/* 评分筛选 */}
          <div className="flex flex-wrap items-center gap-3">
            <span className="flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <Star size={12} /> {t('search.minRating')}:
            </span>
            {[0, 6, 7, 8, 9].map((r) => (
              <button
                key={r}
                onClick={() => setMinRating(r)}
                className={clsx(
                  'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                  minRating === r
                    ? 'bg-neon-blue/15 text-neon border border-neon-blue/30'
                    : 'text-surface-400 hover:text-surface-300'
                )}
                style={minRating !== r ? { border: '1px solid var(--border-default)' } : {}}
              >
                {r === 0 ? t('search.ratingAll') : `≥${r}分`}
              </button>
            ))}
          </div>

          {/* 排序 */}
          <div className="flex flex-wrap items-center gap-3">
            <span className="flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <ArrowUpDown size={12} /> {t('search.sort')}:
            </span>
            {SORT_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setSortBy(opt.value)}
                className={clsx(
                  'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                  sortBy === opt.value
                    ? 'bg-neon-blue/15 text-neon border border-neon-blue/30'
                    : 'text-surface-400 hover:text-surface-300'
                )}
                style={sortBy !== opt.value ? { border: '1px solid var(--border-default)' } : {}}
              >
                {t(opt.labelKey)}
              </button>
            ))}
          </div>

          {/* 清除筛选 */}
          {hasActiveFilters && (
            <button
              onClick={clearFilters}
              className="text-xs text-red-400 hover:text-red-300 transition-colors"
            >
              ✕ {t('search.clearFilters')}
            </button>
          )}
        </div>
      )}

      {/* 搜索结果摘要 */}
      {searched && (
        <div className="mb-4 flex items-center gap-3 text-sm text-surface-400">
          <span>
            {t('search.found')} <span className="font-semibold text-neon">{total}</span> {t('search.results2')}
          </span>
          {aiParsed?.parsed && (
            <span className="flex items-center gap-1 rounded-md bg-purple-500/10 px-2 py-0.5 text-[10px] text-purple-400 border border-purple-500/20">
              <Sparkles size={10} />
              {t('search.aiUnderstand')}: "{aiParsed.query}"
              {aiParsed.genre && ` · ${aiParsed.genre}`}
              {aiParsed.year_min && aiParsed.year_max ? ` · ${aiParsed.year_min}-${aiParsed.year_max}` : ''}
            </span>
          )}
          {aiLoading && (
            <span className="flex items-center gap-1 text-[10px] text-purple-400">
              <Sparkles size={10} className="animate-pulse" />
              {t('search.aiAnalyzing')}
            </span>
          )}
          {hasActiveFilters && (
            <span className="rounded-md bg-neon-blue/10 px-2 py-0.5 text-[10px] text-neon">
              {t('search.filtered')}
            </span>
          )}
        </div>
      )}

      <MediaGrid items={results} loading={loading} />

      {/* 空状态 */}
      {searched && !loading && results.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div
            className="mb-6 flex h-20 w-20 items-center justify-center rounded-2xl"
            style={{
              background: 'var(--neon-blue-5)',
              border: '1px solid var(--neon-blue-8)',
            }}
          >
            <SearchIcon size={36} className="text-surface-600" />
          </div>
          <p className="font-display text-base font-semibold tracking-wide text-surface-300">{t('search.noMatch')}</p>
          <p className="mt-1 text-sm text-surface-600">
            {hasActiveFilters ? t('search.noMatchHintFiltered') : t('search.noMatchHint')}
          </p>
          {hasActiveFilters && (
            <button
              onClick={clearFilters}
              className="mt-3 text-sm text-neon hover:text-neon/80 transition-colors"
            >
              {t('search.clearFilterConditions')}
            </button>
          )}
        </div>
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
