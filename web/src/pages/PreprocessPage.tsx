import { useState, useEffect, useCallback, useRef, useMemo, useLayoutEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { pageVariants, staggerContainerVariants, staggerItemVariants, modalOverlayVariants, modalContentVariants, easeSmooth, durations } from '@/lib/motion'

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
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import type { PreprocessTask, PreprocessStatistics, SystemLoadInfo, Library } from '@/types'
import api from '@/api/client'
import {
  Play,
  Pause,
  RotateCcw,
  Trash2,
  X,
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
  const toastRef = useRef(toast)
  toastRef.current = toast
  const { on, off } = useWebSocket()
  const [tasks, setTasks] = useState<PreprocessTask[]>([])
  const [total, setTotal] = useState(0)
  const { page, size: pageSize, setPage, setSize, totalPages: calcTotalPages } = usePagination({ initialSize: 20 })
  const [statusFilter, setStatusFilter] = useState('')
  const [stats, setStats] = useState<PreprocessStatistics | null>(null)
  const [sysLoad, setSysLoad] = useState<SystemLoadInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [submitting, setSubmitting] = useState<string | null>(null)
  // 批量选择
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchLoading, setBatchLoading] = useState(false)

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

  // 加载媒体库列表
  const loadLibraries = useCallback(async () => {
    try {
      const res = await api.get<{ data: Library[] }>('/libraries')
      setLibraries(res.data.data || [])
    } catch {
      // 忽略
    }
  }, [])

  // 初始加载
  useEffect(() => {
    setLoading(true)
    const promises: Promise<void>[] = [loadTasks(), loadStats(), loadLibraries()]
    Promise.all(promises).finally(() => setLoading(false))
  }, [loadTasks, loadStats, loadLibraries])

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

    on(WS_EVENTS.PREPROCESS_PROGRESS, scheduleRefresh)
    on(WS_EVENTS.PREPROCESS_COMPLETED, scheduleRefresh)
    on(WS_EVENTS.PREPROCESS_FAILED, scheduleRefresh)
    on(WS_EVENTS.PREPROCESS_STARTED, scheduleRefresh)
    return () => {
      off(WS_EVENTS.PREPROCESS_PROGRESS, scheduleRefresh)
      off(WS_EVENTS.PREPROCESS_COMPLETED, scheduleRefresh)
      off(WS_EVENTS.PREPROCESS_FAILED, scheduleRefresh)
      off(WS_EVENTS.PREPROCESS_STARTED, scheduleRefresh)
      if (refreshTimer) clearTimeout(refreshTimer)
    }
  }, [on, off, loadTasks, loadStats])

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
            onClick={() => { loadTasks(); loadStats() }}
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
          className="grid grid-cols-2 gap-4 sm:grid-cols-4"
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
        </motion.div>
      )}

      {/* 媒体库批量预处理 */}
      {libraries.length > 0 && (
        <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
          <h2 className="text-sm font-medium mb-3 flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
            <FolderOpen size={16} className="text-neon-blue" />
            媒体库批量预处理
          </h2>
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
    </motion.div>
  )
}

