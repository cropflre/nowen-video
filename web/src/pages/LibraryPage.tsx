import { useEffect, useState, useMemo, useCallback } from 'react'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import { mediaApi, seriesApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { Media, Series, MixedItem } from '@/types'
import MediaCard from '@/components/MediaCard'
import Pagination from '@/components/Pagination'
import {
  Tv,
  Film,
  Star,
  Calendar,
  Search,
  Grid3X3,
  LayoutList,
  ArrowUpDown,
  ChevronDown,
  X,
  Filter,
} from 'lucide-react'
import clsx from 'clsx'

// 排序选项
const SORT_OPTIONS = [
  { value: 'created_desc', label: '最近添加' },
  { value: 'created_asc', label: '最早添加' },
  { value: 'title_asc', label: '名称 A-Z' },
  { value: 'title_desc', label: '名称 Z-A' },
  { value: 'year_desc', label: '年份最新' },
  { value: 'year_asc', label: '年份最早' },
  { value: 'rating_desc', label: '评分最高' },
]

export default function LibraryPage() {
  const { id } = useParams<{ id: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const [mixedItems, setMixedItems] = useState<MixedItem[]>([])
  const [seriesList, setSeriesList] = useState<Series[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [viewTab, setViewTab] = useState<'all' | 'series'>('all')

  // 从 URL 参数读取分页状态
  const page = parseInt(searchParams.get('page') || '1', 10) || 1
  const size = parseInt(searchParams.get('limit') || '30', 10) || 30

  // 筛选与搜索状态
  const [searchQuery, setSearchQuery] = useState('')
  const [sortValue, setSortValue] = useState('created_desc')
  const [showSortDropdown, setShowSortDropdown] = useState(false)
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid')
  const [filterGenre, setFilterGenre] = useState<string | null>(null)
  const [showFilters, setShowFilters] = useState(false)
  const toast = useToast()

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
      params.delete('limit')
    } else {
      params.set('limit', String(newSize))
    }
    params.delete('page')
    setSearchParams(params, { replace: true })
  }, [searchParams, setSearchParams])

  // 切换媒体库时重置状态
  useEffect(() => {
    // 重置分页参数
    const params = new URLSearchParams(searchParams)
    params.delete('page')
    setSearchParams(params, { replace: true })
    setLoading(true)
    setSearchQuery('')
    setFilterGenre(null)
  }, [id])

  useEffect(() => {
    if (!id) return
    setLoading(true)

    Promise.all([
      mediaApi.listMixed({ page, size, library_id: id }),
      seriesApi.list({ library_id: id }),
    ])
      .then(([mixedRes, seriesRes]) => {
        setMixedItems(mixedRes.data.data || [])
        setTotal(mixedRes.data.total)
        setSeriesList(seriesRes.data.data || [])
      })
      .catch(() => { toast.error('加载媒体库内容失败') })
      .finally(() => setLoading(false))
  }, [id, page, size])

  const totalPages = Math.ceil(total / size)
  const hasSeries = seriesList.length > 0

  // 从混合列表和系列列表中提取类型标签
  const allGenres = useMemo(() => {
    const genres = new Set<string>()
    // 从混合列表提取
    mixedItems.forEach((item) => {
      const g = item.type === 'series' ? item.series?.genres : item.media?.genres
      if (g) {
        g.split(',').forEach((genre) => {
          const trimmed = genre.trim()
          if (trimmed) genres.add(trimmed)
        })
      }
    })
    // 从系列列表提取（确保系列视图下也有分类标签可用）
    seriesList.forEach((s) => {
      if (s.genres) {
        s.genres.split(',').forEach((genre) => {
          const trimmed = genre.trim()
          if (trimmed) genres.add(trimmed)
        })
      }
    })
    return Array.from(genres).sort()
  }, [mixedItems, seriesList])

  // 辅助函数：获取混合项的属性
  const getItemTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.title || '') : (item.media?.title || '')
  const getItemOrigTitle = (item: MixedItem) => item.type === 'series' ? (item.series?.orig_title || '') : (item.media?.orig_title || '')
  const getItemOverview = (item: MixedItem) => item.type === 'series' ? (item.series?.overview || '') : (item.media?.overview || '')
  const getItemGenres = (item: MixedItem) => item.type === 'series' ? (item.series?.genres || '') : (item.media?.genres || '')
  const getItemYear = (item: MixedItem) => item.type === 'series' ? (item.series?.year || 0) : (item.media?.year || 0)
  const getItemRating = (item: MixedItem) => item.type === 'series' ? (item.series?.rating || 0) : (item.media?.rating || 0)
  const getItemTime = (item: MixedItem) => item.type === 'series' ? (item.series?.created_at || '') : (item.media?.created_at || '')

  // 筛选和排序后的混合列表
  const filteredMixed = useMemo(() => {
    let items = [...mixedItems]

    // 搜索
    if (searchQuery.trim()) {
      const q = searchQuery.trim().toLowerCase()
      items = items.filter((item) =>
        getItemTitle(item).toLowerCase().includes(q) ||
        getItemOrigTitle(item).toLowerCase().includes(q) ||
        getItemOverview(item).toLowerCase().includes(q)
      )
    }

    // 类型筛选
    if (filterGenre) {
      items = items.filter((item) => getItemGenres(item).includes(filterGenre))
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
  }, [mixedItems, searchQuery, filterGenre, sortValue])

  // 筛选后的剧集
  // 对同名系列去重：标准化标题相同的多个 Series 只展示元数据最丰富的那个
  const deduplicatedSeries = useMemo(() => {
    const normalize = (title: string) => {
      return title
        .replace(/\s*S\d{1,2}\s*$/i, '')
        .replace(/\s*Season\s*\d{1,2}\s*$/i, '')
        .replace(/\s*第\s*[一二三四五六七八九十\d]+\s*季\s*$/, '')
        .replace(/\s*第\s*[一二三四五六七八九十\d]+\s*部\s*$/, '')
        .replace(/\s*[\(（]\s*Season\s*\d{1,2}\s*[\)）]\s*$/i, '')
        .replace(/\s*【\s*第?\s*[一二三四五六七八九十\d]+\s*季?\s*】\s*$/, '')
        .trim() || title
    }
    const groups = new Map<string, { best: typeof seriesList[0]; totalSeasons: number; totalEps: number }>()
    const order: string[] = []
    for (const s of seriesList) {
      const key = `${s.library_id}:${normalize(s.title)}`
      const existing = groups.get(key)
      if (existing) {
        existing.totalSeasons += s.season_count
        existing.totalEps += s.episode_count
        // 选择元数据更丰富的作为代表
        const score = (s2: typeof s) => (s2.overview ? 3 : 0) + (s2.poster_path ? 3 : 0) + (s2.rating > 0 ? 2 : 0) + (s2.tmdb_id > 0 ? 2 : 0) + s2.episode_count
        if (score(s) > score(existing.best)) {
          existing.best = s
        }
      } else {
        groups.set(key, { best: s, totalSeasons: s.season_count, totalEps: s.episode_count })
        order.push(key)
      }
    }
    return order.map(key => {
      const g = groups.get(key)!
      return { ...g.best, season_count: g.totalSeasons, episode_count: g.totalEps }
    })
  }, [seriesList])

  const filteredSeries = useMemo(() => {
    let items = [...deduplicatedSeries]

    // 搜索
    if (searchQuery.trim()) {
      const q = searchQuery.trim().toLowerCase()
      items = items.filter(
        (s) =>
          s.title.toLowerCase().includes(q) ||
          s.orig_title?.toLowerCase().includes(q) ||
          s.overview?.toLowerCase().includes(q)
      )
    }

    // 分类筛选
    if (filterGenre) {
      items = items.filter((s) => (s.genres || '').includes(filterGenre))
    }

    // 排序
    const [field, dir] = sortValue.split('_')
    items.sort((a, b) => {
      let cmp = 0
      if (field === 'title') cmp = a.title.localeCompare(b.title)
      else if (field === 'year') cmp = (a.year || 0) - (b.year || 0)
      else if (field === 'rating') cmp = (a.rating || 0) - (b.rating || 0)
      else cmp = new Date(a.created_at || '').getTime() - new Date(b.created_at || '').getTime()
      return dir === 'desc' ? -cmp : cmp
    })

    return items
  }, [deduplicatedSeries, searchQuery, filterGenre, sortValue])

  const currentSortLabel = SORT_OPTIONS.find((o) => o.value === sortValue)?.label || '排序'

  return (
    <div>
      {/* ===== 工具栏 ===== */}
      <div className="mb-6 space-y-4">
        {/* 第一行：标签切换 + 搜索 */}
        <div className="flex flex-wrap items-center gap-3">
          {/* 视图切换标签 */}
          <div className="flex items-center gap-1">
            <button
              onClick={() => setViewTab('all')}
              className={clsx(
                'flex items-center gap-1.5 rounded-xl px-4 py-2 text-sm font-semibold transition-all duration-300'
              )}
              style={viewTab === 'all' ? {
                background: 'var(--nav-active-bg)',
                border: '1px solid var(--border-hover)',
                color: 'var(--neon-blue)',
                boxShadow: 'var(--shadow-neon)',
              } : {
                border: '1px solid transparent',
                color: 'var(--text-secondary)',
              }}
            >
              <Film size={14} />
              全部内容
            </button>
            {hasSeries && (
              <button
                onClick={() => setViewTab('series')}
                className={clsx(
                  'flex items-center gap-1.5 rounded-xl px-4 py-2 text-sm font-semibold transition-all duration-300'
                )}
                style={viewTab === 'series' ? {
                  background: 'var(--nav-active-bg)',
                  border: '1px solid var(--border-hover)',
                  color: 'var(--neon-blue)',
                  boxShadow: 'var(--shadow-neon)',
                } : {
                  border: '1px solid transparent',
                  color: 'var(--text-secondary)',
                }}
              >
                <Tv size={14} />
                剧集合集 ({seriesList.length})
              </button>
            )}
          </div>

          {/* 搜索框 */}
          <div className="relative ml-auto flex-1 max-w-xs">
            <Search
              size={16}
              className="absolute left-3 top-1/2 -translate-y-1/2"
              style={{ color: 'var(--text-muted)' }}
            />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input pl-9 pr-8 py-2 text-sm"
              placeholder="搜索此媒体库..."
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

          {/* 排序下拉 */}
          <div className="relative">
            <button
              onClick={() => setShowSortDropdown(!showSortDropdown)}
              className="flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all"
              style={{
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
              }}
            >
              <ArrowUpDown size={14} />
              {currentSortLabel}
              <ChevronDown size={12} />
            </button>
            {showSortDropdown && (
              <>
                <div className="fixed inset-0 z-30" onClick={() => setShowSortDropdown(false)} />
                <div
                  className="absolute right-0 top-full z-40 mt-1 w-40 overflow-hidden rounded-xl py-1 animate-slide-up"
                  style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--border-strong)',
                    boxShadow: 'var(--shadow-elevated)',
                  }}
                >
                  {SORT_OPTIONS.map((opt) => (
                    <button
                      key={opt.value}
                      onClick={() => {
                        setSortValue(opt.value)
                        setShowSortDropdown(false)
                      }}
                      className={clsx(
                        'w-full px-3 py-2 text-left text-sm transition-colors',
                        sortValue === opt.value
                          ? 'text-neon bg-[var(--nav-active-bg)]'
                          : 'hover:bg-[var(--nav-hover-bg)]'
                      )}
                      style={sortValue !== opt.value ? { color: 'var(--text-secondary)' } : undefined}
                    >
                      {opt.label}
                    </button>
                  ))}
                </div>
              </>
            )}
          </div>

          {/* 筛选按钮 */}
          {allGenres.length > 0 && (
            <button
              onClick={() => setShowFilters(!showFilters)}
              className={clsx(
                'flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all',
                filterGenre && 'text-neon'
              )}
              style={{
                border: `1px solid ${filterGenre ? 'var(--border-hover)' : 'var(--border-default)'}`,
                color: filterGenre ? 'var(--neon-blue)' : 'var(--text-secondary)',
                background: filterGenre ? 'var(--nav-active-bg)' : 'transparent',
              }}
            >
              <Filter size={14} />
              筛选
              {filterGenre && (
                <span
                  className="ml-1 rounded-full px-1.5 text-[10px] font-bold"
                  style={{
                    background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                    color: 'var(--text-on-neon)',
                  }}
                >
                  1
                </span>
              )}
            </button>
          )}

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
            >
              <LayoutList size={16} />
            </button>
          </div>
        </div>

        {/* 类型筛选标签行 */}
        {showFilters && allGenres.length > 0 && (
          <div
            className="flex flex-wrap items-center gap-2 rounded-xl p-3 animate-slide-up"
            style={{
              background: 'var(--nav-hover-bg)',
              border: '1px solid var(--border-default)',
            }}
          >
            <span className="text-xs font-medium" style={{ color: 'var(--text-tertiary)' }}>
              类型:
            </span>
            <button
              onClick={() => setFilterGenre(null)}
              className={clsx(
                'rounded-lg px-2.5 py-1 text-xs font-medium transition-all',
                !filterGenre && 'text-neon'
              )}
              style={{
                background: !filterGenre ? 'var(--nav-active-bg)' : 'transparent',
                border: `1px solid ${!filterGenre ? 'var(--border-hover)' : 'transparent'}`,
                color: filterGenre ? 'var(--text-secondary)' : undefined,
              }}
            >
              全部
            </button>
            {allGenres.map((genre) => (
              <button
                key={genre}
                onClick={() => setFilterGenre(filterGenre === genre ? null : genre)}
                className={clsx(
                  'rounded-lg px-2.5 py-1 text-xs font-medium transition-all',
                  filterGenre === genre && 'text-neon'
                )}
                style={{
                  background: filterGenre === genre ? 'var(--nav-active-bg)' : 'transparent',
                  border: `1px solid ${filterGenre === genre ? 'var(--border-hover)' : 'transparent'}`,
                  color: filterGenre !== genre ? 'var(--text-secondary)' : undefined,
                }}
              >
                {genre}
              </button>
            ))}
          </div>
        )}

        {/* 搜索结果提示 */}
        {(searchQuery || filterGenre) && (
          <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--text-tertiary)' }}>
            <span>
              找到 <strong className="text-neon">{viewTab === 'all' ? filteredMixed.length : filteredSeries.length}</strong> 个结果
            </span>
            {(searchQuery || filterGenre) && (
              <button
                onClick={() => { setSearchQuery(''); setFilterGenre(null) }}
                className="flex items-center gap-1 rounded-lg px-2 py-1 text-xs transition-colors hover:bg-[var(--nav-hover-bg)]"
                style={{ color: 'var(--text-secondary)' }}
              >
                <X size={12} />
                清除筛选
              </button>
            )}
          </div>
        )}
      </div>

      {/* ===== 全部内容视图 ===== */}
      {viewTab === 'all' && (
        <>
          {viewMode === 'grid' ? (
            // 网格视图
            loading ? (
              <div className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
                {Array.from({ length: 12 }).map((_, i) => (
                  <div key={i}>
                    <div className="skeleton aspect-[2/3] rounded-xl" />
                    <div className="skeleton mt-2 h-4 w-3/4 rounded" />
                    <div className="skeleton mt-1 h-3 w-1/2 rounded" />
                  </div>
                ))}
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-x-4 gap-y-6 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 animate-fade-in">
                {filteredMixed.map((item) => {
                  if (item.type === 'series' && item.series) {
                    return <MediaCard key={`s-${item.series.id}`} series={item.series} />
                  }
                  if (item.media) {
                    return <MediaCard key={`m-${item.media.id}`} media={item.media} />
                  }
                  return null
                })}
              </div>
            )
          ) : (
            // 列表视图
            <div className="space-y-2 animate-fade-in">
              {filteredMixed.map((item) => {
                if (item.type === 'series' && item.series) {
                  return <ListSeriesItem key={`s-${item.series.id}`} series={item.series} />
                }
                if (item.media) {
                  return <ListMediaItem key={`m-${item.media.id}`} media={item.media} />
                }
                return null
              })}
            </div>
          )}

          {/* 分页 */}
          <Pagination
            page={page}
            totalPages={totalPages}
            total={total}
            pageSize={size}
            pageSizeOptions={[20, 30, 50, 100]}
            onPageChange={setPage}
            onPageSizeChange={setSize}
          />

          {/* 空状态 */}
          {!loading && filteredMixed.length === 0 && (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <Film size={48} className="mb-4 text-surface-700" />
              <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
                {searchQuery || filterGenre ? '没有找到匹配的内容' : '此媒体库暂无内容'}
              </p>
            </div>
          )}
        </>
      )}

      {/* ===== 剧集合集视图 ===== */}
      {viewTab === 'series' && (
        <>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {filteredSeries.map((series) => (
              <SeriesCard key={series.id} series={series} />
            ))}
          </div>
          {!loading && filteredSeries.length === 0 && (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <Tv size={48} className="mb-4 text-surface-700" />
              <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
                {searchQuery || filterGenre ? '没有找到匹配的剧集' : '此媒体库暂无剧集合集'}
              </p>
            </div>
          )}
        </>
      )}
    </div>
  )
}

// 列表视图的媒体项
function ListMediaItem({ media }: { media: Media }) {
  const formatDuration = (seconds: number) => {
    if (!seconds) return ''
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    if (h > 0) return `${h}h ${m}m`
    return `${m}m`
  }

  return (
    <Link
      to={media.series_id ? `/series/${media.series_id}` : `/media/${media.id}`}
      className="group flex items-center gap-4 rounded-xl p-3 transition-all duration-300"
      style={{ border: '1px solid var(--border-default)' }}
      onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--nav-hover-bg)'; e.currentTarget.style.borderColor = 'var(--border-hover)' }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.borderColor = 'var(--border-default)' }}
    >
      {/* 缩略图 */}
      <div
        className="h-16 w-12 flex-shrink-0 overflow-hidden rounded-lg"
        style={{ background: 'var(--bg-surface)' }}
      >
        <img
          src={streamApi.getPosterUrl(media.id)}
          alt={media.title}
          className="h-full w-full object-cover"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
      </div>

      {/* 信息 */}
      <div className="min-w-0 flex-1">
        <h3
          className="truncate text-sm font-medium transition-colors group-hover:text-neon"
          style={{ color: 'var(--text-primary)' }}
        >
          {media.title}
        </h3>
        <div className="mt-0.5 flex items-center gap-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {media.year > 0 && <span>{media.year}</span>}
          {media.duration > 0 && (
            <>
              <span style={{ color: 'var(--text-muted)' }}>·</span>
              <span>{formatDuration(media.duration)}</span>
            </>
          )}
          {media.resolution && (
            <>
              <span style={{ color: 'var(--text-muted)' }}>·</span>
              <span className="badge-neon text-[10px] px-1.5 py-0">{media.resolution}</span>
            </>
          )}
        </div>
      </div>

      {/* 评分 */}
      {media.rating > 0 && (
        <div className="flex items-center gap-1 text-sm" style={{ color: 'var(--text-secondary)' }}>
          <Star size={14} className="text-yellow-400" fill="currentColor" />
          <span className="font-display font-semibold">{media.rating.toFixed(1)}</span>
        </div>
      )}
    </Link>
  )
}

// 列表视图的合集项
function ListSeriesItem({ series }: { series: Series }) {
  return (
    <Link
      to={`/series/${series.id}`}
      className="group flex items-center gap-4 rounded-xl p-3 transition-all duration-300"
      style={{ border: '1px solid var(--border-default)' }}
      onMouseEnter={(e) => { e.currentTarget.style.background = 'var(--nav-hover-bg)'; e.currentTarget.style.borderColor = 'var(--border-hover)' }}
      onMouseLeave={(e) => { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.borderColor = 'var(--border-default)' }}
    >
      {/* 缩略图 */}
      <div
        className="h-16 w-12 flex-shrink-0 overflow-hidden rounded-lg"
        style={{ background: 'var(--bg-surface)' }}
      >
        {series.poster_path ? (
          <img
            src={streamApi.getSeriesPosterUrl(series.id)}
            alt={series.title}
            className="h-full w-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-surface-700">
            <Tv size={16} />
          </div>
        )}
      </div>

      {/* 信息 */}
      <div className="min-w-0 flex-1">
        <h3
          className="truncate text-sm font-medium transition-colors group-hover:text-neon"
          style={{ color: 'var(--text-primary)' }}
        >
          {series.title}
        </h3>
        <div className="mt-0.5 flex items-center gap-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {series.year > 0 && <span>{series.year}</span>}
          <span style={{ color: 'var(--text-muted)' }}>·</span>
          <span>{series.season_count} 季 · {series.episode_count} 集</span>
        </div>
      </div>

      {/* 评分 */}
      {series.rating > 0 && (
        <div className="flex items-center gap-1 text-sm" style={{ color: 'var(--text-secondary)' }}>
          <Star size={14} className="text-yellow-400" fill="currentColor" />
          <span className="font-display font-semibold">{series.rating.toFixed(1)}</span>
        </div>
      )}
    </Link>
  )
}

// 剧集合集卡片
function SeriesCard({ series }: { series: Series }) {
  return (
    <Link
      to={`/series/${series.id}`}
      className="media-card group block overflow-hidden rounded-xl"
    >
      {/* 海报 */}
      <div className="relative aspect-video overflow-hidden" style={{ background: 'var(--bg-surface)' }}>
        {series.poster_path ? (
          <img
            src={streamApi.getSeriesPosterUrl(series.id)}
            alt={series.title}
            className="h-full w-full object-cover transition-all duration-500 group-hover:scale-105 group-hover:brightness-110"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-surface-700">
            <Tv size={48} />
          </div>
        )}
        {/* 集数标签 */}
        <div className="badge-neon absolute bottom-2 right-2">
          {series.season_count} 季 · {series.episode_count} 集
        </div>
      </div>
      {/* 信息 */}
      <div className="p-3">
        <h3 className="truncate text-sm font-semibold transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
          {series.title}
        </h3>
        <div className="mt-1 flex items-center gap-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {series.year > 0 && (
            <span className="flex items-center gap-1">
              <Calendar size={12} />
              {series.year}
            </span>
          )}
          {series.rating > 0 && (
            <span className="flex items-center gap-1 text-yellow-400">
              <Star size={12} fill="currentColor" />
              {series.rating.toFixed(1)}
            </span>
          )}
        </div>
        {series.overview && (
          <p className="mt-1.5 line-clamp-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>{series.overview}</p>
        )}
      </div>
    </Link>
  )
}
