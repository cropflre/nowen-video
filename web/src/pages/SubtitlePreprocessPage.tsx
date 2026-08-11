import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { subtitlePreprocessApi } from '@/api/subtitlePreprocess'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import {
  Button,
  EmptyState,
  PageContainer,
  Surface,
  Tag,
  type TagTone,
} from '@/components/design-system'
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
  X,
  ChevronDown,
  Zap,
  ShieldAlert,
  ShieldCheck,
  ToggleLeft,
  ToggleRight,
} from 'lucide-react'
import clsx from 'clsx'

const statusColors: Record<string, string> = {
  pending: 'text-[var(--nv-status-warning)]',
  running: 'text-[var(--nv-action-primary)]',
  completed: 'text-[var(--nv-status-success)]',
  failed: 'text-[var(--nv-status-danger)]',
  cancelled: 'text-[var(--nv-text-tertiary)]',
  skipped: 'text-[var(--nv-status-warning)]',
}

const statusTones: Record<string, TagTone> = {
  pending: 'warning',
  running: 'brand',
  completed: 'success',
  failed: 'danger',
  cancelled: 'neutral',
  skipped: 'warning',
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
  pending: <Clock size={14} aria-hidden="true" />,
  running: <Loader2 size={14} className="animate-spin" aria-hidden="true" />,
  completed: <CheckCircle2 size={14} aria-hidden="true" />,
  failed: <AlertCircle size={14} aria-hidden="true" />,
  cancelled: <XCircle size={14} aria-hidden="true" />,
  skipped: <SkipForward size={14} aria-hidden="true" />,
}

const phaseLabels: Record<string, string> = {
  check: '检查字幕',
  extract: '提取字幕',
  clean: '字幕清洗',
  generate: 'AI 生成',
  translate: '多语言翻译',
  done: '完成',
}

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
  const { page, size: pageSize, setPage, setSize, totalPages: calcTotalPages } = usePagination({ initialSize: 20 })
  const [statusFilter, setStatusFilter] = useState('')
  const [stats, setStats] = useState<SubtitlePreprocessStatistics | null>(null)
  const [loading, setLoading] = useState(true)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [submitting, setSubmitting] = useState<string | null>(null)
  const [selectedTargetLangs, setSelectedTargetLangs] = useState<string[]>([])
  const [showLangDropdown, setShowLangDropdown] = useState(false)
  const langDropdownRef = useRef<HTMLDivElement>(null)
  const [forceRegenerate, setForceRegenerate] = useState(false)
  const [asrHealth, setAsrHealth] = useState<ASRHealthStatus | null>(null)
  const [checkingHealth, setCheckingHealth] = useState(false)

  const availableLangs = useMemo(() => LANGUAGE_OPTIONS.filter((language) => language.value !== ''), [])
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchLoading, setBatchLoading] = useState(false)

  const totalPages = useMemo(() => calcTotalPages(total), [calcTotalPages, total])
  const isAllSelected = tasks.length > 0 && tasks.every((task) => selectedIds.has(task.id))
  const isSomeSelected = selectedIds.size > 0

  const toggleSelectAll = () => {
    if (isAllSelected) {
      const newSet = new Set(selectedIds)
      tasks.forEach((task) => newSet.delete(task.id))
      setSelectedIds(newSet)
    } else {
      const newSet = new Set(selectedIds)
      tasks.forEach((task) => newSet.add(task.id))
      setSelectedIds(newSet)
    }
  }

  const toggleSelect = (id: string) => {
    const newSet = new Set(selectedIds)
    if (newSet.has(id)) newSet.delete(id)
    else newSet.add(id)
    setSelectedIds(newSet)
  }

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (langDropdownRef.current && !langDropdownRef.current.contains(event.target as Node)) {
        setShowLangDropdown(false)
      }
    }
    if (showLangDropdown) document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [showLangDropdown])

  const loadTasks = useCallback(async () => {
    try {
      const res = await subtitlePreprocessApi.listTasks({ page, page_size: pageSize, status: statusFilter })
      setTasks(res.data.data.tasks || [])
      setTotal(res.data.data.total)
    } catch {
      toastRef.current.error('加载字幕预处理任务失败')
    }
  }, [page, pageSize, statusFilter])

  const loadStats = useCallback(async () => {
    try {
      const res = await subtitlePreprocessApi.getStatistics()
      setStats(res.data.data)
    } catch {
      // 忽略
    }
  }, [])

  const loadLibraries = useCallback(async () => {
    try {
      const res = await api.get<{ data: Library[] }>('/libraries')
      setLibraries(res.data.data || [])
    } catch {
      // 忽略
    }
  }, [])

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

  const errorSummary = useMemo(() => {
    if (!stats?.status_counts?.failed) return null
    const failedCount = stats.status_counts.failed
    if (failedCount === 0) return null

    const errorMap = new Map<string, number>()
    tasks.filter((task) => task.status === 'failed' && task.error).forEach((task) => {
      let key = task.error
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
      <PageContainer width="wide" className="space-y-6 py-6">
        <div className="flex items-center justify-between">
          <div className="space-y-2">
            <div className="skeleton h-7 w-36 rounded-lg" />
            <div className="skeleton h-4 w-72 rounded" />
          </div>
          <div className="skeleton h-9 w-20 rounded-lg" />
        </div>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
          {Array.from({ length: 5 }).map((_, index) => (
            <Surface key={index} className="p-4">
              <div className="mb-2 flex items-center gap-2">
                <div className="skeleton h-3.5 w-3.5 rounded" />
                <div className="skeleton h-3 w-12 rounded" />
              </div>
              <div className="skeleton h-7 w-10 rounded-lg" />
              <div className="skeleton mt-1.5 h-3 w-20 rounded" />
            </Surface>
          ))}
        </div>
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, index) => (
            <Surface key={index} className="flex items-center gap-4 p-4">
              <div className="skeleton h-9 w-9 rounded-lg" />
              <div className="flex-1 space-y-2">
                <div className="skeleton h-4 w-1/3 rounded" />
                <div className="skeleton h-1.5 w-full rounded-full" />
                <div className="skeleton h-3 w-1/2 rounded" />
              </div>
              <div className="skeleton h-8 w-20 rounded-lg" />
            </Surface>
          ))}
        </div>
      </PageContainer>
    )
  }

  return (
    <PageContainer width="wide" className="space-y-6 py-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold text-[var(--nv-text-primary)]">
            <Subtitles className="text-[var(--nv-action-primary)]" size={24} aria-hidden="true" />
            字幕预处理
          </h1>
          <p className="mt-1 text-sm text-[var(--nv-text-tertiary)]">
            自动生成、提取和翻译字幕，支持 AI 语音识别和图形字幕 OCR
          </p>
        </div>
        <Button type="button" variant="secondary" size="sm" onClick={() => { loadTasks(); loadStats() }}>
          <RefreshCw size={14} aria-hidden="true" />
          刷新
        </Button>
      </div>

      {stats && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-5">
          <Surface className="p-4">
            <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
              <Activity size={14} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
              处理中
            </div>
            <div className="text-2xl font-bold text-[var(--nv-text-primary)]">{stats.status_counts?.running || 0}</div>
            <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{stats.active_workers}/{stats.max_workers} 工作线程</div>
          </Surface>

          <Surface className="p-4">
            <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
              <Clock size={14} className="text-[var(--nv-status-warning)]" aria-hidden="true" />
              队列
            </div>
            <div className="text-2xl font-bold text-[var(--nv-text-primary)]">{stats.queue_size}</div>
            <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">等待处理 {stats.status_counts?.pending || 0} 个</div>
          </Surface>

          <Surface className="p-4">
            <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
              <CheckCircle2 size={14} className="text-[var(--nv-status-success)]" aria-hidden="true" />
              已完成
            </div>
            <div className="text-2xl font-bold text-[var(--nv-text-primary)]">{stats.status_counts?.completed || 0}</div>
            <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">跳过 {stats.status_counts?.skipped || 0} 个</div>
          </Surface>

          <Surface className="p-4">
            <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
              <AlertCircle size={14} className="text-[var(--nv-status-danger)]" aria-hidden="true" />
              失败
            </div>
            <div className="text-2xl font-bold text-[var(--nv-text-primary)]">{stats.status_counts?.failed || 0}</div>
            {(stats.status_counts?.failed || 0) > 0 && (
              <Button type="button" variant="ghost" size="sm" className="mt-1 h-auto px-0 py-0 text-xs" onClick={handleRetryAllFailed} disabled={batchLoading}>
                <RotateCcw size={10} aria-hidden="true" />
                一键重试全部
              </Button>
            )}
          </Surface>

          <Surface className={clsx('p-4', asrHealth?.configured && !asrHealth.healthy && 'border-[var(--nv-status-danger)]')}>
            <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
              {asrHealth?.healthy
                ? <ShieldCheck size={14} className="text-[var(--nv-status-success)]" aria-hidden="true" />
                : <ShieldAlert size={14} className="text-[var(--nv-status-danger)]" aria-hidden="true" />}
              ASR 服务
            </div>
            <div className="text-lg font-bold text-[var(--nv-text-primary)]">
              {checkingHealth ? (
                <Loader2 size={18} className="animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" />
              ) : asrHealth?.healthy ? (
                <span className="text-[var(--nv-status-success)]">可用</span>
              ) : asrHealth?.configured ? (
                <span className="text-[var(--nv-status-danger)]">不可用</span>
              ) : (
                <span className="text-[var(--nv-text-tertiary)]">未配置</span>
              )}
            </div>
            <div className="mt-1 truncate text-xs text-[var(--nv-text-tertiary)]" title={asrHealth?.message}>
              {asrHealth?.engine ? `引擎: ${asrHealth.engine}` : asrHealth?.message || '点击检测'}
            </div>
            <Button type="button" variant="ghost" size="sm" className="mt-1 h-auto px-0 py-0 text-xs" onClick={checkASRHealth} disabled={checkingHealth}>
              <Zap size={10} aria-hidden="true" />
              {checkingHealth ? '检测中...' : '重新检测'}
            </Button>
          </Surface>
        </div>
      )}

      {asrHealth && !asrHealth.healthy && asrHealth.configured && (
        <Surface
          className="flex flex-wrap items-center gap-3 p-4"
          style={{
            background: 'color-mix(in srgb, var(--nv-status-danger) 7%, var(--nv-bg-surface))',
            borderColor: 'color-mix(in srgb, var(--nv-status-danger) 30%, var(--nv-border-default))',
          }}
        >
          <ShieldAlert size={16} className="shrink-0 text-[var(--nv-status-danger)]" aria-hidden="true" />
          <div className="min-w-0 flex-1">
            <span className="text-xs font-semibold text-[var(--nv-status-danger)]">ASR 服务不可用</span>
            <span className="ml-2 text-xs text-[var(--nv-text-tertiary)]">
              {asrHealth.message}。没有内嵌字幕的视频将被跳过而非失败。
            </span>
          </div>
          <Button type="button" variant="secondary" size="sm" onClick={checkASRHealth} disabled={checkingHealth}>
            重新检测
          </Button>
        </Surface>
      )}

      {errorSummary && errorSummary.total > 0 && (
        <Surface
          className="space-y-3 p-4"
          style={{
            background: 'color-mix(in srgb, var(--nv-status-danger) 5%, var(--nv-bg-surface))',
            borderColor: 'color-mix(in srgb, var(--nv-status-danger) 24%, var(--nv-border-default))',
          }}
        >
          <div className="flex flex-wrap items-center justify-between gap-3">
            <span className="flex items-center gap-1.5 text-xs font-semibold text-[var(--nv-status-danger)]">
              <AlertCircle size={12} aria-hidden="true" />
              {errorSummary.total} 个任务失败
            </span>
            <div className="flex flex-wrap items-center gap-2">
              <Button type="button" variant="secondary" size="sm" onClick={handleRetryAllFailed} disabled={batchLoading}>
                <RotateCcw size={10} aria-hidden="true" />
                一键重试全部
              </Button>
              <Button type="button" variant="danger" size="sm" onClick={() => handleDeleteByStatus('failed')} disabled={batchLoading}>
                <Trash2 size={10} aria-hidden="true" />
                清理全部失败
              </Button>
            </div>
          </div>
          {errorSummary.errors.length > 0 && (
            <div className="space-y-1">
              {errorSummary.errors.map(([error, count]) => (
                <div key={error} className="flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
                  <span className="text-[var(--nv-status-danger)]">×{count}</span>
                  <span className="truncate">{error}</span>
                </div>
              ))}
            </div>
          )}
        </Surface>
      )}

      {libraries.length > 0 && (
        <Surface className="p-4">
          <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
            <FolderOpen size={16} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
            媒体库批量字幕预处理
          </h2>

          <div className="mb-3 flex flex-wrap items-center gap-3">
            <Button
              type="button"
              variant={forceRegenerate ? 'secondary' : 'ghost'}
              size="sm"
              onClick={() => setForceRegenerate(!forceRegenerate)}
              className={forceRegenerate ? 'text-[var(--nv-action-primary)]' : undefined}
              aria-pressed={forceRegenerate}
              title="启用后将覆盖已有字幕，重新生成"
            >
              {forceRegenerate
                ? <ToggleRight size={18} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                : <ToggleLeft size={18} aria-hidden="true" />}
              强制重新生成
            </Button>
            <div className="h-4 w-px bg-[var(--nv-border-subtle)]" aria-hidden="true" />
            <div className="flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
              <Languages size={14} aria-hidden="true" />
              翻译目标语言
            </div>
          </div>

          <div className="mb-3 flex flex-wrap items-center gap-2">
            <div className="relative" ref={langDropdownRef}>
              <button
                type="button"
                onClick={() => setShowLangDropdown(!showLangDropdown)}
                aria-haspopup="listbox"
                aria-expanded={showLangDropdown}
                className={clsx(
                  'flex min-h-9 min-w-[200px] items-center gap-2 rounded-[var(--nv-radius-control)] border bg-[var(--nv-bg-control)] px-3 py-2 text-xs text-[var(--nv-text-primary)] outline-none transition-[background-color,border-color,box-shadow]',
                  showLangDropdown
                    ? 'border-[var(--nv-border-hover)] shadow-[var(--nv-shadow-focus)]'
                    : 'border-[var(--nv-border-default)] hover:border-[var(--nv-border-hover)]',
                )}
              >
                <span className="min-w-0 flex-1 truncate text-left">
                  {selectedTargetLangs.length === 0
                    ? <span className="text-[var(--nv-text-tertiary)]">选择语言（留空则不翻译）</span>
                    : `已选 ${selectedTargetLangs.length} 种语言`}
                </span>
                <ChevronDown
                  size={12}
                  className={clsx('shrink-0 text-[var(--nv-text-tertiary)] transition-transform', showLangDropdown && 'rotate-180')}
                  aria-hidden="true"
                />
              </button>

              {showLangDropdown && (
                <div
                  className="nv-surface absolute left-0 top-full z-[var(--nv-z-dropdown)] mt-1 max-h-64 w-60 overflow-y-auto py-1 shadow-[var(--nv-shadow-elevated)]"
                  role="listbox"
                  aria-label="翻译目标语言"
                  aria-multiselectable="true"
                >
                  {availableLangs.map((language) => {
                    const isSelected = selectedTargetLangs.includes(language.value)
                    return (
                      <button
                        key={language.value}
                        type="button"
                        role="option"
                        aria-selected={isSelected}
                        onClick={() => {
                          if (isSelected) {
                            setSelectedTargetLangs(selectedTargetLangs.filter((value) => value !== language.value))
                          } else {
                            setSelectedTargetLangs([...selectedTargetLangs, language.value])
                          }
                        }}
                        className={clsx(
                          'flex w-full items-center gap-2 px-3 py-2 text-xs transition-colors',
                          isSelected
                            ? 'bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]'
                            : 'text-[var(--nv-text-secondary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]',
                        )}
                      >
                        {isSelected ? <CheckSquare size={12} aria-hidden="true" /> : <Square size={12} aria-hidden="true" />}
                        <span aria-hidden="true">{language.flag}</span>
                        <span>{language.label}</span>
                        <span className="ml-auto text-[10px] text-[var(--nv-text-tertiary)]">{language.value}</span>
                      </button>
                    )
                  })}
                  {selectedTargetLangs.length > 0 && (
                    <div className="border-t border-[var(--nv-border-subtle)] px-3 py-2">
                      <Button type="button" variant="ghost" size="sm" className="h-auto px-0 py-0 text-xs" onClick={() => setSelectedTargetLangs([])}>
                        清除所有
                      </Button>
                    </div>
                  )}
                </div>
              )}
            </div>

            {selectedTargetLangs.map((code) => {
              const language = availableLangs.find((item) => item.value === code)
              return (
                <button
                  key={code}
                  type="button"
                  onClick={() => setSelectedTargetLangs(selectedTargetLangs.filter((value) => value !== code))}
                  className="rounded-[var(--nv-radius-pill)] outline-none focus-visible:shadow-[var(--nv-shadow-focus)]"
                  title={`移除 ${language?.label || code}`}
                >
                  <Tag tone="brand">
                    {language?.flag} {language?.label || code}
                    <X size={10} aria-hidden="true" />
                  </Tag>
                </button>
              )
            })}
          </div>

          <div className="flex flex-wrap gap-2">
            {libraries.map((library) => (
              <Button
                key={library.id}
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => handleSubmitLibrary(library.id)}
                loading={submitting === library.id}
              >
                {submitting !== library.id && <Send size={12} aria-hidden="true" />}
                {library.name}
                <span className="text-[var(--nv-text-tertiary)]">({library.type})</span>
              </Button>
            ))}
          </div>
        </Surface>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          {['', 'running', 'pending', 'completed', 'failed', 'skipped', 'cancelled'].map((status) => {
            const active = statusFilter === status
            return (
              <Button
                key={status}
                type="button"
                variant={active ? 'primary' : 'secondary'}
                size="sm"
                aria-pressed={active}
                onClick={() => { setStatusFilter(status); setPage(1); setSelectedIds(new Set()) }}
              >
                {status === '' ? '全部' : statusLabels[status] || status}
                {status && stats?.status_counts?.[status] ? ` (${stats.status_counts[status]})` : ''}
              </Button>
            )
          })}
        </div>

        {statusFilter && statusFilter !== 'running' && (stats?.status_counts?.[statusFilter] || 0) > 0 && (
          <div className="flex flex-wrap items-center gap-2">
            {statusFilter === 'failed' && (
              <Button type="button" variant="secondary" size="sm" onClick={handleRetryAllFailed} disabled={batchLoading}>
                <RotateCcw size={10} aria-hidden="true" />
                重试全部失败
              </Button>
            )}
            <Button type="button" variant="danger" size="sm" onClick={() => handleDeleteByStatus(statusFilter)} disabled={batchLoading}>
              <Trash2 size={10} aria-hidden="true" />
              清理全部{statusLabels[statusFilter]}
            </Button>
          </div>
        )}
      </div>

      {isSomeSelected && (
        <Surface className="flex flex-wrap items-center gap-3 border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] px-4 py-3">
          <Button type="button" variant="ghost" size="sm" onClick={toggleSelectAll}>
            {isAllSelected
              ? <CheckSquare size={14} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
              : <Square size={14} aria-hidden="true" />}
            {isAllSelected ? '取消全选' : '全选当前页'}
          </Button>
          <span className="text-xs text-[var(--nv-text-tertiary)]">
            已选择 <strong className="text-[var(--nv-action-primary)]">{selectedIds.size}</strong> 项
          </span>
          <div className="flex-1" />
          <Button type="button" variant="secondary" size="sm" onClick={handleBatchCancel} disabled={batchLoading}>
            <XCircle size={12} aria-hidden="true" />
            批量取消
          </Button>
          <Button type="button" variant="secondary" size="sm" onClick={handleBatchRetry} disabled={batchLoading}>
            <RotateCcw size={12} aria-hidden="true" />
            批量重试
          </Button>
          <Button type="button" variant="danger" size="sm" onClick={handleBatchDelete} loading={batchLoading}>
            {!batchLoading && <Trash2 size={12} aria-hidden="true" />}
            批量删除
          </Button>
          <Button type="button" variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>
            清除选择
          </Button>
        </Surface>
      )}

      <div className="space-y-3">
        {tasks.length > 0 && (
          <div className="flex flex-wrap items-center gap-3 px-1 py-1">
            <Button type="button" variant="ghost" size="sm" onClick={toggleSelectAll}>
              {isAllSelected
                ? <CheckSquare size={16} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                : <Square size={16} aria-hidden="true" />}
              {isAllSelected ? '取消全选' : '全选'}
            </Button>
            <span className="text-xs text-[var(--nv-text-tertiary)]">共 {total} 条，当前第 {page}/{totalPages} 页</span>
          </div>
        )}

        {tasks.length === 0 ? (
          <Surface>
            <EmptyState
              icon={<Subtitles size={28} aria-hidden="true" />}
              title="暂无字幕预处理任务"
              description="选择媒体库提交批量字幕预处理，或在配置中启用自动预处理。"
            />
          </Surface>
        ) : (
          tasks.map((task) => {
            const selected = selectedIds.has(task.id)
            return (
              <Surface
                key={task.id}
                className={clsx(
                  'p-4 transition-[background-color,border-color,box-shadow] duration-200',
                  selected && 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] shadow-[var(--nv-shadow-card)]',
                )}
              >
                <div className="flex items-start justify-between gap-4">
                  <button
                    type="button"
                    onClick={() => toggleSelect(task.id)}
                    className={clsx(
                      'mt-0.5 shrink-0 rounded-[var(--nv-radius-sm)] p-1 transition-colors',
                      selected ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-tertiary)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]',
                    )}
                    aria-label={selected ? `取消选择 ${task.media_title || task.media_id}` : `选择 ${task.media_title || task.media_id}`}
                    aria-pressed={selected}
                  >
                    {selected ? <CheckSquare size={16} aria-hidden="true" /> : <Square size={16} aria-hidden="true" />}
                  </button>

                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className={statusColors[task.status]} aria-hidden="true">{statusIcons[task.status]}</span>
                      <h3 className="min-w-0 flex-1 truncate text-sm font-semibold text-[var(--nv-text-primary)]">
                        {task.media_title || task.media_id}
                      </h3>
                      <Tag tone={statusTones[task.status] || 'neutral'}>{statusLabels[task.status] || task.status}</Tag>
                      {task.subtitle_source && <Tag tone="neutral">{sourceLabels[task.subtitle_source] || task.subtitle_source}</Tag>}
                    </div>

                    {task.status === 'running' && (
                      <div className="mt-3">
                        <div className="mb-1 flex items-center justify-between gap-3 text-xs text-[var(--nv-text-tertiary)]">
                          <span className="flex min-w-0 items-center gap-1 truncate">
                            {task.phase === 'generate' && <Sparkles size={10} aria-hidden="true" />}
                            {task.phase === 'translate' && <Globe size={10} aria-hidden="true" />}
                            {task.phase === 'extract' && <FileText size={10} aria-hidden="true" />}
                            {task.phase === 'clean' && <Sparkles size={10} aria-hidden="true" />}
                            {phaseLabels[task.phase] || task.phase}
                            {task.message && ` · ${task.message}`}
                          </span>
                          <span className="shrink-0 tabular-nums">{task.progress.toFixed(1)}%</span>
                        </div>
                        <div className="h-1.5 w-full overflow-hidden rounded-full bg-[var(--nv-bg-active)]">
                          <div
                            className="h-full rounded-full bg-[var(--nv-action-primary)] transition-[width] duration-500"
                            style={{ width: `${task.progress}%` }}
                          />
                        </div>
                      </div>
                    )}

                    <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--nv-text-tertiary)]">
                      {task.cue_count > 0 && <span className="flex items-center gap-1"><FileText size={10} aria-hidden="true" />{task.cue_count} 条字幕</span>}
                      {task.detected_language && <span className="flex items-center gap-1"><Globe size={10} aria-hidden="true" />源语言: {task.detected_language}</span>}
                      {task.target_langs && <span className="flex items-center gap-1"><Languages size={10} aria-hidden="true" />翻译: {task.target_langs}</span>}
                      {task.failed_langs && <span className="flex items-center gap-1 text-[var(--nv-status-danger)]" title={`翻译失败的语言: ${task.failed_langs}`}><AlertCircle size={10} aria-hidden="true" />失败语言: {task.failed_langs}</span>}
                      {task.elapsed_sec > 0 && <span>耗时 {formatDuration(task.elapsed_sec)}</span>}
                      {task.force_regenerate && <span className="text-[var(--nv-status-warning)]">强制重新生成</span>}
                      {task.error && <span className="text-[var(--nv-status-danger)]">{task.error}</span>}
                      {task.status === 'completed' && task.message && <span className="text-[var(--nv-status-success)]">{task.message}</span>}
                      {task.status === 'skipped' && task.message && <span className="text-[var(--nv-status-warning)]">{task.message}</span>}
                    </div>
                  </div>

                  <div className="flex shrink-0 items-center gap-1">
                    {(task.status === 'running' || task.status === 'pending') && (
                      <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => handleCancel(task.id)} aria-label="取消" title="取消">
                        <XCircle size={14} aria-hidden="true" />
                      </Button>
                    )}
                    {task.status === 'failed' && (
                      <Button type="button" variant="secondary" size="sm" iconOnly onClick={() => handleRetry(task.id)} aria-label="重试" title="重试">
                        <RotateCcw size={14} aria-hidden="true" />
                      </Button>
                    )}
                    {(task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled' || task.status === 'skipped') && (
                      <Button type="button" variant="danger" size="sm" iconOnly onClick={() => handleDelete(task.id)} aria-label="删除" title="删除">
                        <Trash2 size={14} aria-hidden="true" />
                      </Button>
                    )}
                  </div>
                </div>
              </Surface>
            )
          })
        )}
      </div>

      <Pagination
        page={page}
        totalPages={totalPages}
        total={total}
        pageSize={pageSize}
        pageSizeOptions={[10, 20, 50, 100]}
        onPageChange={setPage}
        onPageSizeChange={(newSize) => {
          setSize(newSize)
          setSelectedIds(new Set())
        }}
      />
    </PageContainer>
  )
}
