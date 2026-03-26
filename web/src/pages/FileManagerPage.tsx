import { useState, useEffect, useCallback } from 'react'
import type { Media, Library, FileManagerStats, RenamePreview, RenameTemplate, FileOperationLog, ScannedFile, FileImportRequest } from '@/types'
import { fileManagerApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import AIAssistant from '@/components/AIAssistant'
import { useWebSocket } from '@/hooks/useWebSocket'
import {
  FolderOpen,
  Plus,
  Upload,
  Search,
  Trash2,
  Download,
  RefreshCw,
  Loader2,
  Check,
  X,
  AlertCircle,
  Edit3,
  Eye,
  Clock,
  BarChart3,
  FileText,
  Sparkles,
  Filter,
  CheckSquare,
  Square,
  Film,
  Tv,
  HardDrive,
  Tag,
  Wand2,
  ChevronDown,
  ChevronRight,
  ArrowUpDown,
  History,
  ScanLine,
  FileVideo,
  Copy,
  Languages,
  ChevronsUpDown,
} from 'lucide-react'
import clsx from 'clsx'

// 数据源选项
const SOURCE_OPTIONS = [
  { value: '', label: '自动 (TMDb)' },
  { value: 'tmdb', label: 'TMDb' },
  { value: 'bangumi', label: 'Bangumi' },
  { value: 'ai', label: 'AI增强' },
]

// 语言选项
const LANGUAGE_OPTIONS = [
  { value: '', label: '不翻译（保持原语言）', flag: '🌐' },
  { value: 'zh', label: '中文', flag: '🇨🇳' },
  { value: 'en', label: 'English', flag: '🇺🇸' },
  { value: 'ja', label: '日本語', flag: '🇯🇵' },
  { value: 'ko', label: '한국어', flag: '🇰🇷' },
  { value: 'fr', label: 'Français', flag: '🇫🇷' },
  { value: 'de', label: 'Deutsch', flag: '🇩🇪' },
  { value: 'es', label: 'Español', flag: '🇪🇸' },
  { value: 'pt', label: 'Português', flag: '🇧🇷' },
  { value: 'ru', label: 'Русский', flag: '🇷🇺' },
  { value: 'it', label: 'Italiano', flag: '🇮🇹' },
  { value: 'th', label: 'ไทย', flag: '🇹🇭' },
  { value: 'vi', label: 'Tiếng Việt', flag: '🇻🇳' },
]

// 排序选项
const SORT_OPTIONS = [
  { value: 'created_at', label: '导入时间' },
  { value: 'title', label: '标题' },
  { value: 'year', label: '年份' },
  { value: 'rating', label: '评分' },
  { value: 'file_size', label: '文件大小' },
  { value: 'updated_at', label: '更新时间' },
]

// 格式化文件大小
function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

export default function FileManagerPage() {
  const toast = useToast()
  const { on, off } = useWebSocket()

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
  const [activeDialog, setActiveDialog] = useState<
    'none' | 'import' | 'batchImport' | 'scanDir' | 'edit' | 'detail' | 'rename' | 'logs'
  >('none')

  // 导入相关
  const [importPath, setImportPath] = useState('')
  const [importTitle, setImportTitle] = useState('')
  const [importMediaType, setImportMediaType] = useState('movie')
  const [importLibraryId, setImportLibraryId] = useState('')
  const [importing, setImporting] = useState(false)

  // 扫描目录
  const [scanPath, setScanPath] = useState('')
  const [scannedFiles, setScannedFiles] = useState<ScannedFile[]>([])
  const [scanning, setScanning] = useState(false)
  const [scanSelectedPaths, setScanSelectedPaths] = useState<Set<string>>(new Set())

  // 编辑
  const [editMedia, setEditMedia] = useState<Media | null>(null)
  const [editForm, setEditForm] = useState<Record<string, unknown>>({})

  // 详情
  const [detailMedia, setDetailMedia] = useState<Media | null>(null)

  // 重命名
  const [renameTemplate, setRenameTemplate] = useState('{title} ({year}) [{resolution}]')
  const [renamePreviews, setRenamePreviews] = useState<RenamePreview[]>([])
  const [renameTemplates, setRenameTemplates] = useState<RenameTemplate[]>([])
  const [renaming, setRenaming] = useState(false)
  const [useAIRename, setUseAIRename] = useState(false)
  const [targetLang, setTargetLang] = useState(() => {
    // 从 localStorage 恢复语言偏好
    return localStorage.getItem('rename_target_lang') || ''
  })
  const [previewsExpanded, setPreviewsExpanded] = useState(true)

  // 操作日志
  const [opLogs, setOpLogs] = useState<FileOperationLog[]>([])

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

  useEffect(() => {
    fetchFiles()
  }, [fetchFiles])

  useEffect(() => {
    fetchStats()
    fetchLibraries()
  }, [fetchStats, fetchLibraries])

  // WebSocket 实时更新
  useEffect(() => {
    const handleUpdate = () => {
      fetchFiles()
      fetchStats()
    }
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
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (selectedIds.size === files.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(files.map(f => f.id)))
    }
  }

  // ==================== 导入操作 ====================

  const handleImportFile = async () => {
    if (!importPath) {
      toast.error('请输入文件路径')
      return
    }
    setImporting(true)
    try {
      await fileManagerApi.importFile({
        file_path: importPath,
        title: importTitle || undefined,
        media_type: importMediaType,
        library_id: importLibraryId || undefined,
      })
      toast.success('文件导入成功')
      setActiveDialog('none')
      setImportPath('')
      setImportTitle('')
      fetchFiles()
      fetchStats()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '导入失败')
    } finally {
      setImporting(false)
    }
  }

  const handleScanDirectory = async () => {
    if (!scanPath) {
      toast.error('请输入目录路径')
      return
    }
    setScanning(true)
    try {
      const res = await fileManagerApi.scanDirectory(scanPath)
      setScannedFiles(res.data.data || [])
      setScanSelectedPaths(new Set())
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '扫描失败')
    } finally {
      setScanning(false)
    }
  }

  const handleBatchImportScanned = async () => {
    const filesToImport: FileImportRequest[] = Array.from(scanSelectedPaths).map(path => {
      const file = scannedFiles.find(f => f.path === path)
      return {
        file_path: path,
        title: file?.title || '',
        media_type: importMediaType,
        library_id: importLibraryId || undefined,
      }
    })

    if (filesToImport.length === 0) {
      toast.error('请选择要导入的文件')
      return
    }

    setImporting(true)
    try {
      const res = await fileManagerApi.batchImportFiles(filesToImport)
      const result = res.data.data
      toast.success(`导入完成: 成功 ${result.success}, 跳过 ${result.skipped}, 失败 ${result.failed}`)
      setActiveDialog('none')
      fetchFiles()
      fetchStats()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '批量导入失败')
    } finally {
      setImporting(false)
    }
  }

  // ==================== 编辑操作 ====================

  const openEdit = (media: Media) => {
    setEditMedia(media)
    setEditForm({
      title: media.title,
      orig_title: media.orig_title,
      year: media.year,
      overview: media.overview,
      genres: media.genres,
      rating: media.rating,
      media_type: media.media_type,
      country: media.country,
      language: media.language,
    })
    setActiveDialog('edit')
  }

  const handleSaveEdit = async () => {
    if (!editMedia) return
    try {
      await fileManagerApi.updateFile(editMedia.id, editForm)
      toast.success('更新成功')
      setActiveDialog('none')
      fetchFiles()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '更新失败')
    }
  }

  // ==================== 删除操作 ====================

  const handleDeleteFile = async (id: string) => {
    if (!confirm('确定要删除此文件记录吗？（原始文件不会被删除）')) return
    try {
      await fileManagerApi.deleteFile(id)
      toast.success('文件记录已删除')
      setSelectedIds(prev => { const n = new Set(prev); n.delete(id); return n })
      fetchFiles()
      fetchStats()
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
      fetchFiles()
      fetchStats()
    } catch {
      toast.error('批量删除失败')
    }
  }

  // ==================== 刮削操作 ====================

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

  // ==================== 重命名操作 ====================

  const openRenameDialog = async () => {
    if (selectedIds.size === 0) {
      toast.error('请先选择要重命名的文件')
      return
    }
    setActiveDialog('rename')
    setRenamePreviews([])
    try {
      const res = await fileManagerApi.getRenameTemplates()
      setRenameTemplates(res.data.data || [])
    } catch { /* ignore */ }
  }

  // 保存语言偏好到 localStorage
  const handleTargetLangChange = (lang: string) => {
    setTargetLang(lang)
    localStorage.setItem('rename_target_lang', lang)
    // 语言切换后清空预览，提示用户重新生成
    if (renamePreviews.length > 0) {
      setRenamePreviews([])
    }
  }

  const handlePreviewRename = async () => {
    setRenaming(true)
    try {
      let res
      if (useAIRename) {
        res = await fileManagerApi.aiGenerateRenames(Array.from(selectedIds), targetLang || undefined)
      } else {
        res = await fileManagerApi.previewRename(Array.from(selectedIds), renameTemplate)
      }
      setRenamePreviews(res.data.data || [])
      setPreviewsExpanded(true)
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '生成预览失败')
    } finally {
      setRenaming(false)
    }
  }

  const handleExecuteRename = async () => {
    setRenaming(true)
    try {
      const res = await fileManagerApi.executeRename(Array.from(selectedIds), renameTemplate)
      toast.success(`已重命名 ${res.data.renamed} 个文件`)
      setActiveDialog('none')
      setRenamePreviews([])
      fetchFiles()
    } catch {
      toast.error('重命名失败')
    } finally {
      setRenaming(false)
    }
  }

  // ==================== 日志 ====================

  const openLogs = async () => {
    setActiveDialog('logs')
    try {
      const res = await fileManagerApi.getOperationLogs(50)
      setOpLogs(res.data.data || [])
    } catch { /* ignore */ }
  }

  // ==================== 渲染 ====================

  const totalPages = Math.ceil(total / 20)

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
        <div className="flex items-center gap-2">
          <button onClick={openLogs} className="btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
            <History size={16} /> 操作日志
          </button>
          <button onClick={() => { fetchFiles(); fetchStats() }} className="btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
            <RefreshCw size={16} /> 刷新
          </button>
        </div>
      </div>

      {/* 统计卡片 */}
      {stats && (
        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-8 gap-3">
          {[
            { label: '总文件', value: stats.total_files, icon: FileVideo, color: 'text-blue-400' },
            { label: '电影', value: stats.movie_count, icon: Film, color: 'text-purple-400' },
            { label: '剧集', value: stats.episode_count, icon: Tv, color: 'text-green-400' },
            { label: '已刮削', value: stats.scraped_count, icon: Check, color: 'text-emerald-400' },
            { label: '未刮削', value: stats.unscraped_count, icon: AlertCircle, color: 'text-amber-400' },
            { label: '总大小', value: formatFileSize(stats.total_size_bytes), icon: HardDrive, color: 'text-cyan-400' },
            { label: '近7天导入', value: stats.recent_imports, icon: Download, color: 'text-indigo-400' },
            { label: '操作记录', value: stats.recent_operations, icon: FileText, color: 'text-pink-400' },
          ].map((item, i) => (
            <div key={i} className="glass-panel rounded-xl p-3 text-center">
              <item.icon size={18} className={clsx('mx-auto mb-1', item.color)} />
              <div className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>{item.value}</div>
              <div className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{item.label}</div>
            </div>
          ))}
        </div>
      )}

      {/* 工具栏 */}
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
              onChange={e => { setKeyword(e.target.value); setPage(1) }}
              className="input-field w-full pl-9 pr-3 py-2 text-sm rounded-lg"
            />
          </div>

          {/* 筛选切换 */}
          <button
            onClick={() => setShowFilters(!showFilters)}
            className={clsx('btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm', showFilters && 'text-neon')}
          >
            <Filter size={16} /> 筛选
            {showFilters ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          </button>

          <div className="h-6 w-px bg-surface-700" />

          {/* 导入按钮 */}
          <button onClick={() => setActiveDialog('import')} className="btn-primary flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
            <Plus size={16} /> 导入文件
          </button>
          <button onClick={() => { setActiveDialog('scanDir'); setScannedFiles([]); setScanPath('') }} className="btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
            <ScanLine size={16} /> 扫描目录
          </button>

          <div className="h-6 w-px bg-surface-700" />

          {/* 视图切换 */}
          <div className="flex items-center rounded-lg overflow-hidden border" style={{ borderColor: 'var(--border-default)' }}>
            <button
              onClick={() => setViewMode('table')}
              className={clsx('px-2.5 py-1.5 text-xs', viewMode === 'table' ? 'bg-neon-blue/20 text-neon' : 'text-surface-400')}
            >
              列表
            </button>
            <button
              onClick={() => setViewMode('grid')}
              className={clsx('px-2.5 py-1.5 text-xs', viewMode === 'grid' ? 'bg-neon-blue/20 text-neon' : 'text-surface-400')}
            >
              网格
            </button>
          </div>
        </div>

        {/* 筛选行 */}
        {showFilters && (
          <div className="flex flex-wrap items-center gap-3 pt-2 border-t" style={{ borderColor: 'var(--border-default)' }}>
            <select value={filterLibrary} onChange={e => { setFilterLibrary(e.target.value); setPage(1) }}
              className="input-field px-3 py-1.5 text-sm rounded-lg">
              <option value="">全部媒体库</option>
              {libraries.map(lib => <option key={lib.id} value={lib.id}>{lib.name}</option>)}
            </select>
            <select value={filterMediaType} onChange={e => { setFilterMediaType(e.target.value); setPage(1) }}
              className="input-field px-3 py-1.5 text-sm rounded-lg">
              <option value="">全部类型</option>
              <option value="movie">电影</option>
              <option value="episode">剧集</option>
            </select>
            <select value={filterScraped} onChange={e => { setFilterScraped(e.target.value); setPage(1) }}
              className="input-field px-3 py-1.5 text-sm rounded-lg">
              <option value="">全部状态</option>
              <option value="true">已刮削</option>
              <option value="false">未刮削</option>
            </select>
            <select value={sortBy} onChange={e => setSortBy(e.target.value)}
              className="input-field px-3 py-1.5 text-sm rounded-lg">
              {SORT_OPTIONS.map(opt => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
            </select>
            <button onClick={() => setSortOrder(sortOrder === 'desc' ? 'asc' : 'desc')}
              className="btn-ghost flex items-center gap-1 px-2 py-1.5 rounded-lg text-sm">
              <ArrowUpDown size={14} /> {sortOrder === 'desc' ? '降序' : '升序'}
            </button>
          </div>
        )}

        {/* 批量操作栏 */}
        {selectedIds.size > 0 && (
          <div className="flex items-center gap-2 pt-2 border-t" style={{ borderColor: 'var(--border-default)' }}>
            <span className="text-sm text-neon font-medium">已选 {selectedIds.size} 项</span>
            <div className="h-4 w-px bg-surface-700" />

            <select value={scrapeSource} onChange={e => setScrapeSource(e.target.value)}
              className="input-field px-2 py-1 text-xs rounded">
              {SOURCE_OPTIONS.map(opt => <option key={opt.value} value={opt.value}>{opt.label}</option>)}
            </select>
            <button onClick={handleBatchScrape} className="btn-ghost flex items-center gap-1 px-2.5 py-1.5 rounded text-xs text-blue-400 hover:bg-blue-400/10">
              <Sparkles size={14} /> 批量刮削
            </button>
            <button onClick={openRenameDialog} className="btn-ghost flex items-center gap-1 px-2.5 py-1.5 rounded text-xs text-purple-400 hover:bg-purple-400/10">
              <Wand2 size={14} /> 批量重命名
            </button>
            <button onClick={handleBatchDelete} className="btn-ghost flex items-center gap-1 px-2.5 py-1.5 rounded text-xs text-red-400 hover:bg-red-400/10">
              <Trash2 size={14} /> 批量删除
            </button>
            <button onClick={() => setSelectedIds(new Set())} className="btn-ghost px-2.5 py-1.5 rounded text-xs">
              取消选择
            </button>
          </div>
        )}
      </div>

      {/* 文件列表 */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 size={32} className="animate-spin text-neon" />
        </div>
      ) : files.length === 0 ? (
        <div className="glass-panel rounded-xl p-12 text-center">
          <FolderOpen size={48} className="mx-auto mb-4 text-surface-500" />
          <p className="text-lg font-medium" style={{ color: 'var(--text-secondary)' }}>暂无影视文件</p>
          <p className="text-sm mt-1" style={{ color: 'var(--text-tertiary)' }}>点击"导入文件"或"扫描目录"开始添加</p>
        </div>
      ) : viewMode === 'table' ? (
        /* 表格视图 */
        <div className="glass-panel rounded-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b" style={{ borderColor: 'var(--border-default)' }}>
                  <th className="px-3 py-3 text-left w-10">
                    <button onClick={toggleSelectAll}>
                      {selectedIds.size === files.length ? <CheckSquare size={16} className="text-neon" /> : <Square size={16} className="text-surface-500" />}
                    </button>
                  </th>
                  <th className="px-3 py-3 text-left" style={{ color: 'var(--text-secondary)' }}>标题</th>
                  <th className="px-3 py-3 text-left hidden md:table-cell" style={{ color: 'var(--text-secondary)' }}>类型</th>
                  <th className="px-3 py-3 text-left hidden lg:table-cell" style={{ color: 'var(--text-secondary)' }}>年份</th>
                  <th className="px-3 py-3 text-left hidden lg:table-cell" style={{ color: 'var(--text-secondary)' }}>评分</th>
                  <th className="px-3 py-3 text-left hidden xl:table-cell" style={{ color: 'var(--text-secondary)' }}>大小</th>
                  <th className="px-3 py-3 text-left hidden xl:table-cell" style={{ color: 'var(--text-secondary)' }}>状态</th>
                  <th className="px-3 py-3 text-right" style={{ color: 'var(--text-secondary)' }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {files.map(file => {
                  const isScraped = file.tmdb_id > 0 || file.bangumi_id > 0 || (file.douban_id && file.douban_id !== '')
                  return (
                    <tr key={file.id} className="border-b transition-colors hover:bg-white/[0.02]" style={{ borderColor: 'var(--border-default)' }}>
                      <td className="px-3 py-3">
                        <button onClick={() => toggleSelect(file.id)}>
                          {selectedIds.has(file.id) ? <CheckSquare size={16} className="text-neon" /> : <Square size={16} className="text-surface-500" />}
                        </button>
                      </td>
                      <td className="px-3 py-3">
                        <div className="flex items-center gap-3">
                          {file.poster_path ? (
                            <img src={file.poster_path} alt="" className="w-8 h-12 rounded object-cover flex-shrink-0" />
                          ) : (
                            <div className="w-8 h-12 rounded bg-surface-800 flex items-center justify-center flex-shrink-0">
                              <FileVideo size={16} className="text-surface-500" />
                            </div>
                          )}
                          <div className="min-w-0">
                            <div className="font-medium truncate" style={{ color: 'var(--text-primary)' }}>{file.title}</div>
                            {file.orig_title && file.orig_title !== file.title && (
                              <div className="text-xs truncate" style={{ color: 'var(--text-tertiary)' }}>{file.orig_title}</div>
                            )}
                            <div className="text-xs truncate mt-0.5" style={{ color: 'var(--text-tertiary)' }}>{file.file_path}</div>
                          </div>
                        </div>
                      </td>
                      <td className="px-3 py-3 hidden md:table-cell">
                        <span className={clsx('inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs',
                          file.media_type === 'movie' ? 'bg-purple-500/10 text-purple-400' : 'bg-green-500/10 text-green-400'
                        )}>
                          {file.media_type === 'movie' ? <Film size={12} /> : <Tv size={12} />}
                          {file.media_type === 'movie' ? '电影' : '剧集'}
                        </span>
                      </td>
                      <td className="px-3 py-3 hidden lg:table-cell" style={{ color: 'var(--text-secondary)' }}>
                        {file.year || '-'}
                      </td>
                      <td className="px-3 py-3 hidden lg:table-cell">
                        {file.rating > 0 ? (
                          <span className="text-amber-400">★ {file.rating.toFixed(1)}</span>
                        ) : (
                          <span style={{ color: 'var(--text-tertiary)' }}>-</span>
                        )}
                      </td>
                      <td className="px-3 py-3 hidden xl:table-cell" style={{ color: 'var(--text-secondary)' }}>
                        {file.file_size > 0 ? formatFileSize(file.file_size) : '-'}
                      </td>
                      <td className="px-3 py-3 hidden xl:table-cell">
                        {isScraped ? (
                          <span className="inline-flex items-center gap-1 text-xs text-green-400">
                            <Check size={12} /> 已刮削
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-xs text-amber-400">
                            <AlertCircle size={12} /> 未刮削
                          </span>
                        )}
                      </td>
                      <td className="px-3 py-3 text-right">
                        <div className="flex items-center justify-end gap-1">
                          <button onClick={() => { setDetailMedia(file); setActiveDialog('detail') }}
                            className="p-1.5 rounded hover:bg-white/5 text-surface-400 hover:text-blue-400" title="查看详情">
                            <Eye size={14} />
                          </button>
                          <button onClick={() => openEdit(file)}
                            className="p-1.5 rounded hover:bg-white/5 text-surface-400 hover:text-amber-400" title="编辑">
                            <Edit3 size={14} />
                          </button>
                          <button onClick={() => handleScrapeFile(file.id)}
                            className="p-1.5 rounded hover:bg-white/5 text-surface-400 hover:text-purple-400" title="刮削">
                            <Sparkles size={14} />
                          </button>
                          <button onClick={() => handleDeleteFile(file.id)}
                            className="p-1.5 rounded hover:bg-white/5 text-surface-400 hover:text-red-400" title="删除">
                            <Trash2 size={14} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        /* 网格视图 */
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
          {files.map(file => {
            const isScraped = file.tmdb_id > 0 || file.bangumi_id > 0 || (file.douban_id && file.douban_id !== '')
            return (
              <div key={file.id} className="glass-panel rounded-xl overflow-hidden group relative cursor-pointer"
                onClick={() => { setDetailMedia(file); setActiveDialog('detail') }}>
                {/* 选择框 */}
                <button className="absolute top-2 left-2 z-10" onClick={e => { e.stopPropagation(); toggleSelect(file.id) }}>
                  {selectedIds.has(file.id) ? <CheckSquare size={18} className="text-neon" /> : <Square size={18} className="text-white/50 group-hover:text-white/80" />}
                </button>
                {/* 海报 */}
                <div className="aspect-[2/3] bg-surface-800 relative">
                  {file.poster_path ? (
                    <img src={file.poster_path} alt="" className="w-full h-full object-cover" />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center">
                      <FileVideo size={32} className="text-surface-600" />
                    </div>
                  )}
                  {/* 状态标签 */}
                  <div className="absolute top-2 right-2">
                    {isScraped ? (
                      <span className="bg-green-500/80 text-white text-[10px] px-1.5 py-0.5 rounded">已刮削</span>
                    ) : (
                      <span className="bg-amber-500/80 text-white text-[10px] px-1.5 py-0.5 rounded">未刮削</span>
                    )}
                  </div>
                  {/* 悬浮操作 */}
                  <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-2">
                    <button onClick={e => { e.stopPropagation(); openEdit(file) }}
                      className="p-2 rounded-full bg-white/20 hover:bg-white/30 text-white" title="编辑">
                      <Edit3 size={16} />
                    </button>
                    <button onClick={e => { e.stopPropagation(); handleScrapeFile(file.id) }}
                      className="p-2 rounded-full bg-white/20 hover:bg-white/30 text-white" title="刮削">
                      <Sparkles size={16} />
                    </button>
                    <button onClick={e => { e.stopPropagation(); handleDeleteFile(file.id) }}
                      className="p-2 rounded-full bg-white/20 hover:bg-white/30 text-white" title="删除">
                      <Trash2 size={16} />
                    </button>
                  </div>
                </div>
                {/* 信息 */}
                <div className="p-2.5">
                  <div className="font-medium text-sm truncate" style={{ color: 'var(--text-primary)' }}>{file.title}</div>
                  <div className="flex items-center gap-2 mt-1 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                    <span>{file.year || '-'}</span>
                    {file.rating > 0 && <span className="text-amber-400">★ {file.rating.toFixed(1)}</span>}
                    <span className={file.media_type === 'movie' ? 'text-purple-400' : 'text-green-400'}>
                      {file.media_type === 'movie' ? '电影' : '剧集'}
                    </span>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <button onClick={() => setPage(Math.max(1, page - 1))} disabled={page <= 1}
            className="btn-ghost px-3 py-1.5 rounded text-sm disabled:opacity-30">上一页</button>
          <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {page} / {totalPages} (共 {total} 条)
          </span>
          <button onClick={() => setPage(Math.min(totalPages, page + 1))} disabled={page >= totalPages}
            className="btn-ghost px-3 py-1.5 rounded text-sm disabled:opacity-30">下一页</button>
        </div>
      )}

      {/* ==================== 对话框 ==================== */}

      {/* 导入文件对话框 */}
      {activeDialog === 'import' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setActiveDialog('none')}>
          <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-lg mx-4" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>
              <Plus className="inline-block mr-2 mb-0.5" size={20} /> 导入影视文件
            </h3>
            <div className="space-y-3">
              <div>
                <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>文件路径 *</label>
                <input type="text" value={importPath} onChange={e => setImportPath(e.target.value)}
                  placeholder="/path/to/movie.mkv" className="input-field w-full px-3 py-2 rounded-lg text-sm" />
              </div>
              <div>
                <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>标题（留空自动提取）</label>
                <input type="text" value={importTitle} onChange={e => setImportTitle(e.target.value)}
                  placeholder="自动从文件名提取" className="input-field w-full px-3 py-2 rounded-lg text-sm" />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>媒体类型</label>
                  <select value={importMediaType} onChange={e => setImportMediaType(e.target.value)}
                    className="input-field w-full px-3 py-2 rounded-lg text-sm">
                    <option value="movie">电影</option>
                    <option value="episode">剧集</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>媒体库</label>
                  <select value={importLibraryId} onChange={e => setImportLibraryId(e.target.value)}
                    className="input-field w-full px-3 py-2 rounded-lg text-sm">
                    <option value="">不指定</option>
                    {libraries.map(lib => <option key={lib.id} value={lib.id}>{lib.name}</option>)}
                  </select>
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-6">
              <button onClick={() => setActiveDialog('none')} className="btn-ghost px-4 py-2 rounded-lg text-sm">取消</button>
              <button onClick={handleImportFile} disabled={importing}
                className="btn-primary flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm">
                {importing ? <Loader2 size={14} className="animate-spin" /> : <Upload size={14} />}
                {importing ? '导入中...' : '导入'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 扫描目录对话框 */}
      {activeDialog === 'scanDir' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setActiveDialog('none')}>
          <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-3xl mx-4 max-h-[80vh] flex flex-col" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>
              <ScanLine className="inline-block mr-2 mb-0.5" size={20} /> 扫描目录导入
            </h3>
            {/* 扫描输入 */}
            <div className="flex gap-2 mb-4">
              <input type="text" value={scanPath} onChange={e => setScanPath(e.target.value)}
                placeholder="输入目录路径，如 /media/movies" className="input-field flex-1 px-3 py-2 rounded-lg text-sm" />
              <button onClick={handleScanDirectory} disabled={scanning}
                className="btn-primary flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm whitespace-nowrap">
                {scanning ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />}
                扫描
              </button>
            </div>
            {/* 导入设置 */}
            <div className="flex items-center gap-3 mb-3">
              <select value={importMediaType} onChange={e => setImportMediaType(e.target.value)}
                className="input-field px-2 py-1 text-xs rounded">
                <option value="movie">电影</option>
                <option value="episode">剧集</option>
              </select>
              <select value={importLibraryId} onChange={e => setImportLibraryId(e.target.value)}
                className="input-field px-2 py-1 text-xs rounded">
                <option value="">不指定媒体库</option>
                {libraries.map(lib => <option key={lib.id} value={lib.id}>{lib.name}</option>)}
              </select>
              {scannedFiles.length > 0 && (
                <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  找到 {scannedFiles.length} 个视频文件，已选 {scanSelectedPaths.size} 个
                </span>
              )}
            </div>
            {/* 文件列表 */}
            <div className="flex-1 overflow-y-auto min-h-0">
              {scannedFiles.length > 0 ? (
                <div className="space-y-1">
                  {/* 全选 */}
                  <button onClick={() => {
                    const unimported = scannedFiles.filter(f => !f.imported).map(f => f.path)
                    if (scanSelectedPaths.size === unimported.length) {
                      setScanSelectedPaths(new Set())
                    } else {
                      setScanSelectedPaths(new Set(unimported))
                    }
                  }} className="text-xs text-neon hover:underline mb-2">
                    {scanSelectedPaths.size === scannedFiles.filter(f => !f.imported).length ? '取消全选' : '全选未导入'}
                  </button>
                  {scannedFiles.map((file, i) => (
                    <div key={i} className={clsx(
                      'flex items-center gap-3 px-3 py-2 rounded-lg text-sm',
                      file.imported ? 'opacity-50' : 'hover:bg-white/[0.02]'
                    )}>
                      <button onClick={() => {
                        if (file.imported) return
                        setScanSelectedPaths(prev => {
                          const next = new Set(prev)
                          if (next.has(file.path)) next.delete(file.path)
                          else next.add(file.path)
                          return next
                        })
                      }} disabled={file.imported}>
                        {file.imported ? (
                          <Check size={16} className="text-green-400" />
                        ) : scanSelectedPaths.has(file.path) ? (
                          <CheckSquare size={16} className="text-neon" />
                        ) : (
                          <Square size={16} className="text-surface-500" />
                        )}
                      </button>
                      <FileVideo size={16} className="text-surface-400 flex-shrink-0" />
                      <div className="flex-1 min-w-0">
                        <div className="truncate" style={{ color: 'var(--text-primary)' }}>{file.name}</div>
                        <div className="text-xs truncate" style={{ color: 'var(--text-tertiary)' }}>{file.path}</div>
                      </div>
                      <span className="text-xs flex-shrink-0" style={{ color: 'var(--text-tertiary)' }}>
                        {formatFileSize(file.size)}
                      </span>
                      {file.imported && (
                        <span className="text-xs text-green-400 flex-shrink-0">已导入</span>
                      )}
                    </div>
                  ))}
                </div>
              ) : !scanning && (
                <div className="text-center py-8" style={{ color: 'var(--text-tertiary)' }}>
                  输入目录路径后点击扫描
                </div>
              )}
            </div>
            {/* 底部操作 */}
            <div className="flex justify-end gap-2 mt-4 pt-4 border-t" style={{ borderColor: 'var(--border-default)' }}>
              <button onClick={() => setActiveDialog('none')} className="btn-ghost px-4 py-2 rounded-lg text-sm">取消</button>
              <button onClick={handleBatchImportScanned} disabled={importing || scanSelectedPaths.size === 0}
                className="btn-primary flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm">
                {importing ? <Loader2 size={14} className="animate-spin" /> : <Upload size={14} />}
                导入选中 ({scanSelectedPaths.size})
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 编辑对话框 */}
      {activeDialog === 'edit' && editMedia && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setActiveDialog('none')}>
          <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-lg mx-4 max-h-[80vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>
              <Edit3 className="inline-block mr-2 mb-0.5" size={20} /> 编辑文件信息
            </h3>
            <div className="space-y-3">
              {[
                { key: 'title', label: '标题', type: 'text' },
                { key: 'orig_title', label: '原始标题', type: 'text' },
                { key: 'year', label: '年份', type: 'number' },
                { key: 'genres', label: '类型', type: 'text', placeholder: '动作,科幻,冒险' },
                { key: 'rating', label: '评分', type: 'number' },
                { key: 'country', label: '国家/地区', type: 'text' },
                { key: 'language', label: '语言', type: 'text' },
              ].map(field => (
                <div key={field.key}>
                  <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>{field.label}</label>
                  <input
                    type={field.type}
                    value={editForm[field.key] as string || ''}
                    onChange={e => setEditForm(prev => ({ ...prev, [field.key]: field.type === 'number' ? Number(e.target.value) : e.target.value }))}
                    placeholder={field.placeholder}
                    className="input-field w-full px-3 py-2 rounded-lg text-sm"
                  />
                </div>
              ))}
              <div>
                <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>简介</label>
                <textarea
                  value={editForm.overview as string || ''}
                  onChange={e => setEditForm(prev => ({ ...prev, overview: e.target.value }))}
                  rows={4}
                  className="input-field w-full px-3 py-2 rounded-lg text-sm resize-none"
                />
              </div>
              <div>
                <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>媒体类型</label>
                <select value={editForm.media_type as string || 'movie'}
                  onChange={e => setEditForm(prev => ({ ...prev, media_type: e.target.value }))}
                  className="input-field w-full px-3 py-2 rounded-lg text-sm">
                  <option value="movie">电影</option>
                  <option value="episode">剧集</option>
                </select>
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-6">
              <button onClick={() => setActiveDialog('none')} className="btn-ghost px-4 py-2 rounded-lg text-sm">取消</button>
              <button onClick={handleSaveEdit} className="btn-primary px-4 py-2 rounded-lg text-sm">保存</button>
            </div>
          </div>
        </div>
      )}

      {/* 详情对话框 */}
      {activeDialog === 'detail' && detailMedia && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setActiveDialog('none')}>
          <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-2xl mx-4 max-h-[80vh] overflow-y-auto" onClick={e => e.stopPropagation()}>
            <div className="flex justify-between items-start mb-4">
              <h3 className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>
                <Eye className="inline-block mr-2 mb-0.5" size={20} /> 文件详情
              </h3>
              <button onClick={() => setActiveDialog('none')} className="p-1 rounded hover:bg-white/10">
                <X size={18} />
              </button>
            </div>
            <div className="flex gap-4">
              {detailMedia.poster_path ? (
                <img src={detailMedia.poster_path} alt="" className="w-32 h-48 rounded-lg object-cover flex-shrink-0" />
              ) : (
                <div className="w-32 h-48 rounded-lg bg-surface-800 flex items-center justify-center flex-shrink-0">
                  <FileVideo size={32} className="text-surface-600" />
                </div>
              )}
              <div className="flex-1 space-y-2 text-sm">
                <div><span style={{ color: 'var(--text-tertiary)' }}>标题：</span><span style={{ color: 'var(--text-primary)' }}>{detailMedia.title}</span></div>
                {detailMedia.orig_title && <div><span style={{ color: 'var(--text-tertiary)' }}>原始标题：</span><span style={{ color: 'var(--text-secondary)' }}>{detailMedia.orig_title}</span></div>}
                <div><span style={{ color: 'var(--text-tertiary)' }}>年份：</span><span style={{ color: 'var(--text-secondary)' }}>{detailMedia.year || '-'}</span></div>
                <div><span style={{ color: 'var(--text-tertiary)' }}>评分：</span>{detailMedia.rating > 0 ? <span className="text-amber-400">★ {detailMedia.rating.toFixed(1)}</span> : <span style={{ color: 'var(--text-secondary)' }}>-</span>}</div>
                <div><span style={{ color: 'var(--text-tertiary)' }}>类型：</span><span style={{ color: 'var(--text-secondary)' }}>{detailMedia.genres || '-'}</span></div>
                <div><span style={{ color: 'var(--text-tertiary)' }}>媒体类型：</span><span className={detailMedia.media_type === 'movie' ? 'text-purple-400' : 'text-green-400'}>{detailMedia.media_type === 'movie' ? '电影' : '剧集'}</span></div>
                <div><span style={{ color: 'var(--text-tertiary)' }}>分辨率：</span><span style={{ color: 'var(--text-secondary)' }}>{detailMedia.resolution || '-'}</span></div>
                <div><span style={{ color: 'var(--text-tertiary)' }}>文件大小：</span><span style={{ color: 'var(--text-secondary)' }}>{detailMedia.file_size > 0 ? formatFileSize(detailMedia.file_size) : '-'}</span></div>
                <div><span style={{ color: 'var(--text-tertiary)' }}>国家：</span><span style={{ color: 'var(--text-secondary)' }}>{detailMedia.country || '-'}</span></div>
                <div><span style={{ color: 'var(--text-tertiary)' }}>语言：</span><span style={{ color: 'var(--text-secondary)' }}>{detailMedia.language || '-'}</span></div>
                <div className="flex items-center gap-2">
                  <span style={{ color: 'var(--text-tertiary)' }}>TMDb ID：</span>
                  <span style={{ color: 'var(--text-secondary)' }}>{detailMedia.tmdb_id || '-'}</span>
                  {detailMedia.bangumi_id > 0 && <><span style={{ color: 'var(--text-tertiary)' }}>Bangumi：</span><span style={{ color: 'var(--text-secondary)' }}>{detailMedia.bangumi_id}</span></>}
                </div>
              </div>
            </div>
            {detailMedia.overview && (
              <div className="mt-4">
                <div className="text-sm mb-1" style={{ color: 'var(--text-tertiary)' }}>简介</div>
                <p className="text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>{detailMedia.overview}</p>
              </div>
            )}
            <div className="mt-4 p-3 rounded-lg" style={{ background: 'var(--bg-secondary)' }}>
              <div className="text-xs flex items-center gap-1 mb-1" style={{ color: 'var(--text-tertiary)' }}>
                <HardDrive size={12} /> 文件路径
              </div>
              <div className="text-xs font-mono break-all" style={{ color: 'var(--text-secondary)' }}>{detailMedia.file_path}</div>
            </div>
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => { openEdit(detailMedia); }} className="btn-ghost flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
                <Edit3 size={14} /> 编辑
              </button>
              <button onClick={() => { handleScrapeFile(detailMedia.id); setActiveDialog('none') }}
                className="btn-primary flex items-center gap-1.5 px-3 py-2 rounded-lg text-sm">
                <Sparkles size={14} /> 刮削元数据
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 重命名对话框 */}
      {activeDialog === 'rename' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setActiveDialog('none')}>
          <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-3xl mx-4 max-h-[85vh] flex flex-col" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>
              <Wand2 className="inline-block mr-2 mb-0.5" size={20} /> AI批量重命名
              <span className="text-xs font-normal ml-2 px-2 py-0.5 rounded-full bg-purple-500/10 text-purple-400">
                已选 {selectedIds.size} 个文件
              </span>
            </h3>

            {/* 模式选择 */}
            <div className="flex items-center gap-4 mb-4">
              <label className="flex items-center gap-2 cursor-pointer group">
                <input type="radio" checked={!useAIRename} onChange={() => setUseAIRename(false)}
                  className="accent-[var(--neon-blue)]" />
                <span className={clsx('text-sm transition-colors', !useAIRename ? 'text-neon font-medium' : '')}
                  style={{ color: useAIRename ? 'var(--text-secondary)' : undefined }}>模板重命名</span>
              </label>
              <label className="flex items-center gap-2 cursor-pointer group">
                <input type="radio" checked={useAIRename} onChange={() => setUseAIRename(true)}
                  className="accent-[var(--neon-blue)]" />
                <span className={clsx('text-sm transition-colors', useAIRename ? 'text-neon font-medium' : '')}
                  style={{ color: !useAIRename ? 'var(--text-secondary)' : undefined }}>
                  <Sparkles size={12} className="inline-block mr-1 mb-0.5" />AI智能重命名
                </span>
              </label>
            </div>

            {/* 模板选择 */}
            {!useAIRename && (
              <div className="mb-4 space-y-2">
                <label className="block text-sm" style={{ color: 'var(--text-secondary)' }}>命名模板</label>
                <input type="text" value={renameTemplate} onChange={e => setRenameTemplate(e.target.value)}
                  className="input-field w-full px-3 py-2 rounded-lg text-sm font-mono" />
                <div className="flex flex-wrap gap-1.5">
                  {renameTemplates.map((t, i) => (
                    <button key={i} onClick={() => setRenameTemplate(t.pattern)}
                      className={clsx('px-2 py-1 rounded text-xs border transition-all', renameTemplate === t.pattern ? 'border-neon text-neon bg-neon-blue/5' : 'border-surface-700 text-surface-400 hover:border-surface-500')}
                      title={`示例: ${t.example}`}>
                      {t.pattern}
                    </button>
                  ))}
                </div>
                <div className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  可用变量: {'{title}'} {'{orig_title}'} {'{year}'} {'{resolution}'} {'{media_type}'}
                </div>
              </div>
            )}

            {/* AI模式 - 语言选择器 */}
            {useAIRename && (
              <div className="mb-4 space-y-2">
                <label className="flex items-center gap-1.5 text-sm" style={{ color: 'var(--text-secondary)' }}>
                  <Languages size={14} /> 目标翻译语言
                </label>
                <div className="flex flex-wrap gap-1.5">
                  {LANGUAGE_OPTIONS.map(lang => (
                    <button
                      key={lang.value}
                      onClick={() => handleTargetLangChange(lang.value)}
                      className={clsx(
                        'flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs border transition-all',
                        targetLang === lang.value
                          ? 'border-neon text-neon bg-neon-blue/10 shadow-[0_0_8px_rgba(0,170,255,0.15)]'
                          : 'border-surface-700 text-surface-400 hover:border-surface-500 hover:text-surface-300'
                      )}
                    >
                      <span>{lang.flag}</span>
                      <span>{lang.label}</span>
                    </button>
                  ))}
                </div>
                {targetLang && (
                  <div className="text-xs flex items-center gap-1" style={{ color: 'var(--text-tertiary)' }}>
                    <Sparkles size={10} />
                    AI将生成规范化标题并翻译为 {LANGUAGE_OPTIONS.find(l => l.value === targetLang)?.label}
                  </div>
                )}
              </div>
            )}

            {/* 预览按钮 */}
            <div className="flex items-center gap-3 mb-4">
              <button onClick={handlePreviewRename} disabled={renaming}
                className="btn-ghost flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm">
                {renaming ? <Loader2 size={14} className="animate-spin" /> : <Eye size={14} />}
                {renaming ? '生成中...' : '生成预览'}
              </button>
              {renamePreviews.length > 0 && (
                <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  共 {renamePreviews.length} 条预览结果
                </span>
              )}
            </div>

            {/* 预览列表 - 带滚动条和折叠控制 */}
            <div className="flex-1 min-h-0 flex flex-col">
              {renamePreviews.length > 0 ? (
                <>
                  {/* 折叠/展开控制 */}
                  <div className="flex items-center justify-between mb-2">
                    <button
                      onClick={() => setPreviewsExpanded(!previewsExpanded)}
                      className="flex items-center gap-1 text-xs hover:text-neon transition-colors"
                      style={{ color: 'var(--text-tertiary)' }}
                    >
                      <ChevronsUpDown size={12} />
                      {previewsExpanded ? '折叠预览' : '展开预览'}
                    </button>
                    <span className="text-xs px-2 py-0.5 rounded-full bg-green-500/10 text-green-400">
                      {renamePreviews.length} 项将被重命名
                    </span>
                  </div>
                  {/* 预览内容区域 - 带自定义滚动条 */}
                  {previewsExpanded && (
                    <div
                      className="overflow-y-auto pr-1"
                      style={{
                        maxHeight: 'calc(85vh - 420px)',
                        minHeight: '120px',
                        scrollBehavior: 'smooth',
                        scrollbarWidth: 'thin',
                        scrollbarColor: 'rgba(255,255,255,0.15) transparent',
                      }}
                    >
                      <div className="space-y-2">
                        {renamePreviews.map((p, i) => (
                          <div key={i} className="p-3 rounded-lg transition-colors hover:ring-1 hover:ring-white/5" style={{ background: 'var(--bg-secondary)' }}>
                            <div className="flex items-start gap-3 text-sm">
                              <span className="text-xs font-mono text-surface-500 mt-0.5 flex-shrink-0 w-5 text-right">{i + 1}</span>
                              <div className="flex-1 min-w-0 space-y-1">
                                <div className="flex items-center gap-2 flex-wrap">
                                  <span className="text-red-400/80 line-through break-all" style={{ wordBreak: 'break-word' }}>{p.old_title}</span>
                                </div>
                                <div className="flex items-center gap-2">
                                  <span style={{ color: 'var(--text-tertiary)' }}>↓</span>
                                  <span className="text-green-400 font-medium break-all" style={{ wordBreak: 'break-word' }}>{p.new_title}</span>
                                </div>
                              </div>
                            </div>
                            {p.reason && (
                              <div className="text-xs mt-2 ml-8 flex items-center gap-1" style={{ color: 'var(--text-tertiary)' }}>
                                <Sparkles size={10} className="flex-shrink-0" />
                                {p.reason}
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </>
              ) : (
                <div className="flex-1 flex items-center justify-center">
                  <div className="text-center py-8">
                    <Wand2 size={32} className="mx-auto mb-3 text-surface-600" />
                    <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>
                      {useAIRename ? '选择语言后点击"生成预览"查看AI重命名效果' : '点击"生成预览"查看重命名效果'}
                    </p>
                  </div>
                </div>
              )}
            </div>

            {/* 底部操作 */}
            <div className="flex justify-end gap-2 mt-4 pt-4 border-t" style={{ borderColor: 'var(--border-default)' }}>
              <button onClick={() => setActiveDialog('none')} className="btn-ghost px-4 py-2 rounded-lg text-sm">取消</button>
              <button onClick={handleExecuteRename} disabled={renaming || renamePreviews.length === 0}
                className="btn-primary flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm">
                {renaming ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                执行重命名 {renamePreviews.length > 0 && `(${renamePreviews.length})`}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 操作日志对话框 */}
      {activeDialog === 'logs' && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setActiveDialog('none')}>
          <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-2xl mx-4 max-h-[80vh] flex flex-col" onClick={e => e.stopPropagation()}>
            <h3 className="text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>
              <History className="inline-block mr-2 mb-0.5" size={20} /> 操作日志
            </h3>
            <div className="flex-1 overflow-y-auto min-h-0">
              {opLogs.length > 0 ? (
                <div className="space-y-2">
                  {opLogs.map(log => (
                    <div key={log.id} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: 'var(--bg-secondary)' }}>
                      <div className={clsx('mt-0.5 p-1 rounded', {
                        'bg-green-500/10 text-green-400': log.action === 'import',
                        'bg-blue-500/10 text-blue-400': log.action === 'edit',
                        'bg-red-500/10 text-red-400': log.action === 'delete',
                        'bg-purple-500/10 text-purple-400': log.action === 'scrape',
                        'bg-amber-500/10 text-amber-400': log.action === 'rename',
                      })}>
                        {log.action === 'import' && <Upload size={14} />}
                        {log.action === 'edit' && <Edit3 size={14} />}
                        {log.action === 'delete' && <Trash2 size={14} />}
                        {log.action === 'scrape' && <Sparkles size={14} />}
                        {log.action === 'rename' && <Wand2 size={14} />}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="text-sm" style={{ color: 'var(--text-primary)' }}>{log.detail}</div>
                        <div className="text-xs mt-0.5" style={{ color: 'var(--text-tertiary)' }}>
                          {new Date(log.created_at).toLocaleString()}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-center py-8 text-sm" style={{ color: 'var(--text-tertiary)' }}>暂无操作记录</div>
              )}
            </div>
            <div className="flex justify-end mt-4 pt-4 border-t" style={{ borderColor: 'var(--border-default)' }}>
              <button onClick={() => setActiveDialog('none')} className="btn-ghost px-4 py-2 rounded-lg text-sm">关闭</button>
            </div>
          </div>
        </div>
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
