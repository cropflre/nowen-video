import { useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { useNavigate, useSearchParams, Link } from 'react-router-dom'
import { collectionApi } from '@/api'
import { streamApi } from '@/api'
import { libraryApi } from '@/api'
import { usePageCache, invalidatePageCachePrefix } from '@/hooks/usePageCache'
import type { MovieCollection, Library } from '@/types'
import Pagination from '@/components/Pagination'
import { Library as LibraryIcon, Search, Film, Loader2, X, Play, Merge, Trash2, RefreshCw, Grid3X3, LayoutList, ArrowUpDown, ChevronDown, Filter } from 'lucide-react'
import clsx from 'clsx'

type ViewMode = 'grid' | 'list'

const SORT_OPTIONS = [
  { value: 'created_desc', label: '最近创建' },
  { value: 'created_asc', label: '最早创建' },
  { value: 'updated_desc', label: '最近更新' },
  { value: 'updated_asc', label: '最早更新' },
  { value: 'name_asc', label: '名称 A-Z' },
  { value: 'name_desc', label: '名称 Z-A' },
  { value: 'count_desc', label: '电影最多' },
  { value: 'count_asc', label: '电影最少' },
] as const

type SortValue = typeof SORT_OPTIONS[number]['value']

interface CollectionsData {
  list: MovieCollection[]
  total: number
}

export default function CollectionsPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const sortRef = useRef<HTMLDivElement>(null)
  const page = Number(searchParams.get('page')) || 1
  const pageSize = Number(searchParams.get('size')) || 24
  const viewMode = (searchParams.get('view') as ViewMode) || 'grid'
  const sortValue = (searchParams.get('sort') as SortValue) || 'created_desc'
  const filterAuto = searchParams.get('auto') || '' // '' | 'true' | 'false'
  const filterLibrary = searchParams.get('library_id') || ''
  const [searchKeyword, setSearchKeyword] = useState('')
  const [searchResults, setSearchResults] = useState<MovieCollection[] | null>(null)
  const [operating, setOperating] = useState(false)
  const [operationMsg, setOperationMsg] = useState('')
  const [showSortDropdown, setShowSortDropdown] = useState(false)
  const [showFilters, setShowFilters] = useState(false)
  const [libraries, setLibraries] = useState<Library[]>([])
  const pageSizeOptions = [12, 24, 36, 48]

  // 加载媒体库列表
  useEffect(() => {
    libraryApi.list().then(res => {
      setLibraries(res.data.data || [])
    }).catch(() => {})
  }, [])

  // 点击外部关闭排序下拉
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (sortRef.current && !sortRef.current.contains(e.target as Node)) {
        setShowSortDropdown(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  // 加载合集列表（排序和筛选全部下沉到后端，保证分页和总数一致）
  // 使用 usePageCache：按"分页/排序/筛选"组合的 key 分桶缓存，跨导航命中时零 loading
  const { data, loading, refetch } = usePageCache<CollectionsData>(
    `collections:list:page=${page}:size=${pageSize}:sort=${sortValue}:auto=${filterAuto}:lib=${filterLibrary}`,
    async () => {
      const res = await collectionApi.list({
        page,
        size: pageSize,
        sort: sortValue,
        auto: filterAuto || undefined,
        library_id: filterLibrary || undefined,
      })
      return { list: res.data.data || [], total: res.data.total || 0 }
    },
    { ttl: 20_000 },
  )
  const collections = data?.list ?? []
  const total = data?.total ?? 0

  // 搜索合集
  const handleSearch = useCallback(async () => {
    if (!searchKeyword.trim()) {
      setSearchResults(null)
      return
    }
    try {
      const res = await collectionApi.search(searchKeyword.trim(), 20)
      setSearchResults(res.data.data || [])
    } catch {
      setSearchResults([])
    }
  }, [searchKeyword])

  // 合并同名重复合集
  const handleMergeDuplicates = useCallback(async () => {
    if (operating) return
    setOperating(true)
    setOperationMsg('')
    try {
      const res = await collectionApi.mergeDuplicates()
      setOperationMsg(res.data.message || `已合并 ${res.data.merged} 组重复合集`)
      invalidatePageCachePrefix('collections:')
      refetch(true)
    } catch {
      setOperationMsg('合并失败，请重试')
    } finally {
      setOperating(false)
    }
  }, [operating, refetch])

  // 清理空壳合集
  const handleCleanupEmpty = useCallback(async () => {
    if (operating) return
    setOperating(true)
    setOperationMsg('')
    try {
      const res = await collectionApi.cleanupEmpty()
      setOperationMsg(res.data.message || `已清理 ${res.data.cleaned} 个空壳合集`)
      invalidatePageCachePrefix('collections:')
      refetch(true)
    } catch {
      setOperationMsg('清理失败，请重试')
    } finally {
      setOperating(false)
    }
  }, [operating, refetch])

  // 重新匹配合集
  const handleRematch = useCallback(async () => {
    if (operating) return
    setOperating(true)
    setOperationMsg('')
    try {
      const res = await collectionApi.rematch()
      setOperationMsg(res.data.message || `重新匹配完成，新建 ${res.data.created} 个合集`)
      setSearchParams(prev => { prev.set('page', '1'); return prev })
      invalidatePageCachePrefix('collections:')
      refetch(true)
    } catch {
      setOperationMsg('重新匹配失败，请重试')
    } finally {
      setOperating(false)
    }
  }, [operating, refetch, setSearchParams])

  // ===== 显示列表 =====
  // 正常浏览模式下，后端已经完成排序和筛选，直接使用 collections
  // 搜索模式下，对搜索结果做客户端筛选（搜索 API 目前未下沉 auto 筛选）
  const displayList = useMemo(() => {
    if (searchResults === null) return collections
    let items = [...searchResults]
    if (filterAuto !== '') {
      const isAuto = filterAuto === 'true'
      items = items.filter(c => c.auto_matched === isAuto)
    }
    return items
  }, [collections, searchResults, filterAuto])

  // 分页总页数：搜索结果按搜索结果数量计算；浏览模式按后端 total 计算（已按筛选条件过滤）
  const totalPages = searchResults === null
    ? Math.ceil(total / pageSize)
    : Math.ceil(displayList.length / pageSize)
  const currentSortLabel = SORT_OPTIONS.find(o => o.value === sortValue)?.label || '排序'
  const hasActiveFilter = filterAuto !== '' || filterLibrary !== ''
  const activeFilterCount = (filterAuto !== '' ? 1 : 0) + (filterLibrary !== '' ? 1 : 0)

  // 切换分页时同步 URL
  const handlePageChange = useCallback((p: number) => {
    setSearchParams(prev => {
      if (p <= 1) prev.delete('page'); else prev.set('page', String(p))
      return prev
    })
  }, [setSearchParams])

  // 切换每页数量时重置到第一页
  const handlePageSizeChange = useCallback((size: number) => {
    setSearchParams(prev => {
      prev.delete('page')
      prev.set('size', String(size))
      return prev
    })
  }, [setSearchParams])

  // 切换视图模式
  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setSearchParams(prev => {
      if (mode === 'grid') prev.delete('view'); else prev.set('view', mode)
      return prev
    })
  }, [setSearchParams])

  // 切换排序
  const handleSortChange = useCallback((value: SortValue) => {
    setSearchParams(prev => {
      if (value === 'created_desc') prev.delete('sort'); else prev.set('sort', value)
      return prev
    })
    setShowSortDropdown(false)
  }, [setSearchParams])

  // 切换筛选
  const handleFilterAuto = useCallback((value: string) => {
    setSearchParams(prev => {
      if (value === '') prev.delete('auto'); else prev.set('auto', value)
      prev.delete('page')
      return prev
    })
  }, [setSearchParams])

  // 切换媒体库筛选
  const handleFilterLibrary = useCallback((value: string) => {
    setSearchParams(prev => {
      if (value === '') prev.delete('library_id'); else prev.set('library_id', value)
      prev.delete('page')
      return prev
    })
  }, [setSearchParams])

  return (
    <div className="space-y-6">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <LibraryIcon size={24} className="text-neon" />
          <h1 className="font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            影视合集
          </h1>
          <span className="rounded-full px-2.5 py-0.5 text-xs font-medium"
            style={{ background: 'var(--neon-blue-10)', color: 'var(--neon-blue)' }}>
            {total} 个合集
          </span>
        </div>
      </div>

      {/* 搜索栏 + 管理操作 */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative flex-1 max-w-md">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-surface-400" />
          <input
            type="text"
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            placeholder="搜索合集名称..."
            className="input w-full pl-9 pr-8"
          />
          {searchKeyword && (
            <button
              onClick={() => { setSearchKeyword(''); setSearchResults(null) }}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-surface-400 hover:text-surface-300"
            >
              <X size={14} />
            </button>
          )}
        </div>
        <button onClick={handleSearch} className="btn-primary px-4 py-2 text-sm">
          搜索
        </button>
        {/* 筛选按钮 */}
        <button
          onClick={() => setShowFilters(!showFilters)}
          className={clsx('flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all')}
          style={{
            border: `1px solid ${hasActiveFilter ? 'var(--border-hover)' : 'var(--border-default)'}`,
            color: hasActiveFilter ? 'var(--neon-blue)' : 'var(--text-secondary)',
            background: hasActiveFilter ? 'var(--nav-active-bg)' : 'transparent',
          }}
        >
          <Filter size={14} />
          筛选
          {hasActiveFilter && (
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
        {/* 排序 */}
        <div className="relative" ref={sortRef}>
          <button
            onClick={() => setShowSortDropdown(!showSortDropdown)}
            className="flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all"
            style={{ border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
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
                  boxShadow: '0 8px 30px rgba(0,0,0,0.4)',
                }}
              >
                {SORT_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    onClick={() => handleSortChange(opt.value)}
                    className={clsx(
                      'w-full px-3 py-2 text-left text-sm transition-colors',
                      sortValue === opt.value
                        ? 'text-neon bg-[var(--nav-active-bg)]'
                        : 'hover:bg-[var(--nav-hover-bg)]',
                    )}
                    style={{ color: sortValue === opt.value ? 'var(--neon-blue)' : 'var(--text-secondary)' }}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </>
          )}
        </div>
        {/* 视图切换 */}
        <div className="flex items-center rounded-lg" style={{ border: '1px solid var(--border-default)' }}>
          <button
            onClick={() => handleViewModeChange('grid')}
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
            onClick={() => handleViewModeChange('list')}
            className="p-2 transition-all"
            style={{
              background: viewMode === 'list' ? 'var(--nav-active-bg)' : 'transparent',
              color: viewMode === 'list' ? 'var(--neon-blue)' : 'var(--text-tertiary)',
            }}
            title="列表视图"
          >
            <LayoutList size={16} />
          </button>
        </div>
        <div className="flex gap-2 ml-auto">
          <button
            onClick={handleRematch}
            disabled={operating}
            className="btn-ghost flex items-center gap-1.5 px-3 py-2 text-xs"
            title="清除所有自动匹配的合集并重新匹配，手动创建的合集不受影响"
          >
            {operating ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
            重新匹配
          </button>
          <button
            onClick={handleMergeDuplicates}
            disabled={operating}
            className="btn-ghost flex items-center gap-1.5 px-3 py-2 text-xs"
            title="合并所有同名重复合集，保留最早创建的合集并将电影迁移过来"
          >
            {operating ? <Loader2 size={14} className="animate-spin" /> : <Merge size={14} />}
            合并重复
          </button>
          <button
            onClick={handleCleanupEmpty}
            disabled={operating}
            className="btn-ghost flex items-center gap-1.5 px-3 py-2 text-xs"
            title="删除所有没有关联电影的空壳合集"
          >
            {operating ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
            清理空壳
          </button>
        </div>
      </div>

      {/* 筛选面板 */}
      {showFilters && (
        <div
          className="flex flex-wrap items-center gap-2 rounded-xl p-3 animate-slide-up"
          style={{
            background: 'var(--nav-hover-bg)',
            border: '1px solid var(--border-default)',
          }}
        >
          <span className="text-xs font-medium mr-1" style={{ color: 'var(--text-tertiary)' }}>来源</span>
          {[
            { value: '', label: '全部' },
            { value: 'true', label: '自动匹配' },
            { value: 'false', label: '手动创建' },
          ].map(opt => (
            <button
              key={opt.value}
              onClick={() => handleFilterAuto(opt.value)}
              className={clsx('rounded-lg px-3 py-1.5 text-xs font-medium transition-all')}
              style={{
                background: filterAuto === opt.value ? 'var(--nav-active-bg)' : 'transparent',
                border: `1px solid ${filterAuto === opt.value ? 'var(--border-hover)' : 'var(--border-default)'}`,
                color: filterAuto === opt.value ? 'var(--neon-blue)' : 'var(--text-secondary)',
              }}
            >
              {opt.label}
            </button>
          ))}
          {hasActiveFilter && (
            <button
              onClick={() => { handleFilterAuto(''); handleFilterLibrary('') }}
              className="ml-2 flex items-center gap-1 text-xs"
              style={{ color: 'var(--text-tertiary)' }}
            >
              <X size={12} />
              清除
            </button>
          )}
          {libraries.length > 0 && (
            <>
              <span className="text-xs font-medium ml-3 mr-1" style={{ color: 'var(--text-tertiary)' }}>媒体库</span>
              {[
                { value: '', label: '全部' },
                ...libraries.map(lib => ({ value: lib.id, label: lib.name })),
              ].map(opt => (
                <button
                  key={opt.value}
                  onClick={() => handleFilterLibrary(opt.value)}
                  className={clsx('rounded-lg px-3 py-1.5 text-xs font-medium transition-all')}
                  style={{
                    background: filterLibrary === opt.value ? 'var(--nav-active-bg)' : 'transparent',
                    border: `1px solid ${filterLibrary === opt.value ? 'var(--border-hover)' : 'var(--border-default)'}`,
                    color: filterLibrary === opt.value ? 'var(--neon-blue)' : 'var(--text-secondary)',
                  }}
                >
                  {opt.label}
                </button>
              ))}
            </>
          )}
        </div>
      )}

      {/* 操作结果提示 */}
      {operationMsg && (
        <div className="flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm"
          style={{ background: 'var(--neon-blue-10)', color: 'var(--neon-blue)', border: '1px solid var(--neon-blue-20)' }}>
          <span>{operationMsg}</span>
          <button onClick={() => setOperationMsg('')} className="ml-auto">
            <X size={14} />
          </button>
        </div>
      )}

      {/* 合集内容 */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 size={24} className="animate-spin text-neon" />
        </div>
      ) : displayList.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <LibraryIcon size={48} className="mb-4 text-surface-600" />
          <p className="text-lg font-medium" style={{ color: 'var(--text-secondary)' }}>
            {searchResults !== null ? '未找到匹配的合集' : hasActiveFilter ? '没有符合条件的合集' : '暂无影视合集'}
          </p>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-tertiary)' }}>
            {searchResults !== null ? '请尝试其他关键词' : hasActiveFilter ? '尝试调整筛选条件' : '扫描媒体库后系统会自动匹配电影系列合集'}
          </p>
        </div>
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {displayList.map((coll) => (
            <CollectionCard
              key={coll.id}
              collection={coll}
              onClick={() => navigate(`/collections/${coll.id}`)}
            />
          ))}
        </div>
      ) : (
        <div className="space-y-2">
          {displayList.map((coll) => (
            <CollectionListItem key={coll.id} collection={coll} />
          ))}
        </div>
      )}

      {/* 分页 */}
      {searchResults === null && (
        <Pagination
          page={page}
          totalPages={totalPages}
          total={total}
          pageSize={pageSize}
          pageSizeOptions={pageSizeOptions}
          onPageChange={handlePageChange}
          onPageSizeChange={handlePageSizeChange}
        />
      )}
    </div>
  )
}

/** 合集卡片 — 点击跳转到合集详情页 */
function CollectionCard({ collection: coll, onClick }: { collection: MovieCollection; onClick: () => void }) {
  return (
    <div
      className="media-card group cursor-pointer overflow-hidden rounded-xl transition-all duration-300 hover:scale-[1.02]"
      style={{ border: '1px solid var(--border-default)' }}
      onClick={onClick}
    >
      {/* 海报 */}
      <div className="relative aspect-[2/3] overflow-hidden" style={{ background: 'var(--bg-surface)' }}>
        <img
          src={streamApi.getCollectionPosterUrl(coll.id)}
          alt={coll.name}
          className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        <div className="absolute inset-0 -z-10 flex items-center justify-center" style={{ color: 'var(--text-muted)' }}>
          <Film size={32} />
        </div>

        {/* 数量标签：角标主显「N 部电影」，右下再叠一行「N 文件」帮助识别多版本合集 */}
        <div className="absolute bottom-2 right-2 flex flex-col items-end gap-1">
          <div className="rounded-md px-2 py-0.5 text-xs font-bold backdrop-blur-md"
            style={{ background: 'rgba(0,0,0,0.7)', color: 'var(--neon-blue)' }}>
            {coll.media_count} 部
          </div>
          {coll.file_count != null && coll.file_count > coll.media_count && (
            <div className="rounded-md px-1.5 py-0.5 text-[10px] font-medium backdrop-blur-md"
              style={{ background: 'rgba(0,0,0,0.6)', color: 'var(--text-tertiary)' }}>
              {coll.file_count} 文件
            </div>
          )}
        </div>

        {/* 来源标签 */}
        {coll.auto_matched && (
          <div className="absolute top-2 left-2 rounded-md px-1.5 py-0.5 text-[10px] font-medium backdrop-blur-md"
            style={{ background: 'rgba(0,0,0,0.6)', color: 'var(--text-tertiary)' }}>
            自动
          </div>
        )}

        {/* 悬停遮罩 */}
        <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
          <div className="absolute bottom-2 left-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-full"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                boxShadow: '0 0 12px var(--neon-blue-40)',
              }}
            >
              <Play size={14} className="ml-0.5 text-white" fill="white" />
            </div>
          </div>
        </div>
      </div>

      {/* 信息 */}
      <div className="p-3">
        <h3 className="truncate text-sm font-semibold transition-colors group-hover:text-neon"
          style={{ color: 'var(--text-primary)' }}>
          {coll.name}
        </h3>
        <div className="mt-1 flex items-center gap-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {coll.year_range && <span>{coll.year_range}</span>}
          {coll.year_range && <span style={{ color: 'var(--text-muted)' }}>·</span>}
          <span>
            {coll.media_count} 部
            {coll.file_count != null && coll.file_count > coll.media_count && (
              <span className="ml-1 opacity-70">/ {coll.file_count} 文件</span>
            )}
          </span>
        </div>
      </div>
    </div>
  )
}

/** 合集列表项 — 列表视图 */
function CollectionListItem({ collection: coll }: { collection: MovieCollection }) {
  return (
    <Link
      to={`/collections/${coll.id}`}
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
          src={streamApi.getCollectionPosterUrl(coll.id)}
          alt={coll.name}
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
          {coll.name}
        </h3>
        <div className="mt-0.5 flex items-center gap-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {coll.year_range && <span>{coll.year_range}</span>}
          {coll.year_range && <span style={{ color: 'var(--text-muted)' }}>·</span>}
          <span className="rounded px-1 py-0.5 text-[10px]" style={{ background: 'var(--nav-hover-bg)' }}>
            {coll.auto_matched ? '自动匹配' : '手动创建'}
          </span>
        </div>
      </div>

      {/* 数量 */}
      <div className="flex items-center gap-1.5 text-xs font-medium flex-shrink-0"
        style={{ color: 'var(--text-secondary)' }}>
        <Film size={14} />
        <span>
          {coll.media_count} 部
          {coll.file_count != null && coll.file_count > coll.media_count && (
            <span className="ml-1 opacity-60">/ {coll.file_count} 文件</span>
          )}
        </span>
      </div>
    </Link>
  )
}
