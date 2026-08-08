import { useCallback, useEffect, useMemo, useState } from 'react'
import { Grid3X3, LayoutList, Library as LibraryIcon, Merge, RefreshCw, Search, SlidersHorizontal, Trash2, X } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { collectionApi, libraryApi } from '@/api'
import { usePageCache, invalidatePageCachePrefix } from '@/hooks/usePageCache'
import type { Library, MovieCollection } from '@/types'
import Pagination from '@/components/Pagination'
import CollectionCard from '@/components/media/CollectionCard'
import { Button, EmptyState, Input, PageContainer, Select, Surface, Tag } from '@/components/design-system'

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
  const [searchParams, setSearchParams] = useSearchParams()
  const page = Number(searchParams.get('page')) || 1
  const pageSize = Number(searchParams.get('size')) || 24
  const viewMode = (searchParams.get('view') as ViewMode) || 'grid'
  const sortValue = (searchParams.get('sort') as SortValue) || 'created_desc'
  const filterAuto = searchParams.get('auto') || ''
  const filterLibrary = searchParams.get('library_id') || ''
  const [searchKeyword, setSearchKeyword] = useState('')
  const [searchResults, setSearchResults] = useState<MovieCollection[] | null>(null)
  const [operating, setOperating] = useState(false)
  const [operationMsg, setOperationMsg] = useState('')
  const [showFilters, setShowFilters] = useState(false)
  const [libraries, setLibraries] = useState<Library[]>([])
  const pageSizeOptions = [12, 24, 36, 48]

  useEffect(() => {
    libraryApi.list().then((res) => setLibraries(res.data.data || [])).catch(() => {})
  }, [])

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

  const runMaintenance = useCallback(async (operation: 'rematch' | 'merge' | 'cleanup') => {
    if (operating) return
    setOperating(true)
    setOperationMsg('')
    try {
      if (operation === 'rematch') {
        const res = await collectionApi.rematch()
        setOperationMsg(res.data.message || `重新匹配完成，新建 ${res.data.created} 个合集`)
        setSearchParams((prev) => { prev.set('page', '1'); return prev })
      } else if (operation === 'merge') {
        const res = await collectionApi.mergeDuplicates()
        setOperationMsg(res.data.message || `已合并 ${res.data.merged} 组重复合集`)
      } else {
        const res = await collectionApi.cleanupEmpty()
        setOperationMsg(res.data.message || `已清理 ${res.data.cleaned} 个空壳合集`)
      }
      invalidatePageCachePrefix('collections:')
      refetch(true)
    } catch {
      setOperationMsg(operation === 'rematch' ? '重新匹配失败，请重试' : operation === 'merge' ? '合并失败，请重试' : '清理失败，请重试')
    } finally {
      setOperating(false)
    }
  }, [operating, refetch, setSearchParams])

  const displayList = useMemo(() => {
    if (searchResults === null) return collections
    let items = [...searchResults]
    if (filterAuto !== '') items = items.filter((collection) => collection.auto_matched === (filterAuto === 'true'))
    return items
  }, [collections, filterAuto, searchResults])

  const totalPages = searchResults === null ? Math.ceil(total / pageSize) : Math.ceil(displayList.length / pageSize)
  const hasActiveFilter = filterAuto !== '' || filterLibrary !== ''
  const activeFilterCount = (filterAuto !== '' ? 1 : 0) + (filterLibrary !== '' ? 1 : 0)

  const handlePageChange = useCallback((nextPage: number) => {
    setSearchParams((prev) => {
      if (nextPage <= 1) prev.delete('page')
      else prev.set('page', String(nextPage))
      return prev
    })
  }, [setSearchParams])

  const handlePageSizeChange = useCallback((size: number) => {
    setSearchParams((prev) => {
      prev.delete('page')
      prev.set('size', String(size))
      return prev
    })
  }, [setSearchParams])

  const handleViewModeChange = useCallback((mode: ViewMode) => {
    setSearchParams((prev) => {
      if (mode === 'grid') prev.delete('view')
      else prev.set('view', mode)
      return prev
    })
  }, [setSearchParams])

  const handleSortChange = useCallback((value: SortValue) => {
    setSearchParams((prev) => {
      if (value === 'created_desc') prev.delete('sort')
      else prev.set('sort', value)
      return prev
    })
  }, [setSearchParams])

  const handleFilterAuto = useCallback((value: string) => {
    setSearchParams((prev) => {
      if (value === '') prev.delete('auto')
      else prev.set('auto', value)
      prev.delete('page')
      return prev
    })
  }, [setSearchParams])

  const handleFilterLibrary = useCallback((value: string) => {
    setSearchParams((prev) => {
      if (value === '') prev.delete('library_id')
      else prev.set('library_id', value)
      prev.delete('page')
      return prev
    })
  }, [setSearchParams])

  const clearFilters = () => {
    setSearchParams((prev) => {
      prev.delete('auto')
      prev.delete('library_id')
      prev.delete('page')
      return prev
    })
  }

  const emptyTitle = searchResults !== null ? '未找到匹配的合集' : hasActiveFilter ? '没有符合条件的合集' : '暂无影视合集'
  const emptyDescription = searchResults !== null ? '请尝试其他关键词。' : hasActiveFilter ? '尝试调整筛选条件。' : '扫描媒体库后，系统会自动匹配电影系列合集。'

  return (
    <PageContainer width="wide" className="space-y-6">
      <header className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="flex h-10 w-10 items-center justify-center rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
              <LibraryIcon size={20} aria-hidden="true" />
            </div>
            <div>
              <h1 className="text-2xl font-bold tracking-[-0.02em] text-[var(--nv-text-primary)]">影视合集</h1>
              <p className="mt-1 text-sm text-[var(--nv-text-tertiary)]">浏览自动匹配与手动创建的电影系列合集。</p>
            </div>
            <Tag tone="brand">{total} 个合集</Tag>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="secondary" size="sm" onClick={() => runMaintenance('rematch')} loading={operating} title="清除所有自动匹配的合集并重新匹配，手动创建的合集不受影响">
            <RefreshCw size={14} />重新匹配
          </Button>
          <Button type="button" variant="secondary" size="sm" onClick={() => runMaintenance('merge')} disabled={operating} title="合并所有同名重复合集，保留最早创建的合集并迁移电影">
            <Merge size={14} />合并重复
          </Button>
          <Button type="button" variant="danger" size="sm" onClick={() => runMaintenance('cleanup')} disabled={operating} title="删除所有没有关联电影的空壳合集">
            <Trash2 size={14} />清理空壳
          </Button>
        </div>
      </header>

      <Surface className="p-4">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-center">
          <div className="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row">
            <div className="relative min-w-0 flex-1 sm:max-w-md">
              <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--nv-text-tertiary)]" />
              <Input
                value={searchKeyword}
                onChange={(event) => setSearchKeyword(event.target.value)}
                onKeyDown={(event) => event.key === 'Enter' && handleSearch()}
                placeholder="搜索合集名称..."
                className="pl-9 pr-9"
              />
              {searchKeyword && (
                <button type="button" onClick={() => { setSearchKeyword(''); setSearchResults(null) }} className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded p-1 text-[var(--nv-text-tertiary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]" aria-label="清除搜索">
                  <X size={14} />
                </button>
              )}
            </div>
            <Button type="button" variant="primary" onClick={handleSearch}><Search size={15} />搜索</Button>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Button type="button" variant={hasActiveFilter ? 'secondary' : 'ghost'} onClick={() => setShowFilters((show) => !show)} aria-expanded={showFilters}>
              <SlidersHorizontal size={15} />筛选
              {hasActiveFilter && <Tag tone="brand">{activeFilterCount}</Tag>}
            </Button>
            <Select value={sortValue} onChange={(event) => handleSortChange(event.target.value as SortValue)} aria-label="合集排序">
              {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </Select>
            <div className="flex items-center gap-1 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-1">
              <Button type="button" variant={viewMode === 'grid' ? 'secondary' : 'ghost'} size="sm" iconOnly onClick={() => handleViewModeChange('grid')} aria-label="网格视图" title="网格视图"><Grid3X3 size={15} /></Button>
              <Button type="button" variant={viewMode === 'list' ? 'secondary' : 'ghost'} size="sm" iconOnly onClick={() => handleViewModeChange('list')} aria-label="列表视图" title="列表视图"><LayoutList size={15} /></Button>
            </div>
          </div>
        </div>

        {showFilters && (
          <div className="mt-4 grid gap-4 border-t border-[var(--nv-border-subtle)] pt-4 lg:grid-cols-2">
            <FilterGroup label="来源" value={filterAuto} options={[{ value: '', label: '全部' }, { value: 'true', label: '自动匹配' }, { value: 'false', label: '手动创建' }]} onChange={handleFilterAuto} />
            {libraries.length > 0 && (
              <FilterGroup label="媒体库" value={filterLibrary} options={[{ value: '', label: '全部' }, ...libraries.map((library) => ({ value: library.id, label: library.name }))]} onChange={handleFilterLibrary} />
            )}
            {hasActiveFilter && <div className="lg:col-span-2"><Button type="button" variant="ghost" size="sm" onClick={clearFilters}><X size={13} />清除筛选</Button></div>}
          </div>
        )}
      </Surface>

      {operationMsg && (
        <Surface className="flex items-center gap-3 border-[var(--nv-action-primary)]/30 bg-[var(--nv-bg-active)] px-4 py-3 text-sm text-[var(--nv-text-secondary)]" role="status">
          <Tag tone="brand">操作完成</Tag>
          <span className="min-w-0 flex-1">{operationMsg}</span>
          <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => setOperationMsg('')} aria-label="关闭提示"><X size={14} /></Button>
        </Surface>
      )}

      {loading ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6" aria-label="正在加载合集">
          {Array.from({ length: 12 }).map((_, index) => <div key={index} className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />)}
        </div>
      ) : displayList.length === 0 ? (
        <Surface><EmptyState icon={<LibraryIcon size={24} />} title={emptyTitle} description={emptyDescription} /></Surface>
      ) : viewMode === 'grid' ? (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {displayList.map((collection) => <CollectionCard key={collection.id} collection={collection} />)}
        </div>
      ) : (
        <div className="space-y-2">
          {displayList.map((collection) => <CollectionCard key={collection.id} collection={collection} variant="list" />)}
        </div>
      )}

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
    </PageContainer>
  )
}

function FilterGroup({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: { value: string; label: string }[]
  onChange: (value: string) => void
}) {
  return (
    <div className="space-y-2">
      <div className="text-xs font-medium text-[var(--nv-text-tertiary)]">{label}</div>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => {
          const active = value === option.value
          return (
            <button
              key={option.value}
              type="button"
              onClick={() => onChange(option.value)}
              className={`rounded-[var(--nv-radius-control)] border px-3 py-1.5 text-xs font-medium transition-colors ${active ? 'border-[var(--nv-action-primary)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]' : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] text-[var(--nv-text-secondary)] hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)]'}`}
              aria-pressed={active}
            >
              {option.label}
            </button>
          )
        })}
      </div>
    </div>
  )
}
