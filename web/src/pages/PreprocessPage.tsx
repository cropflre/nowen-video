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
import type { PreprocessTask, PreprocessStatistics, SystemLoadInfo, Library, PerformanceConfig } from '@/types'
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
  ChevronLeft,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Settings,
  Save,
  Info,
  Gauge,
  Layers,
  MonitorSpeaker,
  Thermometer,
  Shield,
  ShieldAlert,
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
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [statusFilter, setStatusFilter] = useState('')
  const [stats, setStats] = useState<PreprocessStatistics | null>(null)
  const [sysLoad, setSysLoad] = useState<SystemLoadInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [submitting, setSubmitting] = useState<string | null>(null)
  // 批量选择
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [batchLoading, setBatchLoading] = useState(false)
  // 性能配置
  const [showPerfConfig, setShowPerfConfig] = useState(false)
  const [perfConfig, setPerfConfig] = useState<PerformanceConfig | null>(null)
  const [perfDraft, setPerfDraft] = useState<Partial<PerformanceConfig>>({})
  const [perfSaving, setPerfSaving] = useState(false)
  const [perfLoading, setPerfLoading] = useState(false)
  const [perfLoadError, setPerfLoadError] = useState(false)
  const perfConfigLoaded = useRef(false)

  // 计算总页数
  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / pageSize)), [total, pageSize])

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

  // 加载性能配置
  const loadPerfConfig = useCallback(async () => {
    setPerfLoading(true)
    setPerfLoadError(false)
    try {
      const res = await preprocessApi.getPerformanceConfig()
      setPerfConfig(res.data.data)
      setPerfDraft({})
      perfConfigLoaded.current = true
    } catch {
      setPerfLoadError(true)
      toastRef.current.error('加载性能配置失败')
    } finally {
      setPerfLoading(false)
    }
  }, [])

  // 保存性能配置
  const savePerfConfig = async () => {
    if (Object.keys(perfDraft).length === 0) return
    setPerfSaving(true)
    try {
      const res = await preprocessApi.updatePerformanceConfig(perfDraft)
      setPerfConfig(res.data.data)
      setPerfDraft({})
      toastRef.current.success('性能配置已保存（部分配置需重启服务生效）')
      // 同步刷新系统负载信息，确保统计卡片中的限制值等实时更新
      await loadStats()
    } catch {
      toastRef.current.error('保存性能配置失败')
    } finally {
      setPerfSaving(false)
    }
  }

  // 初始加载
  useEffect(() => {
    setLoading(true)
    const promises: Promise<void>[] = [loadTasks(), loadStats(), loadLibraries()]
    if (!perfConfigLoaded.current) {
      promises.push(loadPerfConfig())
    }
    Promise.all(promises).finally(() => setLoading(false))
  }, [loadTasks, loadStats, loadLibraries, loadPerfConfig])

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
          {/* 性能配置按钮 */}
          <button
            onClick={() => {
              setShowPerfConfig(true)
              if (!perfConfig && !perfLoading) loadPerfConfig()
            }}
            className="flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm transition-all duration-200 shrink-0 active:scale-95"
            style={{
              background: 'var(--neon-blue-6)',
              border: '1px solid var(--neon-blue-6)',
              color: 'var(--text-secondary)',
            }}
          >
            <Settings size={14} />
            性能配置
          </button>
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

      {/* 性能配置弹窗 */}
      <AnimatePresence>
      {showPerfConfig && (
        <PerfConfigModal
          perfLoading={perfLoading}
          perfLoadError={perfLoadError}
          perfConfig={perfConfig}
          perfDraft={perfDraft}
          perfSaving={perfSaving}
          sysLoad={sysLoad}
          onClose={() => setShowPerfConfig(false)}
          onRetry={loadPerfConfig}
          onDraftChange={setPerfDraft}
          onSave={savePerfConfig}
        />
      )}
      </AnimatePresence>

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
              {sysLoad.cpu_count} CPU · 限制 {sysLoad.resource_limit || 5}%
            </div>
            {/* CPU 使用率进度条（颜色基于资源限制阈值） */}
            {sysLoad.cpu_percent != null && (
              <div className="mt-2 h-1 w-full rounded-full" style={{ background: 'var(--progress-track-bg)' }}>
                <div
                  className="h-full rounded-full transition-all duration-500"
                  style={{
                    width: `${Math.min(100, sysLoad.cpu_percent)}%`,
                    background: sysLoad.cpu_percent > (sysLoad.resource_limit || 5) ? '#ef4444'
                      : sysLoad.cpu_percent > (sysLoad.resource_limit || 5) * 0.8 ? '#f59e0b'
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
    </motion.div>
  )
}

// ==================== 性能配置弹窗组件 ====================
interface PerfConfigModalProps {
  perfLoading: boolean
  perfLoadError: boolean
  perfConfig: PerformanceConfig | null
  perfDraft: Partial<PerformanceConfig>
  perfSaving: boolean
  sysLoad: SystemLoadInfo | null
  onClose: () => void
  onRetry: () => void
  onDraftChange: (draft: Partial<PerformanceConfig>) => void
  onSave: () => void
}

function PerfConfigModal({
  perfLoading,
  perfLoadError,
  perfConfig,
  perfDraft,
  perfSaving,
  sysLoad,
  onClose,
  onRetry,
  onDraftChange,
  onSave,
}: PerfConfigModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null)

  // ESC 关闭
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKey)
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', handleKey)
      document.body.style.overflow = ''
    }
  }, [onClose])

  // 点击遮罩关闭
  const handleOverlayClick = (e: React.MouseEvent) => {
    if (e.target === overlayRef.current) onClose()
  }

  // 弹窗内容
  const renderContent = () => {
    if (perfLoading) {
      return (
        <div className="flex items-center justify-center py-16">
          <Loader2 size={24} className="animate-spin text-neon-blue" />
          <span className="ml-3 text-sm" style={{ color: 'var(--text-secondary)' }}>加载性能配置...</span>
        </div>
      )
    }

    if (perfLoadError && !perfConfig) {
      return (
        <div className="flex flex-col items-center justify-center py-16 gap-4">
          <AlertCircle size={32} className="text-red-400 opacity-60" />
          <span className="text-sm" style={{ color: 'var(--text-tertiary)' }}>加载性能配置失败</span>
          <button
            onClick={onRetry}
            className="flex items-center gap-1.5 rounded-lg px-4 py-2 text-xs transition-all duration-200"
            style={{ background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)', color: 'var(--text-primary)' }}
          >
            <RefreshCw size={12} />
            重试
          </button>
        </div>
      )
    }

    if (!perfConfig) return null

    return (
      <div className="space-y-5">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
          {/* 资源使用率上限 */}
          <div className="space-y-2">
            <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <Gauge size={14} className="text-emerald-400" />
              CPU 资源使用率上限
            </label>
            <div className="flex items-center gap-3">
              {(() => {
                const val = perfDraft.resource_limit ?? perfConfig.resource_limit
                const pct = ((val - 5) / (80 - 5)) * 100
                return (
                  <input
                    type="range"
                    min={5}
                    max={80}
                    step={5}
                    value={val}
                    onChange={(e) => onDraftChange({ ...perfDraft, resource_limit: Number(e.target.value) })}
                    className="flex-1 h-1.5 rounded-full appearance-none cursor-pointer"
                    style={{
                      background: `linear-gradient(to right, #10b981 0%, #10b981 ${pct}%, var(--neon-blue-6) ${pct}%, var(--neon-blue-6) 100%)`,
                      accentColor: '#10b981',
                    }}
                  />
                )
              })()}
              <span className="text-sm font-mono font-bold min-w-[3rem] text-right" style={{ color: 'var(--text-primary)' }}>
                {perfDraft.resource_limit ?? perfConfig.resource_limit}%
              </span>
            </div>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              系统自动保留 20% 缓冲 · 当前 FFmpeg 线程数: {perfConfig.ffmpeg_threads} / {perfConfig.cpu_count} 核
            </p>
          </div>

          {/* 并行任务数 */}
          <div className="space-y-2">
            <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <Layers size={14} className="text-blue-400" />
              最大并行转码任务数
            </label>
            <div className="flex items-center gap-2">
              {[1, 2, 3, 4].map((n) => (
                <button
                  key={n}
                  onClick={() => onDraftChange({ ...perfDraft, max_transcode_jobs: n })}
                  className="flex-1 rounded-lg py-1.5 text-xs font-medium transition-colors"
                  style={
                    (perfDraft.max_transcode_jobs ?? perfConfig.max_transcode_jobs) === n
                      ? { background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)', color: 'var(--text-primary)' }
                      : { background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)', color: 'var(--text-muted)' }
                  }
                >
                  {n}
                </button>
              ))}
            </div>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              更多并行任务需要更多 CPU/GPU 资源 · 需重启生效
            </p>
          </div>

          {/* 转码预设 */}
          <div className="space-y-2">
            <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <Zap size={14} className="text-yellow-400" />
              转码质量预设
            </label>
            <div className="flex items-center gap-2 flex-wrap">
              {[
                { value: 'ultrafast', label: '极速', desc: '最快速度，较低质量' },
                { value: 'veryfast', label: '快速', desc: '平衡速度与质量' },
                { value: 'fast', label: '标准', desc: '较好质量' },
                { value: 'medium', label: '高质量', desc: '最佳质量，速度较慢' },
              ].map((preset) => (
                <button
                  key={preset.value}
                  onClick={() => onDraftChange({ ...perfDraft, transcode_preset: preset.value })}
                  className="flex-1 rounded-lg py-1.5 text-xs font-medium transition-colors min-w-[60px]"
                  style={
                    (perfDraft.transcode_preset ?? perfConfig.transcode_preset) === preset.value
                      ? { background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)', color: 'var(--text-primary)' }
                      : { background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)', color: 'var(--text-muted)' }
                  }
                  title={preset.desc}
                >
                  {preset.label}
                </button>
              ))}
            </div>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              当前: {perfDraft.transcode_preset ?? perfConfig.transcode_preset} · 立即生效
            </p>
          </div>

          {/* GPU 加速 */}
          <div className="space-y-2">
            <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <MonitorSpeaker size={14} className="text-purple-400" />
              GPU 硬件加速
            </label>
            <div className="flex items-center gap-2 flex-wrap">
              {[
                { value: 'auto', label: '自动检测' },
                { value: 'nvenc', label: 'NVIDIA' },
                { value: 'qsv', label: 'Intel QSV' },
                { value: 'vaapi', label: 'VAAPI' },
                { value: 'none', label: '仅 CPU' },
              ].map((accel) => (
                <button
                  key={accel.value}
                  onClick={() => onDraftChange({ ...perfDraft, hw_accel: accel.value })}
                  className="flex-1 rounded-lg py-1.5 text-xs font-medium transition-colors min-w-[60px]"
                  style={
                    (perfDraft.hw_accel ?? perfConfig.hw_accel) === accel.value
                      ? { background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)', color: 'var(--text-primary)' }
                      : { background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)', color: 'var(--text-muted)' }
                  }
                >
                  {accel.label}
                </button>
              ))}
            </div>
            <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
              当前配置: {perfDraft.hw_accel ?? perfConfig.hw_accel}
              {perfConfig.detected_hw_accel && (
                <span style={{ color: perfConfig.detected_hw_accel !== 'none' ? '#16A34A' : '#CA8A04' }}>
                  {' · '}实际检测: {perfConfig.detected_hw_accel === 'nvenc' ? 'NVIDIA NVENC' : perfConfig.detected_hw_accel === 'qsv' ? 'Intel QSV' : perfConfig.detected_hw_accel === 'vaapi' ? 'VAAPI' : perfConfig.detected_hw_accel === 'none' ? '未检测到 GPU' : perfConfig.detected_hw_accel}
                </span>
              )}
              {' · '}需重启生效
            </p>
          </div>
        </div>

        {/* 性能建议 */}
        {sysLoad?.suggestions && sysLoad.suggestions.length > 0 && (
          <div className="rounded-lg p-3 space-y-1.5" style={{ background: 'var(--neon-blue-6)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-1.5 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              <Info size={12} className="text-neon-blue" />
              性能优化建议
            </div>
            {sysLoad.suggestions.map((s, i) => (
              <p key={i} className="text-xs pl-5" style={{ color: 'var(--text-muted)' }}>• {s}</p>
            ))}
          </div>
        )}

        {/* GPU 安全保护配置 */}
        {perfConfig.detected_hw_accel && perfConfig.detected_hw_accel !== 'none' && (
          <GPUSafetyPanel
            perfConfig={perfConfig}
            perfDraft={perfDraft}
            onDraftChange={onDraftChange}
          />
        )}
      </div>
    )
  }

  return (
    <motion.div
      ref={overlayRef}
      className="modal-overlay flex items-start justify-center pt-4"
      onClick={handleOverlayClick}
      variants={modalOverlayVariants}
      initial="hidden"
      animate="visible"
      exit="exit"
    >
      <motion.div
        className="relative w-full max-w-2xl mx-4 rounded-2xl overflow-hidden"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-strong)',
          boxShadow: 'var(--shadow-elevated), var(--modal-panel-glow)',
          backdropFilter: 'blur(30px)',
          maxHeight: '85vh',
        }}
        variants={modalContentVariants}
        initial="hidden"
        animate="visible"
        exit="exit"
      >
        {/* 顶部霓虹光条 */}
        <div
          className="absolute top-0 left-0 right-0 h-[2px] z-10"
          style={{
            background: 'linear-gradient(90deg, transparent, var(--neon-blue), var(--neon-purple), transparent)',
          }}
        />

        {/* 可滚动内容容器 */}
        <div className="overflow-y-auto" style={{ maxHeight: '85vh' }}>
          {/* 头部 */}
          <div
            className="flex items-center justify-between px-6 pt-6 pb-4 sticky top-0 z-10"
            style={{ background: 'var(--bg-elevated)' }}
          >
            <h2 className="text-base font-semibold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
              <Settings size={18} className="text-neon-blue" />
              性能参数配置
            </h2>
            <div className="flex items-center gap-3">
              {Object.keys(perfDraft).length > 0 && (
                <span className="text-xs px-2.5 py-1 rounded-full" style={{ background: 'var(--neon-blue-15)', color: 'var(--neon-blue)' }}>
                  有未保存的更改
                </span>
              )}
              <button
                onClick={onSave}
                disabled={perfSaving || Object.keys(perfDraft).length === 0}
                className="flex items-center gap-1.5 rounded-lg px-4 py-2 text-xs font-medium transition-all duration-200 disabled:opacity-30"
                style={{
                  background: Object.keys(perfDraft).length > 0 ? 'var(--neon-blue-30)' : 'var(--neon-blue-15)',
                  border: Object.keys(perfDraft).length > 0 ? '1px solid var(--neon-blue)' : '1px solid var(--neon-blue-30)',
                  color: 'var(--text-primary)',
                }}
              >
                {perfSaving ? <Loader2 size={12} className="animate-spin" /> : <Save size={12} />}
                保存配置
              </button>
              <button
                onClick={onClose}
                className="btn-close-ghost flex h-8 w-8 items-center justify-center rounded-lg transition-all active:scale-90"
              >
                <X size={16} />
              </button>
            </div>
          </div>

          {/* 内容区域 */}
          <div className="px-6 pb-6">
            {renderContent()}
          </div>
        </div>
      </motion.div>
    </motion.div>
  )
}

// ==================== GPU 安全保护面板组件 ====================
interface GPUSafetyPanelProps {
  perfConfig: PerformanceConfig
  perfDraft: Partial<PerformanceConfig>
  onDraftChange: (draft: Partial<PerformanceConfig>) => void
}

function GPUSafetyPanel({ perfConfig, perfDraft, onDraftChange }: GPUSafetyPanelProps) {
  const gpuStatus = perfConfig.gpu_status
  const metrics = gpuStatus?.metrics
  const isDegraded = gpuStatus?.degraded ?? false

  return (
    <div className="space-y-4 rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: `1px solid ${isDegraded ? '#ef4444' : 'var(--neon-blue-6)'}` }}>
      {/* 标题栏 */}
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
          <Shield size={16} className={isDegraded ? 'text-red-400' : 'text-emerald-400'} />
          GPU 安全保护
          {isDegraded && (
            <span className="text-xs px-2 py-0.5 rounded-full bg-red-400/10 text-red-400 flex items-center gap-1">
              <ShieldAlert size={10} />
              已降级
            </span>
          )}
        </h3>
        {/* 启用/禁用开关 */}
        <button
          onClick={() => onDraftChange({ ...perfDraft, gpu_safety_enabled: !(perfDraft.gpu_safety_enabled ?? perfConfig.gpu_safety_enabled) } as Partial<PerformanceConfig>)}
          className="flex items-center gap-1.5 rounded-lg px-2.5 py-1 text-xs transition-colors"
          style={
            (perfDraft.gpu_safety_enabled ?? perfConfig.gpu_safety_enabled)
              ? { background: 'rgba(16, 185, 129, 0.15)', border: '1px solid rgba(16, 185, 129, 0.3)', color: '#10b981' }
              : { background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)', color: 'var(--text-muted)' }
          }
        >
          {(perfDraft.gpu_safety_enabled ?? perfConfig.gpu_safety_enabled) ? '已启用' : '已禁用'}
        </button>
      </div>

      {/* 降级警告 */}
      {isDegraded && gpuStatus?.degrade_reason && (
        <div className="rounded-lg p-3 flex items-start gap-2" style={{ background: 'rgba(239, 68, 68, 0.08)', border: '1px solid rgba(239, 68, 68, 0.2)' }}>
          <ShieldAlert size={14} className="text-red-400 mt-0.5 shrink-0" />
          <div>
            <p className="text-xs font-medium text-red-400">GPU 过载保护已触发</p>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>{gpuStatus.degrade_reason}</p>
            <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>
              新任务已自动降级为 CPU 编码 · 已降级 {gpuStatus.degraded_task_count} 个任务
              {gpuStatus.pending_gpu_tasks > 0 && ` · ${gpuStatus.pending_gpu_tasks} 个任务等待 GPU 恢复`}
            </p>
          </div>
        </div>
      )}

      {/* GPU 实时指标 */}
      {metrics?.available && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {/* GPU 使用率 */}
          <div className="rounded-lg p-2.5" style={{ background: 'var(--neon-blue-6)' }}>
            <div className="flex items-center gap-1.5 text-xs mb-1.5" style={{ color: 'var(--text-muted)' }}>
              <Activity size={12} />
              使用率
            </div>
            <div className="text-lg font-bold" style={{
              color: metrics.utilization >= (perfConfig.gpu_utilization_threshold || 85) ? '#ef4444'
                : metrics.utilization >= (perfConfig.gpu_recovery_threshold || 60) ? '#f59e0b'
                : '#10b981'
            }}>
              {metrics.utilization.toFixed(0)}%
            </div>
            <div className="mt-1 h-1 w-full rounded-full" style={{ background: 'var(--progress-track-bg)' }}>
              <div
                className="h-full rounded-full transition-all duration-500"
                style={{
                  width: `${Math.min(100, metrics.utilization)}%`,
                  background: metrics.utilization >= (perfConfig.gpu_utilization_threshold || 85) ? '#ef4444'
                    : metrics.utilization >= (perfConfig.gpu_recovery_threshold || 60) ? '#f59e0b'
                    : '#10b981',
                }}
              />
            </div>
          </div>

          {/* GPU 温度 */}
          <div className="rounded-lg p-2.5" style={{ background: 'var(--neon-blue-6)' }}>
            <div className="flex items-center gap-1.5 text-xs mb-1.5" style={{ color: 'var(--text-muted)' }}>
              <Thermometer size={12} />
              温度
            </div>
            <div className="text-lg font-bold" style={{
              color: metrics.temperature >= (perfConfig.gpu_temperature_threshold || 80) ? '#ef4444'
                : metrics.temperature >= (perfConfig.gpu_temperature_recovery || 70) ? '#f59e0b'
                : '#10b981'
            }}>
              {metrics.temperature}°C
            </div>
            <div className="mt-1 h-1 w-full rounded-full" style={{ background: 'var(--progress-track-bg)' }}>
              <div
                className="h-full rounded-full transition-all duration-500"
                style={{
                  width: `${Math.min(100, (metrics.temperature / 100) * 100)}%`,
                  background: metrics.temperature >= (perfConfig.gpu_temperature_threshold || 80) ? '#ef4444'
                    : metrics.temperature >= (perfConfig.gpu_temperature_recovery || 70) ? '#f59e0b'
                    : '#10b981',
                }}
              />
            </div>
          </div>

          {/* 显存 */}
          <div className="rounded-lg p-2.5" style={{ background: 'var(--neon-blue-6)' }}>
            <div className="flex items-center gap-1.5 text-xs mb-1.5" style={{ color: 'var(--text-muted)' }}>
              <HardDrive size={12} />
              显存
            </div>
            <div className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>
              {metrics.memory_percent.toFixed(0)}%
            </div>
            <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
              {metrics.memory_used} / {metrics.memory_total} MB
            </p>
          </div>

          {/* 编码器 */}
          <div className="rounded-lg p-2.5" style={{ background: 'var(--neon-blue-6)' }}>
            <div className="flex items-center gap-1.5 text-xs mb-1.5" style={{ color: 'var(--text-muted)' }}>
              <Zap size={12} />
              编码器
            </div>
            <div className="text-lg font-bold" style={{
              color: metrics.encoder_util >= 90 ? '#ef4444'
                : metrics.encoder_util >= 70 ? '#f59e0b'
                : '#10b981'
            }}>
              {metrics.encoder_util.toFixed(0)}%
            </div>
            {metrics.power_draw > 0 && (
              <p className="text-xs mt-0.5" style={{ color: 'var(--text-muted)' }}>
                {metrics.power_draw.toFixed(0)}W / {metrics.power_limit.toFixed(0)}W
              </p>
            )}
          </div>
        </div>
      )}

      {/* GPU 名称信息 */}
      {metrics?.gpu_name && (
        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
          {metrics.gpu_name} · 驱动 {metrics.driver_version}
          {metrics.fan_speed > 0 && ` · 风扇 ${metrics.fan_speed}%`}
        </p>
      )}

      {/* 阈值配置 */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2" style={{ borderTop: '1px solid var(--neon-blue-6)' }}>
        {/* 使用率阈值 */}
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            <Activity size={12} className="text-red-400" />
            GPU 使用率安全上限
          </label>
          <div className="flex items-center gap-3">
            {(() => {
              const val = (perfDraft as Record<string, unknown>).gpu_utilization_threshold as number ?? (perfConfig.gpu_utilization_threshold || 85)
              const pct = ((val - 50) / (99 - 50)) * 100
              return (
                <input
                  type="range"
                  min={50}
                  max={99}
                  step={5}
                  value={val}
                  onChange={(e) => onDraftChange({ ...perfDraft, gpu_utilization_threshold: Number(e.target.value) } as Partial<PerformanceConfig>)}
                  className="flex-1 h-1.5 rounded-full appearance-none cursor-pointer"
                  style={{
                    background: `linear-gradient(to right, #ef4444 0%, #ef4444 ${pct}%, var(--neon-blue-6) ${pct}%, var(--neon-blue-6) 100%)`,
                    accentColor: '#ef4444',
                  }}
                />
              )
            })()}
            <span className="text-sm font-mono font-bold min-w-[3rem] text-right" style={{ color: 'var(--text-primary)' }}>
              {(perfDraft as Record<string, unknown>).gpu_utilization_threshold as number ?? (perfConfig.gpu_utilization_threshold || 85)}%
            </span>
          </div>
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            超过此值时新任务自动降级为 CPU 编码
          </p>
        </div>

        {/* 温度阈值 */}
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            <Thermometer size={12} className="text-orange-400" />
            GPU 温度安全上限
          </label>
          <div className="flex items-center gap-3">
            {(() => {
              const val = (perfDraft as Record<string, unknown>).gpu_temperature_threshold as number ?? (perfConfig.gpu_temperature_threshold || 80)
              const pct = ((val - 60) / (95 - 60)) * 100
              return (
                <input
                  type="range"
                  min={60}
                  max={95}
                  step={5}
                  value={val}
                  onChange={(e) => onDraftChange({ ...perfDraft, gpu_temperature_threshold: Number(e.target.value) } as Partial<PerformanceConfig>)}
                  className="flex-1 h-1.5 rounded-full appearance-none cursor-pointer"
                  style={{
                    background: `linear-gradient(to right, #f59e0b 0%, #f59e0b ${pct}%, var(--neon-blue-6) ${pct}%, var(--neon-blue-6) 100%)`,
                    accentColor: '#f59e0b',
                  }}
                />
              )
            })()}
            <span className="text-sm font-mono font-bold min-w-[3rem] text-right" style={{ color: 'var(--text-primary)' }}>
              {(perfDraft as Record<string, unknown>).gpu_temperature_threshold as number ?? (perfConfig.gpu_temperature_threshold || 80)}°C
            </span>
          </div>
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            超过此温度时立即暂停所有 GPU 任务
          </p>
        </div>

        {/* 恢复阈值 */}
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            <Activity size={12} className="text-emerald-400" />
            冷却恢复阈值（使用率）
          </label>
          <div className="flex items-center gap-3">
            {(() => {
              const val = (perfDraft as Record<string, unknown>).gpu_recovery_threshold as number ?? (perfConfig.gpu_recovery_threshold || 60)
              const pct = ((val - 30) / (80 - 30)) * 100
              return (
                <input
                  type="range"
                  min={30}
                  max={80}
                  step={5}
                  value={val}
                  onChange={(e) => onDraftChange({ ...perfDraft, gpu_recovery_threshold: Number(e.target.value) } as Partial<PerformanceConfig>)}
                  className="flex-1 h-1.5 rounded-full appearance-none cursor-pointer"
                  style={{
                    background: `linear-gradient(to right, #10b981 0%, #10b981 ${pct}%, var(--neon-blue-6) ${pct}%, var(--neon-blue-6) 100%)`,
                    accentColor: '#10b981',
                  }}
                />
              )
            })()}
            <span className="text-sm font-mono font-bold min-w-[3rem] text-right" style={{ color: 'var(--text-primary)' }}>
              {(perfDraft as Record<string, unknown>).gpu_recovery_threshold as number ?? (perfConfig.gpu_recovery_threshold || 60)}%
            </span>
          </div>
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            GPU 使用率降至此值以下时恢复硬件加速
          </p>
        </div>

        {/* 温度恢复阈值 */}
        <div className="space-y-2">
          <label className="flex items-center gap-2 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            <Thermometer size={12} className="text-emerald-400" />
            冷却恢复阈值（温度）
          </label>
          <div className="flex items-center gap-3">
            {(() => {
              const val = (perfDraft as Record<string, unknown>).gpu_temperature_recovery as number ?? (perfConfig.gpu_temperature_recovery || 70)
              const pct = ((val - 50) / (85 - 50)) * 100
              return (
                <input
                  type="range"
                  min={50}
                  max={85}
                  step={5}
                  value={val}
                  onChange={(e) => onDraftChange({ ...perfDraft, gpu_temperature_recovery: Number(e.target.value) } as Partial<PerformanceConfig>)}
                  className="flex-1 h-1.5 rounded-full appearance-none cursor-pointer"
                  style={{
                    background: `linear-gradient(to right, #10b981 0%, #10b981 ${pct}%, var(--neon-blue-6) ${pct}%, var(--neon-blue-6) 100%)`,
                    accentColor: '#10b981',
                  }}
                />
              )
            })()}
            <span className="text-sm font-mono font-bold min-w-[3rem] text-right" style={{ color: 'var(--text-primary)' }}>
              {(perfDraft as Record<string, unknown>).gpu_temperature_recovery as number ?? (perfConfig.gpu_temperature_recovery || 70)}°C
            </span>
          </div>
          <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
            GPU 温度降至此值以下时恢复硬件加速
          </p>
        </div>
      </div>
    </div>
  )
}
