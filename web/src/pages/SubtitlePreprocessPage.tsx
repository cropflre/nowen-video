import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { subtitlePreprocessApi } from '@/api/subtitlePreprocess'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import type { SubtitlePreprocessTask, SubtitlePreprocessStatistics, ASRHealthStatus, Library } from '@/types'
import api from '@/api/client'
import { LANGUAGE_OPTIONS } from '@/components/file-manager/constants'
import {
  RotateCcw,
  Trash2,
  XCircle,
  Activity,
  RefreshCw,
  CheckCircle2,
  Clock,
  AlertCircle,
  Loader2,
  Subtitles,
  FolderOpen,
  Send,
  Languages,
  SkipForward,
  Sparkles,
  FileText,
  Globe,
  CheckSquare,
  Square,
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  X,
  ChevronDown,
  Zap,
  ShieldAlert,
  ShieldCheck,
  ToggleLeft,
  ToggleRight,
} from 'lucide-react'
import clsx from 'clsx'

// 状态颜色映射
const statusColors: Record<string, string> = {
  pending: 'text-yellow-400',
  running: 'text-neon-blue',
  completed: 'text-emerald-400',
  failed: 'text-red-400',
  cancelled: 'text-surface-500',
  skipped: 'text-orange-400',
}

const statusLabels: Record<string, string> = {
  pending: '等待中',
  running: '处理中',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
  skipped: '已跳过',
}

const statusIcons: Record<string, React.ReactNode> = {
  pending: <Clock size={14} />,
  running: <Loader2 size={14} className="animate-spin" />,
  completed: <CheckCircle2 size={14} />,
  failed: <AlertCircle size={14} />,
  cancelled: <XCircle size={14} />,
  skipped: <SkipForward size={14} />,
}

// 阶段标签
const phaseLabels: Record<string, string> = {
  check: '检查字幕',
  extract: '提取字幕',
  clean: '字幕清洗',
  generate: 'AI 生成',
  translate: '多语言翻译',
  done: '完成',
}

// 字幕来源标签
const sourceLabels: Record<string, string> = {
  ai_cached: 'AI 缓存',
  external_vtt: '外挂 VTT',
  extracted: '内嵌提取',
  ai_generated: 'AI 生成',
  ocr_extracted: 'OCR 识别',
}

export default function SubtitlePreprocessPage() {
  const toast = useToast()
  const toastRef = useRef(toast)
  toastRef.current = toast
  const { on, off } = useWebSocket()
  const [tasks, setTasks] = useState<SubtitlePreprocessTask[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [statusFilter, setStatusFilter] = useState('')
  const [stats, setStats] = useState<SubtitlePreprocessStatistics | null>(null)
  const [loading, setLoading] = useState(true)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [submitting, setSubmitting] = useState<string | null>(null)
  // 翻译目标语言（多选）
  const [selectedTargetLangs, setSelectedTargetLangs] = useState<string[]>([])
  const [showLangDropdown, setShowLangDropdown] = useState(false)
  const langDropdownRef = useRef<HTMLDivElement>(null)
  // P1: 强制重新生成开关
  const [forceRegenerate, setForceRegenerate] = useState(false)
  // P0: ASR 健康状态
  const [asrHealth, setAsrHealth] = useState<ASRHealthStatus | null>(null)
  const [checkingHealth, setCheckingHealth] = useState(false)

  // 可选的翻译语言列表（排除空值选项）
  const availableLangs = useMemo(() => LANGUAGE_OPTIONS.filter((l) => l.value !== ''), [])  
  // 批量选择
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchLoading, setBatchLoading] = useState(false)

  // 计算总页数
  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / pageSize)), [total, pageSize])

  // 全选/取消全选当前页
  const isAllSelected = tasks.length > 0 && tasks.every((t) => selectedIds.has(t.id))
  const isSomeSelected = selectedIds.size > 0

  const toggleSelectAll = () => {
    if (isAllSelected) {
      // 取消当前页全选
      const newSet = new Set(selectedIds)
      tasks.forEach((t) => newSet.delete(t.id))
      setSelectedIds(newSet)
    } else {
      // 全选当前页
      const newSet = new Set(selectedIds)
      tasks.forEach((t) => newSet.add(t.id))
      setSelectedIds(newSet)
    }
  }

  const toggleSelect = (id: string) => {
    const newSet = new Set(selectedIds)
    if (newSet.has(id)) {
      newSet.delete(id)
    } else {
      newSet.add(id)
    }
    setSelectedIds(newSet)
  }

  // 点击外部关闭语言下拉框
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (langDropdownRef.current && !langDropdownRef.current.contains(e.target as Node)) {
        setShowLangDropdown(false)
      }
    }
    if (showLangDropdown) {
      document.addEventListener('mousedown', handleClickOutside)
    }
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [showLangDropdown])

  // 加载任务列表
  const loadTasks = useCallback(async () => {
    try {
      const res = await subtitlePreprocessApi.listTasks({ page, page_size: pageSize, status: statusFilter })
      setTasks(res.data.data.tasks || [])
      setTotal(res.data.data.total)
    } catch {
      toastRef.current.error('加载字幕预处理任务失败')
    }
  }, [page, pageSize, statusFilter])

  // 加载统计
  const loadStats = useCallback(async () => {
    try {
      const res = await subtitlePreprocessApi.getStatistics()
      setStats(res.data.data)
    } catch {
      // 忽略
    }
  }, [])

  // 加载媒体库列表
  const loadLibraries = useCallback(async () => {
    try {
      const res = await api.get<{ data: Library[] }>('/libraries')
      setLibraries(res.data.data || [])
    } catch {
      // 忽略
    }
  }, [])

  // P0: 检查 ASR 健康状态
  const checkASRHealth = useCallback(async () => {
    setCheckingHealth(true)
    try {
      const res = await subtitlePreprocessApi.checkASRHealth()
      setAsrHealth(res.data.data)
    } catch {
      // 忽略
    } finally {
      setCheckingHealth(false)
    }
  }, [])

  useEffect(() => {
    setLoading(true)
    Promise.all([loadTasks(), loadStats(), loadLibraries(), checkASRHealth()]).finally(() => setLoading(false))
  }, [loadTasks, loadStats, loadLibraries, checkASRHealth])

  // WebSocket 实时更新（节流：最多每 3 秒刷新一次）
  useEffect(() => {
    let refreshTimer: ReturnType<typeof setTimeout> | null = null
    let needsRefresh = false

    const scheduleRefresh = () => {
      if (refreshTimer) {
        needsRefresh = true
        return
      }
      loadTasks()
      loadStats()
      refreshTimer = setTimeout(() => {
        refreshTimer = null
        if (needsRefresh) {
          needsRefresh = false
          scheduleRefresh()
        }
      }, 3000)
    }

    on(WS_EVENTS.SUB_PREPROCESS_PROGRESS, scheduleRefresh)
    on(WS_EVENTS.SUB_PREPROCESS_COMPLETED, scheduleRefresh)
    on(WS_EVENTS.SUB_PREPROCESS_FAILED, scheduleRefresh)
    on(WS_EVENTS.SUB_PREPROCESS_STARTED, scheduleRefresh)
    return () => {
      off(WS_EVENTS.SUB_PREPROCESS_PROGRESS, scheduleRefresh)
      off(WS_EVENTS.SUB_PREPROCESS_COMPLETED, scheduleRefresh)
      off(WS_EVENTS.SUB_PREPROCESS_FAILED, scheduleRefresh)
      off(WS_EVENTS.SUB_PREPROCESS_STARTED, scheduleRefresh)
      if (refreshTimer) clearTimeout(refreshTimer)
    }
  }, [on, off, loadTasks, loadStats])

  // 任务操作
  const handleCancel = async (id: string) => {
    try {
      await subtitlePreprocessApi.cancelTask(id)
      toastRef.current.success('任务已取消')
      loadTasks()
    } catch { toastRef.current.error('取消失败') }
  }

  const handleRetry = async (id: string) => {
    try {
      await subtitlePreprocessApi.retryTask(id)
      toastRef.current.success('任务已重新提交')
      loadTasks()
    } catch { toastRef.current.error('重试失败') }
  }

  const handleDelete = async (id: string) => {
    try {
      await subtitlePreprocessApi.deleteTask(id)
      toastRef.current.success('任务已删除')
      loadTasks()
    } catch { toastRef.current.error('删除失败') }
  }

  // 批量操作
  const handleBatchDelete = async () => {
    if (selectedIds.size === 0) return
    setBatchLoading(true)
    try {
      const res = await subtitlePreprocessApi.batchDeleteTasks(Array.from(selectedIds))
      const deleted = res.data.data.deleted
      toastRef.current.success(`已删除 ${deleted} 个任务`)
      setSelectedIds(new Set())
      loadTasks()
      loadStats()
    } catch {
      toastRef.current.error('批量删除失败')
    } finally {
      setBatchLoading(false)
    }
  }

  const handleBatchCancel = async () => {
    if (selectedIds.size === 0) return
    setBatchLoading(true)
    try {
      const res = await subtitlePreprocessApi.batchCancelTasks(Array.from(selectedIds))
      const cancelled = res.data.data.cancelled
      toastRef.current.success(`已取消 ${cancelled} 个任务`)
      setSelectedIds(new Set())
      loadTasks()
      loadStats()
    } catch {
      toastRef.current.error('批量取消失败')
    } finally {
      setBatchLoading(false)
    }
  }

  const handleBatchRetry = async () => {
    if (selectedIds.size === 0) return
    setBatchLoading(true)
    try {
      const res = await subtitlePreprocessApi.batchRetryTasks(Array.from(selectedIds))
      const retried = res.data.data.retried
      toastRef.current.success(`已重试 ${retried} 个任务`)
      setSelectedIds(new Set())
      loadTasks()
      loadStats()
    } catch {
      toastRef.current.error('批量重试失败')
    } finally {
      setBatchLoading(false)
    }
  }

  // 提交整个媒体库字幕预处理
  const handleSubmitLibrary = async (libraryId: string) => {
    setSubmitting(libraryId)
    try {
      const targetLangs = selectedTargetLangs
      const res = await subtitlePreprocessApi.submitLibrary(libraryId, targetLangs, forceRegenerate)
      const count = res.data.data.submitted
      if (count > 0) {
        toastRef.current.success(`已提交 ${count} 个字幕预处理任务`)
        loadTasks()
        loadStats()
      } else {
        toastRef.current.info('该媒体库没有需要字幕预处理的视频')
      }
    } catch {
      toastRef.current.error('提交失败')
    } finally {
      setSubmitting(null)
    }
  }

  // P0: 一键重试所有失败任务
  const handleRetryAllFailed = async () => {
    setBatchLoading(true)
    try {
      const res = await subtitlePreprocessApi.retryAllFailed()
      const retried = res.data.data.retried
      if (retried > 0) {
        toastRef.current.success(`已重试 ${retried} 个失败任务`)
        loadTasks()
        loadStats()
      } else {
        toastRef.current.info('没有失败的任务需要重试')
      }
    } catch {
      toastRef.current.error('一键重试失败')
    } finally {
      setBatchLoading(false)
    }
  }

  // P0: 按状态批量删除
  const handleDeleteByStatus = async (status: string) => {
    setBatchLoading(true)
    try {
      const res = await subtitlePreprocessApi.deleteByStatus(status)
      const deleted = res.data.data.deleted
      if (deleted > 0) {
        toastRef.current.success(`已清理 ${deleted} 个${statusLabels[status] || status}任务`)
        loadTasks()
        loadStats()
      } else {
        toastRef.current.info(`没有${statusLabels[status] || status}的任务需要清理`)
      }
    } catch {
      toastRef.current.error('清理失败')
    } finally {
      setBatchLoading(false)
    }
  }

  // P1: 错误信息聚合
  const errorSummary = useMemo(() => {
    if (!stats?.status_counts?.failed) return null
    const failedCount = stats.status_counts.failed
    if (failedCount === 0) return null

    // 从当前页任务中统计错误类型
    const errorMap = new Map<string, number>()
    tasks.filter(t => t.status === 'failed' && t.error).forEach(t => {
      // 提取错误关键信息（去掉具体的媒体名称等）
      let key = t.error
      if (key.includes('Whisper API 返回 HTTP 404')) {
        key = 'Whisper API 端点不存在 (HTTP 404)'
      } else if (key.includes('ASR 失败')) {
        key = 'ASR 语音识别失败'
      } else if (key.includes('音频提取失败')) {
        key = '音频提取失败'
      }
      errorMap.set(key, (errorMap.get(key) || 0) + 1)
    })

    return { total: failedCount, errors: Array.from(errorMap.entries()) }
  }, [stats, tasks])

  const formatDuration = (sec: number) => {
    if (sec <= 0) return '-'
    const h = Math.floor(sec / 3600)
    const m = Math.floor((sec % 3600) / 60)
    const s = Math.floor(sec % 60)
    if (h > 0) return `${h}h ${m}m ${s}s`
    if (m > 0) return `${m}m ${s}s`
    return `${s}s`
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl space-y-6 p-6 animate-fade-in">
        {/* 页面标题骨架 */}
        <div className="flex items-center justify-between">
          <div className="space-y-2">
            <div className="skeleton h-7 w-36 rounded-lg" />
            <div className="skeleton h-4 w-72 rounded" />
          </div>
          <div className="skeleton h-9 w-20 rounded-lg" />
        </div>
        {/* 统计卡片骨架 */}
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
              <div className="flex items-center gap-2 mb-2">
                <div className="skeleton h-3.5 w-3.5 rounded" />
                <div className="skeleton h-3 w-12 rounded" />
              </div>
              <div className="skeleton h-7 w-10 rounded-lg" />
              <div className="skeleton mt-1.5 h-3 w-20 rounded" />
            </div>
          ))}
        </div>
        {/* 任务列表骨架 */}
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="flex items-center gap-4 rounded-xl p-4" style={{ background: 'var(--bg-card)', border: '1px solid var(--border-default)' }}>
              <div className="skeleton h-9 w-9 rounded-lg" />
              <div className="flex-1 space-y-2">
                <div className="skeleton h-4 w-1/3 rounded" />
                <div className="skeleton h-1.5 w-full rounded-full" />
                <div className="skeleton h-3 w-1/2 rounded" />
              </div>
              <div className="skeleton h-8 w-20 rounded-lg" />
            </div>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
            <Subtitles className="text-neon-blue" size={24} />
            字幕预处理
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-tertiary)' }}>
            自动生成、提取和翻译字幕，支持 AI 语音识别和图形字幕 OCR
          </p>
        </div>
        <button
          onClick={() => { loadTasks(); loadStats() }}
          className="flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm transition-colors"
          style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}
        >
          <RefreshCw size={14} />
          刷新
        </button>
      </div>

      {/* 统计卡片 */}
      {stats && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
          <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
              <Activity size={14} className="text-neon-blue" />
              处理中
            </div>
            <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
              {stats.status_counts?.running || 0}
            </div>
            <div className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              {stats.active_workers}/{stats.max_workers} 工作线程
            </div>
          </div>

          <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
              <Clock size={14} className="text-yellow-400" />
              队列
            </div>
            <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>{stats.queue_size}</div>
            <div className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              等待处理 {stats.status_counts?.pending || 0} 个
            </div>
          </div>

          <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
              <CheckCircle2 size={14} className="text-emerald-400" />
              已完成
            </div>
            <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
              {stats.status_counts?.completed || 0}
            </div>
            <div className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              跳过 {stats.status_counts?.skipped || 0} 个
            </div>
          </div>

          <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
              <AlertCircle size={14} className="text-red-400" />
              失败
            </div>
            <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
              {stats.status_counts?.failed || 0}
            </div>
            {(stats.status_counts?.failed || 0) > 0 && (
              <button
                onClick={handleRetryAllFailed}
                disabled={batchLoading}
                className="text-xs mt-1 flex items-center gap-1 transition-colors hover:text-neon-blue"
                style={{ color: 'var(--text-muted)' }}
              >
                <RotateCcw size={10} />
                一键重试全部
              </button>
            )}
          </div>

          {/* P0: ASR 服务健康状态卡片 */}
          <div className="rounded-xl p-4" style={{
            background: 'var(--glass-bg)',
            border: asrHealth?.healthy ? '1px solid var(--neon-blue-6)' : '1px solid rgba(239,68,68,0.2)',
          }}>
            <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
              {asrHealth?.healthy ? (
                <ShieldCheck size={14} className="text-emerald-400" />
              ) : (
                <ShieldAlert size={14} className="text-red-400" />
              )}
              ASR 服务
            </div>
            <div className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>
              {checkingHealth ? (
                <Loader2 size={18} className="animate-spin text-neon-blue" />
              ) : asrHealth?.healthy ? (
                <span className="text-emerald-400">可用</span>
              ) : asrHealth?.configured ? (
                <span className="text-red-400">不可用</span>
              ) : (
                <span style={{ color: 'var(--text-muted)' }}>未配置</span>
              )}
            </div>
            <div className="text-xs mt-1 truncate" style={{ color: 'var(--text-muted)' }} title={asrHealth?.message}>
              {asrHealth?.engine ? `引擎: ${asrHealth.engine}` : asrHealth?.message || '点击检测'}
            </div>
            <button
              onClick={checkASRHealth}
              disabled={checkingHealth}
              className="text-xs mt-1 flex items-center gap-1 transition-colors hover:text-neon-blue"
              style={{ color: 'var(--text-muted)' }}
            >
              <Zap size={10} />
              {checkingHealth ? '检测中...' : '重新检测'}
            </button>
          </div>
        </div>
      )}

      {/* P0: ASR 不可用警告横幅 */}
      {asrHealth && !asrHealth.healthy && asrHealth.configured && (
        <div
          className="flex items-center gap-3 rounded-xl px-4 py-3"
          style={{ background: 'rgba(239,68,68,0.08)', border: '1px solid rgba(239,68,68,0.2)' }}
        >
          <ShieldAlert size={16} className="text-red-400 shrink-0" />
          <div className="flex-1">
            <span className="text-xs font-medium text-red-400">ASR 服务不可用</span>
            <span className="text-xs ml-2" style={{ color: 'var(--text-muted)' }}>
              {asrHealth.message}。没有内嵌字幕的视频将被跳过而非失败。
            </span>
          </div>
          <button
            onClick={checkASRHealth}
            disabled={checkingHealth}
            className="text-xs px-2 py-1 rounded-lg transition-colors hover:bg-white/5"
            style={{ color: 'var(--text-muted)', border: '1px solid rgba(239,68,68,0.2)' }}
          >
            重新检测
          </button>
        </div>
      )}

      {/* P1: 错误信息聚合横幅 */}
      {errorSummary && errorSummary.total > 0 && (
        <div
          className="rounded-xl px-4 py-3"
          style={{ background: 'rgba(239,68,68,0.06)', border: '1px solid rgba(239,68,68,0.15)' }}
        >
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-red-400 flex items-center gap-1.5">
              <AlertCircle size={12} />
              {errorSummary.total} 个任务失败
            </span>
            <div className="flex items-center gap-2">
              <button
                onClick={handleRetryAllFailed}
                disabled={batchLoading}
                className="text-xs px-2.5 py-1 rounded-lg flex items-center gap-1 transition-colors hover:bg-neon-blue/10 disabled:opacity-50"
                style={{ color: 'var(--neon-blue)', border: '1px solid var(--neon-blue-15)' }}
              >
                <RotateCcw size={10} />
                一键重试全部
              </button>
              <button
                onClick={() => handleDeleteByStatus('failed')}
                disabled={batchLoading}
                className="text-xs px-2.5 py-1 rounded-lg flex items-center gap-1 transition-colors hover:bg-red-400/10 disabled:opacity-50"
                style={{ color: 'var(--text-muted)', border: '1px solid rgba(239,68,68,0.15)' }}
              >
                <Trash2 size={10} />
                清理全部失败
              </button>
            </div>
          </div>
          {errorSummary.errors.length > 0 && (
            <div className="space-y-1">
              {errorSummary.errors.map(([error, count]) => (
                <div key={error} className="text-xs flex items-center gap-2" style={{ color: 'var(--text-muted)' }}>
                  <span className="text-red-400/60">×{count}</span>
                  <span className="truncate">{error}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* 媒体库批量字幕预处理 */}
      {libraries.length > 0 && (
        <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
          <h2 className="text-sm font-medium mb-3 flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
            <FolderOpen size={16} className="text-neon-blue" />
            媒体库批量字幕预处理
          </h2>

          {/* P1: 强制重新生成开关 + 翻译语言配置 */}
          <div className="flex items-center gap-4 mb-3 flex-wrap">
            {/* 强制重新生成开关 */}
            <button
              onClick={() => setForceRegenerate(!forceRegenerate)}
              className="flex items-center gap-1.5 text-xs transition-colors"
              style={{ color: forceRegenerate ? 'var(--neon-blue)' : 'var(--text-muted)' }}
              title="启用后将覆盖已有字幕，重新生成"
            >
              {forceRegenerate ? <ToggleRight size={18} className="text-neon-blue" /> : <ToggleLeft size={18} />}
              强制重新生成
            </button>

            <div className="w-px h-4" style={{ background: 'var(--neon-blue-6)' }} />
          </div>

          <div className="flex items-center gap-2 mb-3 flex-wrap">
            <Languages size={14} style={{ color: 'var(--text-muted)' }} />
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>翻译目标语言：</span>

            {/* 多选下拉框 */}
            <div className="relative" ref={langDropdownRef}>
              <button
                onClick={() => setShowLangDropdown(!showLangDropdown)}
                className="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs min-w-[180px] transition-colors"
                style={{
                  background: 'var(--bg-input)',
                  border: showLangDropdown ? '1px solid var(--neon-blue-30)' : '1px solid var(--border-default)',
                  color: 'var(--text-primary)',
                }}
              >
                <span className="flex-1 text-left truncate">
                  {selectedTargetLangs.length === 0 ? (
                    <span style={{ color: 'var(--text-muted)' }}>选择语言（留空则不翻译）</span>
                  ) : (
                    <span className="flex items-center gap-1 flex-wrap">
                      {selectedTargetLangs.map((code) => {
                        const lang = availableLangs.find((l) => l.value === code)
                        return (
                          <span
                            key={code}
                            className="inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-xs"
                            style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}
                          >
                            {lang?.flag} {lang?.label || code}
                            <button
                              onClick={(e) => {
                                e.stopPropagation()
                                setSelectedTargetLangs(selectedTargetLangs.filter((l) => l !== code))
                              }}
                              className="ml-0.5 hover:text-red-400 transition-colors"
                            >
                              <X size={10} />
                            </button>
                          </span>
                        )
                      })}
                    </span>
                  )}
                </span>
                <ChevronDown
                  size={12}
                  className={clsx('transition-transform shrink-0', showLangDropdown && 'rotate-180')}
                  style={{ color: 'var(--text-muted)' }}
                />
              </button>

              {/* 下拉选项 */}
              {showLangDropdown && (
                <div
                  className="absolute top-full left-0 z-50 mt-1 w-56 rounded-xl py-1 shadow-xl max-h-64 overflow-y-auto"
                  style={{
                    background: 'var(--glass-bg)',
                    border: '1px solid var(--neon-blue-15)',
                    backdropFilter: 'blur(12px)',
                  }}
                >
                  {availableLangs.map((lang) => {
                    const isSelected = selectedTargetLangs.includes(lang.value)
                    return (
                      <button
                        key={lang.value}
                        onClick={() => {
                          if (isSelected) {
                            setSelectedTargetLangs(selectedTargetLangs.filter((l) => l !== lang.value))
                          } else {
                            setSelectedTargetLangs([...selectedTargetLangs, lang.value])
                          }
                        }}
                        className={clsx(
                          'flex items-center gap-2 w-full px-3 py-2 text-xs transition-colors',
                          isSelected ? 'text-neon-blue' : 'hover:bg-white/5'
                        )}
                        style={{ color: isSelected ? 'var(--neon-blue)' : 'var(--text-secondary)' }}
                      >
                        {isSelected ? <CheckSquare size={12} /> : <Square size={12} />}
                        <span>{lang.flag}</span>
                        <span>{lang.label}</span>
                        <span className="ml-auto text-[10px]" style={{ color: 'var(--text-muted)' }}>{lang.value}</span>
                      </button>
                    )
                  })}

                  {selectedTargetLangs.length > 0 && (
                    <div className="border-t px-3 py-2" style={{ borderColor: 'var(--neon-blue-6)' }}>
                      <button
                        onClick={() => setSelectedTargetLangs([])}
                        className="text-xs transition-colors hover:text-red-400"
                        style={{ color: 'var(--text-muted)' }}
                      >
                        清除所有
                      </button>
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* 已选标签（当下拉框关闭时显示） */}
            {!showLangDropdown && selectedTargetLangs.length > 0 && (
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
                已选 {selectedTargetLangs.length} 种语言
              </span>
            )}
          </div>

          <div className="flex flex-wrap gap-2">
            {libraries.map((lib) => (
              <button
                key={lib.id}
                onClick={() => handleSubmitLibrary(lib.id)}
                disabled={submitting === lib.id}
                className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-colors disabled:opacity-50"
                style={{ background: 'var(--neon-blue-6)', border: '1px solid var(--neon-blue-15)', color: 'var(--text-secondary)' }}
              >
                {submitting === lib.id ? (
                  <Loader2 size={12} className="animate-spin" />
                ) : (
                  <Send size={12} />
                )}
                {lib.name}
                <span style={{ color: 'var(--text-muted)' }}>({lib.type})</span>
              </button>
            ))}
          </div>
        </div>
      )}

      {/* P2: 状态过滤 + 按状态快捷操作 */}
      <div className="flex items-center justify-between gap-2 flex-wrap">
        <div className="flex items-center gap-2 flex-wrap">
          {['', 'running', 'pending', 'completed', 'failed', 'skipped', 'cancelled'].map((s) => (
            <button
              key={s}
              onClick={() => { setStatusFilter(s); setPage(1); setSelectedIds(new Set()) }}
              className={clsx(
                'rounded-lg px-3 py-1.5 text-xs transition-colors',
              )}
              style={statusFilter === s ? { background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)', color: 'var(--text-primary)' } : { background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)', color: 'var(--text-muted)' }}
            >
              {s === '' ? '全部' : statusLabels[s] || s}
              {s && stats?.status_counts?.[s] ? ` (${stats.status_counts[s]})` : ''}
            </button>
          ))}
        </div>

        {/* P2: 按状态快捷操作按钮 */}
        {statusFilter && statusFilter !== 'running' && (stats?.status_counts?.[statusFilter] || 0) > 0 && (
          <div className="flex items-center gap-2">
            {statusFilter === 'failed' && (
              <button
                onClick={handleRetryAllFailed}
                disabled={batchLoading}
                className="flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs transition-colors hover:bg-neon-blue/10 disabled:opacity-50"
                style={{ color: 'var(--neon-blue)', border: '1px solid var(--neon-blue-15)' }}
              >
                <RotateCcw size={10} />
                重试全部失败
              </button>
            )}
            <button
              onClick={() => handleDeleteByStatus(statusFilter)}
              disabled={batchLoading}
              className="flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs transition-colors hover:bg-red-400/10 hover:text-red-400 disabled:opacity-50"
              style={{ color: 'var(--text-muted)', border: '1px solid var(--neon-blue-6)' }}
            >
              <Trash2 size={10} />
              清理全部{statusLabels[statusFilter]}
            </button>
          </div>
        )}
      </div>

      {/* 批量操作工具栏 */}
      {isSomeSelected && (
        <div
          className="flex items-center gap-3 rounded-xl px-4 py-3 animate-in fade-in slide-in-from-top-2 duration-200"
          style={{ background: 'var(--neon-blue-6)', border: '1px solid var(--neon-blue-15)' }}
        >
          <button
            onClick={toggleSelectAll}
            className="flex items-center gap-1.5 text-xs font-medium transition-colors"
            style={{ color: 'var(--text-primary)' }}
          >
            {isAllSelected ? <CheckSquare size={14} className="text-neon-blue" /> : <Square size={14} />}
            {isAllSelected ? '取消全选' : '全选当前页'}
          </button>

          <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
            已选择 <span className="font-medium text-neon-blue">{selectedIds.size}</span> 项
          </span>

          <div className="flex-1" />

          <button
            onClick={handleBatchCancel}
            disabled={batchLoading}
            className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-colors hover:bg-yellow-400/10 disabled:opacity-50"
            style={{ color: 'var(--text-muted)', border: '1px solid var(--neon-blue-6)' }}
          >
            <XCircle size={12} />
            批量取消
          </button>

          <button
            onClick={handleBatchRetry}
            disabled={batchLoading}
            className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-colors hover:bg-neon-blue/10 disabled:opacity-50"
            style={{ color: 'var(--text-muted)', border: '1px solid var(--neon-blue-6)' }}
          >
            <RotateCcw size={12} />
            批量重试
          </button>

          <button
            onClick={handleBatchDelete}
            disabled={batchLoading}
            className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-colors hover:bg-red-400/10 hover:text-red-400 disabled:opacity-50"
            style={{ color: 'var(--text-muted)', border: '1px solid var(--neon-blue-6)' }}
          >
            {batchLoading ? <Loader2 size={12} className="animate-spin" /> : <Trash2 size={12} />}
            批量删除
          </button>

          <button
            onClick={() => setSelectedIds(new Set())}
            className="text-xs transition-colors hover:text-red-400"
            style={{ color: 'var(--text-muted)' }}
          >
            清除选择
          </button>
        </div>
      )}

      {/* 任务列表 */}
      <div className="space-y-3">
        {/* 列表头部：全选复选框 */}
        {tasks.length > 0 && (
          <div className="flex items-center gap-3 px-4 py-2">
            <button
              onClick={toggleSelectAll}
              className="flex items-center gap-2 text-xs transition-colors"
              style={{ color: 'var(--text-muted)' }}
            >
              {isAllSelected ? (
                <CheckSquare size={16} className="text-neon-blue" />
              ) : (
                <Square size={16} />
              )}
              {isAllSelected ? '取消全选' : '全选'}
            </button>
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              共 {total} 条，当前第 {page}/{totalPages} 页
            </span>
          </div>
        )}

        {tasks.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16" style={{ color: 'var(--text-muted)' }}>
            <Subtitles size={48} className="mb-4 opacity-30" />
            <p>暂无字幕预处理任务</p>
            <p className="text-xs mt-1">选择媒体库提交批量字幕预处理，或在配置中启用自动预处理</p>
          </div>
        ) : (
          tasks.map((task) => (
            <div
              key={task.id}
              className={clsx('rounded-xl p-4 transition-colors', selectedIds.has(task.id) && 'ring-1 ring-neon-blue/30')}
              style={{
                background: selectedIds.has(task.id) ? 'var(--neon-blue-6)' : 'var(--glass-bg)',
                border: '1px solid var(--neon-blue-6)',
              }}
            >
              <div className="flex items-start justify-between gap-4">
                {/* 复选框 */}
                <button
                  onClick={() => toggleSelect(task.id)}
                  className="mt-0.5 shrink-0 transition-colors"
                  style={{ color: selectedIds.has(task.id) ? 'var(--neon-blue)' : 'var(--text-muted)' }}
                >
                  {selectedIds.has(task.id) ? <CheckSquare size={16} /> : <Square size={16} />}
                </button>

                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className={statusColors[task.status]}>
                      {statusIcons[task.status]}
                    </span>
                    <h3 className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                      {task.media_title || task.media_id}
                    </h3>
                    <span className={clsx('text-xs px-1.5 py-0.5 rounded', statusColors[task.status])}
                      style={{ background: 'var(--neon-blue-6)' }}>
                      {statusLabels[task.status] || task.status}
                    </span>
                    {task.subtitle_source && (
                      <span className="text-xs px-1.5 py-0.5 rounded"
                        style={{ background: 'var(--neon-blue-6)', color: 'var(--text-muted)' }}>
                        {sourceLabels[task.subtitle_source] || task.subtitle_source}
                      </span>
                    )}
                  </div>

                  {/* 进度条 */}
                  {task.status === 'running' && (
                    <div className="mt-2">
                      <div className="flex items-center justify-between text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
                        <span className="flex items-center gap-1">
                          {task.phase === 'generate' && <Sparkles size={10} />}
                          {task.phase === 'translate' && <Globe size={10} />}
                          {task.phase === 'extract' && <FileText size={10} />}
                          {task.phase === 'clean' && <Sparkles size={10} />}
                          {phaseLabels[task.phase] || task.phase}
                          {task.message && ` · ${task.message}`}
                        </span>
                        <span>{task.progress.toFixed(1)}%</span>
                      </div>
                      <div className="h-1.5 w-full rounded-full" style={{ background: 'var(--progress-track-bg)' }}>
                        <div
                          className="h-full rounded-full transition-all duration-500"
                          style={{
                            width: `${task.progress}%`,
                            background: 'linear-gradient(90deg, var(--neon-purple), var(--neon-blue))',
                          }}
                        />
                      </div>
                    </div>
                  )}

                  {/* 详细信息 */}
                  <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                    {task.cue_count > 0 && (
                      <span className="flex items-center gap-1">
                        <FileText size={10} />
                        {task.cue_count} 条字幕
                      </span>
                    )}
                    {task.detected_language && (
                      <span className="flex items-center gap-1">
                        <Globe size={10} />
                        源语言: {task.detected_language}
                      </span>
                    )}
                    {task.target_langs && (
                      <span className="flex items-center gap-1">
                        <Languages size={10} />
                        翻译: {task.target_langs}
                      </span>
                    )}
                    {task.failed_langs && (
                      <span className="flex items-center gap-1 text-red-400" title={`翻译失败的语言: ${task.failed_langs}`}>
                        <AlertCircle size={10} />
                        失败语言: {task.failed_langs}
                      </span>
                    )}
                    {task.elapsed_sec > 0 && (
                      <span>耗时 {formatDuration(task.elapsed_sec)}</span>
                    )}
                    {task.force_regenerate && (
                      <span className="text-yellow-400">强制重新生成</span>
                    )}
                    {task.error && (
                      <span className="text-red-400">{task.error}</span>
                    )}
                    {task.status === 'completed' && task.message && (
                      <span className="text-emerald-400">{task.message}</span>
                    )}
                    {task.status === 'skipped' && task.message && (
                      <span className="text-orange-400">{task.message}</span>
                    )}
                  </div>
                </div>

                {/* 操作按钮 */}
                <div className="flex items-center gap-1 shrink-0">
                  {(task.status === 'running' || task.status === 'pending') && (
                    <button onClick={() => handleCancel(task.id)} className="p-1.5 rounded-lg hover:text-red-400 hover:bg-red-400/10 transition-colors" style={{ color: 'var(--text-muted)' }} title="取消">
                      <XCircle size={14} />
                    </button>
                  )}
                  {task.status === 'failed' && (
                    <button onClick={() => handleRetry(task.id)} className="p-1.5 rounded-lg hover:text-neon-blue hover:bg-neon-blue/10 transition-colors" style={{ color: 'var(--text-muted)' }} title="重试">
                      <RotateCcw size={14} />
                    </button>
                  )}
                  {(task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled' || task.status === 'skipped') && (
                    <button onClick={() => handleDelete(task.id)} className="p-1.5 rounded-lg hover:text-red-400 hover:bg-red-400/10 transition-colors" style={{ color: 'var(--text-muted)' }} title="删除">
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      {/* 增强分页 */}
      {total > 0 && (
        <div className="flex items-center justify-between gap-4 flex-wrap">
          {/* 左侧：分页信息 */}
          <div className="flex items-center gap-3 text-xs" style={{ color: 'var(--text-muted)' }}>
            <span>
              共 <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{total}</span> 条
            </span>
            <span>·</span>
            <span>
              第 <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{page}</span> / {totalPages} 页
            </span>
            <span>·</span>
            <span>
              显示第 {Math.min((page - 1) * pageSize + 1, total)}-{Math.min(page * pageSize, total)} 条
            </span>
          </div>

          {/* 中间：分页按钮 */}
          <div className="flex items-center gap-1">
            <button
              onClick={() => setPage(1)}
              disabled={page <= 1}
              className="p-1.5 rounded-lg disabled:opacity-20 transition-colors hover:bg-white/5"
              style={{ color: 'var(--text-muted)' }}
              title="第一页"
            >
              <ChevronsLeft size={14} />
            </button>
            <button
              onClick={() => setPage(Math.max(1, page - 1))}
              disabled={page <= 1}
              className="p-1.5 rounded-lg disabled:opacity-20 transition-colors hover:bg-white/5"
              style={{ color: 'var(--text-muted)' }}
              title="上一页"
            >
              <ChevronLeft size={14} />
            </button>

            {/* 页码按钮 */}
            {(() => {
              const pages: (number | string)[] = []
              const maxVisible = 5
              let start = Math.max(1, page - Math.floor(maxVisible / 2))
              let end = Math.min(totalPages, start + maxVisible - 1)
              if (end - start + 1 < maxVisible) {
                start = Math.max(1, end - maxVisible + 1)
              }
              if (start > 1) {
                pages.push(1)
                if (start > 2) pages.push('...')
              }
              for (let i = start; i <= end; i++) pages.push(i)
              if (end < totalPages) {
                if (end < totalPages - 1) pages.push('...')
                pages.push(totalPages)
              }
              return pages.map((p, idx) =>
                typeof p === 'string' ? (
                  <span key={`ellipsis-${idx}`} className="px-1 text-xs" style={{ color: 'var(--text-muted)' }}>…</span>
                ) : (
                  <button
                    key={p}
                    onClick={() => setPage(p)}
                    className={clsx('min-w-[28px] h-7 rounded-lg text-xs transition-colors')}
                    style={
                      p === page
                        ? { background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)', color: 'var(--text-primary)' }
                        : { color: 'var(--text-muted)' }
                    }
                  >
                    {p}
                  </button>
                )
              )
            })()}

            <button
              onClick={() => setPage(Math.min(totalPages, page + 1))}
              disabled={page >= totalPages}
              className="p-1.5 rounded-lg disabled:opacity-20 transition-colors hover:bg-white/5"
              style={{ color: 'var(--text-muted)' }}
              title="下一页"
            >
              <ChevronRight size={14} />
            </button>
            <button
              onClick={() => setPage(totalPages)}
              disabled={page >= totalPages}
              className="p-1.5 rounded-lg disabled:opacity-20 transition-colors hover:bg-white/5"
              style={{ color: 'var(--text-muted)' }}
              title="最后一页"
            >
              <ChevronsRight size={14} />
            </button>
          </div>

          {/* 右侧：每页数量选择 */}
          <div className="flex items-center gap-2 text-xs" style={{ color: 'var(--text-muted)' }}>
            <span>每页</span>
            <select
              value={pageSize}
              onChange={(e) => {
                const newSize = Number(e.target.value)
                setPageSize(newSize)
                setPage(1)
                setSelectedIds(new Set())
              }}
              className="rounded-lg px-2 py-1 text-xs appearance-none cursor-pointer"
              style={{
                background: 'var(--glass-bg)',
                border: '1px solid var(--neon-blue-6)',
                color: 'var(--text-primary)',
              }}
            >
              {[10, 20, 50, 100].map((size) => (
                <option key={size} value={size}>{size}</option>
              ))}
            </select>
            <span>条</span>
          </div>
        </div>
      )}
    </div>
  )
}
