import { useState, useEffect, useCallback, useRef, useMemo, useLayoutEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { pageVariants, staggerContainerVariants, staggerItemVariants, easeSmooth, durations } from '@/lib/motion'

function useAnimatedCounter(value: number, duration = 600): number {
  const [display, setDisplay] = useState(value)
  const prevRef = useRef(value)
  const rafRef = useRef<number>(0)

  useEffect(() => {
    const from = prevRef.current
    const to = value
    prevRef.current = value
    if (from === to) return

    const startTime = performance.now()
    const diff = to - from

    const tick = (now: number) => {
      const elapsed = now - startTime
      const progress = Math.min(elapsed / duration, 1)
      const eased = progress === 1 ? 1 : 1 - Math.pow(2, -10 * progress)
      setDisplay(Math.round(from + diff * eased))
      if (progress < 1) {
        rafRef.current = requestAnimationFrame(tick)
      }
    }
    rafRef.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(rafRef.current)
  }, [value, duration])

  return display
}

function RingProgress({ value, max, size = 44, strokeWidth = 3, color = 'var(--nv-action-primary)' }: {
  value: number; max: number; size?: number; strokeWidth?: number; color?: string
}) {
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius
  const ratio = max > 0 ? Math.min(value / max, 1) : 0
  const offset = circumference * (1 - ratio)

  return (
    <svg width={size} height={size} className="-rotate-90" viewBox={`0 0 ${size} ${size}`} aria-hidden={true}>
      <circle
        cx={size / 2}
        cy={size / 2}
        r={radius}
        fill="none"
        stroke="var(--nv-border-subtle)"
        strokeWidth={strokeWidth}
      />
      <circle
        cx={size / 2}
        cy={size / 2}
        r={radius}
        fill="none"
        stroke={color}
        strokeWidth={strokeWidth}
        strokeDasharray={circumference}
        strokeDashoffset={offset}
        strokeLinecap="round"
        className="transition-all duration-700 ease-out"
      />
    </svg>
  )
}

import { preprocessApi } from '@/api/preprocess'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import { useDialog } from '@/components/Dialog'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import {
  Button,
  EmptyState,
  Input,
  PageContainer,
  Select,
  Surface,
  Tag,
  type TagTone,
} from '@/components/design-system'
import type {
  PreprocessTask,
  PreprocessStatistics,
  SystemLoadInfo,
  Library,
  PreprocessStorageUsage,
  CacheUsage,
  PreprocessFilter,
  PreprocessFilterPreview,
  PreprocessCandidate,
} from '@/types'
import api from '@/api/client'
import {
  Play,
  Pause,
  RotateCcw,
  Trash2,
  XCircle,
  Cpu,
  HardDrive,
  Activity,
  RefreshCw,
  CheckCircle2,
  Clock,
  AlertCircle,
  Loader2,
  Zap,
  Film,
  FolderOpen,
  Send,
  CheckSquare,
  Square,
  Database,
  X,
  Eraser,
  Filter,
  Sparkles,
} from 'lucide-react'
import clsx from 'clsx'

const statusColors: Record<string, string> = {
  pending: 'text-[var(--nv-status-warning)]',
  queued: 'text-[var(--nv-status-warning)]',
  running: 'text-[var(--nv-action-primary)]',
  paused: 'text-[var(--nv-status-warning)]',
  completed: 'text-[var(--nv-status-success)]',
  failed: 'text-[var(--nv-status-danger)]',
  cancelled: 'text-[var(--nv-text-tertiary)]',
}

const statusTones: Record<string, TagTone> = {
  pending: 'warning',
  queued: 'warning',
  running: 'brand',
  paused: 'warning',
  completed: 'success',
  failed: 'danger',
  cancelled: 'neutral',
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  let value = bytes
  let i = 0
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024
    i++
  }
  return `${value.toFixed(value >= 100 || i === 0 ? 0 : value >= 10 ? 1 : 2)} ${units[i]}`
}

const statusLabels: Record<string, string> = {
  pending: '等待中',
  queued: '排队中',
  running: '处理中',
  paused: '已暂停',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

const statusIcons: Record<string, React.ReactNode> = {
  pending: <Clock size={14} aria-hidden={true} />,
  queued: <Clock size={14} aria-hidden={true} />,
  running: <Loader2 size={14} className="animate-spin" aria-hidden={true} />,
  paused: <Pause size={14} aria-hidden={true} />,
  completed: <CheckCircle2 size={14} aria-hidden={true} />,
  failed: <AlertCircle size={14} aria-hidden={true} />,
  cancelled: <XCircle size={14} aria-hidden={true} />,
}

export default function PreprocessPage() {
  const toast = useToast()
  const dialog = useDialog()
  const toastRef = useRef(toast)
  toastRef.current = toast
  const { on, off } = useWebSocket()
  const [tasks, setTasks] = useState<PreprocessTask[]>([])
  const [total, setTotal] = useState(0)
  const { page, size: pageSize, setPage, setSize, totalPages: calcTotalPages } = usePagination({ initialSize: 10 })
  const [statusFilter, setStatusFilter] = useState('')
  const [mainTab, setMainTab] = useState<'submit' | 'tasks'>(() => {
    const hash = typeof window !== 'undefined' ? window.location.hash.replace('#', '') : ''
    return hash === 'submit' ? 'submit' : 'tasks'
  })
  const [stats, setStats] = useState<PreprocessStatistics | null>(null)
  const [sysLoad, setSysLoad] = useState<SystemLoadInfo | null>(null)
  const [storage, setStorage] = useState<PreprocessStorageUsage | null>(null)
  const [storageOpen, setStorageOpen] = useState(false)
  const [storageLoading, setStorageLoading] = useState(false)
  const [cleaningOrphan, setCleaningOrphan] = useState(false)
  const [cleaningCategory, setCleaningCategory] = useState<string | null>(null)
  const [cacheUsage, setCacheUsage] = useState<CacheUsage | null>(null)
  const [cacheLoading, setCacheLoading] = useState(false)
  const [expandedCategoryKey, setExpandedCategoryKey] = useState<string>('preprocess')
  const [filterOpen, setFilterOpen] = useState(false)
  const [filter, setFilter] = useState<PreprocessFilter>({
    library_ids: [],
    media_types: [],
    video_codecs: [],
    audio_codecs: [],
    containers: [],
    resolutions: [],
    keyword: '',
    min_size_bytes: 0,
    max_size_bytes: 0,
    min_year: 0,
    max_year: 0,
    min_duration: 0,
    max_duration: 0,
    exclude_already_preprocessed: true,
    exclude_directly_playable: true,
    exclude_strm: true,
  })
  const [filterPreview, setFilterPreview] = useState<PreprocessFilterPreview | null>(null)
  const [previewing, setPreviewing] = useState(false)
  const [submittingFilter, setSubmittingFilter] = useState(false)
  const [filterForce, setFilterForce] = useState(false)
  const [loading, setLoading] = useState(true)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [submitting, setSubmitting] = useState<string | null>(null)
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchLoading, setBatchLoading] = useState(false)
  const [candidates, setCandidates] = useState<PreprocessCandidate[]>([])
  const [candidatesTotal, setCandidatesTotal] = useState(0)
  const [candidatesLoading, setCandidatesLoading] = useState(false)
  const {
    page: candPage,
    size: candSize,
    setPage: setCandPage,
    setSize: setCandSize,
    totalPages: candCalcTotalPages,
  } = usePagination({ initialSize: 12 })
  const [candKeyword, setCandKeyword] = useState('')
  const [candKeywordInput, setCandKeywordInput] = useState('')
  const [candLibraryID, setCandLibraryID] = useState('')
  const [candMediaType, setCandMediaType] = useState('')
  const [candOnlyNeed, setCandOnlyNeed] = useState(true)
  const [candSelected, setCandSelected] = useState<Set<string>>(new Set())
  const [candSubmitting, setCandSubmitting] = useState(false)

  const totalPages = useMemo(() => calcTotalPages(total), [calcTotalPages, total])
  const filterContainerRef = useRef<HTMLDivElement>(null)
  const filterBtnRefs = useRef<Record<string, HTMLButtonElement | null>>({})
  const [filterIndicator, setFilterIndicator] = useState<{ left: number; width: number } | null>(null)

  useLayoutEffect(() => {
    const btn = filterBtnRefs.current[statusFilter]
    const container = filterContainerRef.current
    if (btn && container) {
      const containerRect = container.getBoundingClientRect()
      const btnRect = btn.getBoundingClientRect()
      setFilterIndicator({
        left: btnRect.left - containerRect.left,
        width: btnRect.width,
      })
    }
  }, [statusFilter, stats])

  const animRunning = useAnimatedCounter(stats?.running_count ?? 0)
  const animQueue = useAnimatedCounter(stats?.queue_size ?? 0)
  const isAllSelected = tasks.length > 0 && tasks.every((t) => selectedIds.has(t.id))
  const isSomeSelected = selectedIds.size > 0

  const toggleSelectAll = () => {
    if (isAllSelected) {
      const newSet = new Set(selectedIds)
      tasks.forEach((t) => newSet.delete(t.id))
      setSelectedIds(newSet)
    } else {
      const newSet = new Set(selectedIds)
      tasks.forEach((t) => newSet.add(t.id))
      setSelectedIds(newSet)
    }
  }

  const toggleSelect = (id: string) => {
    const newSet = new Set(selectedIds)
    if (newSet.has(id)) newSet.delete(id)
    else newSet.add(id)
    setSelectedIds(newSet)
  }

  const loadTasks = useCallback(async () => {
    try {
      const res = await preprocessApi.listTasks(page, pageSize, statusFilter)
      setTasks(res.data.data.tasks || [])
      setTotal(res.data.data.total)
    } catch {
      toastRef.current.error('加载预处理任务失败')
    }
  }, [page, pageSize, statusFilter])

  const loadStats = useCallback(async () => {
    try {
      const [statsRes, loadRes] = await Promise.all([
        preprocessApi.getStatistics(),
        preprocessApi.getSystemLoad(),
      ])
      setStats(statsRes.data.data)
      setSysLoad(loadRes.data.data)
    } catch {
      // 忽略
    }
  }, [])

  const loadStorage = useCallback(async (limit = 20) => {
    setStorageLoading(true)
    try {
      const res = await preprocessApi.getStorageUsage(limit)
      setStorage(res.data.data)
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '存储占用统计失败')
    } finally {
      setStorageLoading(false)
    }
  }, [])

  const loadCacheUsage = useCallback(async (force = false) => {
    setCacheLoading(true)
    try {
      const res = await preprocessApi.getCacheUsage(force)
      setCacheUsage(res.data.data)
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '缓存占用统计失败')
    } finally {
      setCacheLoading(false)
    }
  }, [])

  useEffect(() => {
    loadStorage(20)
    loadCacheUsage(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const handleCleanOrphan = useCallback(async () => {
    if (!storage || storage.orphan_count === 0) return
    const ok = await dialog.confirm({
      title: '清理孤儿预处理目录',
      message: `确认清理 ${storage.orphan_count} 个孤儿预处理目录？此操作不可恢复。`,
      confirmText: '清理',
      variant: 'danger',
    })
    if (!ok) return
    setCleaningOrphan(true)
    try {
      const res = await preprocessApi.cleanOrphanCache()
      const { cleaned, freed_bytes } = res.data.data
      toastRef.current.success(`已清理 ${cleaned} 个目录，释放 ${formatBytes(freed_bytes)}`)
      await loadStorage(20)
      await loadCacheUsage(true)
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '清理失败')
    } finally {
      setCleaningOrphan(false)
    }
  }, [storage, loadStorage, loadCacheUsage])

  const handleCleanOne = useCallback(async (mediaId: string, mediaTitle: string) => {
    const ok = await dialog.confirm({
      title: '清理预处理缓存',
      message: `确认清理「${mediaTitle || mediaId}」的预处理缓存？`,
      confirmText: '清理',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await preprocessApi.cleanCache(mediaId)
      toastRef.current.success('已清理')
      await loadStorage(20)
      await loadCacheUsage(true)
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '清理失败')
    }
  }, [loadStorage, loadCacheUsage])

  const handleCleanCategory = useCallback(async (key: string, label: string, size: number) => {
    if (cleaningCategory) return
    const sizeText = formatBytes(size)
    const tip = key === 'preprocess'
      ? `确认清空「${label}」？\n这将删除该目录下全部 ${sizeText} 内容（约等于把所有已预处理过的影视重置为「未处理」），下次播放时系统会按需重新生成。\n正在运行的预处理任务不会被中断。\n如果只想清理「数据库中已无对应任务」的孤儿目录，请展开预处理产物分类后使用「一键清理孤儿」按钮。`
      : `确认清空「${label}」？\n这将删除该目录下全部 ${sizeText} 内容，系统会在需要时自动重新生成。`
    const ok = await dialog.confirm({
      title: `清空缓存：${label}`,
      message: tip,
      confirmText: '清空',
      variant: 'danger',
    })
    if (!ok) return
    setCleaningCategory(key)
    try {
      const res = await preprocessApi.cleanCacheCategory(key, 'all')
      const { freed_bytes, freed_count, skipped, skipped_note } = res.data.data
      if (skipped) {
        toastRef.current.info(`已跳过「${label}」：${skipped_note || '无可清理内容'}`)
      } else if (freed_bytes === 0 && freed_count === 0) {
        toastRef.current.info(skipped_note
          ? `「${label}」无可清理内容（${skipped_note}）`
          : `「${label}」当前无可清理内容`)
      } else {
        const note = skipped_note ? `，${skipped_note}` : ''
        toastRef.current.success(`已清理「${label}」，释放 ${formatBytes(freed_bytes)}（${freed_count} 文件）${note}`)
      }
      await loadStorage(20)
      await loadCacheUsage(true)
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '清理失败')
    } finally {
      setCleaningCategory(null)
    }
  }, [cleaningCategory, loadStorage, loadCacheUsage])

  const handleCleanAllCache = useCallback(async () => {
    if (cleaningCategory || !cacheUsage) return
    const ok = await dialog.confirm({
      title: '一键清空所有缓存',
      message: '确认一键清空所有可清理的缓存分类？\n这会清空：在线转码缓存、自适应码率缓存、缩略图/雪碧图、WebDAV 临时下载，并清理预处理的孤儿目录。\n海报/字幕/离线下载等不会被删除。此操作不可恢复。',
      confirmText: '全部清空',
      variant: 'danger',
    })
    if (!ok) return
    setCleaningCategory('__all__')
    try {
      const res = await preprocessApi.cleanAllCache()
      const { total_freed, total_count, category_num } = res.data.data
      toastRef.current.success(`已清理 ${category_num} 个分类，释放 ${formatBytes(total_freed)}（${total_count} 文件）`)
      await loadStorage(20)
      await loadCacheUsage(true)
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '一键清理失败')
    } finally {
      setCleaningCategory(null)
    }
  }, [cleaningCategory, cacheUsage, loadStorage, loadCacheUsage])

  const handlePreviewFilter = useCallback(async () => {
    setPreviewing(true)
    try {
      const res = await preprocessApi.previewByFilter(filter)
      setFilterPreview(res.data.data)
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '预览失败')
    } finally {
      setPreviewing(false)
    }
  }, [filter])

  const handleSubmitFilter = useCallback(async () => {
    if (!filterPreview || filterPreview.matched_count === 0) {
      toastRef.current.error('请先预览，确认有命中再提交')
      return
    }
    const ok = await dialog.confirm({
      title: '提交预处理任务',
      message: `确认按当前筛选条件提交 ${filterPreview.matched_count} 个预处理任务？`,
      confirmText: '提交',
      variant: 'primary',
    })
    if (!ok) return
    setSubmittingFilter(true)
    try {
      const res = await preprocessApi.submitByFilter(filter, 0, filterForce)
      const { submitted, skipped } = res.data.data
      toastRef.current.success(`已提交 ${submitted} 个任务${skipped ? `，跳过 ${skipped} 个` : ''}`)
      setFilterOpen(false)
      setFilterPreview(null)
      loadTasks()
      loadStats()
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '提交失败')
    } finally {
      setSubmittingFilter(false)
    }
  }, [filter, filterPreview, filterForce, loadTasks, loadStats])

  const toggleArrayFilter = useCallback(<K extends keyof PreprocessFilter>(key: K, value: string) => {
    setFilter((prev) => {
      const arr = (prev[key] as string[] | undefined) ?? []
      const next = arr.includes(value) ? arr.filter((v) => v !== value) : [...arr, value]
      return { ...prev, [key]: next } as PreprocessFilter
    })
    setFilterPreview(null)
  }, [])

  const loadLibraries = useCallback(async () => {
    try {
      const res = await api.get<{ data: Library[] }>('/libraries')
      setLibraries(res.data.data || [])
    } catch {
      // 忽略
    }
  }, [])

  const loadCandidates = useCallback(async () => {
    setCandidatesLoading(true)
    try {
      const res = await preprocessApi.listCandidates({
        page: candPage,
        size: candSize,
        keyword: candKeyword,
        library_id: candLibraryID,
        media_type: candMediaType,
        only_need_preprocess: candOnlyNeed,
        sort_by: 'updated_at',
        sort_order: 'desc',
      })
      const list = res.data.data
      setCandidates(list?.items || [])
      setCandidatesTotal(list?.total || 0)
    } catch (e) {
      const msg = e instanceof Error ? e.message : '加载候选影视失败'
      toastRef.current.error(msg)
    } finally {
      setCandidatesLoading(false)
    }
  }, [candPage, candSize, candKeyword, candLibraryID, candMediaType, candOnlyNeed])

  useEffect(() => {
    setLoading(true)
    const promises: Promise<void>[] = [loadTasks(), loadStats(), loadLibraries()]
    Promise.all(promises).finally(() => setLoading(false))
  }, [loadTasks, loadStats, loadLibraries])

  useEffect(() => {
    loadCandidates()
  }, [loadCandidates])

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

    let storageTimer: ReturnType<typeof setTimeout> | null = null
    const scheduleStorageRefresh = () => {
      if (storageTimer) return
      storageTimer = setTimeout(() => {
        storageTimer = null
        loadStorage(20)
        loadCacheUsage(false)
      }, 1500)
    }
    const onTaskFinished = () => {
      scheduleRefresh()
      scheduleStorageRefresh()
    }

    on(WS_EVENTS.PREPROCESS_PROGRESS, scheduleRefresh)
    on(WS_EVENTS.PREPROCESS_COMPLETED, onTaskFinished)
    on(WS_EVENTS.PREPROCESS_FAILED, onTaskFinished)
    on(WS_EVENTS.PREPROCESS_STARTED, scheduleRefresh)
    return () => {
      off(WS_EVENTS.PREPROCESS_PROGRESS, scheduleRefresh)
      off(WS_EVENTS.PREPROCESS_COMPLETED, onTaskFinished)
      off(WS_EVENTS.PREPROCESS_FAILED, onTaskFinished)
      off(WS_EVENTS.PREPROCESS_STARTED, scheduleRefresh)
      if (refreshTimer) clearTimeout(refreshTimer)
      if (storageTimer) clearTimeout(storageTimer)
    }
  }, [on, off, loadTasks, loadStats, loadStorage, loadCacheUsage])

  const handlePause = async (id: string) => {
    try {
      await preprocessApi.pauseTask(id)
      toastRef.current.success('任务已暂停')
      loadTasks()
    } catch { toastRef.current.error('暂停失败') }
  }

  const handleResume = async (id: string) => {
    try {
      await preprocessApi.resumeTask(id)
      toastRef.current.success('任务已恢复')
      loadTasks()
    } catch { toastRef.current.error('恢复失败') }
  }

  const handleCancel = async (id: string) => {
    try {
      await preprocessApi.cancelTask(id)
      toastRef.current.success('任务已取消')
      loadTasks()
    } catch { toastRef.current.error('取消失败') }
  }

  const handleRetry = async (id: string) => {
    try {
      await preprocessApi.retryTask(id)
      toastRef.current.success('任务已重新提交')
      loadTasks()
    } catch { toastRef.current.error('重试失败') }
  }

  const handleDelete = async (id: string) => {
    try {
      await preprocessApi.deleteTask(id)
      toastRef.current.success('任务已删除')
      loadTasks()
    } catch { toastRef.current.error('删除失败') }
  }

  const handleBatchDelete = async () => {
    if (selectedIds.size === 0) return
    setBatchLoading(true)
    try {
      const res = await preprocessApi.batchDeleteTasks(Array.from(selectedIds))
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
      const res = await preprocessApi.batchCancelTasks(Array.from(selectedIds))
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
      const res = await preprocessApi.batchRetryTasks(Array.from(selectedIds))
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
      const res = await preprocessApi.submitLibrary(libraryId)
      const count = res.data.data.submitted
      if (count > 0) {
        toastRef.current.success(`已提交 ${count} 个预处理任务`)
        loadTasks()
        loadStats()
      } else {
        toastRef.current.info('该媒体库没有需要预处理的视频')
      }
    } catch {
      toastRef.current.error('提交失败')
    } finally {
      setSubmitting(null)
    }
  }

  const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
  }

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
            <div className="skeleton h-7 w-40 rounded-lg" />
            <div className="skeleton h-4 w-64 rounded" />
          </div>
          <div className="skeleton h-9 w-24 rounded-lg" />
        </div>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
          {Array.from({ length: 5 }).map((_, i) => (
            <Surface key={i} className="p-4">
              <div className="mb-2 flex items-center gap-2">
                <div className="skeleton h-3.5 w-3.5 rounded" />
                <div className="skeleton h-3 w-14 rounded" />
              </div>
              <div className="skeleton h-7 w-12 rounded-lg" />
              <div className="skeleton mt-1.5 h-3 w-24 rounded" />
            </Surface>
          ))}
        </div>
        <div className="space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <Surface key={i} className="flex items-center gap-4 p-4">
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
    <motion.div variants={pageVariants} initial="initial" animate="enter">
      <PageContainer width="wide" className="space-y-6 py-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="flex items-center gap-2 font-display text-2xl font-bold tracking-tight text-[var(--nv-text-primary)]">
              <Zap className="text-[var(--nv-action-primary)]" size={24} aria-hidden={true} />
              视频预处理
            </h1>
            <p className="mt-1 text-sm text-[var(--nv-text-tertiary)]">
              自动转码生成多码率 HLS 流，实现秒开播放
            </p>
          </div>
          <Button type="button" variant="secondary" size="sm" onClick={() => { loadTasks(); loadStats(); loadStorage(20) }}>
            <RefreshCw size={14} aria-hidden={true} />
            刷新
          </Button>
        </div>

        {stats && sysLoad && (
          <motion.div
            className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5"
            variants={staggerContainerVariants}
            initial="hidden"
            animate="visible"
          >
            <motion.div variants={staggerItemVariants}>
              <Surface className="h-full p-4 transition-shadow hover:shadow-[var(--nv-shadow-card-hover)]">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
                      <Activity size={14} className="text-[var(--nv-action-primary)]" aria-hidden={true} />
                      处理中
                    </div>
                    <div className="text-2xl font-bold text-[var(--nv-text-primary)]">{animRunning}</div>
                    <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">
                      {stats.active_workers}/{sysLoad.cur_workers || stats.max_workers} 工作线程
                    </div>
                  </div>
                  <div className="relative shrink-0">
                    <RingProgress value={stats.active_workers} max={sysLoad.cur_workers || stats.max_workers} />
                    <span className="absolute inset-0 flex items-center justify-center text-[10px] font-bold text-[var(--nv-text-primary)]">
                      {stats.active_workers}
                    </span>
                  </div>
                </div>
              </Surface>
            </motion.div>

            <motion.div variants={staggerItemVariants}>
              <Surface className="h-full p-4 transition-shadow hover:shadow-[var(--nv-shadow-card-hover)]">
                <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
                  <Clock size={14} className="text-[var(--nv-status-warning)]" aria-hidden={true} />
                  队列
                </div>
                <div className="text-2xl font-bold text-[var(--nv-text-primary)]">{animQueue}</div>
                <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">等待处理</div>
              </Surface>
            </motion.div>

            <motion.div variants={staggerItemVariants}>
              <Surface className="h-full p-4 transition-shadow hover:shadow-[var(--nv-shadow-card-hover)]">
                <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
                  <Cpu size={14} className="text-[var(--nv-status-success)]" aria-hidden={true} />
                  系统负载
                </div>
                <div className="text-2xl font-bold text-[var(--nv-text-primary)]">
                  {sysLoad.cpu_percent != null ? `${sysLoad.cpu_percent.toFixed(0)}%` : `${sysLoad.mem_alloc_mb.toFixed(0)} MB`}
                </div>
                <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">
                  {sysLoad.cpu_count} CPU · {sysLoad.max_workers} worker
                </div>
                {sysLoad.cpu_percent != null && (
                  <div className="mt-2 h-1 w-full overflow-hidden rounded-full bg-[var(--nv-bg-active)]">
                    <div
                      className="h-full rounded-full transition-all duration-500"
                      style={{
                        width: `${Math.min(100, sysLoad.cpu_percent)}%`,
                        background: sysLoad.cpu_percent > 80
                          ? 'var(--nv-status-danger)'
                          : sysLoad.cpu_percent > 60
                            ? 'var(--nv-status-warning)'
                            : 'var(--nv-status-success)',
                      }}
                    />
                  </div>
                )}
              </Surface>
            </motion.div>

            <motion.div variants={staggerItemVariants}>
              <Surface className="h-full p-4 transition-shadow hover:shadow-[var(--nv-shadow-card-hover)]">
                <div className="mb-2 flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
                  <HardDrive size={14} className="text-[var(--nv-action-primary)]" aria-hidden={true} />
                  硬件加速
                </div>
                <div className="text-lg font-bold text-[var(--nv-text-primary)]">{stats.hw_accel || 'CPU'}</div>
                <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">
                  {sysLoad.gpu_status?.degraded ? (
                    <span className="text-[var(--nv-status-danger)]">⚠ GPU 过载 · 已降级为 CPU</span>
                  ) : (
                    <>已完成 {stats.status_counts?.completed || 0} 个</>
                  )}
                </div>
                {sysLoad.gpu_status?.metrics?.available && (
                  <div className="mt-2 h-1 w-full overflow-hidden rounded-full bg-[var(--nv-bg-active)]">
                    <div
                      className="h-full rounded-full transition-all duration-500"
                      style={{
                        width: `${Math.min(100, sysLoad.gpu_status.metrics.utilization)}%`,
                        background: sysLoad.gpu_status.degraded
                          ? 'var(--nv-status-danger)'
                          : sysLoad.gpu_status.metrics.utilization > 80
                            ? 'var(--nv-status-warning)'
                            : 'var(--nv-action-primary)',
                      }}
                    />
                  </div>
                )}
              </Surface>
            </motion.div>

            <motion.button
              type="button"
              variants={staggerItemVariants}
              onClick={() => { setStorageOpen(true); loadStorage(0); loadCacheUsage(false) }}
              className="nv-surface h-full p-4 text-left transition-[border-color,box-shadow,transform] hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:shadow-[var(--nv-shadow-card-hover)] focus:outline-none focus-visible:shadow-[var(--nv-shadow-focus)]"
              title="点击查看缓存占用明细（preprocess + transcode + thumbnails 等）"
            >
              <div className="mb-2 flex items-center justify-between">
                <div className="flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
                  <Database size={14} className="text-[var(--nv-status-warning)]" aria-hidden={true} />
                  缓存占用
                </div>
                {(storageLoading || cacheLoading) && <Loader2 size={12} className="animate-spin text-[var(--nv-text-tertiary)]" aria-hidden={true} />}
              </div>
              <div className="text-2xl font-bold text-[var(--nv-text-primary)]">
                {cacheUsage ? formatBytes(cacheUsage.total_size) : storage ? formatBytes(storage.total_size) : '—'}
              </div>
              <div className="mt-1 text-xs text-[var(--nv-text-tertiary)]">
                {cacheUsage ? (
                  <>
                    {cacheUsage.total_count.toLocaleString()} 文件 · {cacheUsage.categories.length} 个分类
                    {storage && storage.orphan_count > 0 && (
                      <span className="ml-1 text-[var(--nv-status-danger)]">· 孤儿 {storage.orphan_count}</span>
                    )}
                  </>
                ) : storage ? (
                  <>
                    {storage.total_count} 个目录
                    {storage.orphan_count > 0 && (
                      <span className="ml-1 text-[var(--nv-status-danger)]">· 孤儿 {storage.orphan_count}</span>
                    )}
                  </>
                ) : '点击加载'}
              </div>
              {cacheUsage && cacheUsage.total_size > 0 && (
                <div className="mt-2 h-1 w-full overflow-hidden rounded-full bg-[var(--nv-bg-active)]">
                  <div
                    className="h-full rounded-full transition-all duration-500"
                    style={{
                      width: storage && storage.orphan_size > 0
                        ? `${Math.min(100, (storage.orphan_size / cacheUsage.total_size) * 100)}%`
                        : '100%',
                      background: storage && storage.orphan_size > 0
                        ? 'var(--nv-status-danger)'
                        : 'var(--nv-status-success)',
                    }}
                  />
                </div>
              )}
            </motion.button>
          </motion.div>
        )}

        {libraries.length > 0 && (
          <Surface className="p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
              <h2 className="flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
                <FolderOpen size={16} className="text-[var(--nv-action-primary)]" aria-hidden={true} />
                媒体库批量预处理
              </h2>
              <Button type="button" variant="secondary" size="sm" onClick={() => { setFilterOpen(true); setFilterPreview(null) }} title="按条件自定义选择哪些影视进行预处理">
                <Filter size={12} aria-hidden={true} />
                自定义筛选
              </Button>
            </div>
            <div className="flex flex-wrap gap-2">
              {libraries.map((lib) => (
                <Button
                  key={lib.id}
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => handleSubmitLibrary(lib.id)}
                  loading={submitting === lib.id}
                >
                  {submitting !== lib.id && <Send size={12} aria-hidden={true} />}
                  {lib.name}
                  <span className="text-[var(--nv-text-tertiary)]">({lib.type})</span>
                </Button>
              ))}
            </div>
          </Surface>
        )}

        <div className="flex flex-wrap items-center gap-2">
          {([
            { key: 'submit', label: '选源提交', count: candidatesTotal },
            { key: 'tasks', label: '处理进度', count: total },
          ] as const).map((tab) => {
            const active = mainTab === tab.key
            return (
              <Button
                key={tab.key}
                type="button"
                variant={active ? 'primary' : 'secondary'}
                size="sm"
                aria-pressed={active}
                onClick={() => {
                  setMainTab(tab.key)
                  if (typeof window !== 'undefined') window.location.hash = tab.key
                }}
              >
                {tab.label}
                {tab.count > 0 && <Tag tone={active ? 'neutral' : 'brand'}>{tab.count.toLocaleString()}</Tag>}
              </Button>
            )
          })}
        </div>

        {mainTab === 'submit' && (
          <Surface className="space-y-3 p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <h2 className="flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
                <Film size={16} className="text-[var(--nv-action-primary)]" aria-hidden={true} />
                影视文件列表
                <span className="text-xs font-normal text-[var(--nv-text-tertiary)]">（共 {candidatesTotal.toLocaleString()} 条）</span>
              </h2>
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-xs text-[var(--nv-text-tertiary)]">
                  已选 <strong className="text-[var(--nv-action-primary)]">{candSelected.size}</strong> 项
                </span>
                <Button type="button" variant="ghost" size="sm" onClick={() => setCandSelected(new Set())} disabled={candSelected.size === 0}>
                  清空
                </Button>
                <Button
                  type="button"
                  variant="primary"
                  size="sm"
                  disabled={candSelected.size === 0 || candSubmitting}
                  loading={candSubmitting}
                  onClick={async () => {
                    if (candSelected.size === 0) return
                    setCandSubmitting(true)
                    try {
                      const ids = Array.from(candSelected)
                      const res = await preprocessApi.batchSubmit(ids, 0, false)
                      const submitted = res.data.data?.submitted || 0
                      toast.success(`已提交 ${submitted} / ${ids.length} 个预处理任务`)
                      setCandSelected(new Set())
                      loadCandidates()
                      loadTasks()
                      loadStats()
                    } catch (e) {
                      const msg = e instanceof Error ? e.message : '批量提交失败'
                      toast.error(msg)
                    } finally {
                      setCandSubmitting(false)
                    }
                  }}
                >
                  {!candSubmitting && <Send size={14} aria-hidden={true} />}
                  提交预处理（{candSelected.size}）
                </Button>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <Input
                value={candKeywordInput}
                onChange={(e) => setCandKeywordInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    setCandPage(1)
                    setCandKeyword(candKeywordInput.trim())
                  }
                }}
                placeholder="搜索标题 / 原名 / 番号，回车确认"
                className="min-w-[220px] flex-1"
              />
              <Select value={candLibraryID} onChange={(e) => { setCandPage(1); setCandLibraryID(e.target.value) }} className="min-w-[140px]">
                <option value="">全部媒体库</option>
                {libraries.map((lib) => <option key={lib.id} value={lib.id}>{lib.name}</option>)}
              </Select>
              <Select value={candMediaType} onChange={(e) => { setCandPage(1); setCandMediaType(e.target.value) }} className="min-w-[120px]">
                <option value="">全部类型</option>
                <option value="movie">电影</option>
                <option value="episode">剧集</option>
              </Select>
              <label className="flex h-10 cursor-pointer items-center gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] px-3 text-xs text-[var(--nv-text-secondary)]">
                <input
                  type="checkbox"
                  checked={candOnlyNeed}
                  onChange={(e) => { setCandPage(1); setCandOnlyNeed(e.target.checked) }}
                  className="accent-[var(--nv-action-primary)]"
                />
                仅显示需要预处理
              </label>
              <Button type="button" variant="secondary" size="sm" onClick={() => loadCandidates()} loading={candidatesLoading}>
                {!candidatesLoading && <RefreshCw size={12} aria-hidden={true} />}
                刷新
              </Button>
            </div>

            <div className="overflow-hidden rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)]">
              <div className="flex items-center gap-2 border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3 py-2 text-xs text-[var(--nv-text-tertiary)]">
                <button
                  type="button"
                  onClick={() => {
                    const eligible = candidates.filter((c) => !c.is_strm && c.preprocess_status !== 'completed' && c.preprocess_status !== 'running' && c.preprocess_status !== 'queued' && c.preprocess_status !== 'pending')
                    const allSelected = eligible.length > 0 && eligible.every((c) => candSelected.has(c.media_id))
                    const next = new Set(candSelected)
                    if (allSelected) eligible.forEach((c) => next.delete(c.media_id))
                    else eligible.forEach((c) => next.add(c.media_id))
                    setCandSelected(next)
                  }}
                  className="flex items-center gap-1 transition-colors hover:text-[var(--nv-action-primary)]"
                  title="全选 / 取消全选当前页可选项"
                >
                  {(() => {
                    const eligible = candidates.filter((c) => !c.is_strm && c.preprocess_status !== 'completed' && c.preprocess_status !== 'running' && c.preprocess_status !== 'queued' && c.preprocess_status !== 'pending')
                    const allSelected = eligible.length > 0 && eligible.every((c) => candSelected.has(c.media_id))
                    return allSelected ? <CheckSquare size={14} className="text-[var(--nv-action-primary)]" aria-hidden={true} /> : <Square size={14} aria-hidden={true} />
                  })()}
                  当前页全选
                </button>
                <span className="ml-auto">第 {candPage} / {candCalcTotalPages(candidatesTotal) || 1} 页</span>
              </div>

              {candidatesLoading && candidates.length === 0 ? (
                <div className="p-6 text-center text-xs text-[var(--nv-text-tertiary)]">
                  <Loader2 size={16} className="mr-2 inline animate-spin" aria-hidden={true} />加载中...
                </div>
              ) : candidates.length === 0 ? (
                <EmptyState icon={<Film size={24} />} title="暂无匹配的影视" className="min-h-48" />
              ) : (
                <ul className="divide-y divide-[var(--nv-border-subtle)]">
                  {candidates.map((candidate) => {
                    const disabled = candidate.is_strm || candidate.preprocess_status === 'completed' || candidate.preprocess_status === 'running' || candidate.preprocess_status === 'queued' || candidate.preprocess_status === 'pending'
                    const checked = candSelected.has(candidate.media_id)
                    return (
                      <li
                        key={candidate.media_id}
                        className={clsx(
                          'flex items-center gap-3 px-3 py-2 transition-colors',
                          !disabled && 'cursor-pointer hover:bg-[var(--nv-bg-hover)]',
                          checked && 'bg-[var(--nv-bg-active)]',
                        )}
                        title={candidate.file_path}
                        onClick={() => {
                          if (disabled) return
                          const next = new Set(candSelected)
                          if (next.has(candidate.media_id)) next.delete(candidate.media_id)
                          else next.add(candidate.media_id)
                          setCandSelected(next)
                        }}
                      >
                        <div className="shrink-0 text-[var(--nv-text-tertiary)]">
                          {disabled ? (
                            <Square size={14} className="opacity-30" aria-hidden={true} />
                          ) : checked ? (
                            <CheckSquare size={14} className="text-[var(--nv-action-primary)]" aria-hidden={true} />
                          ) : (
                            <Square size={14} aria-hidden={true} />
                          )}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            {(() => {
                              const scrapeStatus = (candidate.scrape_status || '').toLowerCase()
                              const unscraped = scrapeStatus === '' || scrapeStatus === 'pending' || scrapeStatus === 'failed'
                              if (unscraped) {
                                const filePath = candidate.file_path || ''
                                const idx = Math.max(filePath.lastIndexOf('/'), filePath.lastIndexOf('\\'))
                                const base = idx >= 0 ? filePath.slice(idx + 1) : filePath
                                const dot = base.lastIndexOf('.')
                                const fileName = dot > 0 ? base.slice(0, dot) : base
                                return (
                                  <>
                                    <Tag tone="warning" title="该影视文件未完成刮削，下方展示的是源文件名">未刮削</Tag>
                                    <span className="truncate text-sm font-medium text-[var(--nv-text-secondary)]" title={fileName || candidate.file_path}>
                                      {fileName || candidate.title || '(无文件名)'}
                                    </span>
                                  </>
                                )
                              }
                              return (
                                <>
                                  <span className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{candidate.title || candidate.orig_title || '(无标题)'}</span>
                                  {candidate.media_type === 'episode' && (candidate.season_num || candidate.episode_num) ? (
                                    <Tag tone="neutral" className="font-mono">{`S${String(candidate.season_num ?? 0).padStart(2, '0')}E${String(candidate.episode_num ?? 0).padStart(2, '0')}`}</Tag>
                                  ) : null}
                                  {candidate.media_type === 'episode' && candidate.episode_title ? (
                                    <span className="truncate text-xs text-[var(--nv-text-tertiary)]">· {candidate.episode_title}</span>
                                  ) : null}
                                  {candidate.year > 0 && <span className="text-xs text-[var(--nv-text-tertiary)]">({candidate.year})</span>}
                                  {candidate.media_type !== 'episode' && candidate.orig_title && candidate.orig_title !== candidate.title && (
                                    <span className="truncate text-xs text-[var(--nv-text-tertiary)]" title={candidate.orig_title}>· {candidate.orig_title}</span>
                                  )}
                                </>
                              )
                            })()}
                            <Tag tone="neutral">{candidate.media_type === 'episode' ? '剧集' : '电影'}</Tag>
                            {candidate.is_strm && <Tag tone="warning">STRM</Tag>}
                            {candidate.can_play_directly && !candidate.is_strm && <Tag tone="success">可直接播放</Tag>}
                            {candidate.preprocess_status !== 'none' && (
                              <Tag tone={statusTones[candidate.preprocess_status] || 'neutral'}>{statusLabels[candidate.preprocess_status] || candidate.preprocess_status}</Tag>
                            )}
                          </div>
                          <div className="mt-0.5 flex flex-wrap items-center gap-3 text-xs text-[var(--nv-text-tertiary)]">
                            {candidate.resolution && <span>{candidate.resolution}</span>}
                            {candidate.video_codec && <span>{candidate.video_codec}</span>}
                            {candidate.audio_codec && <span>{candidate.audio_codec}</span>}
                            {candidate.duration > 0 && <span>{Math.round(candidate.duration / 60)} 分钟</span>}
                            {candidate.file_size > 0 && <span>{formatBytes(candidate.file_size)}</span>}
                          </div>
                        </div>
                      </li>
                    )
                  })}
                </ul>
              )}
            </div>

            {candidatesTotal > candSize && (
              <Pagination
                page={candPage}
                totalPages={candCalcTotalPages(candidatesTotal)}
                total={candidatesTotal}
                pageSize={candSize}
                pageSizeOptions={[12, 20, 50, 100]}
                onPageChange={setCandPage}
                onPageSizeChange={(s) => { setCandPage(1); setCandSize(s) }}
              />
            )}
          </Surface>
        )}

        {mainTab === 'tasks' && (
          <>
            <div ref={filterContainerRef} className="relative flex flex-wrap items-center gap-2 pb-1">
              {filterIndicator && (
                <motion.div
                  className="absolute bottom-0 z-10 h-[2px] rounded-full bg-[var(--nv-action-primary)]"
                  animate={{ left: filterIndicator.left, width: filterIndicator.width }}
                  transition={{ type: 'spring', stiffness: 400, damping: 30 }}
                />
              )}
              {['', 'running', 'pending', 'paused', 'completed', 'failed', 'cancelled'].map((status) => (
                <Button
                  key={status}
                  ref={(el) => { filterBtnRefs.current[status] = el }}
                  type="button"
                  variant={statusFilter === status ? 'primary' : 'secondary'}
                  size="sm"
                  aria-pressed={statusFilter === status}
                  onClick={() => { setStatusFilter(status); setPage(1); setSelectedIds(new Set()) }}
                >
                  {status === '' ? '全部' : statusLabels[status] || status}
                  {status && stats?.status_counts?.[status] ? <Tag tone="neutral">{stats.status_counts[status]}</Tag> : null}
                </Button>
              ))}
            </div>

            <AnimatePresence>
              {isSomeSelected && (
                <motion.div
                  initial={{ opacity: 0, y: -12, filter: 'blur(4px)' }}
                  animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
                  exit={{ opacity: 0, y: -8, filter: 'blur(2px)' }}
                  transition={{ duration: 0.25, ease: easeSmooth as unknown as [number, number, number, number] }}
                >
                  <Surface className="flex flex-wrap items-center gap-3 border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] px-4 py-3">
                    <Button type="button" variant="ghost" size="sm" onClick={toggleSelectAll}>
                      {isAllSelected ? <CheckSquare size={14} className="text-[var(--nv-action-primary)]" aria-hidden={true} /> : <Square size={14} aria-hidden={true} />}
                      {isAllSelected ? '取消全选' : '全选当前页'}
                    </Button>
                    <span className="text-xs text-[var(--nv-text-tertiary)]">已选择 <strong className="text-[var(--nv-action-primary)]">{selectedIds.size}</strong> 项</span>
                    <div className="flex-1" />
                    <Button type="button" variant="secondary" size="sm" onClick={handleBatchCancel} disabled={batchLoading}><XCircle size={12} aria-hidden={true} />批量取消</Button>
                    <Button type="button" variant="secondary" size="sm" onClick={handleBatchRetry} disabled={batchLoading}><RotateCcw size={12} aria-hidden={true} />批量重试</Button>
                    <Button type="button" variant="danger" size="sm" onClick={handleBatchDelete} loading={batchLoading}>{!batchLoading && <Trash2 size={12} aria-hidden={true} />}批量删除</Button>
                    <Button type="button" variant="ghost" size="sm" onClick={() => setSelectedIds(new Set())}>清除选择</Button>
                  </Surface>
                </motion.div>
              )}
            </AnimatePresence>

            <motion.div className="space-y-3" variants={staggerContainerVariants} initial="hidden" animate="visible" key={statusFilter + '-' + page}>
              {tasks.length > 0 && (
                <div className="flex items-center gap-3 px-1 py-1">
                  <Button type="button" variant="ghost" size="sm" onClick={toggleSelectAll}>
                    {isAllSelected ? <CheckSquare size={16} className="text-[var(--nv-action-primary)]" aria-hidden={true} /> : <Square size={16} aria-hidden={true} />}
                    {isAllSelected ? '取消全选' : '全选'}
                  </Button>
                  <span className="text-xs text-[var(--nv-text-tertiary)]">共 {total} 条，当前第 {page}/{totalPages} 页</span>
                </div>
              )}

              {tasks.length === 0 ? (
                <EmptyState icon={<Film size={30} />} title="暂无预处理任务" description="扫描媒体库后将自动提交预处理任务" />
              ) : (
                tasks.map((task) => (
                  <motion.div key={task.id} variants={staggerItemVariants} layout>
                    <Surface className={clsx('p-4 transition-[border-color,background-color,box-shadow]', selectedIds.has(task.id) && 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] shadow-[var(--nv-shadow-focus)]')}>
                      <div className="flex items-start justify-between gap-4">
                        <button
                          type="button"
                          onClick={() => toggleSelect(task.id)}
                          className={clsx('mt-0.5 shrink-0 transition-colors', selectedIds.has(task.id) ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-tertiary)]')}
                          aria-label={selectedIds.has(task.id) ? '取消选择' : '选择任务'}
                        >
                          {selectedIds.has(task.id) ? <CheckSquare size={16} aria-hidden={true} /> : <Square size={16} aria-hidden={true} />}
                        </button>

                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className={statusColors[task.status]}>{statusIcons[task.status]}</span>
                            <h3 className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{task.media_title || task.media_id}</h3>
                            <Tag tone={statusTones[task.status] || 'neutral'}>{statusLabels[task.status] || task.status}</Tag>
                          </div>

                          {(task.status === 'running' || task.status === 'paused') && (
                            <div className="mt-2">
                              <div className="mb-1 flex items-center justify-between text-xs text-[var(--nv-text-tertiary)]">
                                <span>{task.phase || task.message}</span>
                                <span>{task.progress.toFixed(1)}%</span>
                              </div>
                              <div className="h-1.5 w-full overflow-hidden rounded-full bg-[var(--nv-bg-active)]">
                                <div
                                  className="h-full rounded-full transition-all duration-500"
                                  style={{
                                    width: `${task.progress}%`,
                                    background: task.status === 'paused' ? 'var(--nv-status-warning)' : 'var(--nv-action-primary)',
                                  }}
                                />
                              </div>
                            </div>
                          )}

                          <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[var(--nv-text-tertiary)]">
                            {task.source_width > 0 && <span>{task.source_width}×{task.source_height} · {task.source_codec}</span>}
                            {task.source_size > 0 && <span>{formatSize(task.source_size)}</span>}
                            {task.source_duration > 0 && <span>{formatDuration(task.source_duration)}</span>}
                            {task.speed_ratio > 0 && task.status === 'running' && <span className="text-[var(--nv-action-primary)]">{task.speed_ratio.toFixed(1)}x 速度</span>}
                            {task.elapsed_sec > 0 && <span>耗时 {formatDuration(task.elapsed_sec)}</span>}
                            {task.error && <span className="text-[var(--nv-status-danger)]">{task.error}</span>}
                          </div>
                        </div>

                        <div className="flex shrink-0 items-center gap-1">
                          {task.status === 'running' && (
                            <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => handlePause(task.id)} aria-label="暂停" title="暂停"><Pause size={14} aria-hidden={true} /></Button>
                          )}
                          {task.status === 'paused' && (
                            <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => handleResume(task.id)} aria-label="恢复" title="恢复"><Play size={14} aria-hidden={true} /></Button>
                          )}
                          {(task.status === 'running' || task.status === 'paused' || task.status === 'pending' || task.status === 'queued') && (
                            <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => handleCancel(task.id)} aria-label="取消" title="取消"><XCircle size={14} aria-hidden={true} /></Button>
                          )}
                          {task.status === 'failed' && (
                            <Button type="button" variant="secondary" size="sm" iconOnly onClick={() => handleRetry(task.id)} aria-label="重试" title="重试"><RotateCcw size={14} aria-hidden={true} /></Button>
                          )}
                          {(task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled') && (
                            <Button type="button" variant="danger" size="sm" iconOnly onClick={() => handleDelete(task.id)} aria-label="删除" title="删除"><Trash2 size={14} aria-hidden={true} /></Button>
                          )}
                        </div>
                      </div>
                    </Surface>
                  </motion.div>
                ))
              )}
            </motion.div>

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
          </>
        )}

        <AnimatePresence>
          {storageOpen && (
            <motion.div
              className="fixed inset-0 z-50 flex items-center justify-center p-4"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: durations.fast, ease: easeSmooth }}
              onClick={() => setStorageOpen(false)}
            >
              <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />
              <motion.div
                className="nv-surface relative flex max-h-[85vh] w-full max-w-3xl flex-col overflow-hidden shadow-[var(--nv-shadow-elevated)]"
                initial={{ opacity: 0, scale: 0.96, y: 12 }}
                animate={{ opacity: 1, scale: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.96, y: 12 }}
                transition={{ duration: durations.normal, ease: easeSmooth }}
                onClick={(e) => e.stopPropagation()}
              >
                <div className="flex flex-wrap items-start justify-between gap-4 border-b border-[var(--nv-border-subtle)] px-5 py-4">
                  <div className="flex items-start gap-2">
                    <Database size={18} className="mt-0.5 text-[var(--nv-status-warning)]" aria-hidden={true} />
                    <div>
                      <div className="text-base font-semibold text-[var(--nv-text-primary)]">缓存占用总览</div>
                      {cacheUsage ? (
                        <div className="mt-0.5 text-xs text-[var(--nv-text-tertiary)]">
                          根目录: <span className="font-mono">{cacheUsage.root_dir}</span> · 扫描耗时 {cacheUsage.scan_duration_ms} ms
                          {cacheUsage.from_cache && <Tag tone="warning" className="ml-2">缓存</Tag>}
                        </div>
                      ) : (
                        <div className="mt-0.5 text-xs text-[var(--nv-text-tertiary)]">
                          {(storageLoading || cacheLoading) ? '正在扫描 cache 根目录...' : '点击“重新扫描”获取最新数据'}
                        </div>
                      )}
                    </div>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button type="button" variant="danger" size="sm" onClick={handleCleanAllCache} disabled={!cacheUsage || cacheUsage.total_size === 0 || !!cleaningCategory} loading={cleaningCategory === '__all__'}>
                      {cleaningCategory !== '__all__' && <Eraser size={12} aria-hidden={true} />}一键清空
                    </Button>
                    <Button type="button" variant="secondary" size="sm" onClick={() => { loadStorage(0); loadCacheUsage(true) }} loading={storageLoading || cacheLoading}>
                      {!(storageLoading || cacheLoading) && <RefreshCw size={12} aria-hidden={true} />}重新扫描
                    </Button>
                    <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => setStorageOpen(false)} aria-label="关闭"><X size={14} aria-hidden={true} /></Button>
                  </div>
                </div>

                {cacheUsage && cacheUsage.categories.length > 0 && (
                  <div className="border-b border-[var(--nv-border-subtle)] px-5 py-3">
                    <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                      <div className="text-xs font-medium text-[var(--nv-text-secondary)]">按分类汇总</div>
                      <div className="text-xs text-[var(--nv-text-tertiary)]">合计 <strong className="text-[var(--nv-text-primary)]">{formatBytes(cacheUsage.total_size)}</strong> · {cacheUsage.total_count.toLocaleString()} 文件</div>
                    </div>
                    <div className="space-y-1">
                      {cacheUsage.categories.map((category) => {
                        const isPreprocess = category.key === 'preprocess'
                        const isExpanded = expandedCategoryKey === category.key
                        const pct = cacheUsage.total_size > 0 ? (category.size / cacheUsage.total_size) * 100 : 0
                        const isThisCleaning = cleaningCategory === category.key
                        const anyCleaning = !!cleaningCategory
                        return (
                          <div
                            key={category.key}
                            role="button"
                            tabIndex={anyCleaning ? -1 : 0}
                            onClick={() => {
                              if (anyCleaning) return
                              setExpandedCategoryKey(isExpanded && !isPreprocess ? '' : category.key)
                            }}
                            onKeyDown={(e) => {
                              if (anyCleaning) return
                              if (e.key === 'Enter' || e.key === ' ') {
                                e.preventDefault()
                                setExpandedCategoryKey(isExpanded && !isPreprocess ? '' : category.key)
                              }
                            }}
                            className={clsx(
                              'cursor-pointer rounded-[var(--nv-radius-control)] border px-3 py-2 transition-colors',
                              isExpanded ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)]' : 'border-transparent bg-[var(--nv-bg-surface-soft)] hover:bg-[var(--nv-bg-hover)]',
                              anyCleaning && !isThisCleaning && 'opacity-60',
                            )}
                            title={isPreprocess ? '展开下方查看预处理产物明细' : category.path}
                          >
                            <div className="flex items-center gap-3">
                              <div className="min-w-0 flex-1">
                                <div className="flex flex-wrap items-center gap-2">
                                  <span className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{category.label}</span>
                                  {isPreprocess && <Tag tone="brand">可钻取</Tag>}
                                  {category.cleanable && !isPreprocess && <Tag tone="success">可清理</Tag>}
                                </div>
                                <div className="mt-0.5 truncate font-mono text-[11px] text-[var(--nv-text-tertiary)]">{category.path}</div>
                                <div className="mt-1.5 h-1 w-full overflow-hidden rounded-full bg-[var(--nv-bg-active)]">
                                  <div
                                    className="h-full rounded-full transition-all duration-500"
                                    style={{
                                      width: `${Math.min(100, pct)}%`,
                                      background: isPreprocess
                                        ? 'var(--nv-status-warning)'
                                        : category.cleanable
                                          ? 'var(--nv-status-success)'
                                          : 'var(--nv-action-primary)',
                                    }}
                                  />
                                </div>
                              </div>
                              <div className="shrink-0 text-right">
                                <div className="text-sm font-semibold tabular-nums text-[var(--nv-text-primary)]">{formatBytes(category.size)}</div>
                                <div className="text-[11px] tabular-nums text-[var(--nv-text-tertiary)]">{category.count.toLocaleString()} 文件 · {pct.toFixed(1)}%</div>
                              </div>
                              {category.cleanable && category.size > 0 && (
                                <Button
                                  type="button"
                                  variant="danger"
                                  size="sm"
                                  onClick={(e) => {
                                    e.stopPropagation()
                                    handleCleanCategory(category.key, category.label, category.size)
                                  }}
                                  disabled={anyCleaning}
                                  loading={isThisCleaning}
                                  className="shrink-0"
                                >
                                  {!isThisCleaning && <Trash2 size={11} aria-hidden={true} />}清理
                                </Button>
                              )}
                            </div>
                          </div>
                        )
                      })}
                    </div>
                    <p className="mt-2 text-[11px] leading-5 text-[var(--nv-text-tertiary)]">
                      仅“可清理”分类支持清空，删除后系统会按需重新生成；预处理产物可清空全部非运行产物并重置任务，也可仅清理孤儿目录。
                    </p>
                  </div>
                )}

                {expandedCategoryKey === 'preprocess' && storage ? (
                  <div className="grid grid-cols-1 gap-3 border-b border-[var(--nv-border-subtle)] px-5 py-4 sm:grid-cols-3">
                    <Surface className="bg-[var(--nv-bg-surface-soft)] p-3">
                      <div className="text-[11px] text-[var(--nv-text-tertiary)]">预处理总占用</div>
                      <div className="mt-0.5 text-lg font-bold text-[var(--nv-text-primary)]">{formatBytes(storage.total_size)}</div>
                      <div className="mt-0.5 text-[11px] text-[var(--nv-text-tertiary)]">{storage.total_count} 个目录</div>
                    </Surface>
                    <Surface className="bg-[var(--nv-bg-surface-soft)] p-3">
                      <div className="text-[11px] text-[var(--nv-text-tertiary)]">有效任务</div>
                      <div className="mt-0.5 text-lg font-bold text-[var(--nv-status-success)]">{formatBytes(storage.task_size)}</div>
                      <div className="mt-0.5 text-[11px] text-[var(--nv-text-tertiary)]">{storage.total_count - storage.orphan_count} 个目录</div>
                    </Surface>
                    <Surface className={clsx('p-3', storage.orphan_count > 0 && 'border-[var(--nv-status-danger)] bg-[var(--nv-bg-active)]')}>
                      <div className="flex items-center gap-1 text-[11px] text-[var(--nv-text-tertiary)]">孤儿目录 {storage.orphan_count > 0 && <AlertCircle size={11} className="text-[var(--nv-status-danger)]" aria-hidden={true} />}</div>
                      <div className={clsx('mt-0.5 text-lg font-bold', storage.orphan_count > 0 ? 'text-[var(--nv-status-danger)]' : 'text-[var(--nv-text-primary)]')}>{formatBytes(storage.orphan_size)}</div>
                      <div className="mt-0.5 text-[11px] text-[var(--nv-text-tertiary)]">{storage.orphan_count} 个无主目录</div>
                    </Surface>
                  </div>
                ) : expandedCategoryKey === 'preprocess' && !storage ? (
                  <div className="grid grid-cols-1 gap-3 border-b border-[var(--nv-border-subtle)] px-5 py-4 sm:grid-cols-3">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <Surface key={i} className="bg-[var(--nv-bg-surface-soft)] p-3">
                        <div className="skeleton h-3 w-12 rounded" />
                        <div className="skeleton mt-2 h-5 w-20 rounded" />
                        <div className="skeleton mt-2 h-3 w-16 rounded" />
                      </Surface>
                    ))}
                  </div>
                ) : null}

                {expandedCategoryKey === 'preprocess' && storage && storage.orphan_count > 0 && (
                  <div className="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-active)] px-5 py-3">
                    <div className="text-xs text-[var(--nv-text-secondary)]">
                      检测到 <strong className="text-[var(--nv-status-danger)]">{storage.orphan_count}</strong> 个孤儿目录，可释放 <strong className="text-[var(--nv-status-danger)]">{formatBytes(storage.orphan_size)}</strong>。
                    </div>
                    <Button type="button" variant="danger" size="sm" onClick={handleCleanOrphan} loading={cleaningOrphan}>
                      {!cleaningOrphan && <Eraser size={12} aria-hidden={true} />}一键清理孤儿
                    </Button>
                  </div>
                )}

                <div className="overflow-y-auto" style={{ maxHeight: 'calc(85vh - 320px)', minHeight: '120px' }}>
                  {expandedCategoryKey !== 'preprocess' ? (
                    <EmptyState icon={<Database size={24} />} title="该分类暂未提供明细钻取" description="仅预处理产物支持按媒体钻取与清理；其他分类暂时只展示总量。" className="min-h-48" />
                  ) : storageLoading && !storage ? (
                    <div className="flex min-h-48 flex-col items-center justify-center gap-2 text-sm text-[var(--nv-text-tertiary)]"><Loader2 size={20} className="animate-spin text-[var(--nv-action-primary)]" aria-hidden={true} />正在扫描预处理目录...</div>
                  ) : !storage ? (
                    <div className="flex min-h-48 flex-col items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
                      <AlertCircle size={20} className="text-[var(--nv-status-warning)]" aria-hidden={true} />
                      <div>未能获取存储占用数据</div>
                      <Button type="button" variant="secondary" size="sm" onClick={() => loadStorage(0)}><RefreshCw size={12} aria-hidden={true} />重新扫描</Button>
                    </div>
                  ) : storage.items.length === 0 ? (
                    <EmptyState icon={<Database size={24} />} title="暂无预处理产物" className="min-h-48" />
                  ) : (
                    <ul className="divide-y divide-[var(--nv-border-subtle)]">
                      {storage.items.map((item) => (
                        <li key={item.output_dir} className={clsx('flex items-center gap-3 px-5 py-3 transition-colors', item.is_orphan && 'bg-[var(--nv-bg-active)]')}>
                          <Film size={14} className={clsx('shrink-0', item.is_orphan ? 'text-[var(--nv-status-danger)]' : 'text-[var(--nv-action-primary)]')} aria-hidden={true} />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2 text-sm text-[var(--nv-text-primary)]">
                              <span className="truncate">{item.media_title || item.media_id}</span>
                              {item.is_orphan && <Tag tone="danger">孤儿</Tag>}
                              {!item.is_orphan && item.status && <Tag tone={statusTones[item.status] || 'neutral'}>{statusLabels[item.status] || item.status}</Tag>}
                            </div>
                            <div className="mt-0.5 truncate font-mono text-[11px] text-[var(--nv-text-tertiary)]">{item.output_dir}</div>
                          </div>
                          <div className="shrink-0 text-sm font-semibold tabular-nums text-[var(--nv-text-primary)]">{formatBytes(item.size)}</div>
                          <Button type="button" variant="danger" size="sm" iconOnly onClick={() => handleCleanOne(item.media_id, item.media_title)} aria-label="清理此预处理缓存" title="清理此预处理缓存"><Trash2 size={12} aria-hidden={true} /></Button>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              </motion.div>
            </motion.div>
          )}
        </AnimatePresence>

        <AnimatePresence>
          {filterOpen && (
            <motion.div
              className="fixed inset-0 z-50 flex items-center justify-center p-4"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: durations.fast, ease: easeSmooth }}
              onClick={() => setFilterOpen(false)}
            >
              <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" />
              <motion.div
                className="nv-surface relative flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden shadow-[var(--nv-shadow-elevated)]"
                initial={{ opacity: 0, scale: 0.96, y: 12 }}
                animate={{ opacity: 1, scale: 1, y: 0 }}
                exit={{ opacity: 0, scale: 0.96, y: 12 }}
                transition={{ duration: durations.normal, ease: easeSmooth }}
                onClick={(e) => e.stopPropagation()}
              >
                <div className="flex shrink-0 items-start justify-between gap-4 border-b border-[var(--nv-border-subtle)] px-5 py-4">
                  <div className="flex items-start gap-2">
                    <Filter size={18} className="mt-0.5 text-[var(--nv-action-primary)]" aria-hidden={true} />
                    <div>
                      <div className="text-base font-semibold text-[var(--nv-text-primary)]">自定义筛选预处理</div>
                      <div className="mt-0.5 text-xs text-[var(--nv-text-tertiary)]">多条件组合，先预览命中数量，确认后批量提交</div>
                    </div>
                  </div>
                  <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => setFilterOpen(false)} aria-label="关闭"><X size={14} aria-hidden={true} /></Button>
                </div>

                <div className="flex-1 space-y-5 overflow-y-auto px-5 py-4">
                  {libraries.length > 0 && (
                    <FilterSection title="媒体库" hint="不选 = 全部">
                      {libraries.map((lib) => (
                        <FilterChip key={lib.id} label={`${lib.name} (${lib.type})`} active={filter.library_ids?.includes(lib.id) ?? false} onClick={() => toggleArrayFilter('library_ids', lib.id)} />
                      ))}
                    </FilterSection>
                  )}

                  <FilterSection title="媒体类型" hint="不选 = 全部">
                    {[{ k: 'movie', l: '电影' }, { k: 'episode', l: '剧集' }].map((item) => (
                      <FilterChip key={item.k} label={item.l} active={filter.media_types?.includes(item.k) ?? false} onClick={() => toggleArrayFilter('media_types', item.k)} />
                    ))}
                  </FilterSection>

                  <FilterSection title="视频编码" hint="可按编码筛出更需要预处理的目标">
                    {['h264', 'hevc', 'av1', 'vp9', 'mpeg4', 'wmv3'].map((codec) => (
                      <FilterChip key={codec} label={codec.toUpperCase()} active={filter.video_codecs?.includes(codec) ?? false} onClick={() => toggleArrayFilter('video_codecs', codec)} />
                    ))}
                  </FilterSection>

                  <FilterSection title="容器格式" hint="按文件扩展名匹配">
                    {['mkv', 'mp4', 'avi', 'mov', 'ts', 'flv', 'webm', 'wmv', 'rmvb'].map((container) => (
                      <FilterChip key={container} label={`.${container}`} active={filter.containers?.includes(container) ?? false} onClick={() => toggleArrayFilter('containers', container)} />
                    ))}
                  </FilterSection>

                  <FilterSection title="分辨率">
                    {['480p', '720p', '1080p', '2K', '4K'].map((resolution) => (
                      <FilterChip key={resolution} label={resolution} active={filter.resolutions?.includes(resolution) ?? false} onClick={() => toggleArrayFilter('resolutions', resolution)} />
                    ))}
                  </FilterSection>

                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                    <RangeInput
                      label="文件大小（GB）"
                      minValue={filter.min_size_bytes ? filter.min_size_bytes / (1024 ** 3) : undefined}
                      maxValue={filter.max_size_bytes ? filter.max_size_bytes / (1024 ** 3) : undefined}
                      onChange={(min, max) => {
                        setFilter((prev) => ({ ...prev, min_size_bytes: min ? Math.round(min * 1024 ** 3) : 0, max_size_bytes: max ? Math.round(max * 1024 ** 3) : 0 }))
                        setFilterPreview(null)
                      }}
                      step={0.5}
                    />
                    <RangeInput
                      label="年份"
                      minValue={filter.min_year || undefined}
                      maxValue={filter.max_year || undefined}
                      onChange={(min, max) => {
                        setFilter((prev) => ({ ...prev, min_year: min ?? 0, max_year: max ?? 0 }))
                        setFilterPreview(null)
                      }}
                    />
                    <RangeInput
                      label="时长（分钟）"
                      minValue={filter.min_duration ? Math.round(filter.min_duration / 60) : undefined}
                      maxValue={filter.max_duration ? Math.round(filter.max_duration / 60) : undefined}
                      onChange={(min, max) => {
                        setFilter((prev) => ({ ...prev, min_duration: min ? min * 60 : 0, max_duration: max ? max * 60 : 0 }))
                        setFilterPreview(null)
                      }}
                    />
                  </div>

                  <div>
                    <div className="mb-1.5 text-xs text-[var(--nv-text-secondary)]">关键词（标题/原标题/番号）</div>
                    <Input
                      value={filter.keyword ?? ''}
                      onChange={(e) => {
                        setFilter((prev) => ({ ...prev, keyword: e.target.value }))
                        setFilterPreview(null)
                      }}
                      placeholder="留空则不限制"
                    />
                  </div>

                  <FilterSection title="排除策略" hint="未勾选 = 不排除该类媒体（不推荐关闭）">
                    <ExcludeToggle label="排除已有预处理任务的媒体" checked={filter.exclude_already_preprocessed !== false} onChange={(value) => { setFilter((prev) => ({ ...prev, exclude_already_preprocessed: value })); setFilterPreview(null) }} />
                    <ExcludeToggle label="排除浏览器可零转码直接播放的" checked={filter.exclude_directly_playable !== false} onChange={(value) => { setFilter((prev) => ({ ...prev, exclude_directly_playable: value })); setFilterPreview(null) }} />
                    <ExcludeToggle label="排除 STRM 远程流" checked={filter.exclude_strm !== false} onChange={(value) => { setFilter((prev) => ({ ...prev, exclude_strm: value })); setFilterPreview(null) }} />
                  </FilterSection>

                  {filterPreview && (
                    <Surface className="space-y-3 border-[var(--nv-border-hover)] bg-[var(--nv-bg-surface-soft)] p-4">
                      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                        <PreviewStat label="命中" value={filterPreview.matched_count.toString()} accent />
                        <PreviewStat label="原始命中" value={filterPreview.raw_count.toString()} />
                        <PreviewStat label="总大小" value={formatBytes(filterPreview.total_size)} />
                        <PreviewStat label="排除合计" value={(filterPreview.excluded_already + filterPreview.excluded_playable + filterPreview.excluded_strm).toString()} />
                      </div>
                      {(filterPreview.excluded_already + filterPreview.excluded_playable + filterPreview.excluded_strm) > 0 && (
                        <div className="text-[11px] text-[var(--nv-text-tertiary)]">已排除：已预处理 {filterPreview.excluded_already} · 可直接播放 {filterPreview.excluded_playable} · STRM {filterPreview.excluded_strm}</div>
                      )}
                      {Object.keys(filterPreview.codec_histogram).length > 0 && (
                        <div>
                          <div className="mb-1.5 text-xs text-[var(--nv-text-secondary)]">编码分布</div>
                          <div className="flex flex-wrap gap-1.5">
                            {Object.entries(filterPreview.codec_histogram).map(([codec, count]) => <Tag key={codec} tone="neutral">{codec.toUpperCase()} ×{count}</Tag>)}
                          </div>
                        </div>
                      )}
                      {filterPreview.sample.length > 0 && (
                        <div>
                          <div className="mb-1.5 text-xs text-[var(--nv-text-secondary)]">抽样预览（前 {filterPreview.sample.length} 条）</div>
                          <ul className="max-h-56 divide-y divide-[var(--nv-border-subtle)] overflow-y-auto rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)]">
                            {filterPreview.sample.map((sample) => (
                              <li key={sample.media_id} className="flex items-center gap-2 px-3 py-2 text-xs text-[var(--nv-text-secondary)]">
                                <Film size={12} className="shrink-0 text-[var(--nv-action-primary)]" aria-hidden={true} />
                                <span className="flex-1 truncate text-[var(--nv-text-primary)]">{sample.title}{sample.year ? ` (${sample.year})` : ''}</span>
                                <span className="text-[10px] text-[var(--nv-text-tertiary)]">{sample.video_codec || '?'} · {sample.resolution || '?'} · {formatBytes(sample.file_size)}</span>
                              </li>
                            ))}
                          </ul>
                        </div>
                      )}
                    </Surface>
                  )}
                </div>

                <div className="flex shrink-0 flex-wrap items-center justify-between gap-3 border-t border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-5 py-3">
                  <label className="flex cursor-pointer items-center gap-2 text-xs text-[var(--nv-text-secondary)]">
                    <input type="checkbox" checked={filterForce} onChange={(e) => setFilterForce(e.target.checked)} className="accent-[var(--nv-action-primary)]" />
                    强制（绕过“可直接播放则跳过”判定）
                  </label>
                  <div className="flex items-center gap-2">
                    <Button type="button" variant="secondary" size="sm" onClick={handlePreviewFilter} loading={previewing}>
                      {!previewing && <Sparkles size={12} aria-hidden={true} />}预览
                    </Button>
                    <Button type="button" variant="primary" size="sm" onClick={handleSubmitFilter} loading={submittingFilter} disabled={!filterPreview || filterPreview.matched_count === 0} title={!filterPreview ? '请先预览' : filterPreview.matched_count === 0 ? '当前条件无命中' : ''}>
                      {!submittingFilter && <Send size={12} aria-hidden={true} />}{filterPreview ? `提交 ${filterPreview.matched_count} 个` : '提交'}
                    </Button>
                  </div>
                </div>
              </motion.div>
            </motion.div>
          )}
        </AnimatePresence>
      </PageContainer>
    </motion.div>
  )
}

interface FilterSectionProps {
  title: string
  hint?: string
  children: React.ReactNode
}

function FilterSection({ title, hint, children }: FilterSectionProps) {
  return (
    <div>
      <div className="mb-1.5 flex items-baseline gap-2">
        <div className="text-xs font-medium text-[var(--nv-text-primary)]">{title}</div>
        {hint && <div className="text-[11px] text-[var(--nv-text-tertiary)]">{hint}</div>}
      </div>
      <div className="flex flex-wrap gap-1.5">{children}</div>
    </div>
  )
}

interface FilterChipProps {
  label: string
  active: boolean
  onClick: () => void
}

function FilterChip({ label, active, onClick }: FilterChipProps) {
  return (
    <Button type="button" variant={active ? 'primary' : 'secondary'} size="sm" aria-pressed={active} onClick={onClick} className="h-8 text-xs">
      {label}
    </Button>
  )
}

interface RangeInputProps {
  label: string
  minValue?: number
  maxValue?: number
  onChange: (min?: number, max?: number) => void
  step?: number
}

function RangeInput({ label, minValue, maxValue, onChange, step = 1 }: RangeInputProps) {
  return (
    <div>
      <div className="mb-1.5 text-xs text-[var(--nv-text-secondary)]">{label}</div>
      <div className="flex items-center gap-1.5">
        <Input
          type="number"
          step={step}
          min={0}
          value={minValue ?? ''}
          onChange={(e) => {
            const value = e.target.value === '' ? undefined : Number(e.target.value)
            onChange(value, maxValue)
          }}
          placeholder="最小"
          className="h-9 text-xs tabular-nums"
        />
        <span className="text-xs text-[var(--nv-text-tertiary)]">~</span>
        <Input
          type="number"
          step={step}
          min={0}
          value={maxValue ?? ''}
          onChange={(e) => {
            const value = e.target.value === '' ? undefined : Number(e.target.value)
            onChange(minValue, value)
          }}
          placeholder="最大"
          className="h-9 text-xs tabular-nums"
        />
      </div>
    </div>
  )
}

interface ExcludeToggleProps {
  label: string
  checked: boolean
  onChange: (value: boolean) => void
}

function ExcludeToggle({ label, checked, onChange }: ExcludeToggleProps) {
  return (
    <label className={clsx(
      'flex cursor-pointer items-center gap-2 rounded-[var(--nv-radius-control)] border px-2.5 py-1.5 text-xs transition-colors',
      checked
        ? 'border-[var(--nv-status-success)] bg-[var(--nv-bg-active)] text-[var(--nv-status-success)]'
        : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-secondary)]',
    )}>
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} className="accent-[var(--nv-status-success)]" />
      {label}
    </label>
  )
}

interface PreviewStatProps {
  label: string
  value: string
  accent?: boolean
}

function PreviewStat({ label, value, accent = false }: PreviewStatProps) {
  return (
    <div>
      <div className="text-[11px] text-[var(--nv-text-tertiary)]">{label}</div>
      <div className={clsx('mt-0.5 text-lg font-bold', accent ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-primary)]')}>{value}</div>
    </div>
  )
}
