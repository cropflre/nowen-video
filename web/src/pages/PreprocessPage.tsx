import { useState, useEffect, useCallback, useRef, useMemo, useLayoutEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { pageVariants, staggerContainerVariants, staggerItemVariants, easeSmooth, durations } from '@/lib/motion'

// ==================== P3: 数值变化动画 Hook ====================
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
      // easeOutExpo 缓动
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

// ==================== P2: 环形进度 SVG 组件 ====================
function RingProgress({ value, max, size = 44, strokeWidth = 3, color = 'var(--neon-blue)', glowColor = 'var(--neon-blue-30)' }: {
  value: number; max: number; size?: number; strokeWidth?: number; color?: string; glowColor?: string
}) {
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius
  const ratio = max > 0 ? Math.min(value / max, 1) : 0
  const offset = circumference * (1 - ratio)

  return (
    <svg width={size} height={size} className="-rotate-90" viewBox={`0 0 ${size} ${size}`}>
      {/* 轨道 */}
      <circle
        cx={size / 2} cy={size / 2} r={radius}
        fill="none" stroke="var(--neon-blue-6)" strokeWidth={strokeWidth}
      />
      {/* 进度弧 */}
      <circle
        cx={size / 2} cy={size / 2} r={radius}
        fill="none" stroke={color} strokeWidth={strokeWidth}
        strokeDasharray={circumference}
        strokeDashoffset={offset}
        strokeLinecap="round"
        className="transition-all duration-700 ease-out"
        style={{ filter: `drop-shadow(0 0 4px ${glowColor})` }}
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
import TranscodeJobsPanel from '@/components/preprocess/TranscodeJobsPanel'
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

// 状态颜色映射
const statusColors: Record<string, string> = {
  pending: 'text-yellow-400',
  queued: 'text-amber-400',
  running: 'text-neon-blue',
  paused: 'text-orange-400',
  completed: 'text-emerald-400',
  failed: 'text-red-400',
  cancelled: 'text-surface-500',
}

// 字节数格式化（B / KB / MB / GB / TB），保留 2 位小数
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
  pending: <Clock size={14} />,
  queued: <Clock size={14} />,
  running: <Loader2 size={14} className="animate-spin" />,
  paused: <Pause size={14} />,
  completed: <CheckCircle2 size={14} />,
  failed: <AlertCircle size={14} />,
  cancelled: <XCircle size={14} />,
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
  // 主区域 Tab 切换：'submit' = 影视文件列表（选源提交）；'tasks' = HLS 处理任务进度；'transcode' = 转码任务（迁自系统管理）
  // 默认 'tasks' —— 进度查看是日常高频场景；用户主动提交新任务时再切到 'submit'
  // 支持 URL hash：#transcode 直达转码任务子页
  const [mainTab, setMainTab] = useState<'submit' | 'tasks' | 'transcode'>(() => {
    const hash = (typeof window !== 'undefined' ? window.location.hash.replace('#', '') : '')
    if (hash === 'submit' || hash === 'tasks' || hash === 'transcode') return hash
    return 'tasks'
  })
  const [stats, setStats] = useState<PreprocessStatistics | null>(null)
  const [sysLoad, setSysLoad] = useState<SystemLoadInfo | null>(null)
  // 预处理产物存储占用
  const [storage, setStorage] = useState<PreprocessStorageUsage | null>(null)
  const [storageOpen, setStorageOpen] = useState(false)
  const [storageLoading, setStorageLoading] = useState(false)
  const [cleaningOrphan, setCleaningOrphan] = useState(false)
  // 分类清理：记录正在清理的分类 key（'__all__' 代表一键清空）
  const [cleaningCategory, setCleaningCategory] = useState<string | null>(null)
  // cache/ 整盘分类占用（包括 preprocess + transcode + thumbnails 等）
  const [cacheUsage, setCacheUsage] = useState<CacheUsage | null>(null)
  const [cacheLoading, setCacheLoading] = useState(false)
  // 弹窗内：当前展开的分类（'preprocess' 时显示下方 items 钻取列表）
  const [expandedCategoryKey, setExpandedCategoryKey] = useState<string>('preprocess')

  // 自定义筛选预处理
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
  // 批量选择
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchLoading, setBatchLoading] = useState(false)

  // 候选影视列表（手动勾选预处理）
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

  // 计算总页数
  const totalPages = useMemo(() => calcTotalPages(total), [calcTotalPages, total])

  // P2: 状态过滤滑动指示器
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

  // P3: 统计数值动画
  const animRunning = useAnimatedCounter(stats?.running_count ?? 0)
  const animQueue = useAnimatedCounter(stats?.queue_size ?? 0)

  // 全选/取消全选当前页
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
    if (newSet.has(id)) {
      newSet.delete(id)
    } else {
      newSet.add(id)
    }
    setSelectedIds(newSet)
  }

  // 加载任务列表
  const loadTasks = useCallback(async () => {
    try {
      const res = await preprocessApi.listTasks(page, pageSize, statusFilter)
      setTasks(res.data.data.tasks || [])
      setTotal(res.data.data.total)
    } catch {
      toastRef.current.error('加载预处理任务失败')
    }
  }, [page, pageSize, statusFilter])

  // 加载统计和系统负载
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

  // 加载预处理产物磁盘占用（涉及递归 walk，按需调用）
  // limit=0 表示拉取全部明细（弹窗打开时使用）
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

  // 加载整个 cache/ 目录的分类占用
  // force=true 跳过后端 30s 内存缓存（用户主动点"重新扫描"时使用）
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

  // 卡片首次进入时拉一次（只看汇总数即可，明细 limit=20 已足够）
  useEffect(() => {
    loadStorage(20)
    loadCacheUsage(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 一键清理孤儿目录
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
      // 后端 CleanOrphanCache 已主动 invalidate，这里强制刷一次拿到最新值
      await loadCacheUsage(true)
    } catch (e: any) {
      toastRef.current.error(e?.response?.data?.error || '清理失败')
    } finally {
      setCleaningOrphan(false)
    }
  }, [storage, loadStorage, loadCacheUsage])

  // 清理单个媒体的预处理缓存（弹窗内用）
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
  }, [loadStorage])

  // 手动清理单个分类（点击分类行右侧"清理"按钮）
  // preprocess：默认 mode=all，清所有非 running 任务的产物，并把对应任务重置为 pending（下次播放重新生成）
  //            running 任务受保护；如需仅清孤儿，使用「清理孤儿」按钮（独立入口）
  // 其它 cleanable 分类：整目录清空
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

  // 一键清空所有可清分类
  const handleCleanAllCache = useCallback(async () => {
    if (cleaningCategory) return
    if (!cacheUsage) return
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

  // 自定义筛选 - 预览
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

  // 自定义筛选 - 提交
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

  // 数组型筛选条件的多选切换工具
  const toggleArrayFilter = useCallback(<K extends keyof PreprocessFilter>(
    key: K,
    value: string,
  ) => {
    setFilter((prev) => {
      const arr = (prev[key] as string[] | undefined) ?? []
      const next = arr.includes(value) ? arr.filter((v) => v !== value) : [...arr, value]
      return { ...prev, [key]: next } as PreprocessFilter
    })
    setFilterPreview(null) // 条件变更后预览作废
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

  // 加载候选影视列表（手动勾选预处理）
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

  // 初始加载
  useEffect(() => {
    setLoading(true)
    const promises: Promise<void>[] = [loadTasks(), loadStats(), loadLibraries()]
    Promise.all(promises).finally(() => setLoading(false))
  }, [loadTasks, loadStats, loadLibraries])

  // 候选列表筛选/分页变化时刷新
  useEffect(() => {
    loadCandidates()
  }, [loadCandidates])


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

    // 存储占用刷新（独立节流：完成/失败后才扫盘，且 10s 内只扫一次，避免高频全盘扫描）
    let storageTimer: ReturnType<typeof setTimeout> | null = null
    const scheduleStorageRefresh = () => {
      if (storageTimer) return
      // 延迟一点再扫，等文件系统刷新完
      storageTimer = setTimeout(() => {
        storageTimer = null
        loadStorage(20)
        // 后端 30s 内存缓存仍然生效，这里不强制 force，等 TTL 自然过期再重扫
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

  // 任务操作
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

  // 批量操作
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

  // 提交整个媒体库预处理
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
      <div className="mx-auto max-w-7xl space-y-6 p-6 animate-fade-in">
        {/* 页面标题骨架 */}
        <div className="flex items-center justify-between">
          <div className="space-y-2">
            <div className="skeleton h-7 w-40 rounded-lg" />
            <div className="skeleton h-4 w-64 rounded" />
          </div>
          <div className="flex items-center gap-2">
            <div className="skeleton h-9 w-24 rounded-lg" />
            <div className="skeleton h-9 w-20 rounded-lg" />
          </div>
        </div>
        {/* 统计卡片骨架 */}
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
              <div className="flex items-center gap-2 mb-2">
                <div className="skeleton h-3.5 w-3.5 rounded" />
                <div className="skeleton h-3 w-14 rounded" />
              </div>
              <div className="skeleton h-7 w-12 rounded-lg" />
              <div className="skeleton mt-1.5 h-3 w-24 rounded" />
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
    <motion.div
      variants={pageVariants}
      initial="initial"
      animate="enter"
      className="mx-auto max-w-7xl space-y-6 p-6"
    >
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-display font-bold flex items-center gap-2 text-gradient">
            <Zap className="text-neon-blue animate-neon-breathe" size={24} />
            视频预处理
          </h1>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-tertiary)' }}>
            自动转码生成多码率 HLS 流，实现秒开播放
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => { loadTasks(); loadStats(); loadStorage(20) }}
            className="flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm transition-all active:scale-95"
            style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}
          >
            <RefreshCw size={14} />
            刷新
          </button>
        </div>
      </div>

      {/* 统计卡片 */}
      {stats && sysLoad && (
        <motion.div
          className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5"
          variants={staggerContainerVariants}
          initial="hidden"
          animate="visible"
        >
          <motion.div variants={staggerItemVariants} className="relative overflow-hidden rounded-xl p-4 group transition-shadow duration-300 hover:shadow-card-hover" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="absolute top-0 left-0 right-0 h-[1px] opacity-60" style={{ background: 'linear-gradient(90deg, transparent, var(--neon-blue), transparent)' }} />
            <div className="flex items-start justify-between">
              <div>
                <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
                  <Activity size={14} className="text-neon-blue" />
                  处理中
                </div>
                <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>{animRunning}</div>
                <div className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
                  {stats.active_workers}/{sysLoad.cur_workers || stats.max_workers} 工作线程
                </div>
              </div>
              {/* P2: 环形进度 — 工作线程利用率 */}
              <div className="relative flex-shrink-0">
                <RingProgress
                  value={stats.active_workers}
                  max={sysLoad.cur_workers || stats.max_workers}
                  size={44}
                  strokeWidth={3}
                />
                <span className="absolute inset-0 flex items-center justify-center text-[10px] font-bold" style={{ color: 'var(--text-primary)' }}>
                  {stats.active_workers}
                </span>
              </div>
            </div>
          </motion.div>

          <motion.div variants={staggerItemVariants} className="relative overflow-hidden rounded-xl p-4 group transition-shadow duration-300 hover:shadow-card-hover" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="absolute top-0 left-0 right-0 h-[1px] opacity-60" style={{ background: 'linear-gradient(90deg, transparent, var(--neon-blue), transparent)' }} />
            <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
              <Clock size={14} className="text-yellow-400" />
              队列
            </div>
            <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>{animQueue}</div>
            <div className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              等待处理
            </div>
          </motion.div>

          <motion.div variants={staggerItemVariants} className="relative overflow-hidden rounded-xl p-4 group transition-shadow duration-300 hover:shadow-card-hover" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
              <Cpu size={14} className="text-emerald-400" />
              系统负载
            </div>
            <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
              {sysLoad.cpu_percent != null ? `${sysLoad.cpu_percent.toFixed(0)}%` : `${sysLoad.mem_alloc_mb.toFixed(0)} MB`}
            </div>
            <div className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              {sysLoad.cpu_count} CPU · {sysLoad.max_workers} worker
            </div>
            {/* CPU 使用率进度条（按 80/60 阈值着色） */}
            {sysLoad.cpu_percent != null && (
              <div className="mt-2 h-1 w-full rounded-full" style={{ background: 'var(--progress-track-bg)' }}>
                <div
                  className="h-full rounded-full transition-all duration-500"
                  style={{
                    width: `${Math.min(100, sysLoad.cpu_percent)}%`,
                    background: sysLoad.cpu_percent > 80 ? '#ef4444'
                      : sysLoad.cpu_percent > 60 ? '#f59e0b'
                      : '#10b981',
                  }}
                />
              </div>
            )}
          </motion.div>

          <motion.div variants={staggerItemVariants} className="relative overflow-hidden rounded-xl p-4 group transition-shadow duration-300 hover:shadow-card-hover" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="absolute top-0 left-0 right-0 h-[1px] opacity-60" style={{ background: 'linear-gradient(90deg, transparent, var(--neon-purple), transparent)' }} />
            <div className="flex items-center gap-2 text-xs mb-2" style={{ color: 'var(--text-muted)' }}>
              <HardDrive size={14} className="text-purple-400" />
              硬件加速
            </div>
            <div className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>{stats.hw_accel || 'CPU'}</div>
            <div className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              {sysLoad.gpu_status?.degraded ? (
                <span className="text-red-400">⚠ GPU 过载 · 已降级为 CPU</span>
              ) : (
                <>已完成 {stats.status_counts?.completed || 0} 个</>
              )}
            </div>
            {/* GPU 使用率进度条 */}
            {sysLoad.gpu_status?.metrics?.available && (
              <div className="mt-2 h-1 w-full rounded-full" style={{ background: 'var(--progress-track-bg)' }}>
                <div
                  className="h-full rounded-full transition-all duration-500"
                  style={{
                    width: `${Math.min(100, sysLoad.gpu_status.metrics.utilization)}%`,
                    background: sysLoad.gpu_status.degraded ? '#ef4444'
                      : sysLoad.gpu_status.metrics.utilization > 80 ? '#f59e0b'
                      : '#a855f7',
                  }}
                />
              </div>
            )}
          </motion.div>

          {/* 存储占用 - 点击查看明细 */}
          <motion.button
            type="button"
            variants={staggerItemVariants}
            onClick={() => { setStorageOpen(true); loadStorage(0); loadCacheUsage(false) }}
            className="relative overflow-hidden rounded-xl p-4 text-left group transition-shadow duration-300 hover:shadow-card-hover focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-400/50"
            style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}
            title="点击查看缓存占用明细（preprocess + transcode + thumbnails 等）"
          >
            <div className="absolute top-0 left-0 right-0 h-[1px] opacity-60" style={{ background: 'linear-gradient(90deg, transparent, #f59e0b, transparent)' }} />
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                <Database size={14} className="text-amber-400" />
                缓存占用
              </div>
              {(storageLoading || cacheLoading) && <Loader2 size={12} className="animate-spin" style={{ color: 'var(--text-muted)' }} />}
            </div>
            <div className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
              {cacheUsage ? formatBytes(cacheUsage.total_size) : storage ? formatBytes(storage.total_size) : '—'}
            </div>
            <div className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              {cacheUsage ? (
                <>
                  {cacheUsage.total_count.toLocaleString()} 文件 · {cacheUsage.categories.length} 个分类
                  {storage && storage.orphan_count > 0 && (
                    <span className="ml-1 text-red-400">· 孤儿 {storage.orphan_count}</span>
                  )}
                </>
              ) : storage ? (
                <>
                  {storage.total_count} 个目录
                  {storage.orphan_count > 0 && (
                    <span className="ml-1 text-red-400">· 孤儿 {storage.orphan_count}</span>
                  )}
                </>
              ) : (
                '点击加载'
              )}
            </div>
            {/* 孤儿占比进度条（基于 cache 总占用，更直观地反映孤儿在整盘的占比） */}
            {cacheUsage && cacheUsage.total_size > 0 && storage && storage.orphan_size > 0 && (
              <div className="mt-2 h-1 w-full rounded-full overflow-hidden" style={{ background: 'var(--progress-track-bg)' }}>
                <div
                  className="h-full rounded-full transition-all duration-500"
                  style={{
                    width: `${Math.min(100, (storage.orphan_size / cacheUsage.total_size) * 100)}%`,
                    background: '#ef4444',
                  }}
                />
              </div>
            )}
            {cacheUsage && cacheUsage.total_size > 0 && (!storage || storage.orphan_size === 0) && (
              <div className="mt-2 h-1 w-full rounded-full overflow-hidden" style={{ background: 'var(--progress-track-bg)' }}>
                <div
                  className="h-full rounded-full transition-all duration-500"
                  style={{ width: '100%', background: '#10b981' }}
                />
              </div>
            )}
          </motion.button>
        </motion.div>
      )}

      {/* 媒体库批量预处理 */}
      {libraries.length > 0 && (
        <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-medium flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
              <FolderOpen size={16} className="text-neon-blue" />
              媒体库批量预处理
            </h2>
            <button
              onClick={() => { setFilterOpen(true); setFilterPreview(null) }}
              className="nv-amber-soft-btn flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-colors"
              title="按条件自定义选择哪些影视进行预处理"
            >
              <Filter size={12} />
              自定义筛选
            </button>
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

      {/* 主区域 Tab 切换：选源提交 / 处理进度 / 转码任务 —— 避免多块同时铺开造成页面过长 */}
      <div className="flex items-center gap-2 flex-wrap">
        {([
          { key: 'submit', label: '选源提交', count: candidatesTotal },
          { key: 'tasks', label: '处理进度', count: total },
          { key: 'transcode', label: '转码任务', count: 0 },
        ] as const).map((t) => {
          const active = mainTab === t.key
          return (
            <button
              key={t.key}
              type="button"
              onClick={() => {
                setMainTab(t.key)
                if (typeof window !== 'undefined') {
                  window.location.hash = t.key
                }
              }}
              className={clsx(
                'rounded-lg px-3 py-1.5 text-xs transition-all duration-200',
                active && 'font-medium',
              )}
              style={active
                ? { background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)', color: 'var(--text-primary)' }
                : { background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)', color: 'var(--text-muted)' }}
            >
              {t.label}
              {t.count > 0 && (
                <span className="ml-1.5" style={{ color: active ? 'var(--neon-blue)' : 'var(--text-muted)' }}>
                  ({t.count.toLocaleString()})
                </span>
              )}
            </button>
          )
        })}
      </div>

      {/* 影视文件列表 — 用户手动勾选预处理 */}
      {mainTab === 'submit' && (
      <div className="rounded-xl p-4 space-y-3" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
        <div className="flex items-center justify-between flex-wrap gap-2">
          <h2 className="text-sm font-medium flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
            <Film size={16} className="text-neon-blue" />
            影视文件列表
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              （共 {candidatesTotal.toLocaleString()} 条）
            </span>
          </h2>
          <div className="flex items-center gap-2">
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              已选 <span style={{ color: 'var(--neon-blue)' }}>{candSelected.size}</span> 项
            </span>
            <button
              type="button"
              onClick={() => setCandSelected(new Set())}
              disabled={candSelected.size === 0}
              className="text-xs px-2 py-1 rounded transition disabled:opacity-40"
              style={{ background: 'var(--surface-glass-2)', color: 'var(--text-secondary)' }}
            >
              清空
            </button>
            <button
              type="button"
              disabled={candSelected.size === 0 || candSubmitting}
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
              className="text-xs px-3 py-1.5 rounded-md flex items-center gap-1.5 transition disabled:opacity-40 disabled:cursor-not-allowed"
              style={{ background: 'var(--neon-blue)', color: '#fff', boxShadow: '0 0 12px var(--neon-blue-30)' }}
            >
              {candSubmitting ? <Loader2 size={14} className="animate-spin" /> : <Send size={14} />}
              提交预处理（{candSelected.size}）
            </button>
          </div>
        </div>

        {/* 筛选行 */}
        <div className="flex items-center gap-2 flex-wrap">
          <div className="relative flex-1 min-w-[200px]">
            <input
              value={candKeywordInput}
              onChange={(e) => setCandKeywordInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  setCandPage(1)
                  setCandKeyword(candKeywordInput.trim())
                }
              }}
              placeholder="搜索标题 / 原名 / 番号，回车确认"
              className="w-full px-3 py-1.5 text-sm rounded-md outline-none"
              style={{ background: 'var(--surface-glass-2)', color: 'var(--text-primary)', border: '1px solid var(--border-subtle)' }}
            />
          </div>
          <select
            value={candLibraryID}
            onChange={(e) => { setCandPage(1); setCandLibraryID(e.target.value) }}
            className="px-2 py-1.5 text-sm rounded-md outline-none"
            style={{ background: 'var(--surface-glass-2)', color: 'var(--text-primary)', border: '1px solid var(--border-subtle)' }}
          >
            <option value="">全部媒体库</option>
            {libraries.map((lib) => (
              <option key={lib.id} value={lib.id}>{lib.name}</option>
            ))}
          </select>
          <select
            value={candMediaType}
            onChange={(e) => { setCandPage(1); setCandMediaType(e.target.value) }}
            className="px-2 py-1.5 text-sm rounded-md outline-none"
            style={{ background: 'var(--surface-glass-2)', color: 'var(--text-primary)', border: '1px solid var(--border-subtle)' }}
          >
            <option value="">全部类型</option>
            <option value="movie">电影</option>
            <option value="episode">剧集</option>
          </select>
          <label className="flex items-center gap-1.5 text-xs cursor-pointer px-2 py-1.5 rounded-md"
            style={{ background: 'var(--surface-glass-2)', color: 'var(--text-secondary)', border: '1px solid var(--border-subtle)' }}>
            <input
              type="checkbox"
              checked={candOnlyNeed}
              onChange={(e) => { setCandPage(1); setCandOnlyNeed(e.target.checked) }}
              className="accent-neon-blue"
            />
            仅显示需要预处理
          </label>
          <button
            type="button"
            onClick={() => loadCandidates()}
            disabled={candidatesLoading}
            className="px-2 py-1.5 text-xs rounded-md flex items-center gap-1 transition"
            style={{ background: 'var(--surface-glass-2)', color: 'var(--text-secondary)', border: '1px solid var(--border-subtle)' }}
            title="刷新"
          >
            <RefreshCw size={12} className={candidatesLoading ? 'animate-spin' : ''} />
            刷新
          </button>
        </div>

        {/* 列表 */}
        <div className="overflow-hidden rounded-lg" style={{ border: '1px solid var(--border-subtle)' }}>
          {/* 表头：全选 */}
          <div className="flex items-center gap-2 px-3 py-2 text-xs" style={{ background: 'var(--surface-glass-2)', color: 'var(--text-muted)' }}>
            <button
              type="button"
              onClick={() => {
                const eligible = candidates.filter((c) => !c.is_strm && c.preprocess_status !== 'completed' && c.preprocess_status !== 'running' && c.preprocess_status !== 'queued' && c.preprocess_status !== 'pending')
                const allSelected = eligible.length > 0 && eligible.every((c) => candSelected.has(c.media_id))
                const next = new Set(candSelected)
                if (allSelected) {
                  eligible.forEach((c) => next.delete(c.media_id))
                } else {
                  eligible.forEach((c) => next.add(c.media_id))
                }
                setCandSelected(next)
              }}
              className="flex items-center gap-1 hover:text-neon-blue transition"
              title="全选 / 取消全选当前页可选项"
            >
              {(() => {
                const eligible = candidates.filter((c) => !c.is_strm && c.preprocess_status !== 'completed' && c.preprocess_status !== 'running' && c.preprocess_status !== 'queued' && c.preprocess_status !== 'pending')
                const allSelected = eligible.length > 0 && eligible.every((c) => candSelected.has(c.media_id))
                return allSelected ? <CheckSquare size={14} className="text-neon-blue" /> : <Square size={14} />
              })()}
              当前页全选
            </button>
            <span className="ml-auto">第 {candPage} / {candCalcTotalPages(candidatesTotal) || 1} 页</span>
          </div>

          {candidatesLoading && candidates.length === 0 ? (
            <div className="p-6 text-center text-xs" style={{ color: 'var(--text-muted)' }}>
              <Loader2 size={16} className="inline animate-spin mr-2" />
              加载中...
            </div>
          ) : candidates.length === 0 ? (
            <div className="p-6 text-center text-xs" style={{ color: 'var(--text-muted)' }}>
              暂无匹配的影视
            </div>
          ) : (
            <ul className="divide-y" style={{ borderColor: 'var(--border-subtle)' }}>
              {candidates.map((c) => {
                const disabled = c.is_strm || c.preprocess_status === 'completed' || c.preprocess_status === 'running' || c.preprocess_status === 'queued' || c.preprocess_status === 'pending'
                const checked = candSelected.has(c.media_id)
                const statusClass = statusColors[c.preprocess_status] || 'text-surface-500'
                return (
                  <li
                    key={c.media_id}
                    className={clsx(
                      'flex items-center gap-3 px-3 py-2 transition',
                      !disabled && 'cursor-pointer hover:bg-white/5',
                      checked && 'bg-neon-blue/5'
                    )}
                    title={c.file_path}
                    onClick={() => {
                      if (disabled) return
                      const next = new Set(candSelected)
                      if (next.has(c.media_id)) next.delete(c.media_id)
                      else next.add(c.media_id)
                      setCandSelected(next)
                    }}
                  >
                    <div className="flex-shrink-0">
                      {disabled ? (
                        <Square size={14} className="opacity-30" />
                      ) : checked ? (
                        <CheckSquare size={14} className="text-neon-blue" />
                      ) : (
                        <Square size={14} style={{ color: 'var(--text-muted)' }} />
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        {(() => {
                          // 判断刮削状态：pending / failed / 空 → 视为未刮削
                          const ss = (c.scrape_status || '').toLowerCase()
                          const unscraped = ss === '' || ss === 'pending' || ss === 'failed'
                          if (unscraped) {
                            // 未刮削：展示源文件名（去除路径与扩展名），并附"未刮削"提示
                            const fp = c.file_path || ''
                            const idx = Math.max(fp.lastIndexOf('/'), fp.lastIndexOf('\\'))
                            const base = idx >= 0 ? fp.slice(idx + 1) : fp
                            const dot = base.lastIndexOf('.')
                            const fileName = dot > 0 ? base.slice(0, dot) : base
                            return (
                              <>
                                <span
                                  className="text-[10px] font-medium px-1.5 py-0.5 rounded"
                                  style={{ background: 'rgba(245, 158, 11, 0.15)', color: '#f59e0b', border: '1px solid rgba(245, 158, 11, 0.35)' }}
                                  title="该影视文件未完成刮削，下方展示的是源文件名"
                                >
                                  未刮削
                                </span>
                                <span
                                  className="text-sm font-medium truncate"
                                  style={{ color: 'var(--text-secondary)' }}
                                  title={fileName || c.file_path}
                                >
                                  {fileName || c.title || '(无文件名)'}
                                </span>
                              </>
                            )
                          }
                          // 已刮削：展示元数据标题（剧集 / 电影 各自附信息）
                          return (
                            <>
                              <span className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                                {c.title || c.orig_title || '(无标题)'}
                              </span>
                              {c.media_type === 'episode' && (c.season_num || c.episode_num) ? (
                                <span
                                  className="text-xs font-mono px-1.5 py-0.5 rounded"
                                  style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}
                                >
                                  {`S${String(c.season_num ?? 0).padStart(2, '0')}E${String(c.episode_num ?? 0).padStart(2, '0')}`}
                                </span>
                              ) : null}
                              {c.media_type === 'episode' && c.episode_title ? (
                                <span className="text-xs truncate" style={{ color: 'var(--text-tertiary)' }}>
                                  · {c.episode_title}
                                </span>
                              ) : null}
                              {c.year > 0 && (
                                <span className="text-xs" style={{ color: 'var(--text-muted)' }}>({c.year})</span>
                              )}
                              {/* 电影：原始标题（与中文标题不同时展示，便于辨识同名作品） */}
                              {c.media_type !== 'episode' && c.orig_title && c.orig_title !== c.title && (
                                <span className="text-xs truncate" style={{ color: 'var(--text-tertiary)' }} title={c.orig_title}>
                                  · {c.orig_title}
                                </span>
                              )}
                            </>
                          )
                        })()}
                        <span className="text-[10px] px-1.5 py-0.5 rounded" style={{ background: 'var(--surface-glass-2)', color: 'var(--text-muted)' }}>
                          {c.media_type === 'episode' ? '剧集' : '电影'}
                        </span>
                        {c.is_strm && (
                          <span className="text-[10px] px-1.5 py-0.5 rounded text-amber-400" style={{ background: 'rgba(245,158,11,0.1)' }}>
                            STRM
                          </span>
                        )}
                        {c.can_play_directly && !c.is_strm && (
                          <span className="text-[10px] px-1.5 py-0.5 rounded text-emerald-400" style={{ background: 'rgba(16,185,129,0.1)' }}>
                            可直接播放
                          </span>
                        )}
                        {c.preprocess_status !== 'none' && (
                          <span className={clsx('text-[10px] px-1.5 py-0.5 rounded', statusClass)} style={{ background: 'var(--surface-glass-2)' }}>
                            {c.preprocess_status}
                          </span>
                        )}
                      </div>
                      <div className="text-xs mt-0.5 flex items-center gap-3 flex-wrap" style={{ color: 'var(--text-muted)' }}>
                        {c.resolution && <span>{c.resolution}</span>}
                        {c.video_codec && <span>{c.video_codec}</span>}
                        {c.audio_codec && <span>{c.audio_codec}</span>}
                        {c.duration > 0 && <span>{Math.round(c.duration / 60)} 分钟</span>}
                        {c.file_size > 0 && <span>{formatBytes(c.file_size)}</span>}
                      </div>
                    </div>
                  </li>
                )
              })}
            </ul>
          )}
        </div>

        {/* 分页 */}
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
      </div>
      )}

      {/* ====== 处理进度 Tab：状态过滤 + 批量栏 + 任务列表 + 分页 ====== */}
      {mainTab === 'tasks' && (
      <>
      {/* 状态过滤 — P2: 带滑动指示器 */}
      <div ref={filterContainerRef} className="relative flex items-center gap-2 flex-wrap pb-1">
        {/* 滑动指示器 */}
        {filterIndicator && (
          <motion.div
            className="absolute bottom-0 h-[2px] rounded-full z-10"
            style={{ background: 'var(--neon-blue)', boxShadow: '0 0 8px var(--neon-blue-30)' }}
            animate={{ left: filterIndicator.left, width: filterIndicator.width }}
            transition={{ type: 'spring', stiffness: 400, damping: 30 }}
          />
        )}
        {['', 'running', 'pending', 'paused', 'completed', 'failed', 'cancelled'].map((s) => (
          <button
            key={s}
            ref={(el) => { filterBtnRefs.current[s] = el }}
            onClick={() => { setStatusFilter(s); setPage(1); setSelectedIds(new Set()) }}
            className={clsx(
              'rounded-lg px-3 py-1.5 text-xs transition-all duration-200',
              statusFilter === s && 'font-medium',
            )}
            style={statusFilter === s ? { background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)', color: 'var(--text-primary)' } : { background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)', color: 'var(--text-muted)' }}
          >
            {s === '' ? '全部' : statusLabels[s] || s}
            {s && stats?.status_counts?.[s] ? ` (${stats.status_counts[s]})` : ''}
          </button>
        ))}
      </div>

      {/* 批量操作工具栏 */}
      <AnimatePresence>
      {isSomeSelected && (
        <motion.div
          initial={{ opacity: 0, y: -12, filter: 'blur(4px)' }}
          animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
          exit={{ opacity: 0, y: -8, filter: 'blur(2px)' }}
          transition={{ duration: 0.25, ease: easeSmooth as unknown as [number, number, number, number] }}
          className="flex items-center gap-3 rounded-xl px-4 py-3"
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
            className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-all hover:bg-yellow-400/10 active:scale-90 disabled:opacity-50"
            style={{ color: 'var(--text-muted)', border: '1px solid var(--neon-blue-6)' }}
          >
            <XCircle size={12} />
            批量取消
          </button>

          <button
            onClick={handleBatchRetry}
            disabled={batchLoading}
            className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-all hover:bg-neon-blue/10 active:scale-90 disabled:opacity-50"
            style={{ color: 'var(--text-muted)', border: '1px solid var(--neon-blue-6)' }}
          >
            <RotateCcw size={12} />
            批量重试
          </button>

          <button
            onClick={handleBatchDelete}
            disabled={batchLoading}
            className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs transition-all hover:bg-red-400/10 hover:text-red-400 active:scale-90 disabled:opacity-50"
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
        </motion.div>
      )}
      </AnimatePresence>

      {/* 任务列表 */}
      <motion.div
        className="space-y-3"
        variants={staggerContainerVariants}
        initial="hidden"
        animate="visible"
        key={statusFilter + '-' + page}
      >
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
          <motion.div
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: durations.normal, ease: easeSmooth as unknown as [number, number, number, number] }}
            className="flex flex-col items-center justify-center py-16"
            style={{ color: 'var(--text-muted)' }}
          >
            <Film size={48} className="mb-4 opacity-30" />
            <p>暂无预处理任务</p>
            <p className="text-xs mt-1">扫描媒体库后将自动提交预处理任务</p>
          </motion.div>
        ) : (
          tasks.map((task) => (
            <motion.div
              key={task.id}
              variants={staggerItemVariants}
              layout
              className={clsx('rounded-xl p-4 transition-all duration-200', selectedIds.has(task.id) && 'ring-1 ring-neon-blue/30')}
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
                  </div>

                  {/* 进度条 */}
                  {(task.status === 'running' || task.status === 'paused') && (
                    <div className="mt-2">
                      <div className="flex items-center justify-between text-xs mb-1" style={{ color: 'var(--text-muted)' }}>
                        <span>{task.phase || task.message}</span>
                        <span>{task.progress.toFixed(1)}%</span>
                      </div>
                      <div className="h-1.5 w-full rounded-full" style={{ background: 'var(--progress-track-bg)' }}>
                        <div
                          className="h-full rounded-full transition-all duration-500"
                          style={{
                            width: `${task.progress}%`,
                            background: task.status === 'paused'
                              ? 'var(--neon-orange, orange)'
                              : 'linear-gradient(90deg, var(--neon-purple), var(--neon-blue))',
                            boxShadow: task.status === 'running' ? 'var(--progress-bar-glow)' : 'none',
                          }}
                        />
                      </div>
                    </div>
                  )}

                  {/* 详细信息 */}
                  <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                    {task.source_width > 0 && (
                      <span>{task.source_width}×{task.source_height} · {task.source_codec}</span>
                    )}
                    {task.source_size > 0 && (
                      <span>{formatSize(task.source_size)}</span>
                    )}
                    {task.source_duration > 0 && (
                      <span>{formatDuration(task.source_duration)}</span>
                    )}
                    {task.speed_ratio > 0 && task.status === 'running' && (
                      <span className="text-neon-blue">{task.speed_ratio.toFixed(1)}x 速度</span>
                    )}
                    {task.elapsed_sec > 0 && (
                      <span>耗时 {formatDuration(task.elapsed_sec)}</span>
                    )}
                    {task.error && (
                      <span className="text-red-400">{task.error}</span>
                    )}
                  </div>
                </div>

                {/* 操作按钮 */}
                <div className="flex items-center gap-1 shrink-0">
                  {task.status === 'running' && (
                    <button onClick={() => handlePause(task.id)} className="p-1.5 rounded-lg hover:text-yellow-400 hover:bg-yellow-400/10 active:scale-90 transition-all" style={{ color: 'var(--text-muted)' }} title="暂停">
                      <Pause size={14} />
                    </button>
                  )}
                  {task.status === 'paused' && (
                    <button onClick={() => handleResume(task.id)} className="p-1.5 rounded-lg hover:text-emerald-400 hover:bg-emerald-400/10 active:scale-90 transition-all" style={{ color: 'var(--text-muted)' }} title="恢复">
                      <Play size={14} />
                    </button>
                  )}
                  {(task.status === 'running' || task.status === 'paused' || task.status === 'pending' || task.status === 'queued') && (
                    <button onClick={() => handleCancel(task.id)} className="p-1.5 rounded-lg hover:text-red-400 hover:bg-red-400/10 active:scale-90 transition-all" style={{ color: 'var(--text-muted)' }} title="取消">
                      <XCircle size={14} />
                    </button>
                  )}
                  {task.status === 'failed' && (
                    <button onClick={() => handleRetry(task.id)} className="p-1.5 rounded-lg hover:text-neon-blue hover:bg-neon-blue/10 active:scale-90 transition-all" style={{ color: 'var(--text-muted)' }} title="重试">
                      <RotateCcw size={14} />
                    </button>
                  )}
                  {(task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled') && (
                    <button onClick={() => handleDelete(task.id)} className="p-1.5 rounded-lg hover:text-red-400 hover:bg-red-400/10 active:scale-90 transition-all" style={{ color: 'var(--text-muted)' }} title="删除">
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>
              </div>
            </motion.div>
          ))
        )}
      </motion.div>

      {/* 分页 */}
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

      {/* ====== 转码任务 Tab：迁自系统管理 ====== */}
      {mainTab === 'transcode' && (
        <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
          <TranscodeJobsPanel />
        </div>
      )}

      {/* 预处理产物存储占用 - 详情弹窗 */}
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
            {/* 遮罩 */}
            <div className="absolute inset-0" style={{ background: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(4px)' }} />
            {/* 弹窗主体 */}
            <motion.div
              className="relative w-full max-w-3xl rounded-2xl overflow-hidden"
              style={{
                background: 'var(--glass-bg)',
                border: '1px solid var(--storage-enable-row-border, var(--border-strong))',
                boxShadow: '0 20px 48px rgba(0,0,0,0.45)',
                maxHeight: '85vh',
              }}
              initial={{ opacity: 0, scale: 0.96, y: 12 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.96, y: 12 }}
              transition={{ duration: durations.normal, ease: easeSmooth }}
              onClick={(e) => e.stopPropagation()}
            >
              {/* 顶部高光 */}
              <div className="absolute top-0 left-0 right-0 h-[1px] opacity-70" style={{ background: 'linear-gradient(90deg, transparent, #f59e0b, transparent)' }} />

              {/* Header */}
              <div className="flex items-start justify-between gap-4 px-5 py-4" style={{ borderBottom: '1px solid var(--storage-enable-row-border, var(--border-strong))' }}>
                <div className="flex items-center gap-2">
                  <Database size={18} className="text-amber-400" />
                  <div>
                    <div className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
                      缓存占用总览
                    </div>
                    {cacheUsage ? (
                      <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
                        根目录: <span className="font-mono">{cacheUsage.root_dir}</span>
                        {' · '}扫描耗时 {cacheUsage.scan_duration_ms} ms
                        {cacheUsage.from_cache && <span className="ml-1 text-amber-400" title="数据来自后端 30s 内存缓存，点击刷新可强制重扫">（缓存）</span>}
                      </div>
                    ) : (
                      <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
                        {(storageLoading || cacheLoading) ? '正在扫描 cache 根目录...' : '点击右上角"重新扫描"获取最新数据'}
                      </div>
                    )}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={handleCleanAllCache}
                    disabled={!cacheUsage || cacheUsage.total_size === 0 || !!cleaningCategory}
                    className="flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-medium transition-colors disabled:opacity-40"
                    style={{
                      background: 'rgba(239, 68, 68, 0.10)',
                      border: '1px solid rgba(239, 68, 68, 0.28)',
                      color: '#ef4444',
                    }}
                    title="一键清空所有可清理分类（转码 / ABR / 缩略图 / WebDAV 临时下载，并清理预处理孤儿）"
                  >
                    {cleaningCategory === '__all__' ? <Loader2 size={12} className="animate-spin" /> : <Eraser size={12} />}
                    一键清空
                  </button>
                  <button
                    onClick={() => { loadStorage(0); loadCacheUsage(true) }}
                    disabled={storageLoading || cacheLoading}
                    className="flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs transition-colors disabled:opacity-50"
                    style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}
                    title="重新扫描（强制刷新整个 cache 目录）"
                  >
                    {(storageLoading || cacheLoading) ? <Loader2 size={12} className="animate-spin" /> : <RefreshCw size={12} />}
                    重新扫描
                  </button>
                  <button
                    onClick={() => setStorageOpen(false)}
                    className="rounded-lg p-1.5 transition-colors"
                    style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}
                    aria-label="关闭"
                  >
                    <X size={14} />
                  </button>
                </div>
              </div>

              {/* 缓存总览 - 分类列表（preprocess + transcode + thumbnails + ...） */}
              {cacheUsage && cacheUsage.categories.length > 0 && (
                <div className="px-5 py-3" style={{ borderBottom: '1px solid var(--storage-enable-row-border, var(--border-strong))' }}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                      按分类汇总
                    </div>
                    <div className="text-xs" style={{ color: 'var(--text-muted)' }}>
                      合计 <span className="font-semibold" style={{ color: 'var(--text-primary)' }}>{formatBytes(cacheUsage.total_size)}</span> · {cacheUsage.total_count.toLocaleString()} 文件
                    </div>
                  </div>
                  <div className="space-y-1">
                    {cacheUsage.categories.map((cat) => {
                      const isPreprocess = cat.key === 'preprocess'
                      const isExpanded = expandedCategoryKey === cat.key
                      const pct = cacheUsage.total_size > 0 ? (cat.size / cacheUsage.total_size) * 100 : 0
                      const isThisCleaning = cleaningCategory === cat.key
                      const anyCleaning = !!cleaningCategory
                      const canClick = !anyCleaning
                      return (
                        <div
                          key={cat.key}
                          role="button"
                          tabIndex={canClick ? 0 : -1}
                          onClick={() => {
                            if (!canClick) return
                            setExpandedCategoryKey(isExpanded && !isPreprocess ? '' : cat.key)
                          }}
                          onKeyDown={(e) => {
                            if (!canClick) return
                            if (e.key === 'Enter' || e.key === ' ') {
                              e.preventDefault()
                              setExpandedCategoryKey(isExpanded && !isPreprocess ? '' : cat.key)
                            }
                          }}
                          className="w-full text-left rounded-lg px-3 py-2 transition-colors cursor-pointer"
                          style={{
                            background: isExpanded ? 'var(--neon-blue-6)' : 'var(--storage-enable-row-bg, var(--nav-hover-bg))',
                            border: isExpanded ? '1px solid rgba(96,165,250,0.35)' : '1px solid transparent',
                            opacity: anyCleaning && !isThisCleaning ? 0.6 : 1,
                          }}
                          title={isPreprocess ? '展开下方查看预处理产物明细' : cat.path}
                        >
                          <div className="flex items-center gap-3">
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2">
                                <span className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                                  {cat.label}
                                </span>
                                {isPreprocess && (
                                  <span className="text-[10px] px-1.5 py-0.5 rounded" style={{ background: 'rgba(96,165,250,0.18)', color: '#93c5fd' }}>
                                    可钻取
                                  </span>
                                )}
                                {cat.cleanable && !isPreprocess && (
                                  <span className="text-[10px] px-1.5 py-0.5 rounded" style={{ background: 'rgba(16,185,129,0.15)', color: '#6ee7b7' }} title="此目录可安全清空，删除后系统会按需重新生成">
                                    可清理
                                  </span>
                                )}
                              </div>
                              <div className="text-[11px] truncate font-mono mt-0.5" style={{ color: 'var(--text-muted)' }}>
                                {cat.path}
                              </div>
                              {/* 占比进度条 */}
                              <div className="mt-1.5 h-1 w-full rounded-full overflow-hidden" style={{ background: 'var(--progress-track-bg)' }}>
                                <div
                                  className="h-full rounded-full transition-all duration-500"
                                  style={{
                                    width: `${Math.min(100, pct)}%`,
                                    background: isPreprocess ? '#f59e0b' : cat.cleanable ? '#10b981' : '#60a5fa',
                                  }}
                                />
                              </div>
                            </div>
                            <div className="text-right flex-shrink-0">
                              <div className="text-sm font-semibold tabular-nums" style={{ color: 'var(--text-primary)' }}>
                                {formatBytes(cat.size)}
                              </div>
                              <div className="text-[11px] tabular-nums" style={{ color: 'var(--text-tertiary)' }}>
                                {cat.count.toLocaleString()} 文件 · {pct.toFixed(1)}%
                              </div>
                            </div>
                            {/* 手动清理按钮：仅对 cleanable 分类显示 */}
                            {cat.cleanable && cat.size > 0 && (
                              <button
                                type="button"
                                onClick={(e) => {
                                  e.stopPropagation()
                                  handleCleanCategory(cat.key, cat.label, cat.size)
                                }}
                                disabled={anyCleaning}
                                className="flex items-center gap-1 rounded-lg px-2 py-1 text-[11px] transition-colors disabled:opacity-50 flex-shrink-0"
                                style={{
                                  background: 'rgba(239, 68, 68, 0.10)',
                                  border: '1px solid rgba(239, 68, 68, 0.25)',
                                  color: '#ef4444',
                                }}
                                title={isPreprocess ? `清空所有预处理产物（${formatBytes(cat.size)}），运行中任务受保护` : `清空「${cat.label}」目录（${formatBytes(cat.size)}）`}
                              >
                                {isThisCleaning ? <Loader2 size={11} className="animate-spin" /> : <Trash2 size={11} />}
                                清理
                              </button>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                  <div className="mt-2 text-[11px]" style={{ color: 'var(--text-muted)' }}>
                    提示：仅"可清理"分类（绿色标签）支持清空，删除后系统会在需要时自动重新生成；海报 / AI 字幕 / 离线下载等不在自动重建范围内，因此不提供清理按钮。预处理产物的「清理」按钮会清空所有产物并把对应任务重置为待处理状态，正在运行的任务不会被中断；如仅想清理孤儿目录（数据库已无对应任务），请展开预处理产物后使用「一键清理孤儿」按钮。
                  </div>
                </div>
              )}

              {/* 预处理产物 - 三栏汇总（仅在分类钻取到 preprocess 时展示） */}
              {expandedCategoryKey === 'preprocess' && storage ? (
                <div className="grid grid-cols-3 gap-3 px-5 py-4" style={{ borderBottom: '1px solid var(--storage-enable-row-border, var(--border-strong))' }}>
                  <div className="rounded-lg px-3 py-2.5" style={{ background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))' }}>
                    <div className="text-[11px]" style={{ color: 'var(--text-muted)' }}>预处理总占用</div>
                    <div className="text-lg font-bold mt-0.5" style={{ color: 'var(--text-primary)' }}>
                      {formatBytes(storage.total_size)}
                    </div>
                    <div className="text-[11px] mt-0.5" style={{ color: 'var(--text-tertiary)' }}>
                      {storage.total_count} 个目录
                    </div>
                  </div>
                  <div className="rounded-lg px-3 py-2.5" style={{ background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))' }}>
                    <div className="text-[11px]" style={{ color: 'var(--text-muted)' }}>有效任务</div>
                    <div className="text-lg font-bold mt-0.5 text-emerald-400">
                      {formatBytes(storage.task_size)}
                    </div>
                    <div className="text-[11px] mt-0.5" style={{ color: 'var(--text-tertiary)' }}>
                      {storage.total_count - storage.orphan_count} 个目录
                    </div>
                  </div>
                  <div className="rounded-lg px-3 py-2.5" style={{ background: storage.orphan_count > 0 ? 'rgba(239,68,68,0.08)' : 'var(--storage-enable-row-bg, var(--nav-hover-bg))', border: storage.orphan_count > 0 ? '1px solid rgba(239,68,68,0.25)' : undefined }}>
                    <div className="text-[11px] flex items-center gap-1" style={{ color: 'var(--text-muted)' }}>
                      孤儿目录
                      {storage.orphan_count > 0 && <AlertCircle size={11} className="text-red-400" />}
                    </div>
                    <div className={clsx('text-lg font-bold mt-0.5', storage.orphan_count > 0 ? 'text-red-400' : '')} style={storage.orphan_count === 0 ? { color: 'var(--text-primary)' } : undefined}>
                      {formatBytes(storage.orphan_size)}
                    </div>
                    <div className="text-[11px] mt-0.5" style={{ color: 'var(--text-tertiary)' }}>
                      {storage.orphan_count} 个无主目录
                    </div>
                  </div>
                </div>
              ) : expandedCategoryKey === 'preprocess' && !storage ? (
                <div className="grid grid-cols-3 gap-3 px-5 py-4" style={{ borderBottom: '1px solid var(--storage-enable-row-border, var(--border-strong))' }}>
                  {Array.from({ length: 3 }).map((_, i) => (
                    <div key={i} className="rounded-lg px-3 py-2.5" style={{ background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))' }}>
                      <div className="skeleton h-3 w-12 rounded" />
                      <div className="skeleton h-5 w-20 rounded mt-2" />
                      <div className="skeleton h-3 w-16 rounded mt-2" />
                    </div>
                  ))}
                </div>
              ) : null}

              {/* 一键清理孤儿目录 */}
              {expandedCategoryKey === 'preprocess' && storage && storage.orphan_count > 0 && (
                <div className="flex items-center justify-between gap-3 px-5 py-3" style={{ background: 'rgba(239,68,68,0.05)', borderBottom: '1px solid var(--storage-enable-row-border, var(--border-strong))' }}>
                  <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                    检测到 <span className="font-semibold text-red-400">{storage.orphan_count}</span> 个孤儿目录（数据库中已无对应任务记录），可一键清理释放 <span className="font-semibold text-red-400">{formatBytes(storage.orphan_size)}</span>。
                  </div>
                  <button
                    onClick={handleCleanOrphan}
                    disabled={cleaningOrphan}
                    className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50 flex-shrink-0"
                    style={{ background: 'rgba(239,68,68,0.18)', color: '#fca5a5', border: '1px solid rgba(239,68,68,0.35)' }}
                  >
                    {cleaningOrphan ? <Loader2 size={12} className="animate-spin" /> : <Eraser size={12} />}
                    一键清理孤儿
                  </button>
                </div>
              )}

              {/* 明细列表（仅在选中"预处理产物"分类时显示，按 size 降序） */}
              <div className="overflow-y-auto" style={{ maxHeight: 'calc(85vh - 320px)', minHeight: '120px' }}>
                {expandedCategoryKey !== 'preprocess' ? (
                  <div className="py-12 px-5 text-center text-sm flex flex-col items-center gap-2" style={{ color: 'var(--text-tertiary)' }}>
                    <Database size={20} className="opacity-40" />
                    <div>该分类暂未提供明细钻取</div>
                    <div className="text-xs" style={{ color: 'var(--text-muted)' }}>
                      仅"预处理产物"支持按媒体钻取与清理；其他分类暂时只展示总量。
                    </div>
                  </div>
                ) : storageLoading && !storage ? (
                  <div className="py-16 text-center text-sm flex flex-col items-center gap-2" style={{ color: 'var(--text-tertiary)' }}>
                    <Loader2 size={20} className="animate-spin text-neon-blue" />
                    正在扫描预处理目录...
                  </div>
                ) : !storage ? (
                  <div className="py-16 text-center text-sm flex flex-col items-center gap-3" style={{ color: 'var(--text-tertiary)' }}>
                    <AlertCircle size={20} className="text-amber-400" />
                    <div>未能获取存储占用数据</div>
                    <button
                      onClick={() => loadStorage(0)}
                      className="flex items-center gap-1 rounded-lg px-3 py-1.5 text-xs transition-colors"
                      style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}
                    >
                      <RefreshCw size={12} />
                      重新扫描
                    </button>
                  </div>
                ) : storage.items.length === 0 ? (
                  <div className="py-16 text-center text-sm flex flex-col items-center gap-2" style={{ color: 'var(--text-tertiary)' }}>
                    <Database size={20} className="opacity-40" />
                    暂无预处理产物
                  </div>
                ) : (
                  <ul className="divide-y" style={{ borderColor: 'var(--storage-enable-row-border, var(--border-strong))' }}>
                    {storage.items.map((item) => (
                      <li
                        key={item.output_dir}
                        className="flex items-center gap-3 px-5 py-3 transition-colors"
                        style={{ background: item.is_orphan ? 'rgba(239,68,68,0.04)' : undefined }}
                      >
                        <Film size={14} className="flex-shrink-0" style={{ color: item.is_orphan ? '#ef4444' : 'var(--neon-blue)' }} />
                        <div className="flex-1 min-w-0">
                          <div className="text-sm truncate" style={{ color: 'var(--text-primary)' }}>
                            {item.media_title || item.media_id}
                            {item.is_orphan && (
                              <span className="ml-2 inline-block rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'rgba(239,68,68,0.18)', color: '#fca5a5' }}>
                                孤儿
                              </span>
                            )}
                            {!item.is_orphan && item.status && (
                              <span className={clsx('ml-2 text-[10px]', statusColors[item.status] || 'text-surface-500')}>
                                {statusLabels[item.status] || item.status}
                              </span>
                            )}
                          </div>
                          <div className="text-[11px] truncate font-mono mt-0.5" style={{ color: 'var(--text-muted)' }}>
                            {item.output_dir}
                          </div>
                        </div>
                        <div className="text-sm font-semibold tabular-nums flex-shrink-0" style={{ color: 'var(--text-primary)' }}>
                          {formatBytes(item.size)}
                        </div>
                        <button
                          onClick={() => handleCleanOne(item.media_id, item.media_title)}
                          className="rounded-lg p-1.5 transition-colors flex-shrink-0"
                          style={{ background: 'var(--neon-blue-6)', color: 'var(--text-tertiary)' }}
                          title="清理此预处理缓存"
                        >
                          <Trash2 size={12} />
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* 自定义筛选预处理 - 弹窗 */}
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
            <div className="absolute inset-0" style={{ background: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(4px)' }} />
            <motion.div
              className="relative w-full max-w-4xl rounded-2xl overflow-hidden flex flex-col"
              style={{
                background: 'var(--glass-bg)',
                border: '1px solid var(--storage-enable-row-border, var(--border-strong))',
                boxShadow: '0 20px 48px rgba(0,0,0,0.45)',
                maxHeight: '90vh',
              }}
              initial={{ opacity: 0, scale: 0.96, y: 12 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.96, y: 12 }}
              transition={{ duration: durations.normal, ease: easeSmooth }}
              onClick={(e) => e.stopPropagation()}
            >
              <div className="absolute top-0 left-0 right-0 h-[1px] opacity-70" style={{ background: 'linear-gradient(90deg, transparent, #f59e0b, transparent)' }} />

              {/* Header */}
              <div className="flex items-start justify-between gap-4 px-5 py-4 flex-shrink-0" style={{ borderBottom: '1px solid var(--storage-enable-row-border, var(--border-strong))' }}>
                <div className="flex items-center gap-2">
                  <Filter size={18} className="text-amber-400" />
                  <div>
                    <div className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
                      自定义筛选预处理
                    </div>
                    <div className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
                      多条件组合，先预览命中数量，确认后批量提交
                    </div>
                  </div>
                </div>
                <button
                  onClick={() => setFilterOpen(false)}
                  className="rounded-lg p-1.5 transition-colors"
                  style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}
                  aria-label="关闭"
                >
                  <X size={14} />
                </button>
              </div>

              {/* 主体（可滚动） */}
              <div className="overflow-y-auto px-5 py-4 space-y-5" style={{ flex: 1 }}>
                {/* 媒体库 */}
                {libraries.length > 0 && (
                  <FilterSection title="媒体库" hint="不选 = 全部">
                    {libraries.map((lib) => {
                      const checked = filter.library_ids?.includes(lib.id) ?? false
                      return (
                        <FilterChip
                          key={lib.id}
                          label={`${lib.name} (${lib.type})`}
                          active={checked}
                          onClick={() => toggleArrayFilter('library_ids', lib.id)}
                        />
                      )
                    })}
                  </FilterSection>
                )}

                {/* 媒体类型 */}
                <FilterSection title="媒体类型" hint="不选 = 全部">
                  {[{ k: 'movie', l: '电影' }, { k: 'episode', l: '剧集' }].map((it) => (
                    <FilterChip
                      key={it.k}
                      label={it.l}
                      active={filter.media_types?.includes(it.k) ?? false}
                      onClick={() => toggleArrayFilter('media_types', it.k)}
                    />
                  ))}
                </FilterSection>

                {/* 视频编码 */}
                <FilterSection title="视频编码" hint="只选不可零转码直接播放的编码可以高效筛出待预处理目标">
                  {['h264', 'hevc', 'av1', 'vp9', 'mpeg4', 'wmv3'].map((c) => (
                    <FilterChip
                      key={c}
                      label={c.toUpperCase()}
                      active={filter.video_codecs?.includes(c) ?? false}
                      onClick={() => toggleArrayFilter('video_codecs', c)}
                    />
                  ))}
                </FilterSection>

                {/* 容器格式 */}
                <FilterSection title="容器格式" hint="按文件扩展名匹配">
                  {['mkv', 'mp4', 'avi', 'mov', 'ts', 'flv', 'webm', 'wmv', 'rmvb'].map((c) => (
                    <FilterChip
                      key={c}
                      label={`.${c}`}
                      active={filter.containers?.includes(c) ?? false}
                      onClick={() => toggleArrayFilter('containers', c)}
                    />
                  ))}
                </FilterSection>

                {/* 分辨率 */}
                <FilterSection title="分辨率">
                  {['480p', '720p', '1080p', '2K', '4K'].map((c) => (
                    <FilterChip
                      key={c}
                      label={c}
                      active={filter.resolutions?.includes(c) ?? false}
                      onClick={() => toggleArrayFilter('resolutions', c)}
                    />
                  ))}
                </FilterSection>

                {/* 数值区间 */}
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                  <RangeInput
                    label="文件大小（GB）"
                    minValue={filter.min_size_bytes ? filter.min_size_bytes / (1024 ** 3) : undefined}
                    maxValue={filter.max_size_bytes ? filter.max_size_bytes / (1024 ** 3) : undefined}
                    onChange={(min, max) => {
                      setFilter((p) => ({
                        ...p,
                        min_size_bytes: min ? Math.round(min * 1024 ** 3) : 0,
                        max_size_bytes: max ? Math.round(max * 1024 ** 3) : 0,
                      }))
                      setFilterPreview(null)
                    }}
                    step={0.5}
                  />
                  <RangeInput
                    label="年份"
                    minValue={filter.min_year || undefined}
                    maxValue={filter.max_year || undefined}
                    onChange={(min, max) => {
                      setFilter((p) => ({ ...p, min_year: min ?? 0, max_year: max ?? 0 }))
                      setFilterPreview(null)
                    }}
                    step={1}
                  />
                  <RangeInput
                    label="时长（分钟）"
                    minValue={filter.min_duration ? Math.round(filter.min_duration / 60) : undefined}
                    maxValue={filter.max_duration ? Math.round(filter.max_duration / 60) : undefined}
                    onChange={(min, max) => {
                      setFilter((p) => ({
                        ...p,
                        min_duration: min ? min * 60 : 0,
                        max_duration: max ? max * 60 : 0,
                      }))
                      setFilterPreview(null)
                    }}
                    step={1}
                  />
                </div>

                {/* 关键词 */}
                <div>
                  <div className="text-xs mb-1.5" style={{ color: 'var(--text-secondary)' }}>关键词（标题/原标题/番号）</div>
                  <input
                    type="text"
                    value={filter.keyword ?? ''}
                    onChange={(e) => {
                      setFilter((p) => ({ ...p, keyword: e.target.value }))
                      setFilterPreview(null)
                    }}
                    placeholder="留空则不限制"
                    className="w-full rounded-lg px-3 py-2 text-sm focus:outline-none"
                    style={{
                      background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))',
                      border: '1px solid var(--storage-enable-row-border, var(--border-strong))',
                      color: 'var(--text-primary)',
                    }}
                  />
                </div>

                {/* 排除策略 */}
                <FilterSection title="排除策略" hint="未勾选 = 不排除该类媒体（不推荐关闭）">
                  <ExcludeToggle
                    label="排除已有预处理任务的媒体"
                    checked={filter.exclude_already_preprocessed !== false}
                    onChange={(v) => {
                      setFilter((p) => ({ ...p, exclude_already_preprocessed: v }))
                      setFilterPreview(null)
                    }}
                  />
                  <ExcludeToggle
                    label="排除浏览器可零转码直接播放的"
                    checked={filter.exclude_directly_playable !== false}
                    onChange={(v) => {
                      setFilter((p) => ({ ...p, exclude_directly_playable: v }))
                      setFilterPreview(null)
                    }}
                  />
                  <ExcludeToggle
                    label="排除 STRM 远程流"
                    checked={filter.exclude_strm !== false}
                    onChange={(v) => {
                      setFilter((p) => ({ ...p, exclude_strm: v }))
                      setFilterPreview(null)
                    }}
                  />
                </FilterSection>

                {/* 预览结果 */}
                {filterPreview && (
                  <div className="rounded-xl p-4 space-y-3" style={{ background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))', border: '1px solid var(--neon-blue-15)' }}>
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                      <PreviewStat label="命中" value={filterPreview.matched_count.toString()} accent="text-amber-400" />
                      <PreviewStat label="原始命中" value={filterPreview.raw_count.toString()} />
                      <PreviewStat label="总大小" value={formatBytes(filterPreview.total_size)} />
                      <PreviewStat
                        label="排除合计"
                        value={(filterPreview.excluded_already + filterPreview.excluded_playable + filterPreview.excluded_strm).toString()}
                      />
                    </div>
                    {(filterPreview.excluded_already + filterPreview.excluded_playable + filterPreview.excluded_strm) > 0 && (
                      <div className="text-[11px]" style={{ color: 'var(--text-muted)' }}>
                        已排除：已预处理 {filterPreview.excluded_already} · 可直接播放 {filterPreview.excluded_playable} · STRM {filterPreview.excluded_strm}
                      </div>
                    )}
                    {/* 编码分布 */}
                    {Object.keys(filterPreview.codec_histogram).length > 0 && (
                      <div>
                        <div className="text-xs mb-1.5" style={{ color: 'var(--text-secondary)' }}>编码分布</div>
                        <div className="flex flex-wrap gap-1.5">
                          {Object.entries(filterPreview.codec_histogram).map(([k, v]) => (
                            <span key={k} className="rounded px-2 py-0.5 text-[11px]" style={{ background: 'var(--neon-blue-6)', color: 'var(--text-secondary)' }}>
                              {k.toUpperCase()} <span className="font-semibold" style={{ color: 'var(--accent-amber-text)' }}>×{v}</span>
                            </span>
                          ))}
                        </div>
                      </div>
                    )}
                    {/* 抽样列表 */}
                    {filterPreview.sample.length > 0 && (
                      <div>
                        <div className="text-xs mb-1.5" style={{ color: 'var(--text-secondary)' }}>
                          抽样预览（前 {filterPreview.sample.length} 条）
                        </div>
                        <ul className="rounded-lg divide-y max-h-56 overflow-y-auto" style={{ background: 'var(--glass-bg)', borderColor: 'var(--storage-enable-row-border, var(--border-strong))' }}>
                          {filterPreview.sample.map((s) => (
                            <li key={s.media_id} className="flex items-center gap-2 px-3 py-2 text-xs" style={{ color: 'var(--text-secondary)' }}>
                              <Film size={12} className="flex-shrink-0 text-neon-blue" />
                              <span className="flex-1 truncate" style={{ color: 'var(--text-primary)' }}>
                                {s.title}{s.year ? ` (${s.year})` : ''}
                              </span>
                              <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>
                                {s.video_codec || '?'} · {s.resolution || '?'} · {formatBytes(s.file_size)}
                              </span>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                )}
              </div>

              {/* Footer */}
              <div className="flex items-center justify-between gap-3 px-5 py-3 flex-shrink-0" style={{ borderTop: '1px solid var(--storage-enable-row-border, var(--border-strong))', background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))' }}>
                <label className="flex items-center gap-2 text-xs cursor-pointer" style={{ color: 'var(--text-secondary)' }}>
                  <input
                    type="checkbox"
                    checked={filterForce}
                    onChange={(e) => setFilterForce(e.target.checked)}
                    className="rounded"
                  />
                  强制（绕过"可直接播放则跳过"判定）
                </label>
                <div className="flex items-center gap-2">
                  <button
                    onClick={handlePreviewFilter}
                    disabled={previewing}
                    className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50"
                    style={{ background: 'var(--neon-blue-6)', border: '1px solid var(--neon-blue-15)', color: 'var(--text-primary)' }}
                  >
                    {previewing ? <Loader2 size={12} className="animate-spin" /> : <Sparkles size={12} />}
                    预览
                  </button>
                  <button
                    onClick={handleSubmitFilter}
                    disabled={submittingFilter || !filterPreview || filterPreview.matched_count === 0}
                    className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50"
                    style={{ background: 'var(--accent-amber-bg)', border: '1px solid var(--accent-amber-border)', color: 'var(--accent-amber-text)' }}
                    title={!filterPreview ? '请先预览' : filterPreview.matched_count === 0 ? '当前条件无命中' : ''}
                  >
                    {submittingFilter ? <Loader2 size={12} className="animate-spin" /> : <Send size={12} />}
                    {filterPreview ? `提交 ${filterPreview.matched_count} 个` : '提交'}
                  </button>
                </div>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </motion.div>
  )
}

// ==================== 筛选弹窗的子组件 ====================

interface FilterSectionProps {
  title: string
  hint?: string
  children: React.ReactNode
}
function FilterSection({ title, hint, children }: FilterSectionProps) {
  return (
    <div>
      <div className="flex items-baseline gap-2 mb-1.5">
        <div className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>{title}</div>
        {hint && <div className="text-[11px]" style={{ color: 'var(--text-muted)' }}>{hint}</div>}
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
    <button
      type="button"
      onClick={onClick}
      className="rounded-lg px-2.5 py-1 text-xs transition-colors"
      style={{
        background: active ? 'var(--accent-amber-bg)' : 'var(--chip-bg)',
        border: active ? '1px solid var(--accent-amber-border)' : '1px solid var(--chip-border)',
        color: active ? 'var(--accent-amber-text)' : 'var(--chip-text)',
        fontWeight: active ? 600 : 400,
      }}
    >
      {label}
    </button>
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
  const inputCls = 'w-full rounded-lg px-2 py-1.5 text-xs focus:outline-none tabular-nums'
  const inputStyle: React.CSSProperties = {
    background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))',
    border: '1px solid var(--storage-enable-row-border, var(--border-strong))',
    color: 'var(--text-primary)',
  }
  return (
    <div>
      <div className="text-xs mb-1.5" style={{ color: 'var(--text-secondary)' }}>{label}</div>
      <div className="flex items-center gap-1.5">
        <input
          type="number"
          step={step}
          min={0}
          value={minValue ?? ''}
          onChange={(e) => {
            const v = e.target.value === '' ? undefined : Number(e.target.value)
            onChange(v, maxValue)
          }}
          placeholder="最小"
          className={inputCls}
          style={inputStyle}
        />
        <span className="text-xs" style={{ color: 'var(--text-muted)' }}>~</span>
        <input
          type="number"
          step={step}
          min={0}
          value={maxValue ?? ''}
          onChange={(e) => {
            const v = e.target.value === '' ? undefined : Number(e.target.value)
            onChange(minValue, v)
          }}
          placeholder="最大"
          className={inputCls}
          style={inputStyle}
        />
      </div>
    </div>
  )
}

interface ExcludeToggleProps {
  label: string
  checked: boolean
  onChange: (v: boolean) => void
}
function ExcludeToggle({ label, checked, onChange }: ExcludeToggleProps) {
  return (
    <label className="flex items-center gap-2 text-xs cursor-pointer rounded-lg px-2.5 py-1.5"
      style={{
        background: checked ? 'var(--accent-emerald-bg)' : 'var(--chip-bg)',
        border: '1px solid ' + (checked ? 'var(--accent-emerald-border)' : 'var(--chip-border)'),
        color: checked ? 'var(--accent-emerald-text)' : 'var(--text-secondary)',
        fontWeight: checked ? 600 : 400,
      }}
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="rounded"
      />
      {label}
    </label>
  )
}

interface PreviewStatProps {
  label: string
  value: string
  accent?: string
}
function PreviewStat({ label, value, accent }: PreviewStatProps) {
  // accent 当作"是否高亮"开关；具体颜色统一走 CSS 变量，保证日/夜间均有足够对比度
  return (
    <div>
      <div className="text-[11px]" style={{ color: 'var(--text-muted)' }}>{label}</div>
      <div
        className="text-lg font-bold mt-0.5"
        style={{ color: accent ? 'var(--accent-amber-text)' : 'var(--text-primary)' }}
      >
        {value}
      </div>
    </div>
  )
}


