import { useState, useEffect, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { Media, Library, FileManagerStats, FolderNode } from '@/types'
import { fileManagerApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import AIAssistant, { AIAssistantButton, AIAssistantPanel } from '@/components/AIAssistant'
import ScrapeManagerPage from '@/pages/ScrapeManagerPage'
import { useWebSocket } from '@/hooks/useWebSocket'
import {
  FolderOpen,
  Globe,
  RefreshCw,
  History,
  PanelLeftClose,
  PanelLeftOpen,
} from 'lucide-react'
import clsx from 'clsx'

// 导入拆分后的子组件
import {
  FileStatsBar,
  FileToolbar,
  FileListView,
  FolderTree,
  Breadcrumb,
  ImportFileModal,
  ScanDirectoryModal,
  EditFileModal,
  FileDetailModal,
  RenameModal,
  OperationLogsModal,
} from '@/components/file-manager'
import type { TabType, DialogType } from '@/components/file-manager'

// 文件夹操作弹窗类型
type FolderDialogType = 'none' | 'createFolder' | 'renameFolder' | 'deleteFolder'

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
  const [pageSize, setPageSize] = useState(20)
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

  // 文件夹导航
  const [folderTree, setFolderTree] = useState<FolderNode[]>([])
  const [folderTreeLoading, setFolderTreeLoading] = useState(false)
  const [currentFolderPath, setCurrentFolderPath] = useState('')
  const [subFolders, setSubFolders] = useState<string[]>([])
  const [showFolderPanel, setShowFolderPanel] = useState(true)

  // 文件夹操作弹窗状态
  const [folderDialog, setFolderDialog] = useState<FolderDialogType>('none')
  const [folderDialogTarget, setFolderDialogTarget] = useState('') // 目标文件夹路径
  const [folderInputValue, setFolderInputValue] = useState('')

  // AI 助手面板状态
  const [showAIPanel, setShowAIPanel] = useState(false)

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
      if (currentFolderPath) {
        // 文件夹模式：按路径查询
        const res = await fileManagerApi.listFilesByFolder({
          path: currentFolderPath,
          page, size: pageSize, library_id: filterLibrary,
          media_type: filterMediaType, keyword,
          sort_by: sortBy, sort_order: sortOrder,
          scraped: filterScraped,
        })
        setFiles(res.data.data || [])
        setTotal(res.data.total)
        setSubFolders(res.data.sub_folders || [])
      } else {
        // 全局模式：原有平铺列表
        const res = await fileManagerApi.listFiles({
          page, size: pageSize, library_id: filterLibrary,
          media_type: filterMediaType, keyword,
          sort_by: sortBy, sort_order: sortOrder,
          scraped: filterScraped,
        })
        setFiles(res.data.data || [])
        setTotal(res.data.total)
        setSubFolders([])
      }
    } catch {
      toast.error('获取文件列表失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, filterLibrary, filterMediaType, keyword, sortBy, sortOrder, filterScraped, currentFolderPath])

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

  const fetchFolderTree = useCallback(async () => {
    setFolderTreeLoading(true)
    try {
      const res = await fileManagerApi.getFolderTree(filterLibrary || undefined)
      setFolderTree(res.data.data || [])
    } catch { /* ignore */ }
    finally { setFolderTreeLoading(false) }
  }, [filterLibrary])

  useEffect(() => { fetchFiles() }, [fetchFiles])
  useEffect(() => { fetchStats(); fetchLibraries() }, [fetchStats, fetchLibraries])
  useEffect(() => { fetchFolderTree() }, [fetchFolderTree])

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

  const refreshData = () => { fetchFiles(); fetchStats(); fetchFolderTree() }

  const totalPages = Math.ceil(total / pageSize)

  // 每页数量可选项
  const pageSizeOptions = [20, 50, 100, 200]

  // 切换每页数量时重置到第一页
  const handlePageSizeChange = useCallback((size: number) => {
    setPageSize(size)
    setPage(1)
  }, [])

  // 文件夹导航操作
  const handleSelectFolder = useCallback((path: string) => {
    setCurrentFolderPath(path)
    setPage(1)
    setSelectedIds(new Set())
  }, [])

  const handleClearFolder = useCallback(() => {
    setCurrentFolderPath('')
    setPage(1)
    setSelectedIds(new Set())
    setSubFolders([])
  }, [])

  // ==================== 文件夹右键操作 ====================

  const handleCreateFolder = useCallback((parentPath: string) => {
    setFolderDialogTarget(parentPath)
    setFolderInputValue('')
    setFolderDialog('createFolder')
  }, [])

  const handleRenameFolder = useCallback((folderPath: string) => {
    setFolderDialogTarget(folderPath)
    // 默认填充当前文件夹名
    const name = folderPath.replace(/\\/g, '/').split('/').pop() || ''
    setFolderInputValue(name)
    setFolderDialog('renameFolder')
  }, [])

  const handleDeleteFolder = useCallback((folderPath: string) => {
    setFolderDialogTarget(folderPath)
    setFolderDialog('deleteFolder')
  }, [])

  const handleCopyPath = useCallback((path: string) => {
    navigator.clipboard.writeText(path).then(() => {
      toast.success('路径已复制到剪贴板')
    }).catch(() => {
      toast.error('复制失败')
    })
  }, [toast])

  const handlePlayFile = useCallback((media: Media) => {
    // 打开新窗口播放
    window.open(`/play/${media.id}`, '_blank')
  }, [])

  const executeCreateFolder = useCallback(async () => {
    if (!folderInputValue.trim()) { toast.error('文件夹名不能为空'); return }
    try {
      await fileManagerApi.createFolder(folderDialogTarget, folderInputValue.trim())
      toast.success('文件夹创建成功')
      setFolderDialog('none')
      fetchFolderTree()
      fetchFiles()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '创建文件夹失败')
    }
  }, [folderDialogTarget, folderInputValue, toast, fetchFolderTree, fetchFiles])

  const executeRenameFolder = useCallback(async () => {
    if (!folderInputValue.trim()) { toast.error('文件夹名不能为空'); return }
    try {
      await fileManagerApi.renameFolder(folderDialogTarget, folderInputValue.trim())
      toast.success('文件夹重命名成功')
      setFolderDialog('none')
      // 如果当前选中的文件夹被重命名，更新路径
      if (currentFolderPath === folderDialogTarget) {
        const parentDir = folderDialogTarget.replace(/\\/g, '/').replace(/\/[^\/]+$/, '')
        setCurrentFolderPath(parentDir + '/' + folderInputValue.trim())
      }
      fetchFolderTree()
      fetchFiles()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '重命名失败')
    }
  }, [folderDialogTarget, folderInputValue, currentFolderPath, toast, fetchFolderTree, fetchFiles])

  const executeDeleteFolder = useCallback(async (force: boolean) => {
    try {
      await fileManagerApi.deleteFolder(folderDialogTarget, force)
      toast.success('文件夹删除成功')
      setFolderDialog('none')
      // 如果当前选中的文件夹被删除，回到上级
      if (currentFolderPath === folderDialogTarget || currentFolderPath.startsWith(folderDialogTarget + '/')) {
        handleClearFolder()
      }
      fetchFolderTree()
      fetchFiles()
      fetchStats()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '删除失败')
    }
  }, [folderDialogTarget, currentFolderPath, toast, handleClearFolder, fetchFolderTree, fetchFiles, fetchStats])

  // 键盘快捷键
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (activeTab !== 'files') return
      if (activeDialog !== 'none' || folderDialog !== 'none') return
      if (e.key === 'Delete' && selectedIds.size > 0) {
        e.preventDefault()
        handleBatchDelete()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [activeTab, activeDialog, folderDialog, selectedIds, handleBatchDelete])

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
          onFilterLibraryChange={(val) => { setFilterLibrary(val); setPage(1); setCurrentFolderPath('') }}
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
        >
          {/* AI助手按钮 — 嵌入在列表/网格切换按钮右侧 */}
          <AIAssistantButton
            isOpen={showAIPanel}
            onToggle={() => setShowAIPanel(!showAIPanel)}
            selectedCount={selectedIds.size}
          />
        </FileToolbar>

        {/* 面包屑导航 */}
        {currentFolderPath && (
          <div className="flex items-center gap-2">
            <Breadcrumb
              folderPath={currentFolderPath}
              onNavigate={handleSelectFolder}
              onGoHome={handleClearFolder}
            />
          </div>
        )}

        {/* 左侧文件夹树 + 中间文件列表 + 右侧AI助手面板 */}
        <div className="flex gap-4">
          {/* 左侧文件夹树面板 — 使用 CSS 过渡动画而非条件渲染 */}
          <div
            className={clsx(
              'flex-shrink-0 hidden lg:block overflow-hidden transition-all duration-300 ease-out',
              showFolderPanel ? 'w-64 opacity-100' : 'w-0 opacity-0'
            )}
            style={{
              height: showFolderPanel ? 'calc(100vh - 280px)' : 0,
              maxHeight: showFolderPanel ? 'calc(100vh - 280px)' : 0,
            }}
          >
            <div className="w-64 h-full">
              <FolderTree
                tree={folderTree}
                loading={folderTreeLoading}
                selectedPath={currentFolderPath}
                onSelectFolder={handleSelectFolder}
                onClearFolder={handleClearFolder}
                onCreateFolder={handleCreateFolder}
                onRenameFolder={handleRenameFolder}
                onDeleteFolder={handleDeleteFolder}
                onRefreshFolder={fetchFolderTree}
                onCopyPath={handleCopyPath}
              />
            </div>
          </div>

          {/* 右侧文件列表 */}
          <div className="flex-1 min-w-0 space-y-4">
            {/* 文件夹面板切换按钮 */}
            <div className="flex items-center gap-2">
              <button
                onClick={() => setShowFolderPanel(!showFolderPanel)}
                className="btn-ghost hidden lg:flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs"
                title={showFolderPanel ? '收起文件夹面板' : '展开文件夹面板'}
              >
                {showFolderPanel ? <PanelLeftClose size={14} /> : <PanelLeftOpen size={14} />}
                {showFolderPanel ? '收起导航' : '展开导航'}
              </button>
              {currentFolderPath && (
                <span className="text-xs px-2 py-1 rounded-md bg-neon-blue/10 text-neon">
                  当前目录: {currentFolderPath.replace(/\\/g, '/').split('/').pop()}
                </span>
              )}
            </div>

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
              pageSize={pageSize}
              pageSizeOptions={pageSizeOptions}
              onPageChange={setPage}
              onPageSizeChange={handlePageSizeChange}
              subFolders={subFolders}
              currentFolderPath={currentFolderPath}
              onNavigateFolder={handleSelectFolder}
              onPlayFile={handlePlayFile}
              onCopyFilePath={handleCopyPath}
              onCreateSubFolder={handleCreateFolder}
              onRenameSubFolder={handleRenameFolder}
              onDeleteSubFolder={handleDeleteFolder}
              onRefreshSubFolder={fetchFolderTree}
              onCopyFolderPath={handleCopyPath}
            />
          </div>

          {/* 右侧 AI 助手面板 — 使用 CSS 过渡动画 */}
          <AIAssistantPanel isOpen={showAIPanel}>
            <AIAssistant
              selectedMediaIds={Array.from(selectedIds)}
              libraryId={filterLibrary || undefined}
              onOperationComplete={fetchFiles}
              isOpen={showAIPanel}
              onToggle={() => setShowAIPanel(!showAIPanel)}
            />
          </AIAssistantPanel>
        </div>
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

      {/* ==================== 文件夹操作弹窗 ==================== */}

      {/* 新建文件夹弹窗 */}
      {folderDialog === 'createFolder' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setFolderDialog('none')}>
          <div className="glass-panel rounded-2xl p-6 w-full max-w-md mx-4 space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>新建文件夹</h3>
            <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
              在 <code className="px-1.5 py-0.5 rounded bg-surface-700/50 text-xs">{folderDialogTarget.replace(/\\/g, '/').split('/').pop()}</code> 下创建子文件夹
            </p>
            <input
              type="text"
              value={folderInputValue}
              onChange={e => setFolderInputValue(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') executeCreateFolder() }}
              placeholder="输入文件夹名称"
              className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent focus:outline-none focus:ring-1 focus:ring-neon"
              style={{ borderColor: 'var(--border-default)', color: 'var(--text-primary)' }}
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <button onClick={() => setFolderDialog('none')} className="btn-ghost px-4 py-2 rounded-lg text-sm">取消</button>
              <button onClick={executeCreateFolder} className="btn-primary px-4 py-2 rounded-lg text-sm">创建</button>
            </div>
          </div>
        </div>
      )}

      {/* 重命名文件夹弹窗 */}
      {folderDialog === 'renameFolder' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setFolderDialog('none')}>
          <div className="glass-panel rounded-2xl p-6 w-full max-w-md mx-4 space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>重命名文件夹</h3>
            <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
              重命名 <code className="px-1.5 py-0.5 rounded bg-surface-700/50 text-xs">{folderDialogTarget.replace(/\\/g, '/').split('/').pop()}</code>
            </p>
            <input
              type="text"
              value={folderInputValue}
              onChange={e => setFolderInputValue(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter') executeRenameFolder() }}
              placeholder="输入新名称"
              className="w-full px-3 py-2 rounded-lg text-sm border bg-transparent focus:outline-none focus:ring-1 focus:ring-neon"
              style={{ borderColor: 'var(--border-default)', color: 'var(--text-primary)' }}
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <button onClick={() => setFolderDialog('none')} className="btn-ghost px-4 py-2 rounded-lg text-sm">取消</button>
              <button onClick={executeRenameFolder} className="btn-primary px-4 py-2 rounded-lg text-sm">确认重命名</button>
            </div>
          </div>
        </div>
      )}

      {/* 删除文件夹确认弹窗 */}
      {folderDialog === 'deleteFolder' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setFolderDialog('none')}>
          <div className="glass-panel rounded-2xl p-6 w-full max-w-md mx-4 space-y-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-semibold text-red-400">删除文件夹</h3>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              确定要删除文件夹 <code className="px-1.5 py-0.5 rounded bg-surface-700/50 text-xs font-bold">{folderDialogTarget.replace(/\\/g, '/').split('/').pop()}</code> 吗？
            </p>
            <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/20">
              <p className="text-xs text-red-400">
                ⚠️ 此操作将删除文件夹及其中的所有文件，且不可恢复。数据库中对应的文件记录也将被清除。
              </p>
            </div>
            <div className="flex justify-end gap-2">
              <button onClick={() => setFolderDialog('none')} className="btn-ghost px-4 py-2 rounded-lg text-sm">取消</button>
              <button
                onClick={() => executeDeleteFolder(false)}
                className="px-4 py-2 rounded-lg text-sm bg-amber-600/20 text-amber-400 hover:bg-amber-600/30 transition-colors"
              >
                仅删除空文件夹
              </button>
              <button
                onClick={() => executeDeleteFolder(true)}
                className="px-4 py-2 rounded-lg text-sm bg-red-600/20 text-red-400 hover:bg-red-600/30 transition-colors"
              >
              强制删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
