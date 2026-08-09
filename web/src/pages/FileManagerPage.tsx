import { useState, useEffect, useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'
import type { Media, Library, FileManagerStats, FolderNode } from '@/types'
import { fileManagerApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useDialog } from '@/components/Dialog'
import AIAssistant, { AIAssistantButton, AIAssistantPanel } from '@/components/AIAssistant'
import ScrapeManagerPage from '@/pages/ScrapeManagerPage'
import AdultScraperSection from '@/components/admin/AdultScraperTab'
import AdultScraperProSection from '@/components/admin/AdultScraperPro'
import STRMConfigSection from '@/components/admin/STRMConfigSection'
import { useWebSocket } from '@/hooks/useWebSocket'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import { PanelLeftClose, PanelLeftOpen } from 'lucide-react'
import clsx from 'clsx'
import { Button, Tag } from '@/components/design-system'

import {
  FileManagerShell,
  FileStatsBar,
  FileToolbar,
  FileListView,
  FolderTree,
  Breadcrumb,
  FolderOperationModal,
  ImportFileModal,
  ScanDirectoryModal,
  EditFileModal,
  FileDetailModal,
  RenameModal,
  OperationLogsModal,
} from '@/components/file-manager'
import type { TabType, DialogType, FolderDialogType } from '@/components/file-manager'

export default function FileManagerPage() {
  const toast = useToast()
  const dialog = useDialog()
  const { on, off } = useWebSocket()
  const [searchParams, setSearchParams] = useSearchParams()

  const [activeTab, setActiveTab] = useState<TabType>(() => {
    const tab = searchParams.get('tab')
    if (tab === 'scrape') return 'scrape'
    if (tab === 'adult') return 'adult'
    return 'files'
  })

  const handleTabChange = useCallback((tab: TabType) => {
    setActiveTab(tab)
    if (tab === 'files') {
      searchParams.delete('tab')
    } else {
      searchParams.set('tab', tab)
    }
    setSearchParams(searchParams, { replace: true })
  }, [searchParams, setSearchParams])

  const [files, setFiles] = useState<Media[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [loading, setLoading] = useState(true)
  const [stats, setStats] = useState<FileManagerStats | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])

  const [keyword, setKeyword] = useState('')
  const [filterLibrary, setFilterLibrary] = useState('')
  const [filterMediaType, setFilterMediaType] = useState('')
  const [filterScraped, setFilterScraped] = useState('')
  const [sortBy, setSortBy] = useState('created_at')
  const [sortOrder, setSortOrder] = useState('desc')
  const [showFilters, setShowFilters] = useState(false)

  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [viewMode, setViewMode] = useState<'table' | 'grid'>('table')

  const [folderTree, setFolderTree] = useState<FolderNode[]>([])
  const [folderTreeLoading, setFolderTreeLoading] = useState(false)
  const [currentFolderPath, setCurrentFolderPath] = useState('')
  const [subFolders, setSubFolders] = useState<string[]>([])
  const [showFolderPanel, setShowFolderPanel] = useState(true)

  const [folderDialog, setFolderDialog] = useState<FolderDialogType>('none')
  const [folderDialogTarget, setFolderDialogTarget] = useState('')
  const [folderInputValue, setFolderInputValue] = useState('')

  const [showAIPanel, setShowAIPanel] = useState(false)
  const [activeDialog, setActiveDialog] = useState<DialogType>('none')
  const [editMedia, setEditMedia] = useState<Media | null>(null)
  const [detailMedia, setDetailMedia] = useState<Media | null>(null)
  const [scrapeSource, setScrapeSource] = useState('')

  const fetchFiles = useCallback(async () => {
    setLoading(true)
    try {
      if (currentFolderPath) {
        const res = await fileManagerApi.listFilesByFolder({
          path: currentFolderPath,
          page,
          size: pageSize,
          library_id: filterLibrary,
          media_type: filterMediaType,
          keyword,
          sort_by: sortBy,
          sort_order: sortOrder,
          scraped: filterScraped,
        })
        setFiles(res.data.data || [])
        setTotal(res.data.total)
        setSubFolders(res.data.sub_folders || [])
      } else {
        const res = await fileManagerApi.listFiles({
          page,
          size: pageSize,
          library_id: filterLibrary,
          media_type: filterMediaType,
          keyword,
          sort_by: sortBy,
          sort_order: sortOrder,
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
      const res = await fileManagerApi.getStats({
        library_id: filterLibrary || undefined,
        folder_path: currentFolderPath || undefined,
      })
      setStats(res.data.data)
    } catch { /* ignore */ }
  }, [filterLibrary, currentFolderPath])

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

  useEffect(() => {
    const handleUpdate = () => { fetchFiles(); fetchStats() }
    const handleGlobalUpdate = () => { fetchFiles(); fetchStats(); fetchFolderTree() }
    const handleScrapeCompleted = () => {
      bumpPosterVersion()
      fetchFiles()
      fetchStats()
    }

    on('file_imported', handleUpdate)
    on('file_deleted', handleUpdate)
    on('batch_rename_complete', handleUpdate)
    on('file_scrape_progress', handleUpdate)
    on('scan_completed', handleGlobalUpdate)
    on('scan_phase', handleUpdate)
    on('scrape_completed', handleScrapeCompleted)
    on('library_updated', handleGlobalUpdate)
    on('adult_batch_completed', handleGlobalUpdate)
    on('folder_renamed', handleGlobalUpdate)
    on('folder_deleted', handleGlobalUpdate)

    return () => {
      off('file_imported', handleUpdate)
      off('file_deleted', handleUpdate)
      off('batch_rename_complete', handleUpdate)
      off('file_scrape_progress', handleUpdate)
      off('scan_completed', handleGlobalUpdate)
      off('scan_phase', handleUpdate)
      off('scrape_completed', handleScrapeCompleted)
      off('library_updated', handleGlobalUpdate)
      off('adult_batch_completed', handleGlobalUpdate)
      off('folder_renamed', handleGlobalUpdate)
      off('folder_deleted', handleGlobalUpdate)
    }
  }, [on, off, fetchFiles, fetchStats, fetchFolderTree])

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    setSelectedIds(prev => prev.size === files.length ? new Set() : new Set(files.map(f => f.id)))
  }

  const handleDeleteFile = async (id: string) => {
    const ok = await dialog.confirm({
      title: '删除文件记录',
      message: '确定要删除此文件记录吗？（原始文件不会被删除）',
      confirmText: '删除',
      variant: 'danger',
    })
    if (!ok) return
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
    const ok = await dialog.confirm({
      title: '批量删除文件记录',
      message: `确定要删除选中的 ${selectedIds.size} 个文件记录吗？（原始文件不会被删除）`,
      confirmText: '删除',
      variant: 'danger',
    })
    if (!ok) return
    try {
      const res = await fileManagerApi.batchDeleteFiles(Array.from(selectedIds))
      toast.success(`已删除 ${res.data.deleted} 个文件记录`)
      setSelectedIds(new Set())
      fetchFiles(); fetchStats()
    } catch {
      toast.error('批量删除失败')
    }
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
    } catch {
      toast.error('批量刮削失败')
    }
  }

  const refreshData = () => { fetchFiles(); fetchStats(); fetchFolderTree() }
  const totalPages = Math.ceil(total / pageSize)
  const pageSizeOptions = [20, 50, 100, 200]

  const handlePageSizeChange = useCallback((size: number) => {
    setPageSize(size)
    setPage(1)
  }, [])

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

  const handleCreateFolder = useCallback((parentPath: string) => {
    setFolderDialogTarget(parentPath)
    setFolderInputValue('')
    setFolderDialog('createFolder')
  }, [])

  const handleRenameFolder = useCallback((folderPath: string) => {
    setFolderDialogTarget(folderPath)
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
    window.open(`/play/${media.id}`, '_blank')
  }, [])

  const executeCreateFolder = useCallback(async () => {
    if (!folderInputValue.trim()) {
      toast.error('文件夹名不能为空')
      return
    }
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
    if (!folderInputValue.trim()) {
      toast.error('文件夹名不能为空')
      return
    }
    try {
      await fileManagerApi.renameFolder(folderDialogTarget, folderInputValue.trim())
      toast.success('文件夹重命名成功')
      setFolderDialog('none')
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

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (activeTab !== 'files') return
      if (activeDialog !== 'none' || folderDialog !== 'none') return
      if (event.key === 'Delete' && selectedIds.size > 0) {
        event.preventDefault()
        handleBatchDelete()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [activeTab, activeDialog, folderDialog, selectedIds, handleBatchDelete])

  return (
    <FileManagerShell
      activeTab={activeTab}
      onTabChange={handleTabChange}
      onOpenLogs={() => setActiveDialog('logs')}
      onRefresh={refreshData}
    >
      {activeTab === 'scrape' && (
        <div className="space-y-5">
          <STRMConfigSection />
          <ScrapeManagerPage embedded />
        </div>
      )}

      {activeTab === 'adult' && (
        <div className="space-y-5">
          <AdultScraperSection />
          <AdultScraperProSection />
        </div>
      )}

      {activeTab === 'files' && (
        <>
          {stats && <FileStatsBar stats={stats} />}

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
            <AIAssistantButton
              isOpen={showAIPanel}
              onToggle={() => setShowAIPanel(!showAIPanel)}
              selectedCount={selectedIds.size}
            />
          </FileToolbar>

          {currentFolderPath && (
            <div className="flex items-center gap-2">
              <Breadcrumb
                folderPath={currentFolderPath}
                onNavigate={handleSelectFolder}
                onGoHome={handleClearFolder}
              />
            </div>
          )}

          <div className="flex gap-4">
            <div
              className={clsx(
                'hidden flex-shrink-0 overflow-hidden transition-all duration-300 ease-out lg:block',
                showFolderPanel ? 'w-64 opacity-100' : 'w-0 opacity-0',
              )}
              style={{
                height: showFolderPanel ? 'calc(100vh - 280px)' : 0,
                maxHeight: showFolderPanel ? 'calc(100vh - 280px)' : 0,
              }}
            >
              <div className="h-full w-64">
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

            <div className="min-w-0 flex-1 space-y-4">
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowFolderPanel(!showFolderPanel)}
                  className="hidden lg:inline-flex"
                  title={showFolderPanel ? '收起文件夹面板' : '展开文件夹面板'}
                >
                  {showFolderPanel
                    ? <PanelLeftClose size={14} aria-hidden="true" />
                    : <PanelLeftOpen size={14} aria-hidden="true" />}
                  {showFolderPanel ? '收起导航' : '展开导航'}
                </Button>
                {currentFolderPath && (
                  <Tag tone="brand">
                    当前目录：{currentFolderPath.replace(/\\/g, '/').split('/').pop()}
                  </Tag>
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
        </>
      )}

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

      {folderDialog !== 'none' && (
        <FolderOperationModal
          mode={folderDialog}
          targetPath={folderDialogTarget}
          value={folderInputValue}
          onValueChange={setFolderInputValue}
          onClose={() => setFolderDialog('none')}
          onCreate={executeCreateFolder}
          onRename={executeRenameFolder}
          onDelete={executeDeleteFolder}
        />
      )}
    </FileManagerShell>
  )
}
