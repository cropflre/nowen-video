import type { Library } from '@/types'
import {
  ArrowUpDown,
  ChevronDown,
  ChevronRight,
  Filter,
  Plus,
  ScanLine,
  Search,
  Sparkles,
  Trash2,
  Wand2,
} from 'lucide-react'
import clsx from 'clsx'
import { Button, Input, Select, Surface, Tag } from '@/components/design-system'
import { SORT_OPTIONS, SOURCE_OPTIONS } from './constants'

interface FileToolbarProps {
  // 搜索
  keyword: string
  onKeywordChange: (val: string) => void
  // 筛选
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
  // 操作
  onImport: () => void
  onScanDir: () => void
  // 视图
  viewMode: 'table' | 'grid'
  onViewModeChange: (mode: 'table' | 'grid') => void
  // 批量操作
  selectedCount: number
  scrapeSource: string
  onScrapeSourceChange: (val: string) => void
  onBatchScrape: () => void
  onBatchRename: () => void
  onBatchDelete: () => void
  onClearSelection: () => void
  // 额外内容（如AI助手按钮），渲染在视图切换按钮右侧
  children?: React.ReactNode
}

const viewButtonClassName = (active: boolean) => clsx(
  'min-h-8 px-3 py-1.5 text-xs font-medium transition-colors duration-200',
  active
    ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]'
    : 'text-[var(--nv-text-tertiary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]',
)

export default function FileToolbar({
  keyword, onKeywordChange,
  showFilters, onToggleFilters,
  filterLibrary, onFilterLibraryChange,
  filterMediaType, onFilterMediaTypeChange,
  filterScraped, onFilterScrapedChange,
  sortBy, onSortByChange,
  sortOrder, onToggleSortOrder,
  libraries,
  onImport, onScanDir,
  viewMode, onViewModeChange,
  selectedCount,
  scrapeSource, onScrapeSourceChange,
  onBatchScrape, onBatchRename, onBatchDelete, onClearSelection,
  children,
}: FileToolbarProps) {
  return (
    <Surface className="space-y-3 p-4">
      {/* 第一行：搜索和操作按钮 */}
      <div className="flex flex-wrap items-center gap-2">
        {/* 搜索 */}
        <div className="relative min-w-[200px] max-w-md flex-1">
          <Search
            size={16}
            className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--nv-text-tertiary)]"
            aria-hidden="true"
          />
          <Input
            type="text"
            placeholder="搜索标题、原始标题、文件路径..."
            value={keyword}
            onChange={(event) => onKeywordChange(event.target.value)}
            className="pl-9"
          />
        </div>

        {/* 筛选切换 */}
        <Button
          type="button"
          size="sm"
          variant={showFilters ? 'secondary' : 'ghost'}
          onClick={onToggleFilters}
          className={showFilters ? 'text-[var(--nv-action-primary)]' : undefined}
          aria-expanded={showFilters}
        >
          <Filter size={15} aria-hidden="true" />
          筛选
          {showFilters ? <ChevronDown size={14} aria-hidden="true" /> : <ChevronRight size={14} aria-hidden="true" />}
        </Button>

        <div className="hidden h-6 w-px bg-[var(--nv-border-subtle)] sm:block" aria-hidden="true" />

        {/* 导入按钮 */}
        <Button type="button" size="sm" variant="primary" onClick={onImport}>
          <Plus size={15} aria-hidden="true" />
          导入文件
        </Button>
        <Button type="button" size="sm" variant="secondary" onClick={onScanDir}>
          <ScanLine size={15} aria-hidden="true" />
          扫描目录
        </Button>

        <div className="hidden h-6 w-px bg-[var(--nv-border-subtle)] sm:block" aria-hidden="true" />

        {/* 视图切换 */}
        <div
          className="flex items-center overflow-hidden rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)]"
          role="group"
          aria-label="文件视图"
        >
          <button
            type="button"
            onClick={() => onViewModeChange('table')}
            className={viewButtonClassName(viewMode === 'table')}
            aria-pressed={viewMode === 'table'}
          >
            列表
          </button>
          <button
            type="button"
            onClick={() => onViewModeChange('grid')}
            className={clsx(viewButtonClassName(viewMode === 'grid'), 'border-l border-[var(--nv-border-subtle)]')}
            aria-pressed={viewMode === 'grid'}
          >
            网格
          </button>
        </div>

        {/* 额外内容插槽（如AI助手按钮） */}
        {children}
      </div>

      {/* 筛选行 */}
      {showFilters && (
        <div className="flex flex-wrap items-center gap-2 border-t border-[var(--nv-border-subtle)] pt-3">
          <Select value={filterLibrary} onChange={(event) => onFilterLibraryChange(event.target.value)} aria-label="媒体库筛选">
            <option value="">全部媒体库</option>
            {libraries.map((library) => <option key={library.id} value={library.id}>{library.name}</option>)}
          </Select>
          <Select value={filterMediaType} onChange={(event) => onFilterMediaTypeChange(event.target.value)} aria-label="媒体类型筛选">
            <option value="">全部类型</option>
            <option value="movie">电影</option>
            <option value="episode">剧集</option>
          </Select>
          <Select value={filterScraped} onChange={(event) => onFilterScrapedChange(event.target.value)} aria-label="刮削状态筛选">
            <option value="">全部状态</option>
            <option value="true">已刮削</option>
            <option value="false">未刮削</option>
          </Select>
          <Select value={sortBy} onChange={(event) => onSortByChange(event.target.value)} aria-label="排序字段">
            {SORT_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </Select>
          <Button type="button" size="sm" variant="ghost" onClick={onToggleSortOrder}>
            <ArrowUpDown size={14} aria-hidden="true" />
            {sortOrder === 'desc' ? '降序' : '升序'}
          </Button>
        </div>
      )}

      {/* 批量操作栏 */}
      {selectedCount > 0 && (
        <div className="flex flex-wrap items-center gap-2 border-t border-[var(--nv-border-subtle)] pt-3">
          <Tag tone="brand">已选 {selectedCount} 项</Tag>

          <Select
            value={scrapeSource}
            onChange={(event) => onScrapeSourceChange(event.target.value)}
            aria-label="批量刮削源"
            className="h-9 text-xs"
          >
            {SOURCE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </Select>
          <Button type="button" size="sm" variant="secondary" onClick={onBatchScrape}>
            <Sparkles size={14} aria-hidden="true" />
            批量刮削
          </Button>
          <Button type="button" size="sm" variant="secondary" onClick={onBatchRename}>
            <Wand2 size={14} aria-hidden="true" />
            批量重命名
          </Button>
          <Button type="button" size="sm" variant="danger" onClick={onBatchDelete}>
            <Trash2 size={14} aria-hidden="true" />
            批量删除
          </Button>
          <Button type="button" size="sm" variant="ghost" onClick={onClearSelection}>
            取消选择
          </Button>
        </div>
      )}
    </Surface>
  )
}
