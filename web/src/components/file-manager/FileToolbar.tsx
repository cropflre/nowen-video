import type { Library } from '@/types'
import {
  Search,
  Filter,
  Plus,
  ScanLine,
  ChevronDown,
  ChevronRight,
  ArrowUpDown,
} from 'lucide-react'
import clsx from 'clsx'
import { SORT_OPTIONS, SOURCE_OPTIONS } from './constants'
import {
  Sparkles,
  Wand2,
  Trash2,
} from 'lucide-react'

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
}

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
}: FileToolbarProps) {
  return (
    <div className="glass-panel rounded-xl p-4 space-y-3">
      {/* 第一行：搜索和操作按钮 */}
      <div className="flex flex-wrap items-center gap-2">
        {/* 搜索 */}
        <div className="relative flex-1 min-w-[200px] max-w-md">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-surface-400" />
          <input
            type="text"
            placeholder="搜索标题、原始标题、文件路径..."
            value={keyword}
            onChange={e => onKeywordChange(e.target.value)}
            className="input-field w-full pl-9 pr-3 py-2 text-sm rounded-lg"
          />
        </div>

        {/* 筛选切换 */}
        <button
          onClick={onToggleFilters}
          className={clsx('btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm', showFilters && 'text-neon')}
        >
          <Filter size={16} /> 筛选
          {showFilters ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </button>

        <div className="h-6 w-px bg-surface-700" />

        {/* 导入按钮 */}
        <button onClick={onImport} className="btn-primary flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
          <Plus size={16} /> 导入文件
        </button>
        <button onClick={onScanDir} className="btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
          <ScanLine size={16} /> 扫描目录
        </button>

        <div className="h-6 w-px bg-surface-700" />

        {/* 视图切换 */}
        <div className="flex items-center rounded-lg overflow-hidden border" style={{ borderColor: 'var(--border-default)' }}>
          <button
            onClick={() => onViewModeChange('table')}
            className={clsx('px-2.5 py-1.5 text-xs', viewMode === 'table' ? 'bg-neon-blue/20 text-neon' : 'text-surface-400')}
          >
            列表
          </button>
          <button
            onClick={() => onViewModeChange('grid')}
            className={clsx('px-2.5 py-1.5 text-xs', viewMode === 'grid' ? 'bg-neon-blue/20 text-neon' : 'text-surface-400')}
          >
            网格
          </button>
        </div>
      </div>

      {/* 筛选行 */}
      {showFilters && (
        <div className="flex flex-wrap items-center gap-3 pt-2 border-t" style={{ borderColor: 'var(--border-default)' }}>
          <select value={filterLibrary} onChange={e => onFilterLibraryChange(e.target.value)}
            className="input-field px-3 py-1.5 text-sm rounded-lg">
            <option value="">全部媒体库</option>
            {libraries.map(lib => <option key={lib.id} value={lib.id}>{lib.name}</option>)}
          </select>
          <select value={filterMediaType} onChange={e => onFilterMediaTypeChange(e.target.value)}
            className="input-field px-3 py-1.5 text-sm rounded-lg">
            <option value="">全部类型</option>
            <option value="movie">电影</option>
            <option value="episode">剧集</option>
          </select>
          <select value={filterScraped} onChange={e => onFilterScrapedChange(e.target.value)}
            className="input-field px-3 py-1.5 text-sm rounded-lg">
            <option value="">全部状态</option>
            <option value="true">已刮削</option>
            <option value="false">未刮削</option>
          </select>
          <select value={sortBy} onChange={e => onSortByChange(e.target.value)}
            className="input-field px-3 py-1.5 text-sm rounded-lg">
            {SORT_OPTIONS.map(opt => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
          </select>
          <button onClick={onToggleSortOrder}
            className="btn-ghost flex items-center gap-1 px-2 py-1.5 rounded-lg text-sm">
            <ArrowUpDown size={14} /> {sortOrder === 'desc' ? '降序' : '升序'}
          </button>
        </div>
      )}

      {/* 批量操作栏 */}
      {selectedCount > 0 && (
        <div className="flex items-center gap-2 pt-2 border-t" style={{ borderColor: 'var(--border-default)' }}>
          <span className="text-sm text-neon font-medium">已选 {selectedCount} 项</span>
          <div className="h-4 w-px bg-surface-700" />

          <select value={scrapeSource} onChange={e => onScrapeSourceChange(e.target.value)}
            className="input-field px-2 py-1 text-xs rounded">
            {SOURCE_OPTIONS.map(opt => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
          </select>
          <button onClick={onBatchScrape} className="btn-ghost flex items-center gap-1 px-2.5 py-1.5 rounded text-xs text-blue-400 hover:bg-blue-400/10">
            <Sparkles size={14} /> 批量刮削
          </button>
          <button onClick={onBatchRename} className="btn-ghost flex items-center gap-1 px-2.5 py-1.5 rounded text-xs text-purple-400 hover:bg-purple-400/10">
            <Wand2 size={14} /> 批量重命名
          </button>
          <button onClick={onBatchDelete} className="btn-ghost flex items-center gap-1 px-2.5 py-1.5 rounded text-xs text-red-400 hover:bg-red-400/10">
            <Trash2 size={14} /> 批量删除
          </button>
          <button onClick={onClearSelection} className="btn-ghost px-2.5 py-1.5 rounded text-xs">
            取消选择
          </button>
        </div>
      )}
    </div>
  )
}
