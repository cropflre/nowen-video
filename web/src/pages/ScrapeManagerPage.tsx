import { useState, useEffect, useCallback } from 'react'
import type { ScrapeTask, ScrapeStatistics, ScrapeHistory } from '@/types'
import { scrapeApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useWebSocket } from '@/hooks/useWebSocket'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import {
  Globe,
  Plus,
  Upload,
  Play,
  Languages,
  Trash2,
  Download,
  RefreshCw,
  Loader2,
  Check,
  X,
  AlertCircle,
  Edit3,
  Clock,
  BarChart3,
  FileText,
  ExternalLink,
  Filter,
  CheckSquare,
  Square,
} from 'lucide-react'
import clsx from 'clsx'

// 数据源选项
const SOURCE_OPTIONS = [
  { value: '', label: '自动识别' },
  { value: 'tmdb', label: 'TMDb' },
  { value: 'douban', label: '豆瓣' },
  { value: 'bangumi', label: 'Bangumi' },
  { value: 'url', label: '通用URL' },
]

// 翻译语言选项
const LANG_OPTIONS = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁体中文' },
  { value: 'en', label: 'English' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' },
]

// 状态标签
const STATUS_LABELS: Record<string, { label: string; color: string; icon: typeof Check }> = {
  pending: { label: '待处理', color: 'text-surface-400', icon: Clock },
  scraping: { label: '刮削中', color: 'text-amber-400', icon: Loader2 },
  scraped: { label: '已刮削', color: 'text-blue-400', icon: Check },
  failed: { label: '失败', color: 'text-red-400', icon: X },
  translating: { label: '翻译中', color: 'text-purple-400', icon: Loader2 },
  completed: { label: '已完成', color: 'text-green-400', icon: Check },
}

const TRANSLATE_STATUS_LABELS: Record<string, { label: string; color: string }> = {
  none: { label: '未翻译', color: 'text-surface-500' },
  pending: { label: '待翻译', color: 'text-surface-400' },
  translating: { label: '翻译中', color: 'text-purple-400' },
  done: { label: '已翻译', color: 'text-green-400' },
  failed: { label: '翻译失败', color: 'text-red-400' },
}

interface ScrapeManagerPageProps {
  /** 是否作为嵌入组件使用（隐藏独立页面标题） */
  embedded?: boolean
}

export default function ScrapeManagerPage({ embedded = false }: ScrapeManagerPageProps) {
  const toast = useToast()
  const { on, off } = useWebSocket()

  // 数据状态
  const [tasks, setTasks] = useState<ScrapeTask[]>([])
  const [total, setTotal] = useState(0)
  const { page, size: pageSize, setPage, setSize, totalPages } = usePagination({ initialSize: 20 })
  const [loading, setLoading] = useState(true)
  const [stats, setStats] = useState<ScrapeStatistics | null>(null)

  // 筛选
  const [filterStatus, setFilterStatus] = useState('')
  const [filterSource, setFilterSource] = useState('')

  // 选择
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  // 创建表单
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [createMode, setCreateMode] = useState<'single' | 'batch'>('single')
  const [urlInput, setUrlInput] = useState('')
  const [batchUrlInput, setBatchUrlInput] = useState('')
  const [createSource, setCreateSource] = useState('')
  const [createMediaType, setCreateMediaType] = useState('movie')
  const [creating, setCreating] = useState(false)

  // 翻译对话框
  const [showTranslateDialog, setShowTranslateDialog] = useState(false)
  const [translateTargetLang, setTranslateTargetLang] = useState('zh-CN')
  const [translateFields, setTranslateFields] = useState<string[]>([])
  const [translateTaskIds, setTranslateTaskIds] = useState<string[]>([])

  // 详情/编辑
  const [editingTask, setEditingTask] = useState<ScrapeTask | null>(null)
  const [showHistory, setShowHistory] = useState(false)
  const [history, setHistory] = useState<ScrapeHistory[]>([])

  // 加载任务列表
  const fetchTasks = useCallback(async () => {
    try {
      const res = await scrapeApi.listTasks({
        page,
        size: pageSize,
        status: filterStatus || undefined,
        source: filterSource || undefined,
      })
      setTasks(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      toast.error('加载刮削任务失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, filterStatus, filterSource])

  // 加载统计
  const fetchStats = useCallback(async () => {
    try {
      const res = await scrapeApi.getStatistics()
      setStats(res.data.data)
    } catch {
      // 静默
    }
  }, [])

  useEffect(() => {
    fetchTasks()
    fetchStats()
  }, [fetchTasks, fetchStats])

  // WebSocket 实时更新
  useEffect(() => {
    const handleTaskUpdate = (data: ScrapeTask) => {
      setTasks(prev => prev.map(t => t.id === data.id ? data : t))
      // 刷新统计
      fetchStats()
    }

    on('scrape_task_update' as any, handleTaskUpdate)
    return () => {
      off('scrape_task_update' as any, handleTaskUpdate)
    }
  }, [on, off, fetchStats])

  // ==================== 创建任务 ====================
  const handleCreate = async () => {
    if (createMode === 'single') {
      if (!urlInput.trim()) {
        toast.error('请输入URL')
        return
      }
      setCreating(true)
      try {
        await scrapeApi.createTask({
          url: urlInput.trim(),
          source: createSource || undefined,
          media_type: createMediaType,
        })
        toast.success('刮削任务已创建')
        setUrlInput('')
        setShowCreateForm(false)
        fetchTasks()
        fetchStats()
      } catch (err: any) {
        toast.error(err?.response?.data?.error || '创建失败')
      } finally {
        setCreating(false)
      }
    } else {
      const urls = batchUrlInput.split('\n').map(u => u.trim()).filter(Boolean)
      if (urls.length === 0) {
        toast.error('请输入至少一个URL')
        return
      }
      setCreating(true)
      try {
        const res = await scrapeApi.batchCreateTasks({
          urls,
          source: createSource || undefined,
          media_type: createMediaType,
        })
        toast.success(`批量创建完成: 成功 ${res.data.created}, 跳过 ${res.data.skipped}`)
        setBatchUrlInput('')
        setShowCreateForm(false)
        fetchTasks()
        fetchStats()
      } catch {
        toast.error('批量创建失败')
      } finally {
        setCreating(false)
      }
    }
  }

  // ==================== 刮削操作 ====================
  const handleStartScrape = async (id: string) => {
    try {
      await scrapeApi.startScrape(id)
      toast.success('刮削已启动')
      setTasks(prev => prev.map(t => t.id === id ? { ...t, status: 'scraping' as const } : t))
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '启动失败')
    }
  }

  const handleBatchScrape = async () => {
    if (selectedIds.size === 0) {
      toast.error('请先选择任务')
      return
    }
    try {
      const res = await scrapeApi.batchStartScrape(Array.from(selectedIds))
      toast.success(`批量刮削已启动: ${res.data.started} 个任务`)
      setSelectedIds(new Set())
      fetchTasks()
    } catch {
      toast.error('批量刮削失败')
    }
  }

  // ==================== 翻译操作 ====================
  const openTranslateDialog = (taskIds: string[]) => {
    setTranslateTaskIds(taskIds)
    setShowTranslateDialog(true)
  }

  const handleTranslate = async () => {
    try {
      if (translateTaskIds.length === 1) {
        await scrapeApi.translateTask(translateTaskIds[0], {
          target_lang: translateTargetLang,
          fields: translateFields.length > 0 ? translateFields : undefined,
        })
        toast.success('翻译已启动')
      } else {
        const res = await scrapeApi.batchTranslate({
          task_ids: translateTaskIds,
          target_lang: translateTargetLang,
          fields: translateFields.length > 0 ? translateFields : undefined,
        })
        toast.success(`批量翻译已启动: ${res.data.started} 个任务`)
      }
      setShowTranslateDialog(false)
      setSelectedIds(new Set())
      fetchTasks()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '翻译启动失败')
    }
  }

  // ==================== 删除操作 ====================
  const handleDelete = async (id: string) => {
    if (!confirm('确定删除该刮削任务？')) return
    try {
      await scrapeApi.deleteTask(id)
      toast.success('已删除')
      fetchTasks()
      fetchStats()
    } catch {
      toast.error('删除失败')
    }
  }

  const handleBatchDelete = async () => {
    if (selectedIds.size === 0) return
    if (!confirm(`确定删除选中的 ${selectedIds.size} 个任务？`)) return
    try {
      await scrapeApi.batchDeleteTasks(Array.from(selectedIds))
      toast.success('批量删除完成')
      setSelectedIds(new Set())
      fetchTasks()
      fetchStats()
    } catch {
      toast.error('批量删除失败')
    }
  }

  // ==================== 导出 ====================
  const handleExport = async () => {
    const ids = selectedIds.size > 0 ? Array.from(selectedIds) : tasks.map(t => t.id)
    try {
      const res = await scrapeApi.exportTasks(ids)
      const blob = new Blob([JSON.stringify(res.data.data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `scrape-export-${new Date().toISOString().slice(0, 10)}.json`
      a.click()
      URL.revokeObjectURL(url)
      toast.success('导出成功')
    } catch {
      toast.error('导出失败')
    }
  }

  // ==================== 编辑 ====================
  const handleSaveEdit = async () => {
    if (!editingTask) return
    try {
      await scrapeApi.updateTask(editingTask.id, {
        result_title: editingTask.result_title,
        result_orig_title: editingTask.result_orig_title,
        result_year: editingTask.result_year,
        result_overview: editingTask.result_overview,
        result_genres: editingTask.result_genres,
        result_rating: editingTask.result_rating,
        result_country: editingTask.result_country,
        result_language: editingTask.result_language,
      })
      toast.success('保存成功')
      setEditingTask(null)
      fetchTasks()
    } catch {
      toast.error('保存失败')
    }
  }

  // ==================== 历史 ====================
  const loadHistory = async (taskId?: string) => {
    try {
      const res = await scrapeApi.getHistory({ task_id: taskId, limit: 50 })
      setHistory(res.data.data || [])
      setShowHistory(true)
    } catch {
      toast.error('加载历史失败')
    }
  }

  // ==================== 选择 ====================
  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (selectedIds.size === tasks.length) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(tasks.map(t => t.id)))
    }
  }

  // 质量评分颜色
  const qualityColor = (score: number) => {
    if (score >= 80) return 'text-green-400'
    if (score >= 50) return 'text-amber-400'
    return 'text-red-400'
  }

  return (
    <div className={clsx('space-y-6', embedded && 'pt-0')}>
      {/* ==================== 页面标题（独立页面模式下显示） ==================== */}
      {!embedded && (
      <div className="flex items-center justify-between">
        <div>
          <h1 className="font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Globe className="inline-block mr-2 text-neon" size={24} />
            刮削数据管理
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-tertiary)' }}>
            管理元数据刮削任务，支持多数据源、AI增强和多语言翻译
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={() => loadHistory()} className="btn-ghost gap-1.5 px-3 py-2 text-xs">
            <Clock size={14} />
            操作历史
          </button>
          <button onClick={handleExport} className="btn-ghost gap-1.5 px-3 py-2 text-xs">
            <Download size={14} />
            导出
          </button>
          <button onClick={() => { fetchTasks(); fetchStats() }} className="btn-ghost p-2 text-surface-400 hover:text-neon">
            <RefreshCw size={16} />
          </button>
          <button onClick={() => setShowCreateForm(!showCreateForm)} className="btn-primary gap-1.5 px-3.5 py-2 text-xs">
            <Plus size={14} />
            新建任务
          </button>
        </div>
      </div>
      )}

      {/* 嵌入模式下的精简操作栏 */}
      {embedded && (
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <button onClick={() => loadHistory()} className="btn-ghost gap-1.5 px-3 py-2 text-xs">
              <Clock size={14} />
              操作历史
            </button>
            <button onClick={handleExport} className="btn-ghost gap-1.5 px-3 py-2 text-xs">
              <Download size={14} />
              导出
            </button>
            <button onClick={() => { fetchTasks(); fetchStats() }} className="btn-ghost p-2 text-surface-400 hover:text-neon">
              <RefreshCw size={16} />
            </button>
          </div>
          <button onClick={() => setShowCreateForm(!showCreateForm)} className="btn-primary gap-1.5 px-3.5 py-2 text-xs">
            <Plus size={14} />
            新建任务
          </button>
        </div>
      )}

      {/* ==================== 统计卡片 ==================== */}
      {stats && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-7">
          {[
            { label: '总计', value: stats.total, color: 'text-neon' },
            { label: '待处理', value: stats.pending, color: 'text-surface-400' },
            { label: '刮削中', value: stats.scraping, color: 'text-amber-400' },
            { label: '已刮削', value: stats.scraped, color: 'text-blue-400' },
            { label: '翻译中', value: stats.translating, color: 'text-purple-400' },
            { label: '已完成', value: stats.completed, color: 'text-green-400' },
            { label: '失败', value: stats.failed, color: 'text-red-400' },
          ].map(item => (
            <div key={item.label} className="glass-panel-subtle rounded-xl p-3 text-center">
              <p className={clsx('text-xl font-bold', item.color)}>{item.value || 0}</p>
              <p className="text-[10px] mt-0.5" style={{ color: 'var(--text-muted)' }}>{item.label}</p>
            </div>
          ))}
        </div>
      )}

      {/* ==================== 创建表单 ==================== */}
      {showCreateForm && (
        <div className="glass-panel animate-slide-up rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              新建刮削任务
            </h3>
            <div className="flex gap-1 rounded-lg p-0.5" style={{ background: 'var(--nav-hover-bg)' }}>
              <button
                onClick={() => setCreateMode('single')}
                className={clsx('rounded-md px-3 py-1 text-xs font-medium transition-all', createMode === 'single' ? 'bg-neon text-white' : '')}
                style={createMode !== 'single' ? { color: 'var(--text-secondary)' } : undefined}
              >
                单条输入
              </button>
              <button
                onClick={() => setCreateMode('batch')}
                className={clsx('rounded-md px-3 py-1 text-xs font-medium transition-all', createMode === 'batch' ? 'bg-neon text-white' : '')}
                style={createMode !== 'batch' ? { color: 'var(--text-secondary)' } : undefined}
              >
                <Upload size={12} className="inline mr-1" />
                批量导入
              </button>
            </div>
          </div>

          {createMode === 'single' ? (
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>URL 地址</label>
              <input
                type="text"
                value={urlInput}
                onChange={e => setUrlInput(e.target.value)}
                className="input w-full"
                placeholder="输入 TMDb / 豆瓣 / IMDb / Bangumi 链接或任意URL"
                onKeyDown={e => e.key === 'Enter' && handleCreate()}
              />
            </div>
          ) : (
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                URL 列表（每行一个）
              </label>
              <textarea
                value={batchUrlInput}
                onChange={e => setBatchUrlInput(e.target.value)}
                className="input w-full h-32 resize-y font-mono text-xs"
                placeholder={'https://www.themoviedb.org/movie/550\nhttps://movie.douban.com/subject/1292052/\nhttps://bgm.tv/subject/12345'}
              />
              <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                已输入 {batchUrlInput.split('\n').filter(u => u.trim()).length} 条URL
              </p>
            </div>
          )}

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>数据源</label>
              <select value={createSource} onChange={e => setCreateSource(e.target.value)} className="input w-full">
                {SOURCE_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>媒体类型</label>
              <select value={createMediaType} onChange={e => setCreateMediaType(e.target.value)} className="input w-full">
                <option value="movie">电影</option>
                <option value="tvshow">电视剧</option>
              </select>
            </div>
          </div>

          <div className="flex items-center justify-end gap-2 pt-2">
            <button onClick={() => setShowCreateForm(false)} className="rounded-xl px-4 py-2 text-sm font-medium transition-all" style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}>
              取消
            </button>
            <button onClick={handleCreate} disabled={creating} className="btn-primary gap-1.5 px-4 py-2 text-sm">
              {creating ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
              {createMode === 'single' ? '创建任务' : '批量创建'}
            </button>
          </div>
        </div>
      )}

      {/* ==================== 筛选和批量操作栏 ==================== */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Filter size={14} style={{ color: 'var(--text-muted)' }} />
          <select
            value={filterStatus}
            onChange={e => { setFilterStatus(e.target.value); setPage(1) }}
            className="input py-1.5 text-xs"
          >
            <option value="">全部状态</option>
            <option value="pending">待处理</option>
            <option value="scraping">刮削中</option>
            <option value="scraped">已刮削</option>
            <option value="failed">失败</option>
            <option value="translating">翻译中</option>
            <option value="completed">已完成</option>
          </select>
          <select
            value={filterSource}
            onChange={e => { setFilterSource(e.target.value); setPage(1) }}
            className="input py-1.5 text-xs"
          >
            <option value="">全部来源</option>
            <option value="tmdb">TMDb</option>
            <option value="douban">豆瓣</option>
            <option value="bangumi">Bangumi</option>
            <option value="url">通用URL</option>
          </select>
        </div>

        {selectedIds.size > 0 && (
          <div className="flex items-center gap-2">
            <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
              已选 {selectedIds.size} 项
            </span>
            <button onClick={handleBatchScrape} className="btn-ghost gap-1 px-2.5 py-1.5 text-xs text-neon">
              <Play size={12} />
              批量刮削
            </button>
            <button
              onClick={() => openTranslateDialog(Array.from(selectedIds))}
              className="btn-ghost gap-1 px-2.5 py-1.5 text-xs text-purple-400"
            >
              <Languages size={12} />
              批量翻译
            </button>
            <button onClick={handleBatchDelete} className="btn-ghost gap-1 px-2.5 py-1.5 text-xs text-red-400">
              <Trash2 size={12} />
              批量删除
            </button>
          </div>
        )}
      </div>

      {/* ==================== 任务列表 ==================== */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 size={24} className="animate-spin text-neon/40" />
        </div>
      ) : tasks.length > 0 ? (
        <div className="space-y-2">
          {/* 表头 */}
          <div className="flex items-center gap-3 px-4 py-2 text-[10px] font-bold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
            <button onClick={toggleSelectAll} className="flex-shrink-0">
              {selectedIds.size === tasks.length ? <CheckSquare size={14} className="text-neon" /> : <Square size={14} />}
            </button>
            <span className="w-48 flex-shrink-0">标题 / URL</span>
            <span className="w-16 flex-shrink-0 text-center">来源</span>
            <span className="w-16 flex-shrink-0 text-center">状态</span>
            <span className="w-16 flex-shrink-0 text-center">翻译</span>
            <span className="w-12 flex-shrink-0 text-center">质量</span>
            <span className="flex-1" />
            <span className="w-24 flex-shrink-0 text-right">操作</span>
          </div>

          {/* 任务行 */}
          {tasks.map(task => {
            const statusInfo = STATUS_LABELS[task.status] || STATUS_LABELS.pending
            const StatusIcon = statusInfo.icon
            const translateInfo = TRANSLATE_STATUS_LABELS[task.translate_status] || TRANSLATE_STATUS_LABELS.none

            return (
              <div
                key={task.id}
                className={clsx(
                  'glass-panel-subtle group flex items-center gap-3 rounded-xl px-4 py-3 transition-all hover:border-neon-blue/20',
                  selectedIds.has(task.id) && 'border-neon-blue/30'
                )}
              >
                {/* 选择框 */}
                <button onClick={() => toggleSelect(task.id)} className="flex-shrink-0">
                  {selectedIds.has(task.id) ? <CheckSquare size={14} className="text-neon" /> : <Square size={14} className="text-surface-600" />}
                </button>

                {/* 标题/URL */}
                <div className="w-48 flex-shrink-0 min-w-0">
                  <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    {task.result_title || task.title || '未识别'}
                  </p>
                  <p className="truncate text-[10px] mt-0.5" style={{ color: 'var(--text-muted)' }}>
                    {task.url}
                  </p>
                </div>

                {/* 来源 */}
                <div className="w-16 flex-shrink-0 text-center">
                  <span className="rounded-md px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'var(--nav-hover-bg)', color: 'var(--text-tertiary)' }}>
                    {task.source.toUpperCase()}
                  </span>
                </div>

                {/* 状态 */}
                <div className="w-16 flex-shrink-0 text-center">
                  <span className={clsx('inline-flex items-center gap-1 text-[10px] font-medium', statusInfo.color)}>
                    <StatusIcon size={10} className={task.status === 'scraping' || task.status === 'translating' ? 'animate-spin' : ''} />
                    {statusInfo.label}
                  </span>
                </div>

                {/* 翻译状态 */}
                <div className="w-16 flex-shrink-0 text-center">
                  <span className={clsx('text-[10px] font-medium', translateInfo.color)}>
                    {translateInfo.label}
                  </span>
                </div>

                {/* 质量评分 */}
                <div className="w-12 flex-shrink-0 text-center">
                  {task.quality_score > 0 ? (
                    <span className={clsx('text-xs font-bold', qualityColor(task.quality_score))}>
                      {task.quality_score}
                    </span>
                  ) : (
                    <span className="text-[10px] text-surface-600">—</span>
                  )}
                </div>

                {/* 错误信息 */}
                <div className="flex-1 min-w-0">
                  {task.error_message && (
                    <span className="flex items-center gap-1 text-[10px] text-red-400 truncate">
                      <AlertCircle size={10} />
                      {task.error_message}
                    </span>
                  )}
                </div>

                {/* 操作按钮 */}
                <div className="w-24 flex-shrink-0 flex items-center justify-end gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                  {(task.status === 'pending' || task.status === 'failed') && (
                    <button onClick={() => handleStartScrape(task.id)} className="rounded-lg p-1.5 text-surface-400 hover:text-neon hover:bg-neon-blue/5" title="开始刮削">
                      <Play size={13} />
                    </button>
                  )}
                  {(task.status === 'scraped' || task.status === 'completed') && (
                    <button onClick={() => openTranslateDialog([task.id])} className="rounded-lg p-1.5 text-surface-400 hover:text-purple-400 hover:bg-purple-400/5" title="翻译">
                      <Languages size={13} />
                    </button>
                  )}
                  <button onClick={() => setEditingTask({ ...task })} className="rounded-lg p-1.5 text-surface-400 hover:text-blue-400 hover:bg-blue-400/5" title="查看/编辑">
                    <Edit3 size={13} />
                  </button>
                  <button onClick={() => handleDelete(task.id)} className="rounded-lg p-1.5 text-surface-400 hover:text-red-400 hover:bg-red-400/5" title="删除">
                    <Trash2 size={13} />
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      ) : (
        <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-20 text-center">
          <div>
            <Globe size={40} className="mx-auto mb-3 text-surface-600" />
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>暂无刮削任务</p>
            <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>
              点击「新建任务」开始添加刮削数据
            </p>
          </div>
        </div>
      )}

      {/* ==================== 分页 ==================== */}
      <Pagination
        page={page}
        totalPages={totalPages(total)}
        total={total}
        pageSize={pageSize}
        pageSizeOptions={[10, 20, 50, 100]}
        onPageChange={setPage}
        onPageSizeChange={setSize}
      />

      {/* ==================== 翻译对话框 ==================== */}
      {showTranslateDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setShowTranslateDialog(false)}>
          <div className="glass-panel w-full max-w-md rounded-2xl p-6 space-y-4 animate-slide-up" onClick={e => e.stopPropagation()}>
            <h3 className="text-base font-semibold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
              <Languages size={18} className="text-purple-400" />
              翻译设置
            </h3>

            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>目标语言</label>
              <select value={translateTargetLang} onChange={e => setTranslateTargetLang(e.target.value)} className="input w-full">
                {LANG_OPTIONS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            </div>

            <div>
              <label className="mb-1.5 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>翻译字段（留空翻译全部）</label>
              <div className="flex flex-wrap gap-2">
                {['title', 'overview', 'genres', 'tagline'].map(field => (
                  <button
                    key={field}
                    onClick={() => setTranslateFields(prev =>
                      prev.includes(field) ? prev.filter(f => f !== field) : [...prev, field]
                    )}
                    className={clsx(
                      'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                      translateFields.includes(field) ? 'bg-purple-500/20 text-purple-400' : ''
                    )}
                    style={!translateFields.includes(field) ? { background: 'var(--nav-hover-bg)', color: 'var(--text-secondary)' } : undefined}
                  >
                    {{ title: '标题', overview: '简介', genres: '类型', tagline: '宣传语' }[field]}
                  </button>
                ))}
              </div>
            </div>

            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              将对 {translateTaskIds.length} 个任务执行翻译，需要 AI 服务支持
            </p>

            <div className="flex items-center justify-end gap-2 pt-2">
              <button onClick={() => setShowTranslateDialog(false)} className="rounded-xl px-4 py-2 text-sm font-medium" style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}>
                取消
              </button>
              <button onClick={handleTranslate} className="btn-primary gap-1.5 px-4 py-2 text-sm" style={{ background: 'linear-gradient(135deg, #a855f7, #6366f1)' }}>
                <Languages size={14} />
                开始翻译
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ==================== 编辑/详情对话框 ==================== */}
      {editingTask && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setEditingTask(null)}>
          <div className="glass-panel w-full max-w-2xl max-h-[85vh] overflow-y-auto rounded-2xl p-6 space-y-4 animate-slide-up" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
                <Edit3 size={18} className="text-blue-400" />
                编辑刮削结果
              </h3>
              <button onClick={() => setEditingTask(null)} className="rounded-lg p-1.5 text-surface-400 hover:text-surface-200">
                <X size={16} />
              </button>
            </div>

            {/* URL信息 */}
            <div className="rounded-lg p-3 text-xs" style={{ background: 'var(--nav-hover-bg)' }}>
              <span style={{ color: 'var(--text-muted)' }}>URL: </span>
              <a href={editingTask.url} target="_blank" rel="noopener noreferrer" className="text-neon hover:underline inline-flex items-center gap-1">
                {editingTask.url}
                <ExternalLink size={10} />
              </a>
            </div>

            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>标题</label>
                <input type="text" value={editingTask.result_title} onChange={e => setEditingTask({ ...editingTask, result_title: e.target.value })} className="input w-full" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>原始标题</label>
                <input type="text" value={editingTask.result_orig_title} onChange={e => setEditingTask({ ...editingTask, result_orig_title: e.target.value })} className="input w-full" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>年份</label>
                <input type="number" value={editingTask.result_year || ''} onChange={e => setEditingTask({ ...editingTask, result_year: parseInt(e.target.value) || 0 })} className="input w-full" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>评分</label>
                <input type="number" step="0.1" value={editingTask.result_rating || ''} onChange={e => setEditingTask({ ...editingTask, result_rating: parseFloat(e.target.value) || 0 })} className="input w-full" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>类型</label>
                <input type="text" value={editingTask.result_genres} onChange={e => setEditingTask({ ...editingTask, result_genres: e.target.value })} className="input w-full" placeholder="动作,科幻,冒险" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>国家</label>
                <input type="text" value={editingTask.result_country} onChange={e => setEditingTask({ ...editingTask, result_country: e.target.value })} className="input w-full" />
              </div>
            </div>

            <div>
              <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>简介</label>
              <textarea value={editingTask.result_overview} onChange={e => setEditingTask({ ...editingTask, result_overview: e.target.value })} className="input w-full h-24 resize-y text-xs" />
            </div>

            {/* 翻译结果预览 */}
            {editingTask.translate_status === 'done' && (
              <div className="rounded-xl p-4 space-y-2" style={{ background: 'rgba(168, 85, 247, 0.05)', border: '1px solid rgba(168, 85, 247, 0.15)' }}>
                <h4 className="text-xs font-semibold text-purple-400 flex items-center gap-1.5">
                  <Languages size={12} />
                  翻译结果 ({editingTask.translate_lang})
                </h4>
                {editingTask.translated_title && (
                  <p className="text-xs"><span style={{ color: 'var(--text-muted)' }}>标题: </span><span style={{ color: 'var(--text-primary)' }}>{editingTask.translated_title}</span></p>
                )}
                {editingTask.translated_overview && (
                  <p className="text-xs"><span style={{ color: 'var(--text-muted)' }}>简介: </span><span style={{ color: 'var(--text-secondary)' }}>{editingTask.translated_overview}</span></p>
                )}
                {editingTask.translated_genres && (
                  <p className="text-xs"><span style={{ color: 'var(--text-muted)' }}>类型: </span><span style={{ color: 'var(--text-secondary)' }}>{editingTask.translated_genres}</span></p>
                )}
              </div>
            )}

            <div className="flex items-center justify-between pt-2">
              <div className="flex items-center gap-2">
                <span className={clsx('text-xs font-medium', qualityColor(editingTask.quality_score))}>
                  <BarChart3 size={12} className="inline mr-1" />
                  质量评分: {editingTask.quality_score}
                </span>
              </div>
              <div className="flex items-center gap-2">
                <button onClick={() => setEditingTask(null)} className="rounded-xl px-4 py-2 text-sm font-medium" style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}>
                  取消
                </button>
                <button onClick={handleSaveEdit} className="btn-primary gap-1.5 px-4 py-2 text-sm">
                  <Check size={14} />
                  保存修改
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ==================== 操作历史对话框 ==================== */}
      {showHistory && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={() => setShowHistory(false)}>
          <div className="glass-panel w-full max-w-lg max-h-[70vh] overflow-y-auto rounded-2xl p-6 space-y-4 animate-slide-up" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between">
              <h3 className="text-base font-semibold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
                <FileText size={18} className="text-neon" />
                操作历史
              </h3>
              <button onClick={() => setShowHistory(false)} className="rounded-lg p-1.5 text-surface-400 hover:text-surface-200">
                <X size={16} />
              </button>
            </div>

            {history.length > 0 ? (
              <div className="space-y-2">
                {history.map(h => (
                  <div key={h.id} className="flex items-start gap-3 rounded-lg p-3" style={{ background: 'var(--nav-hover-bg)' }}>
                    <div className="mt-0.5">
                      {h.action.includes('fail') ? <X size={12} className="text-red-400" /> :
                       h.action.includes('done') || h.action === 'created' ? <Check size={12} className="text-green-400" /> :
                       <Clock size={12} className="text-surface-400" />}
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{h.action}</p>
                      {h.detail && <p className="text-[10px] mt-0.5 truncate" style={{ color: 'var(--text-muted)' }}>{h.detail}</p>}
                    </div>
                    <span className="text-[10px] flex-shrink-0" style={{ color: 'var(--text-muted)' }}>
                      {new Date(h.created_at).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-center text-sm py-8" style={{ color: 'var(--text-muted)' }}>暂无操作记录</p>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
