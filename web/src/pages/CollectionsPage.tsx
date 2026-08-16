import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { Grid3X3, LayoutList, Layers, Library as LibraryIcon, Merge, Pencil, RefreshCw, Sparkles, Trash2, X } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { collectionApi, libraryApi } from '@/api'
import { usePageCache, invalidatePageCachePrefix } from '@/hooks/usePageCache'
import type { Library, MovieCollection } from '@/types'
import Pagination from '@/components/Pagination'
import CollectionCard from '@/components/media/CollectionCard'
import { Button, EmptyState, SearchField, Select, Surface, Tag } from '@/components/design-system'

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

/** 来源是合集的主分类轴，提升为标签行，与影视库的媒体类型标签保持一致 */
const SOURCE_TABS = [
  { key: '', label: '全部', icon: Layers },
  { key: 'true', label: '自动匹配', icon: Sparkles },
  { key: 'false', label: '手动创建', icon: Pencil },
] as const

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

  const updateParams = useCallback((patch: Record<string, string | null>, resetPage = false) => {
    setSearchParams((prev) => {
      Object.entries(patch).forEach(([key, value]) => {
        if (value === null) prev.delete(key)
        else prev.set(key, value)
      })
      if (resetPage) prev.delete('page')
      return prev
    })
  }, [setSearchParams])

  const handlePageChange = useCallback((nextPage: number) => {
    updateParams({ page: nextPage <= 1 ? null : String(nextPage) })
  }, [updateParams])

  const handlePageSizeChange = useCallback((size: number) => {
    updateParams({ size: String(size) }, true)
  }, [updateParams])

  const emptyTitle = searchResults !== null ? '未找到匹配的合集' : hasActiveFilter ? '没有符合条件的合集' : '暂无影视合集'
  const emptyDescription = searchResults !== null ? '请尝试其他关键词。' : hasActiveFilter ? '尝试调整筛选条件。' : '扫描媒体库后，系统会自动匹配电影系列合集。'

  return (
    <div className="nv-section-stack">
      <div className="nv-browse-type-tabs flex flex-wrap items-center gap-1 border-b border-[var(--nv-border-subtle)] pb-3" aria-label="合集来源">
        {SOURCE_TABS.map(({ key, label, icon: Icon }) => {
          const selected = filterAuto === key
          return (
            <button
              key={key || 'all'}
              type="button"
              onClick={() => updateParams({ auto: key === '' ? null : key }, true)}
              aria-pressed={selected}
              className="nv-button"
              data-variant={selected ? 'secondary' : 'ghost'}
              data-size="sm"
            >
              <Icon size={14} aria-hidden="true" />
              <span>{label}</span>
            </button>
          )
        })}
        <Tag className="ml-1"><LibraryIcon size={10} aria-hidden="true" />{total} 个合集</Tag>
      </div>

      <div className="nv-browse-toolbar flex flex-wrap items-center gap-1.5">
        {libraries.length > 1 && (
          <Select
            value={filterLibrary}
            onChange={(event) => updateParams({ library_id: event.target.value || null }, true)}
            aria-label="媒体库"
            className="!w-auto min-w-28"
          >
            <option value="">全部媒体库</option>
            {libraries.map((library) => <option key={library.id} value={library.id}>{library.name}</option>)}
          </Select>
        )}

        <SearchField
          value={searchKeyword}
          onChange={(event) => {
            setSearchKeyword(event.target.value)
            if (!event.target.value) setSearchResults(null)
          }}
          onKeyDown={(event) => event.key === 'Enter' && handleSearch()}
          placeholder="搜索合集名称"
          wrapperClassName="min-w-[190px] flex-1 lg:max-w-sm"
          aria-label="搜索合集名称"
        />

        <Select
          value={sortValue}
          onChange={(event) => updateParams({ sort: event.target.value === 'created_desc' ? null : event.target.value }, true)}
          aria-label="合集排序"
          className="!w-auto min-w-28"
        >
          {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </Select>

        <div className="flex items-center gap-0.5 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] p-0.5" role="group" aria-label="视图模式">
          <ViewButton active={viewMode === 'grid'} title="网格视图" onClick={() => updateParams({ view: null })}><Grid3X3 size={14} /></ViewButton>
          <ViewButton active={viewMode === 'list'} title="列表视图" onClick={() => updateParams({ view: 'list' })}><LayoutList size={14} /></ViewButton>
        </div>

        <div className="ml-auto flex flex-wrap items-center gap-1.5">
          <Button type="button" variant="ghost" size="sm" onClick={() => runMaintenance('rematch')} loading={operating} title="清除所有自动匹配的合集并重新匹配，手动创建的合集不受影响">
            <RefreshCw size={14} aria-hidden="true" />重新匹配
          </Button>
          <Button type="button" variant="ghost" size="sm" onClick={() => runMaintenance('merge')} disabled={operating} title="合并所有同名重复合集，保留最早创建的合集并迁移电影">
            <Merge size={14} aria-hidden="true" />合并重复
          </Button>
          <Button type="button" variant="danger" size="sm" onClick={() => runMaintenance('cleanup')} disabled={operating} title="删除所有没有关联电影的空壳合集">
            <Trash2 size={14} aria-hidden="true" />清理空壳
          </Button>
        </div>
      </div>

      {operationMsg && (
        <Surface className="nv-collection-notice flex items-center gap-3 px-4 py-3 text-sm text-[var(--nv-text-secondary)]" role="status">
          <Tag tone="brand">操作完成</Tag>
          <span className="min-w-0 flex-1">{operationMsg}</span>
          <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => setOperationMsg('')} aria-label="关闭提示"><X size={14} /></Button>
        </Surface>
      )}

      {loading ? (
        <div className="nv-media-grid" aria-label="正在加载合集">
          {Array.from({ length: 12 }).map((_, index) => <div key={index} className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />)}
        </div>
      ) : displayList.length === 0 ? (
        <Surface><EmptyState icon={<LibraryIcon size={24} />} title={emptyTitle} description={emptyDescription} /></Surface>
      ) : viewMode === 'grid' ? (
        <div className="nv-media-grid">
          {displayList.map((collection) => <CollectionCard key={collection.id} collection={collection} />)}
        </div>
      ) : (
        <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
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
    </div>
  )
}

function ViewButton({ active, title, onClick, children }: { active: boolean; title: string; onClick: () => void; children: ReactNode }) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="sm"
      iconOnly
      onClick={onClick}
      title={title}
      aria-label={title}
      aria-pressed={active}
      className={active ? '!bg-[var(--nv-fill-active)] !text-[var(--nv-text-primary)]' : undefined}
    >
      {children}
    </Button>
  )
}
