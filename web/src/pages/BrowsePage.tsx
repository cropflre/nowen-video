import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { mediaApi, seriesApi, libraryApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type { Series, MixedItem, Library } from '@/types'
import MediaCard from '@/components/MediaCard'
import Pagination from '@/components/Pagination'
import { motion, AnimatePresence } from 'framer-motion'
import {
  pageVariants,
  staggerContainerVariants,
  staggerItemVariants,
  easeSmooth,
  durations,
} from '@/lib/motion'
import {
  Search,
  X,
  Grid3X3,
  LayoutList,
  LayoutGrid,
  ArrowUpDown,
  ChevronDown,
  Film,
  Tv,
  Star,
  Calendar,
  Globe,
  Tag,
  Layers,
  Clock,
  SlidersHorizontal,
  Play,
} from 'lucide-react'
import clsx from 'clsx'

// ==================== 常量定义 ====================

// 排序选项
const SORT_OPTIONS = [
  { value: 'created_desc', label: '最近添加', icon: Clock },
  { value: 'rating_desc', label: '评分最高', icon: Star },
  { value: 'year_desc', label: '年份最新', icon: Calendar },
  { value: 'year_asc', label: '年份最早', icon: Calendar },
  { value: 'title_asc', label: '名称 A-Z', icon: ArrowUpDown },
  { value: 'title_desc', label: '名称 Z-A', icon: ArrowUpDown },
]

// 年份范围快捷选项
const YEAR_RANGES = [
  { label: '全部', min: 0, max: 0 },
  { label: '2024-2026', min: 2024, max: 2026 },
  { label: '2020-2023', min: 2020, max: 2023 },
  { label: '2010-2019', min: 2010, max: 2019 },
  { label: '2000-2009', min: 2000, max: 2009 },
  { label: '更早', min: 0, max: 1999 },
]

// 评分选项
const RATING_OPTIONS = [
  { label: '不限', value: 0 },
  { label: '≥6分', value: 6 },
  { label: '≥7分', value: 7 },
  { label: '≥8分', value: 8 },
  { label: '≥9分', value: 9 },
]

// 视图模式
type ViewMode = 'grid' | 'list' | 'poster'

// ==================== 主组件 ====================

export default function BrowsePage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const toast = useToast()
  const { on, off } = useWebSocket()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // ===== 数据状态 =====
  const [libraries, setLibraries] = useState<Library[]>([])
  const [mixedItems, setMixedItems] = useState<MixedItem[]>([])
  const [seriesList, setSeriesList] = useState<Series[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)

  // ===== 筛选状态（从 URL 参数恢复） =====
  const page = parseInt(searchParams.get('page') || '1', 10) || 1
  const size = 30
  const [searchQuery, setSearchQuery] = useState(searchParams.get('q') || '')
  const [selectedLibrary, setSelectedLibrary] = useState<string>(searchParams.get('lib') || '')
  const [mediaType, setMediaType] = useState<'' | 'movie' | 'series'>(
    (searchParams.get('type') as '' | 'movie' | 'series') || ''
  )
  const [selectedGenres, setSelectedGenres] = useState<string[]>(() => {
    const g = searchParams.get('genres')
    return g ? g.split(',').filter(Boolean) : []
  })
  const [selectedCountry, setSelectedCountry] = useState(searchParams.get('country') || '')
  const [yearRange, setYearRange] = useState<{ min: number; max: number }>(() => ({
    min: parseInt(searchParams.get('year_min') || '0', 10) || 0,
    max: parseInt(searchParams.get('year_max') || '0', 10) || 0,
  }))
  const [minRating, setMinRating] = useState(parseInt(searchParams.get('rating') || '0', 10) || 0)
  const [sortValue, setSortValue] = useState(searchParams.get('sort') || 'created_desc')
  const [viewMode, setViewMode] = useState<ViewMode>(
    (searchParams.get('view') as ViewMode) || 'grid'
  )
  const [showFilters, setShowFilters] = useState(false)
  const [showSortDropdown, setShowSortDropdown] = useState(false)

  // ===== 分页 =====
  const setPage = useCallback((newPage: number) => {
    const params = new URLSearchParams(searchParams)
    if (newPage <= 1) params.delete('page')
    else params.set('page', String(newPage))
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

  // ===== 同步筛选状态到 URL =====
  const syncFiltersToUrl = useCallback(() => {
    const params = new URLSearchParams()
    if (searchQuery) params.set('q', searchQuery)
    if (selectedLibrary) params.set('lib', selectedLibrary)
    if (mediaType) params.set('type', mediaType)
    if (selectedGenres.length > 0) params.set('genres', selectedGenres.join(','))
    if (selectedCountry) params.set('country', selectedCountry)
    if (yearRange.min > 0) params.set('year_min', String(yearRange.min))
    if (yearRange.max > 0) params.set('year_max', String(yearRange.max))
    if (minRating > 0) params.set('rating', String(minRating))
    if (sortValue !== 'created_desc') params.set('sort', sortValue)
    if (viewMode !== 'grid') params.set('view', viewMode)
    setSearchParams(params, { replace: true })
  }, [searchQuery, selectedLibrary, mediaType, selectedGenres, selectedCountry, yearRange, minRating, sortValue, viewMode, setSearchParams])

  // 筛选变化时同步到 URL
  useEffect(() => {
    syncFiltersToUrl()
  }, [syncFiltersToUrl])

  // ===== 加载媒体库列表 =====
  useEffect(() => {
    libraryApi.list().then((res) => {
      setLibraries(res.data.data || [])
    }).catch(() => {})
  }, [])

  // ===== 加载数据 =====
  const fetchData = useCallback(async () => {
    setLoading(true)
    try {
      const libId = selectedLibrary || undefined
      const [mixedRes, seriesRes] = await Promise.all([
        mediaApi.listMixed({ page, size, library_id: libId }),
        seriesApi.list({ library_id: libId }),
      ])
      setMixedItems(mixedRes.data.data || [])
      setTotal(mixedRes.data.total)
      setSeriesList(seriesRes.data.data || [])
    } catch {
      toast.error('加载影视库内容失败')
    } finally {
      setLoading(false)
    }
  }, [page, selectedLibrary])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  // ===== WebSocket 实时更新 =====
  useEffect(() => {
    const debouncedRefresh = () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(() => fetchData(), 1000)
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
  }, [on, off, fetchData])

  // ===== 提取所有分类标签 =====
  const { allGenres, allCountries } = useMemo(() => {
    const genres = new Set<string>()
    const countries = new Set<string>()

    const processItem = (g?: string, c?: string) => {
      if (g) g.split(',').forEach((s) => { const t = s.trim(); if (t) genres.add(t) })
      if (c) c.split(',').forEach((s) => { const t = s.trim(); if (t) countries.add(t) })
    }

    mixedItems.forEach((item) => {
      if (item.type === 'series' && item.series) {
        processItem(item.series.genres, item.series.country)
      } else if (item.media) {
        processItem(item.media.genres, item.media.country)
      }
    })
    seriesList.forEach((s) => processItem(s.genres, s.country))

    return {
      allGenres: Array.from(genres).sort(),
      allCountries: Array.from(countries).sort(),
    }
  }, [mixedItems, seriesList])

  // ===== 辅助函数 =====
  const getItemTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.title || '') : (item.media?.title || '')
  const getItemOrigTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.orig_title || '') : (item.media?.orig_title || '')
  const getItemOverview = (item: MixedItem) => item.type === 'series' ? (item.series?.overview || '') : (item.media?.overview || '')
  const getItemGenres = (item: MixedItem) => item.type === 'series' ? (item.series?.genres || '') : (item.media?.genres || '')
  const getItemCountry = (item: MixedItem) => item.type === 'series' ? (item.series?.country || '') : (item.media?.country || '')
  const getItemYear = (item: MixedItem) => item.type === 'series' ? (item.series?.year || 0) : (item.media?.year || 0)
  const getItemRating = (item: MixedItem) => item.type === 'series' ? (item.series?.rating || 0) : (item.media?.rating || 0)
  const getItemTime = (item: MixedItem) => item.type === 'series' ? (item.series?.created_at || '') : (item.media?.created_at || '')

  // ===== 筛选和排序 =====
  const filteredItems = useMemo(() => {
    let items = [...mixedItems]

    // 媒体类型筛选
    if (mediaType === 'movie') {
      items = items.filter((item) => item.type === 'movie')
    } else if (mediaType === 'series') {
      items = items.filter((item) => item.type === 'series')
    }

    // 搜索
    if (searchQuery.trim()) {
      const q = searchQuery.trim().toLowerCase()
      items = items.filter((item) =>
        getItemTitle(item).toLowerCase().includes(q) ||
        getItemOrigTitle(item).toLowerCase().includes(q) ||
        getItemOverview(item).toLowerCase().includes(q)
      )
    }

    // 类型标签筛选（多选）
    if (selectedGenres.length > 0) {
      items = items.filter((item) => {
        const genres = getItemGenres(item)
        return selectedGenres.every((g) => genres.includes(g))
      })
    }

    // 地区筛选
    if (selectedCountry) {
      items = items.filter((item) => getItemCountry(item).includes(selectedCountry))
    }

    // 年份筛选
    if (yearRange.min > 0 || yearRange.max > 0) {
      items = items.filter((item) => {
        const year = getItemYear(item)
        if (year === 0) return false
        if (yearRange.min > 0 && year < yearRange.min) return false
        if (yearRange.max > 0 && year > yearRange.max) return false
        return true
      })
    }

    // 评分筛选
    if (minRating > 0) {
      items = items.filter((item) => getItemRating(item) >= minRating)
    }

    // 排序
    const [field, dir] = sortValue.split('_')
    items.sort((a, b) => {
      let cmp = 0
      if (field === 'title') cmp = getItemTitle(a).localeCompare(getItemTitle(b))
      else if (field === 'year') cmp = getItemYear(a) - getItemYear(b)
      else if (field === 'rating') cmp = getItemRating(a) - getItemRating(b)
      else cmp = new Date(getItemTime(a)).getTime() - new Date(getItemTime(b)).getTime()
      return dir === 'desc' ? -cmp : cmp
    })

    return items
  }, [mixedItems, mediaType, searchQuery, selectedGenres, selectedCountry, yearRange, minRating, sortValue])

  const totalPages = Math.ceil(total / size)
  const currentSortLabel = SORT_OPTIONS.find((o) => o.value === sortValue)?.label || '排序'

  // 活跃筛选条件数量
  const activeFilterCount = [
    selectedGenres.length > 0,
    selectedCountry !== '',
    yearRange.min > 0 || yearRange.max > 0,
    minRating > 0,
  ].filter(Boolean).length

  // 清除所有筛选
  const clearAllFilters = () => {
    setSearchQuery('')
    setSelectedLibrary('')
    setMediaType('')
    setSelectedGenres([])
    setSelectedCountry('')
    setYearRange({ min: 0, max: 0 })
    setMinRating(0)
    setSortValue('created_desc')
  }

  // 切换类型标签
  const toggleGenre = (genre: string) => {
    setSelectedGenres((prev) =>
      prev.includes(genre) ? prev.filter((g) => g !== genre) : [...prev, genre]
    )
  }

  // ===== 统计信息 =====
  const stats = useMemo(() => {
    let movieCount = 0
    let seriesCount = 0
    mixedItems.forEach((item) => {
      if (item.type === 'movie') movieCount++
      else if (item.type === 'series') seriesCount++
    })
    return { movieCount, seriesCount, total: mixedItems.length }
  }, [mixedItems])

  // ==================== 渲染 ====================
  return (
    <motion.div
      variants={pageVariants}
      initial="initial"
      animate="enter"
      className="space-y-6"
    >
      {/* ===== 页面标题 ===== */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-bold flex items-center gap-2 text-gradient">
            <Layers className="text-neon-blue animate-neon-breathe" size={24} />
            影视库
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-tertiary)' }}>
            浏览和发现你的影视收藏 · {stats.total} 部作品
          </p>
        </div>
      </div>

      {/* ===== 统计卡片 ===== */}
      <motion.div
        className="grid grid-cols-4 gap-3"
        variants={staggerContainerVariants}
        initial="hidden"
        animate="visible"
      >
        {[
          { key: '' as const, label: '全部', icon: Layers, iconClass: 'text-neon-blue', gradientColor: 'var(--neon-blue)', value: stats.total },
          { key: 'movie' as const, label: '电影', icon: Film, iconClass: 'text-purple-400', gradientColor: 'var(--neon-purple)', value: stats.movieCount },
          { key: 'series' as const, label: '剧集', icon: Tv, iconClass: 'text-emerald-400', gradientColor: 'var(--neon-blue)', value: stats.seriesCount },
        ].map((card) => (
          <motion.div
            key={card.key}
            variants={staggerItemVariants}
            className="relative overflow-hidden rounded-xl p-4 cursor-pointer transition-all duration-300 hover:shadow-card-hover"
            style={{
              background: 'var(--glass-bg)',
              border: `1px solid ${mediaType === card.key ? 'var(--neon-blue-30)' : 'var(--neon-blue-6)'}`,
            }}
            onClick={() => setMediaType(mediaType === card.key ? '' : card.key as '' | 'movie' | 'series')}
          >
            <div className="absolute top-0 left-0 right-0 h-[1px] opacity-60" style={{ background: `linear-gradient(90deg, transparent, ${card.gradientColor}, transparent)` }} />
            <div className="flex items-center gap-2 text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
              <card.icon size={14} className={card.iconClass} />
              {card.label}
            </div>
            <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>{card.value}</div>
          </motion.div>
        ))}
        <motion.div
          variants={staggerItemVariants}
          className="relative overflow-hidden rounded-xl p-4 transition-all duration-300 hover:shadow-card-hover"
          style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}
        >
          <div className="absolute top-0 left-0 right-0 h-[1px] opacity-60" style={{ background: 'linear-gradient(90deg, transparent, var(--neon-purple), transparent)' }} />
          <div className="flex items-center gap-2 text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
            <Tag size={14} className="text-yellow-400" />
            类型标签
          </div>
          <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>{allGenres.length}</div>
        </motion.div>
      </motion.div>

      {/* ===== 工具栏 ===== */}
      <div className="space-y-3">
        {/* 第一行：媒体库选择 + 搜索 + 排序 + 视图 */}
        <div className="flex flex-wrap items-center gap-3">
          {/* 媒体库选择 */}
          {libraries.length > 1 && (
            <div className="flex items-center gap-1.5">
              <button
                onClick={() => { setSelectedLibrary(''); setPage(1) }}
                className={clsx(
                  'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                  !selectedLibrary && 'font-semibold'
                )}
                style={!selectedLibrary ? {
                  background: 'var(--neon-blue-15)',
                  border: '1px solid var(--neon-blue-30)',
                  color: 'var(--neon-blue)',
                } : {
                  border: '1px solid var(--neon-blue-6)',
                  color: 'var(--text-muted)',
                }}
              >
                全部库
              </button>
              {libraries.map((lib) => (
                <button
                  key={lib.id}
                  onClick={() => { setSelectedLibrary(lib.id); setPage(1) }}
                  className={clsx(
                    'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                    selectedLibrary === lib.id && 'font-semibold'
                  )}
                  style={selectedLibrary === lib.id ? {
                    background: 'var(--neon-blue-15)',
                    border: '1px solid var(--neon-blue-30)',
                    color: 'var(--neon-blue)',
                  } : {
                    border: '1px solid var(--neon-blue-6)',
                    color: 'var(--text-muted)',
                  }}
                >
                  {lib.name}
                </button>
              ))}
            </div>
          )}

          {/* 搜索框 */}
          <div className="relative ml-auto flex-1 max-w-xs min-w-[200px]">
            <Search
              size={16}
              className="absolute left-3 top-1/2 -translate-y-1/2"
              style={{ color: 'var(--text-muted)' }}
            />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input pl-9 pr-8 py-2 text-sm w-full"
              placeholder="搜索影视作品..."
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-0.5 transition-colors hover:bg-[var(--nav-hover-bg)]"
                style={{ color: 'var(--text-muted)' }}
              >
                <X size={14} />
              </button>
            )}
          </div>

          {/* 筛选按钮 */}
          <button
            onClick={() => setShowFilters(!showFilters)}
            className={clsx(
              'flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all active:scale-95',
              activeFilterCount > 0 && 'text-neon'
            )}
            style={{
              border: `1px solid ${activeFilterCount > 0 ? 'var(--border-hover)' : 'var(--border-default)'}`,
              color: activeFilterCount > 0 ? 'var(--neon-blue)' : 'var(--text-secondary)',
              background: activeFilterCount > 0 ? 'var(--nav-active-bg)' : 'transparent',
            }}
          >
            <SlidersHorizontal size={14} />
            筛选
            {activeFilterCount > 0 && (
              <span
                className="ml-1 rounded-full px-1.5 text-[10px] font-bold"
                style={{
                  background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                  color: 'var(--text-on-neon)',
                }}
              >
                {activeFilterCount}
              </span>
            )}
          </button>

          {/* 排序下拉 */}
          <div className="relative">
            <button
              onClick={() => setShowSortDropdown(!showSortDropdown)}
              className="flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all active:scale-95"
              style={{
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
              }}
            >
              <ArrowUpDown size={14} />
              {currentSortLabel}
              <ChevronDown size={12} className={clsx('transition-transform', showSortDropdown && 'rotate-180')} />
            </button>
            <AnimatePresence>
              {showSortDropdown && (
                <>
                  <div className="fixed inset-0 z-30" onClick={() => setShowSortDropdown(false)} />
                  <motion.div
                    initial={{ opacity: 0, scale: 0.95, y: -4 }}
                    animate={{ opacity: 1, scale: 1, y: 0 }}
                    exit={{ opacity: 0, scale: 0.95, y: -4 }}
                    transition={{ duration: 0.15 }}
                    className="absolute right-0 top-full z-40 mt-1 w-44 overflow-hidden rounded-xl py-1"
                    style={{
                      background: 'var(--bg-elevated)',
                      border: '1px solid var(--border-strong)',
                      boxShadow: 'var(--shadow-elevated)',
                    }}
                  >
                    {SORT_OPTIONS.map((opt) => {
                      const Icon = opt.icon
                      return (
                        <button
                          key={opt.value}
                          onClick={() => { setSortValue(opt.value); setShowSortDropdown(false) }}
                          className={clsx(
                            'w-full flex items-center gap-2 px-3 py-2 text-left text-sm transition-colors',
                            sortValue === opt.value
                              ? 'text-neon bg-[var(--nav-active-bg)]'
                              : 'hover:bg-[var(--nav-hover-bg)]'
                          )}
                          style={sortValue !== opt.value ? { color: 'var(--text-secondary)' } : undefined}
                        >
                          <Icon size={14} />
                          {opt.label}
                        </button>
                      )
                    })}
                  </motion.div>
                </>
              )}
            </AnimatePresence>
          </div>

          {/* 视图切换 */}
          <div
            className="flex items-center rounded-xl overflow-hidden"
            style={{ border: '1px solid var(--border-default)' }}
          >
            <button
              onClick={() => setViewMode('grid')}
              className="p-2 transition-all"
              style={{
                background: viewMode === 'grid' ? 'var(--nav-active-bg)' : 'transparent',
                color: viewMode === 'grid' ? 'var(--neon-blue)' : 'var(--text-tertiary)',
              }}
              title="网格视图"
            >
              <Grid3X3 size={16} />
            </button>
            <button
              onClick={() => setViewMode('list')}
              className="p-2 transition-all"
              style={{
                background: viewMode === 'list' ? 'var(--nav-active-bg)' : 'transparent',
                color: viewMode === 'list' ? 'var(--neon-blue)' : 'var(--text-tertiary)',
              }}
              title="列表视图"
            >
              <LayoutList size={16} />
            </button>
            <button
              onClick={() => setViewMode('poster')}
              className="p-2 transition-all"
              style={{
                background: viewMode === 'poster' ? 'var(--nav-active-bg)' : 'transparent',
                color: viewMode === 'poster' ? 'var(--neon-blue)' : 'var(--text-tertiary)',
              }}
              title="海报墙视图"
            >
              <LayoutGrid size={16} />
            </button>
          </div>
        </div>

        {/* ===== 筛选面板 ===== */}
        <AnimatePresence>
          {showFilters && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.25, ease: easeSmooth as unknown as [number, number, number, number] }}
              className="overflow-hidden"
            >
              <div
                className="rounded-xl p-4 space-y-4"
                style={{
                  background: 'var(--glass-bg)',
                  border: '1px solid var(--neon-blue-6)',
                }}
              >
                {/* 类型标签 */}
                {allGenres.length > 0 && (
                  <div className="space-y-2">
                    <div className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                      <Tag size={12} />
                      类型标签
                      {selectedGenres.length > 0 && (
                        <span className="rounded-full px-1.5 text-[10px] font-bold" style={{ background: 'var(--neon-blue-15)', color: 'var(--neon-blue)' }}>
                          {selectedGenres.length}
                        </span>
                      )}
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {allGenres.map((genre) => (
                        <button
                          key={genre}
                          onClick={() => toggleGenre(genre)}
                          className={clsx(
                            'rounded-lg px-2.5 py-1 text-xs font-medium transition-all active:scale-95',
                            selectedGenres.includes(genre) && 'text-neon'
                          )}
                          style={selectedGenres.includes(genre) ? {
                            background: 'var(--neon-blue-15)',
                            border: '1px solid var(--neon-blue-30)',
                            color: 'var(--neon-blue)',
                          } : {
                            border: '1px solid var(--neon-blue-6)',
                            color: 'var(--text-muted)',
                          }}
                        >
                          {genre}
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {/* 地区筛选 */}
                {allCountries.length > 0 && (
                  <div className="space-y-2">
                    <div className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                      <Globe size={12} />
                      地区
                    </div>
                    <div className="flex flex-wrap gap-2">
                      <button
                        onClick={() => setSelectedCountry('')}
                        className={clsx(
                          'rounded-lg px-2.5 py-1 text-xs font-medium transition-all',
                          !selectedCountry && 'text-neon'
                        )}
                        style={!selectedCountry ? {
                          background: 'var(--neon-blue-15)',
                          border: '1px solid var(--neon-blue-30)',
                        } : {
                          border: '1px solid var(--neon-blue-6)',
                          color: 'var(--text-muted)',
                        }}
                      >
                        全部
                      </button>
                      {allCountries.map((country) => (
                        <button
                          key={country}
                          onClick={() => setSelectedCountry(selectedCountry === country ? '' : country)}
                          className={clsx(
                            'rounded-lg px-2.5 py-1 text-xs font-medium transition-all',
                            selectedCountry === country && 'text-neon'
                          )}
                          style={selectedCountry === country ? {
                            background: 'var(--neon-blue-15)',
                            border: '1px solid var(--neon-blue-30)',
                          } : {
                            border: '1px solid var(--neon-blue-6)',
                            color: 'var(--text-muted)',
                          }}
                        >
                          {country}
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {/* 年份筛选 */}
                <div className="space-y-2">
                  <div className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                    <Calendar size={12} />
                    年份
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {YEAR_RANGES.map((yr) => {
                      const isActive = yearRange.min === yr.min && yearRange.max === yr.max
                      return (
                        <button
                          key={yr.label}
                          onClick={() => setYearRange({ min: yr.min, max: yr.max })}
                          className={clsx(
                            'rounded-lg px-2.5 py-1 text-xs font-medium transition-all',
                            isActive && 'text-neon'
                          )}
                          style={isActive ? {
                            background: 'var(--neon-blue-15)',
                            border: '1px solid var(--neon-blue-30)',
                          } : {
                            border: '1px solid var(--neon-blue-6)',
                            color: 'var(--text-muted)',
                          }}
                        >
                          {yr.label}
                        </button>
                      )
                    })}
                  </div>
                </div>

                {/* 评分筛选 */}
                <div className="space-y-2">
                  <div className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                    <Star size={12} />
                    最低评分
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {RATING_OPTIONS.map((opt) => {
                      const isActive = minRating === opt.value
                      return (
                        <button
                          key={opt.value}
                          onClick={() => setMinRating(opt.value)}
                          className={clsx(
                            'rounded-lg px-2.5 py-1 text-xs font-medium transition-all',
                            isActive && 'text-neon'
                          )}
                          style={isActive ? {
                            background: 'var(--neon-blue-15)',
                            border: '1px solid var(--neon-blue-30)',
                          } : {
                            border: '1px solid var(--neon-blue-6)',
                            color: 'var(--text-muted)',
                          }}
                        >
                          {opt.label}
                        </button>
                      )
                    })}
                  </div>
                </div>

                {/* 清除筛选 */}
                {activeFilterCount > 0 && (
                  <div className="flex items-center justify-between pt-2" style={{ borderTop: '1px solid var(--neon-blue-6)' }}>
                    <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                      已选择 {activeFilterCount} 个筛选条件
                    </span>
                    <button
                      onClick={clearAllFilters}
                      className="flex items-center gap-1 text-xs text-red-400 hover:text-red-300 transition-colors"
                    >
                      <X size={12} />
                      清除所有筛选
                    </button>
                  </div>
                )}
              </div>
            </motion.div>
          )}
        </AnimatePresence>

        {/* 已选标签展示 */}
        {selectedGenres.length > 0 && (
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>已选标签:</span>
            {selectedGenres.map((genre) => (
              <span
                key={genre}
                className="flex items-center gap-1 rounded-lg px-2 py-0.5 text-xs font-medium cursor-pointer transition-all hover:opacity-80"
                style={{
                  background: 'var(--neon-blue-10)',
                  border: '1px solid var(--neon-blue-20)',
                  color: 'var(--neon-blue)',
                }}
                onClick={() => toggleGenre(genre)}
              >
                {genre}
                <X size={10} />
              </span>
            ))}
            <button
              onClick={() => setSelectedGenres([])}
              className="text-xs transition-colors hover:text-red-400"
              style={{ color: 'var(--text-muted)' }}
            >
              清除
            </button>
          </div>
        )}

        {/* 搜索结果提示 */}
        {(searchQuery || activeFilterCount > 0) && (
          <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--text-tertiary)' }}>
            <span>
              找到 <strong className="text-neon">{filteredItems.length}</strong> 个结果
            </span>
            <button
              onClick={clearAllFilters}
              className="flex items-center gap-1 rounded-lg px-2 py-1 text-xs transition-colors hover:bg-[var(--nav-hover-bg)]"
              style={{ color: 'var(--text-secondary)' }}
            >
              <X size={12} />
              清除筛选
            </button>
          </div>
        )}
      </div>

      {/* ===== 内容区域 ===== */}
      <AnimatePresence mode="wait">
        <motion.div
          key={`${viewMode}-${mediaType}-${sortValue}-${page}`}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          {loading ? (
            // 骨架屏
            <div className={clsx(
              viewMode === 'poster'
                ? 'grid grid-cols-3 gap-x-2 gap-y-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8'
                : viewMode === 'list'
                  ? 'space-y-2'
                  : 'grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6'
            )}>
              {Array.from({ length: viewMode === 'list' ? 8 : 12 }).map((_, i) => (
                viewMode === 'list' ? (
                  <div key={i} className="flex items-center gap-4 rounded-xl p-3" style={{ border: '1px solid var(--border-default)' }}>
                    <div className="skeleton h-16 w-12 flex-shrink-0 rounded-lg" />
                    <div className="flex-1 space-y-2">
                      <div className="skeleton h-4 w-3/4 rounded" />
                      <div className="skeleton h-3 w-1/2 rounded" />
                    </div>
                  </div>
                ) : (
                  <div key={i}>
                    <div className="skeleton aspect-[2/3] rounded-xl" />
                    <div className="skeleton mt-2 h-4 w-3/4 rounded" />
                    <div className="skeleton mt-1 h-3 w-1/2 rounded" />
                  </div>
                )
              ))}
            </div>
          ) : filteredItems.length === 0 ? (
            // 空状态
            <motion.div
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: durations.normal, ease: easeSmooth as unknown as [number, number, number, number] }}
              className="flex flex-col items-center justify-center py-20 text-center"
            >
              <div
                className="mb-6 flex h-20 w-20 items-center justify-center rounded-2xl animate-float"
                style={{
                  background: 'linear-gradient(135deg, var(--neon-blue-10), var(--neon-purple-10))',
                  border: '1px solid var(--neon-blue-10)',
                }}
              >
                <Film size={36} className="text-surface-600" />
              </div>
              <h3 className="font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-secondary)' }}>
                {searchQuery || activeFilterCount > 0 ? '没有找到匹配的内容' : '影视库暂无内容'}
              </h3>
              <p className="mt-2 text-sm" style={{ color: 'var(--text-muted)' }}>
                {searchQuery || activeFilterCount > 0 ? '尝试调整筛选条件或使用其他关键词' : '前往管理页面添加媒体库并扫描文件'}
              </p>
              {(searchQuery || activeFilterCount > 0) && (
                <button
                  onClick={clearAllFilters}
                  className="mt-3 text-sm text-neon hover:text-neon/80 transition-colors"
                >
                  清除所有筛选
                </button>
              )}
            </motion.div>
          ) : viewMode === 'grid' ? (
            // 网格视图
            <motion.div
              className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
              variants={staggerContainerVariants}
              initial="hidden"
              animate="visible"
            >
              {filteredItems.map((item) => {
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
            // 列表视图
            <motion.div
              className="space-y-2"
              variants={staggerContainerVariants}
              initial="hidden"
              animate="visible"
            >
              {filteredItems.map((item) => (
                <motion.div key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`} variants={staggerItemVariants}>
                  <BrowseListItem item={item} />
                </motion.div>
              ))}
            </motion.div>
          ) : (
            // 海报墙视图
            <motion.div
              className="grid grid-cols-3 gap-x-2 gap-y-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8"
              variants={staggerContainerVariants}
              initial="hidden"
              animate="visible"
            >
              {filteredItems.map((item) => (
                <motion.div key={item.type === 'series' ? `s-${item.series?.id}` : `m-${item.media?.id}`} variants={staggerItemVariants}>
                  <PosterWallItem item={item} />
                </motion.div>
              ))}
            </motion.div>
          )}
        </motion.div>
      </AnimatePresence>

      {/* ===== 分页 ===== */}
      <Pagination
        page={page}
        totalPages={totalPages}
        total={total}
        pageSize={size}
        onPageChange={setPage}
      />
    </motion.div>
  )
}

// ==================== 列表视图项 ====================
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

  // 解析标签列表
  const genreList = genres ? genres.split(',').map((g: string) => g.trim()).filter(Boolean) : []
  const MAX_VISIBLE_TAGS = 3
  const hasMoreTags = genreList.length > MAX_VISIBLE_TAGS
  const visibleTags = tagsExpanded ? genreList : genreList.slice(0, MAX_VISIBLE_TAGS)

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
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    if (h > 0) return `${h}h ${m}m`
    return `${m}m`
  }

  return (
    <Link
      to={linkTo}
      className="group flex items-center gap-4 rounded-xl p-3 transition-all duration-300"
      style={{ border: '1px solid var(--border-default)' }}
      onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--nav-hover-bg)'; e.currentTarget.style.borderColor = 'var(--border-hover)' }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.borderColor = 'var(--border-default)' }}
    >
      {/* 缩略图 */}
      <div className="h-20 w-14 flex-shrink-0 overflow-hidden rounded-lg" style={{ background: 'var(--bg-surface)' }}>
        <img
          src={posterUrl}
          alt={title}
          className="h-full w-full object-cover"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
      </div>

      {/* 信息 */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <h3
            className="truncate text-sm font-medium transition-colors group-hover:text-neon"
            style={{ color: 'var(--text-primary)' }}
          >
            {title}
          </h3>
          {isSeries && (
            <span className="flex-shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'var(--neon-blue-10)', color: 'var(--neon-blue)' }}>
              剧集
            </span>
          )}
        </div>
        <div className="mt-1 flex items-center gap-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {year > 0 && <span>{year}</span>}
          {country && (
            <>
              <span style={{ color: 'var(--text-muted)' }}>·</span>
              <span>{country}</span>
            </>
          )}
          {duration > 0 && (
            <>
              <span style={{ color: 'var(--text-muted)' }}>·</span>
              <span>{formatDuration(duration)}</span>
            </>
          )}
          {isSeries && series && (
            <>
              <span style={{ color: 'var(--text-muted)' }}>·</span>
              <span>{series.season_count} 季 · {series.episode_count} 集</span>
            </>
          )}
        </div>
        {/* 类型标签（支持展开/收缩） */}
        {genreList.length > 0 && (
          <div className="mt-1.5 flex flex-wrap items-center gap-1">
            {visibleTags.map((genre) => (
              <span
                key={genre}
                className="rounded px-1.5 py-0.5 text-[10px] transition-all duration-200"
                style={{ background: 'var(--neon-blue-6)', color: 'var(--text-muted)' }}
              >
                {genre}
              </span>
            ))}
            {hasMoreTags && (
              <button
                onClick={(e) => { e.preventDefault(); e.stopPropagation(); setTagsExpanded(!tagsExpanded) }}
                className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-medium transition-all duration-200 hover:brightness-125 cursor-pointer"
                style={{
                  background: tagsExpanded ? 'var(--neon-blue-10)' : 'var(--neon-blue-4)',
                  color: 'var(--neon-blue)',
                  border: '1px solid var(--neon-blue-10)',
                }}
                title={tagsExpanded ? '收起标签' : `展开全部 ${genreList.length} 个标签`}
              >
                {tagsExpanded ? '收起' : `+${genreList.length - MAX_VISIBLE_TAGS}`}
                <ChevronDown
                  size={10}
                  className="transition-transform duration-200"
                  style={{ transform: tagsExpanded ? 'rotate(180deg)' : 'rotate(0deg)' }}
                />
              </button>
            )}
          </div>
        )}
        {/* 简介 */}
        {overview && (
          <p className="mt-1 line-clamp-1 text-xs" style={{ color: 'var(--text-muted)' }}>{overview}</p>
        )}
      </div>

      {/* 评分 */}
      {rating > 0 && (
        <div className="flex items-center gap-1 text-sm flex-shrink-0" style={{ color: 'var(--text-secondary)' }}>
          <Star size={14} className="text-yellow-400" fill="currentColor" />
          <span className="font-display font-semibold">{rating.toFixed(1)}</span>
        </div>
      )}
    </Link>
  )
}

// ==================== 海报墙视图项 ====================
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
      className="media-card group block overflow-hidden rounded-lg"
    >
      <div className="relative aspect-[2/3] overflow-hidden bg-theme-bg-surface">
        <img
          src={posterUrl}
          alt={title}
          className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110 group-hover:brightness-110"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        {/* 占位 */}
        <div className="absolute inset-0 -z-10 flex items-center justify-center" style={{ background: 'linear-gradient(180deg, #1a1b2e 0%, #0f1019 100%)' }}>
          {isSeries ? <Tv size={20} style={{ color: '#4a5568' }} /> : <Film size={20} style={{ color: '#4a5568' }} />}
        </div>

        {/* 悬停遮罩 */}
        <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
          <div className="absolute bottom-2 left-2 right-2">
            <h3 className="truncate text-xs font-medium text-white">{title}</h3>
          </div>
          <div className="absolute top-1.5 right-1.5">
            <div
              className="flex h-7 w-7 items-center justify-center rounded-full"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                boxShadow: 'var(--neon-glow-shadow-sm)',
              }}
            >
              <Play size={12} className="ml-0.5 text-white" fill="white" />
            </div>
          </div>
        </div>

        {/* 评分 */}
        {rating > 0 && (
          <span className="absolute left-1.5 top-1.5 flex items-center gap-0.5 rounded-md bg-black/60 px-1.5 py-0.5 text-[10px] text-yellow-400 backdrop-blur-sm">
            <Star size={8} fill="currentColor" />
            {rating.toFixed(1)}
          </span>
        )}

        {/* 剧集标识 */}
        {isSeries && (
          <div className="absolute bottom-1.5 right-1.5 flex h-5 w-5 items-center justify-center rounded-md opacity-0 group-hover:opacity-0"
            style={{ background: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(4px)' }}
          >
            <Tv size={10} className="text-neon" />
          </div>
        )}
      </div>
    </Link>
  )
}
