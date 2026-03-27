import { useState, useEffect, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { Media, Library, FileManagerStats } from '@/types'
import { fileManagerApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import AIAssistant from '@/components/AIAssistant'
import ScrapeManagerPage from '@/pages/ScrapeManagerPage'
import { useWebSocket } from '@/hooks/useWebSocket'
import {
  FolderOpen,
  Globe,
  RefreshCw,
  History,
} from 'lucide-react'
import clsx from 'clsx'

// 导入拆分后的子组件
import {
  FileStatsBar,
  FileToolbar,
  FileListView,
  ImportFileModal,
  ScanDirectoryModal,
  EditFileModal,
  FileDetailModal,
  RenameModal,
  OperationLogsModal,
} from '@/components/file-manager'
import type { TabType, DialogType } from '@/components/file-manager'

export default function FileManagerPage() {
  const toast = useToast()
  const { on, off } = useWebSocket()
  const [searchParams, setSearchParams] = useSearchParams()

  // Tab 状态（支持从URL参数读取，如 /files?tab=scrape）
  const [activeTab, setActiveTab] = useState<TabType>(() => {
    const tab = searchParams.get('tab')
    return tab === 'scrape' ? 'scrape' : 'files'
  })

  // 切换Tab时同步URL参数
  const handleTabChange = useCallback((tab: TabType) => {
    setActiveTab(tab)
    if (tab === 'files') {
      searchParams.delete('tab')
    } else {
      searchParams.set('tab', tab)
    }
    setSearchParams(searchParams, { replace: true })
  }, [searchParams, setSearchParams])

  // 数据状态
  const [files, setFiles] = useState<Media[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [stats, setStats] = useState<FileManagerStats | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])

  // 筛选
  const [keyword, setKeyword] = useState('')
  const [filterLibrary, setFilterLibrary] = useState('')
  const [filterMediaType, setFilterMediaType] = useState('')
  const [filterScraped, setFilterScraped] = useState('')
  const [sortBy, setSortBy] = useState('created_at')
  const [sortOrder, setSortOrder] = useState('desc')
  const [showFilters, setShowFilters] = useState(false)

  // 选择
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  // 视图模式
  const [viewMode, setViewMode] = useState<'table' | 'grid'>('table')

  // 对话框状态
  const [activeDialog, setActiveDialog] = useState<DialogType>('none')

  // 编辑/详情弹窗的目标媒体
  const [editMedia, setEditMedia] = useState<Media | null>(null)
  const [detailMedia, setDetailMedia] = useState<Media | null>(null)

  // 刮削源
  const [scrapeSource, setScrapeSource] = useState('')

  // ==================== 数据加载 ====================

  const fetchFiles = useCallback(async () => {
    setLoading(true)
    try {
      const res = await fileManagerApi.listFiles({
        page, size: 20, library_id: filterLibrary,
        media_type: filterMediaType, keyword,
        sort_by: sortBy, sort_order: sortOrder,
        scraped: filterScraped,
      })
      setFiles(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      toast.error('获取文件列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, filterLibrary, filterMediaType, keyword, sortBy, sortOrder, filterScraped])

  const fetchStats = useCallback(async () => {
    try {
      const res = await fileManagerApi.getStats()
      setStats(res.data.data)
    } catch { /* ignore */ }
  }, [])

  const fetchLibraries = useCallback(async () => {
    try {
      const res = await libraryApi.list()
      setLibraries(res.data.data || [])
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { fetchFiles() }, [fetchFiles])
  useEffect(() => { fetchStats(); fetchLibraries() }, [fetchStats, fetchLibraries])

  // WebSocket 实时更新
  useEffect(() => {
    const handleUpdate = () => { fetchFiles(); fetchStats() }
    on('file_imported', handleUpdate)
    on('file_deleted', handleUpdate)
    on('batch_rename_complete', handleUpdate)
    on('file_scrape_progress', handleUpdate)
    return () => {
      off('file_imported', handleUpdate)
      off('file_deleted', handleUpdate)
      off('batch_rename_complete', handleUpdate)
      off('file_scrape_progress', handleUpdate)
    }
  }, [on, off, fetchFiles, fetchStats])

  // ==================== 选择操作 ====================

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    setSelectedIds(prev => prev.size === files.length ? new Set() : new Set(files.map(f => f.id)))
  }

  // ==================== 操作 ====================

  const handleDeleteFile = async (id: string) => {
    if (!confirm('确定要删除此文件记录吗？（原始文件不会被删除）')) return
    try {
      await fileManagerApi.deleteFile(id)
      toast.success('文件记录已删除')
      setSelectedIds(prev => { const n = new Set(prev); n.delete(id); return n })
      fetchFiles(); fetchStats()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '删除失败')
    }
  }

  const handleBatchDelete = async () => {
    if (selectedIds.size === 0) return
    if (!confirm(`确定要删除选中的 ${selectedIds.size} 个文件记录吗？（原始文件不会被删除）`)) return
    try {
      const res = await fileManagerApi.batchDeleteFiles(Array.from(selectedIds))
      toast.success(`已删除 ${res.data.deleted} 个文件记录`)
      setSelectedIds(new Set())
      fetchFiles(); fetchStats()
    } catch { toast.error('批量删除失败') }
  }

  const handleScrapeFile = async (id: string) => {
    try {
      await fileManagerApi.scrapeFile(id, scrapeSource || undefined)
      toast.success('刮削已启动')
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '刮削失败')
    }
  }

  const handleBatchScrape = async () => {
    if (selectedIds.size === 0) return
    try {
      const res = await fileManagerApi.batchScrapeFiles(Array.from(selectedIds), scrapeSource || undefined)
      toast.success(`已启动 ${res.data.started} 个刮削任务`)
    } catch { toast.error('批量刮削失败') }
  }

  const refreshData = () => { fetchFiles(); fetchStats() }

  const totalPages = Math.ceil(total / 20)

  // ==================== 渲染 ====================

  return (
    <div className="min-h-screen p-4 md:p-6 space-y-6">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
            <FolderOpen className="inline-block mr-2 mb-1" size={24} />
            影视文件管理
          </h1>
          <p className="text-sm mt-1" style={{ color: 'var(--text-tertiary)' }}>
            管理影视文件、智能刮削元数据、AI批量重命名
          </p>
        </div>
        {activeTab === 'files' && (
          <div className="flex items-center gap-2">
            <button onClick={() => setActiveDialog('logs')} className="btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
              <History size={16} /> 操作日志
            </button>
            <button onClick={refreshData} className="btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
              <RefreshCw size={16} /> 刷新
            </button>
          </div>
        )}
      </div>

      {/* Tab 切换栏 */}
      <div className="flex items-center gap-1 p-1 rounded-xl glass-panel" style={{ width: 'fit-content' }}>
        <button
          onClick={() => handleTabChange('files')}
          className={clsx(
            'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200',
            activeTab === 'files'
              ? 'bg-neon-blue/10 text-neon shadow-sm'
              : 'text-surface-400 hover:text-surface-200 hover:bg-white/5'
          )}
        >
          <FolderOpen size={16} />
          文件列表
        </button>
        <button
          onClick={() => handleTabChange('scrape')}
          className={clsx(
            'flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium transition-all duration-200',
            activeTab === 'scrape'
              ? 'bg-neon-blue/10 text-neon shadow-sm'
              : 'text-surface-400 hover:text-surface-200 hover:bg-white/5'
          )}
        >
          <Globe size={16} />
          刮削任务
        </button>
      </div>

      {/* ==================== 刮削任务 Tab ==================== */}
      {activeTab === 'scrape' && (
        <ScrapeManagerPage embedded />
      )}

      {/* ==================== 文件列表 Tab ==================== */}
      {activeTab === 'files' && (<>
        {/* 统计卡片 */}
        {stats && <FileStatsBar stats={stats} />}

        {/* 工具栏 */}
        <FileToolbar
          keyword={keyword}
          onKeywordChange={(val) => { setKeyword(val); setPage(1) }}
          showFilters={showFilters}
          onToggleFilters={() => setShowFilters(!showFilters)}
          filterLibrary={filterLibrary}
          onFilterLibraryChange={(val) => { setFilterLibrary(val); setPage(1) }}
          filterMediaType={filterMediaType}
          onFilterMediaTypeChange={(val) => { setFilterMediaType(val); setPage(1) }}
          filterScraped={filterScraped}
          onFilterScrapedChange={(val) => { setFilterScraped(val); setPage(1) }}
          sortBy={sortBy}
          onSortByChange={setSortBy}
          sortOrder={sortOrder}
          onToggleSortOrder={() => setSortOrder(sortOrder === 'desc' ? 'asc' : 'desc')}
          libraries={libraries}
          onImport={() => setActiveDialog('import')}
          onScanDir={() => setActiveDialog('scanDir')}
          viewMode={viewMode}
          onViewModeChange={setViewMode}
          selectedCount={selectedIds.size}
          scrapeSource={scrapeSource}
          onScrapeSourceChange={setScrapeSource}
          onBatchScrape={handleBatchScrape}
          onBatchRename={() => setActiveDialog('rename')}
          onBatchDelete={handleBatchDelete}
          onClearSelection={() => setSelectedIds(new Set())}
        />

        {/* 文件列表 */}
        <FileListView
          files={files}
          loading={loading}
          viewMode={viewMode}
          selectedIds={selectedIds}
          onToggleSelect={toggleSelect}
          onToggleSelectAll={toggleSelectAll}
          onViewDetail={(media) => { setDetailMedia(media); setActiveDialog('detail') }}
          onEdit={(media) => { setEditMedia(media); setActiveDialog('edit') }}
          onScrape={handleScrapeFile}
          onDelete={handleDeleteFile}
          page={page}
          totalPages={totalPages}
          total={total}
          onPageChange={setPage}
        />
      </>)}

      {/* ==================== 对话框 ==================== */}

      {activeDialog === 'import' && (
        <ImportFileModal
          libraries={libraries}
          onClose={() => setActiveDialog('none')}
          onSuccess={refreshData}
        />
      )}

      {activeDialog === 'scanDir' && (
        <ScanDirectoryModal
          libraries={libraries}
          onClose={() => setActiveDialog('none')}
          onSuccess={refreshData}
        />
      )}

      {activeDialog === 'edit' && editMedia && (
        <EditFileModal
          media={editMedia}
          onClose={() => setActiveDialog('none')}
          onSuccess={() => { fetchFiles() }}
        />
      )}

      {activeDialog === 'detail' && detailMedia && (
        <FileDetailModal
          media={detailMedia}
          onClose={() => setActiveDialog('none')}
          onEdit={() => { setEditMedia(detailMedia); setActiveDialog('edit') }}
          onScrape={() => { handleScrapeFile(detailMedia.id); setActiveDialog('none') }}
        />
      )}

      {activeDialog === 'rename' && (
        <RenameModal
          selectedCount={selectedIds.size}
          selectedIds={selectedIds}
          onClose={() => setActiveDialog('none')}
          onSuccess={() => { fetchFiles(); setActiveDialog('none') }}
        />
      )}

      {activeDialog === 'logs' && (
        <OperationLogsModal onClose={() => setActiveDialog('none')} />
      )}

      {/* AI 助手浮动组件 */}
      <AIAssistant
        selectedMediaIds={Array.from(selectedIds)}
        libraryId={filterLibrary || undefined}
        onOperationComplete={fetchFiles}
      />
    </div>
  )
}
