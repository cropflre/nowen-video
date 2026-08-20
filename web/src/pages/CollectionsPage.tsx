import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react'
import {
  Grid3X3,
  LayoutList,
  Layers,
  Library as LibraryIcon,
  Merge,
  Pencil,
  RefreshCw,
  Search as SearchIcon,
  SlidersHorizontal,
  Sparkles,
  Trash2,
  X,
} from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { collectionApi, libraryApi } from '@/api'
import { usePageCache, invalidatePageCachePrefix } from '@/hooks/usePageCache'
import type { Library, MovieCollection } from '@/types'
import Pagination from '@/components/Pagination'
import CollectionCard from '@/components/media/CollectionCard'
import { Button, EmptyState, SearchField, Select, Surface, Tag } from '@/components/design-system'
import { FilterChip, MediaGrid, SegmentedControl } from '@/ui'

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

const SOURCE_TABS = [
  { key: '', label: '全部', icon: Layers },
  { key: 'true', label: '自动匹配', icon: Sparkles },
  { key: 'false', label: '手动创建', icon: Pencil },
] as const

interface CollectionsData {
  list: MovieCollection[]
  total: number
}

function FilterRow({ icon, label, children }: { icon: ReactNode; label: string; children: ReactNode }) {
  return (
    <div className="nv-search-filter-row flex flex-wrap items-center gap-1.5">
      <span className="mr-1 inline-flex min-w-20 items-center gap-1.5 text-xs font-medium text-[var(--nv-text-tertiary)]">
        {icon}
        {label}
      </span>
      <div className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">{children}</div>
    </div>
  )
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
  const [showFilters, setShowFilters] = useState(false)
  const [operating, setOperating] = useState(false)
  const [operationMsg, setOperationMsg] = useState('')
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

  const handleSearchSubmit = useCallback((event: FormEvent) => {
    event.preventDefault()
    void handleSearch()
  }, [handleSearch])

  const clearSearch = useCallback(() => {
    setSearchKeyword('')
    setSearchResults(null)
  }, [])

  const runMaintenance = useCallback(async (operation: 'rematch' | 'merge' | 'cleanup') => {
    if (operating) return
    setOperating(true)
    setOperationMsg('')
    try {
      if (operation === 'rematch') {
        const res = await collectionApi.rematch()
        setOperationMsg(res.data.message || `重新匹配完成，新建 ${res.data.created} 个合集`)
        setSearchParams((prev) => {
          const next = new URLSearchParams(prev)
          next.set('page', '1')
          return next
        })
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

  const totalPages = searchResults === null ? Math.ceil(total / pageSize) : Math.max(1, Math.ceil(displayList.length / pageSize))
  const hasActiveFilter = filterAuto !== '' || filterLibrary !== '' || sortValue !== 'created_desc' || viewMode !== 'grid'

  const updateParams = useCallback((patch: Record<string, string | null>, resetPage = false) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      Object.entries(patch).forEach(([key, value]) => {
        if (value === null) next.delete(key)
        else next.set(key, value)
      })
      if (resetPage) next.delete('page')
      return next
    }, { replace: true })
  }, [setSearchParams])

  const clearFilters = useCallback(() => {
    updateParams({ auto: null, library_id: null, sort: null, view: null }, true)
  }, [updateParams])

  const handlePageChange = useCallback((nextPage: number) => {
    updateParams({ page: nextPage <= 1 ? null : String(nextPage) })
  }, [updateParams])

  const handlePageSizeChange = useCallback((size: number) => {
    updateParams({ size: String(size) }, true)
  }, [updateParams])

  const emptyTitle = searchResults !== null ? '未找到匹配的合集' : hasActiveFilter ? '没有符合条件的合集' : '暂无影视合集'
  const emptyDescription = searchResults !== null ? '请尝试其他关键词。' : hasActiveFilter ? '尝试调整筛选条件。' : '扫描媒体库后，系统会自动匹配电影系列合集。'
  const resultSummary = loading
    ? '正在加载合集…'
    : searchResults !== null
      ? `“${searchKeyword.trim()}” · ${displayList.length} 个合集结果`
      : `${total} 个合集${filterAuto === 'true' ? ' · 自动匹配' : filterAuto === 'false' ? ' · 手动创建' : ''}`

  return (
    <div className="nv-section-stack nv-library-page nv-search-page nv-collections-page">
      <header className="nv-search-workspace-header">
        <form className="nv-search-workspace-form" role="search" onSubmit={handleSearchSubmit}>
          <div className="nv-search-workspace-field-wrap">
            <SearchField
              value={searchKeyword}
              onChange={(event) => {
                setSearchKeyword(event.target.value)
                if (!event.target.value) setSearchResults(null)
              }}
              placeholder="搜索合集名称"
              aria-label="搜索合集名称"
              enterKeyHint="search"
              autoComplete="off"
              wrapperClassName="nv-search-workspace-field"
            />
          </div>

          {searchKeyword.length > 0 && (
            <Button type="button" variant="ghost" size="md" iconOnly onClick={clearSearch} aria-label="清空搜索">
              <X size={16} aria-hidden="true" />
            </Button>
          )}

          <Button type="submit" variant="primary" size="md" disabled={!searchKeyword.trim()}>
            <SearchIcon size={15} aria-hidden="true" />
            搜索
          </Button>

          <Button
            type="button"
            variant={showFilters || hasActiveFilter ? 'secondary' : 'ghost'}
            size="md"
            onClick={() => setShowFilters((value) => !value)}
            aria-expanded={showFilters}
            aria-controls="collections-filter-panel"
          >
            <SlidersHorizontal size={15} aria-hidden="true" />
            筛选
            {hasActiveFilter && <span className="nv-search-filter-dot" aria-label="已启用筛选" />}
          </Button>
        </form>

        <div className="nv-search-workspace-summary" aria-live="polite">
          <span>{resultSummary}</span>
          <div className="flex flex-wrap items-center justify-end gap-1">
            <Button type="button" variant="ghost" size="sm" onClick={() => runMaintenance('rematch')} loading={operating} title="清除所有自动匹配的合集并重新匹配，手动创建的合集不受影响">
              <RefreshCw size={13} aria-hidden="true" />重新匹配
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={() => runMaintenance('merge')} disabled={operating} title="合并所有同名重复合集，保留最早创建的合集并迁移电影">
              <Merge size={13} aria-hidden="true" />合并重复
            </Button>
            <Button type="button" variant="danger" size="sm" onClick={() => runMaintenance('cleanup')} disabled={operating} title="删除所有没有关联电影的空壳合集">
              <Trash2 size={13} aria-hidden="true" />清理空壳
            </Button>
          </div>
        </div>
      </header>

      {showFilters && (
        <Surface variant="glass" id="collections-filter-panel" className="nv-search-filter-panel space-y-3 p-3 sm:p-4">
          <FilterRow icon={<Layers size={13} aria-hidden="true" />} label="来源:">
            {SOURCE_TABS.map(({ key, label, icon: Icon }) => (
              <FilterChip key={key || 'all'} selected={filterAuto === key} onClick={() => updateParams({ auto: key === '' ? null : key }, true)}>
                <Icon size={12} aria-hidden="true" />
                {label}
              </FilterChip>
            ))}
            <Tag><LibraryIcon size={10} aria-hidden="true" />{total} 个合集</Tag>
          </FilterRow>

          {libraries.length > 1 && (
            <FilterRow icon={<LibraryIcon size={13} aria-hidden="true" />} label="媒体库:">
              <Select
                value={filterLibrary}
                onChange={(event) => updateParams({ library_id: event.target.value || null }, true)}
                aria-label="媒体库"
                className="!w-auto min-w-40"
              >
                <option value="">全部媒体库</option>
                {libraries.map((library) => <option key={library.id} value={library.id}>{library.name}</option>)}
              </Select>
            </FilterRow>
          )}

          <FilterRow icon={<SlidersHorizontal size={13} aria-hidden="true" />} label="排序:">
            <Select
              value={sortValue}
              onChange={(event) => updateParams({ sort: event.target.value === 'created_desc' ? null : event.target.value }, true)}
              aria-label="合集排序"
              className="!w-auto min-w-40"
            >
              {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </Select>
          </FilterRow>

          <FilterRow icon={<Grid3X3 size={13} aria-hidden="true" />} label="视图:">
            <SegmentedControl<ViewMode>
              value={viewMode}
              ariaLabel="合集视图"
              iconOnly
              items={[
                { value: 'grid', label: '网格视图', icon: <Grid3X3 size={14} aria-hidden="true" /> },
                { value: 'list', label: '列表视图', icon: <LayoutList size={14} aria-hidden="true" /> },
              ]}
              onChange={(nextView) => updateParams({ view: nextView === 'grid' ? null : nextView })}
            />
          </FilterRow>

          {hasActiveFilter && (
            <Button variant="ghost" size="sm" onClick={clearFilters}>
              <X size={14} aria-hidden="true" />
              清除筛选
            </Button>
          )}
        </Surface>
      )}

      {operationMsg && (
        <Surface variant="raised" className="nv-collection-notice flex items-center gap-3 px-4 py-3 text-sm text-[var(--nv-text-secondary)]" role="status">
          <Tag tone="brand">操作完成</Tag>
          <span className="min-w-0 flex-1">{operationMsg}</span>
          <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => setOperationMsg('')} aria-label="关闭提示"><X size={14} /></Button>
        </Surface>
      )}

      {loading ? (
        <MediaGrid aria-label="正在加载合集" aria-busy="true">
          {Array.from({ length: 12 }).map((_, index) => (
            <div key={index}>
              <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
              <div className="skeleton mt-2 h-3 w-3/4" />
              <div className="skeleton mt-1.5 h-2.5 w-1/2" />
            </div>
          ))}
        </MediaGrid>
      ) : displayList.length === 0 ? (
        <EmptyState
          className="nv-search-empty-state"
          icon={<LibraryIcon size={24} />}
          title={emptyTitle}
          description={emptyDescription}
          action={hasActiveFilter ? <Button variant="secondary" size="sm" onClick={clearFilters}>清除筛选</Button> : undefined}
        />
      ) : viewMode === 'grid' ? (
        <MediaGrid>
          {displayList.map((collection) => <CollectionCard key={collection.id} collection={collection} />)}
        </MediaGrid>
      ) : (
        <div className="nv-browse-list divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
          {displayList.map((collection) => <CollectionCard key={collection.id} collection={collection} variant="list" />)}
        </div>
      )}

      {searchResults === null && total > 0 && (
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
