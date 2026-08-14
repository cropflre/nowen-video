import type { Library } from '@/types'
import { ArrowUpDown, Filter, Plus, ScanLine, Sparkles, Trash2, Wand2 } from 'lucide-react'
import { Button, SearchField, Select, Tag } from '@/components/design-system'
import { SORT_OPTIONS, SOURCE_OPTIONS } from './constants'

interface FileToolbarProps {
  keyword: string
  onKeywordChange: (val: string) => void
  showFilters: boolean
  onToggleFilters: () => void
  filterLibrary: string
  onFilterLibraryChange: (val: string) => void
  filterMediaType: string
  onFilterMediaTypeChange: (val: string) => void
  filterScraped: string
  onFilterScrapedChange: (val: string) => void
  sortBy: string
  onSortByChange: (val: string) => void
  sortOrder: string
  onToggleSortOrder: () => void
  libraries: Library[]
  onImport: () => void
  onScanDir: () => void
  viewMode: 'table' | 'grid'
  onViewModeChange: (mode: 'table' | 'grid') => void
  selectedCount: number
  scrapeSource: string
  onScrapeSourceChange: (val: string) => void
  onBatchScrape: () => void
  onBatchRename: () => void
  onBatchDelete: () => void
  onClearSelection: () => void
  children?: React.ReactNode
}

export default function FileToolbar(props: FileToolbarProps) {
  const {
    keyword, onKeywordChange, showFilters, onToggleFilters,
    filterLibrary, onFilterLibraryChange, filterMediaType, onFilterMediaTypeChange,
    filterScraped, onFilterScrapedChange, sortBy, onSortByChange, sortOrder, onToggleSortOrder,
    libraries, onImport, onScanDir, viewMode, onViewModeChange,
    selectedCount, scrapeSource, onScrapeSourceChange,
    onBatchScrape, onBatchRename, onBatchDelete, onClearSelection, children,
  } = props

  return (
    <div className="space-y-2.5 border-b border-[var(--nv-border-subtle)] pb-3">
      <div className="flex flex-wrap items-center gap-1.5">
        <SearchField
          value={keyword}
          onChange={(event) => onKeywordChange(event.target.value)}
          placeholder="搜索标题、文件路径"
          wrapperClassName="!w-full min-w-0 basis-full sm:!w-auto sm:min-w-[190px] sm:basis-auto sm:flex-1 lg:max-w-sm"
          aria-label="搜索文件"
        />
        <Button size="sm" variant={showFilters ? 'secondary' : 'ghost'} onClick={onToggleFilters}>
          <Filter size={14} aria-hidden="true" />筛选
        </Button>
        <Select value={sortBy} onChange={(event) => onSortByChange(event.target.value)} className="!w-auto min-w-28 flex-1 sm:flex-none" aria-label="排序字段">
          {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </Select>
        <Button size="sm" variant="ghost" iconOnly onClick={onToggleSortOrder} aria-label={sortOrder === 'desc' ? '降序' : '升序'}>
          <ArrowUpDown size={14} aria-hidden="true" />
        </Button>
        <Button size="sm" variant="primary" onClick={onImport}><Plus size={14} aria-hidden="true" />导入</Button>
        <Button size="sm" variant="ghost" onClick={onScanDir}><ScanLine size={14} aria-hidden="true" />扫描</Button>

        <div className="ml-0 flex items-center gap-0.5 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] p-0.5 sm:ml-auto" role="group" aria-label="文件视图">
          <Button size="sm" variant={viewMode === 'table' ? 'secondary' : 'ghost'} onClick={() => onViewModeChange('table')}>列表</Button>
          <Button size="sm" variant={viewMode === 'grid' ? 'secondary' : 'ghost'} onClick={() => onViewModeChange('grid')}>网格</Button>
        </div>
        {children}
      </div>

      {showFilters && (
        <div className="flex flex-wrap items-center gap-1.5">
          <Select value={filterLibrary} onChange={(event) => onFilterLibraryChange(event.target.value)} className="!w-auto min-w-32 flex-1 sm:flex-none" aria-label="媒体库筛选">
            <option value="">全部媒体库</option>
            {libraries.map((library) => <option key={library.id} value={library.id}>{library.name}</option>)}
          </Select>
          <Select value={filterMediaType} onChange={(event) => onFilterMediaTypeChange(event.target.value)} className="!w-auto min-w-28 flex-1 sm:flex-none" aria-label="媒体类型筛选">
            <option value="">全部类型</option><option value="movie">电影</option><option value="episode">剧集</option>
          </Select>
          <Select value={filterScraped} onChange={(event) => onFilterScrapedChange(event.target.value)} className="!w-auto min-w-28 flex-1 sm:flex-none" aria-label="状态筛选">
            <option value="">全部状态</option><option value="true">已刮削</option><option value="false">未刮削</option>
          </Select>
        </div>
      )}

      {selectedCount > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 border-t border-[var(--nv-border-subtle)] pt-2.5">
          <Tag tone="brand">已选 {selectedCount} 项</Tag>
          <Select value={scrapeSource} onChange={(event) => onScrapeSourceChange(event.target.value)} className="!w-auto min-w-28 flex-1 sm:flex-none" aria-label="批量刮削源">
            {SOURCE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </Select>
          <Button size="sm" variant="ghost" onClick={onBatchScrape}><Sparkles size={14} aria-hidden="true" />刮削</Button>
          <Button size="sm" variant="ghost" onClick={onBatchRename}><Wand2 size={14} aria-hidden="true" />重命名</Button>
          <Button size="sm" variant="danger" onClick={onBatchDelete}><Trash2 size={14} aria-hidden="true" />删除</Button>
          <Button size="sm" variant="ghost" onClick={onClearSelection}>取消选择</Button>
        </div>
      )}
    </div>
  )
}
