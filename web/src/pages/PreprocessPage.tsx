import { useState, useEffect, useCallback } from 'react'
import { preprocessApi } from '@/api/preprocess'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { useToast } from '@/components/Toast'
import type { PreprocessTask, PreprocessStatistics, SystemLoadInfo } from '@/types'
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
  const { on, off } = useWebSocket()
  const [tasks, setTasks] = useState<PreprocessTask[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [statusFilter, setStatusFilter] = useState('')
  const [stats, setStats] = useState<PreprocessStatistics | null>(null)
  const [sysLoad, setSysLoad] = useState<SystemLoadInfo | null>(null)
  const [loading, setLoading] = useState(true)

  // 加载任务列表
  const loadTasks = useCallback(async () => {
    try {
      const res = await preprocessApi.listTasks(page, 20, statusFilter)
      setTasks(res.data.data.tasks || [])
      setTotal(res.data.data.total)
    } catch {
      toast.error('加载预处理任务失败')
    }
  }, [page, statusFilter, toast])

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

  useEffect(() => {
    setLoading(true)
    Promise.all([loadTasks(), loadStats()]).finally(() => setLoading(false))
  }, [loadTasks, loadStats])

  // WebSocket 实时更新
  useEffect(() => {
    const handleProgress = () => {
      loadTasks()
      loadStats()
    }
    on(WS_EVENTS.PREPROCESS_PROGRESS, handleProgress)
    on(WS_EVENTS.PREPROCESS_COMPLETED, handleProgress)
    on(WS_EVENTS.PREPROCESS_FAILED, handleProgress)
    on(WS_EVENTS.PREPROCESS_STARTED, handleProgress)
    return () => {
      off(WS_EVENTS.PREPROCESS_PROGRESS, handleProgress)
      off(WS_EVENTS.PREPROCESS_COMPLETED, handleProgress)
      off(WS_EVENTS.PREPROCESS_FAILED, handleProgress)
      off(WS_EVENTS.PREPROCESS_STARTED, handleProgress)
    }
  }, [on, off, loadTasks, loadStats])

  // 任务操作
  const handlePause = async (id: string) => {
    try {
      await preprocessApi.pauseTask(id)
      toast.success('任务已暂停')
      loadTasks()
    } catch { toast.error('暂停失败') }
  }

  const handleResume = async (id: string) => {
    try {
      await preprocessApi.resumeTask(id)
      toast.success('任务已恢复')
      loadTasks()
    } catch { toast.error('恢复失败') }
  }

  const handleCancel = async (id: string) => {
    try {
      await preprocessApi.cancelTask(id)
      toast.success('任务已取消')
      loadTasks()
    } catch { toast.error('取消失败') }
  }

  const handleRetry = async (id: string) => {
    try {
      await preprocessApi.retryTask(id)
      toast.success('任务已重新提交')
      loadTasks()
    } catch { toast.error('重试失败') }
  }

  const handleDelete = async (id: string) => {
    try {
      await preprocessApi.deleteTask(id)
      toast.success('任务已删除')
      loadTasks()
    } catch { toast.error('删除失败') }
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
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-neon-blue" />
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Zap className="text-neon-blue" size={24} />
            视频预处理
          </h1>
          <p className="mt-1 text-sm text-surface-500">
            自动转码生成多码率 HLS 流，实现秒开播放
          </p>
        </div>
        <button
          onClick={() => { loadTasks(); loadStats() }}
          className="flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm text-surface-300 hover:text-white transition-colors"
          style={{ background: 'var(--neon-blue-6)' }}
        >
          <RefreshCw size={14} />
          刷新
        </button>
      </div>

      {/* 统计卡片 */}
      {stats && sysLoad && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-surface-400 text-xs mb-2">
              <Activity size={14} className="text-neon-blue" />
              处理中
            </div>
            <div className="text-2xl font-bold text-white">{stats.running_count}</div>
            <div className="text-xs text-surface-500 mt-1">
              {stats.active_workers}/{stats.max_workers} 工作线程
            </div>
          </div>

          <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-surface-400 text-xs mb-2">
              <Clock size={14} className="text-yellow-400" />
              队列
            </div>
            <div className="text-2xl font-bold text-white">{stats.queue_size}</div>
            <div className="text-xs text-surface-500 mt-1">
              等待处理
            </div>
          </div>

          <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-surface-400 text-xs mb-2">
              <Cpu size={14} className="text-emerald-400" />
              系统负载
            </div>
            <div className="text-2xl font-bold text-white">{sysLoad.mem_alloc_mb.toFixed(0)} MB</div>
            <div className="text-xs text-surface-500 mt-1">
              {sysLoad.goroutines} goroutines · {sysLoad.cpu_count} CPU
            </div>
          </div>

          <div className="rounded-xl p-4" style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}>
            <div className="flex items-center gap-2 text-surface-400 text-xs mb-2">
              <HardDrive size={14} className="text-purple-400" />
              硬件加速
            </div>
            <div className="text-lg font-bold text-white">{stats.hw_accel || 'CPU'}</div>
            <div className="text-xs text-surface-500 mt-1">
              已完成 {stats.status_counts?.completed || 0} 个
            </div>
          </div>
        </div>
      )}

      {/* 状态过滤 */}
      <div className="flex items-center gap-2 flex-wrap">
        {['', 'running', 'pending', 'paused', 'completed', 'failed', 'cancelled'].map((s) => (
          <button
            key={s}
            onClick={() => { setStatusFilter(s); setPage(1) }}
            className={clsx(
              'rounded-lg px-3 py-1.5 text-xs transition-colors',
              statusFilter === s
                ? 'text-white'
                : 'text-surface-400 hover:text-white'
            )}
            style={statusFilter === s ? { background: 'var(--neon-blue-15)', border: '1px solid var(--neon-blue-30)' } : { background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}
          >
            {s === '' ? '全部' : statusLabels[s] || s}
            {s && stats?.status_counts?.[s] ? ` (${stats.status_counts[s]})` : ''}
          </button>
        ))}
      </div>

      {/* 任务列表 */}
      <div className="space-y-3">
        {tasks.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-surface-500">
            <Film size={48} className="mb-4 opacity-30" />
            <p>暂无预处理任务</p>
            <p className="text-xs mt-1">扫描媒体库后将自动提交预处理任务</p>
          </div>
        ) : (
          tasks.map((task) => (
            <div
              key={task.id}
              className="rounded-xl p-4 transition-colors"
              style={{ background: 'var(--glass-bg)', border: '1px solid var(--neon-blue-6)' }}
            >
              <div className="flex items-start justify-between gap-4">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className={statusColors[task.status]}>
                      {statusIcons[task.status]}
                    </span>
                    <h3 className="text-sm font-medium text-white truncate">
                      {task.media_title || task.media_id}
                    </h3>
                    <span className={clsx('text-xs px-1.5 py-0.5 rounded', statusColors[task.status])}
                      style={{ background: 'rgba(0,0,0,0.3)' }}>
                      {statusLabels[task.status] || task.status}
                    </span>
                  </div>

                  {/* 进度条 */}
                  {(task.status === 'running' || task.status === 'paused') && (
                    <div className="mt-2">
                      <div className="flex items-center justify-between text-xs text-surface-400 mb-1">
                        <span>{task.phase || task.message}</span>
                        <span>{task.progress.toFixed(1)}%</span>
                      </div>
                      <div className="h-1.5 w-full rounded-full bg-surface-700">
                        <div
                          className="h-full rounded-full transition-all duration-500"
                          style={{
                            width: `${task.progress}%`,
                            background: task.status === 'paused'
                              ? 'var(--neon-orange, orange)'
                              : 'linear-gradient(90deg, var(--neon-purple), var(--neon-blue))',
                          }}
                        />
                      </div>
                    </div>
                  )}

                  {/* 详细信息 */}
                  <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-surface-500">
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
                    <button onClick={() => handlePause(task.id)} className="p-1.5 rounded-lg text-surface-400 hover:text-yellow-400 hover:bg-yellow-400/10 transition-colors" title="暂停">
                      <Pause size={14} />
                    </button>
                  )}
                  {task.status === 'paused' && (
                    <button onClick={() => handleResume(task.id)} className="p-1.5 rounded-lg text-surface-400 hover:text-emerald-400 hover:bg-emerald-400/10 transition-colors" title="恢复">
                      <Play size={14} />
                    </button>
                  )}
                  {(task.status === 'running' || task.status === 'paused' || task.status === 'pending' || task.status === 'queued') && (
                    <button onClick={() => handleCancel(task.id)} className="p-1.5 rounded-lg text-surface-400 hover:text-red-400 hover:bg-red-400/10 transition-colors" title="取消">
                      <XCircle size={14} />
                    </button>
                  )}
                  {task.status === 'failed' && (
                    <button onClick={() => handleRetry(task.id)} className="p-1.5 rounded-lg text-surface-400 hover:text-neon-blue hover:bg-neon-blue/10 transition-colors" title="重试">
                      <RotateCcw size={14} />
                    </button>
                  )}
                  {(task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled') && (
                    <button onClick={() => handleDelete(task.id)} className="p-1.5 rounded-lg text-surface-400 hover:text-red-400 hover:bg-red-400/10 transition-colors" title="删除">
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      {/* 分页 */}
      {total > 20 && (
        <div className="flex items-center justify-center gap-2">
          <button
            onClick={() => setPage(Math.max(1, page - 1))}
            disabled={page <= 1}
            className="rounded-lg px-3 py-1.5 text-xs text-surface-400 hover:text-white disabled:opacity-30 transition-colors"
            style={{ background: 'var(--glass-bg)' }}
          >
            上一页
          </button>
          <span className="text-xs text-surface-500">
            {page} / {Math.ceil(total / 20)}
          </span>
          <button
            onClick={() => setPage(page + 1)}
            disabled={page >= Math.ceil(total / 20)}
            className="rounded-lg px-3 py-1.5 text-xs text-surface-400 hover:text-white disabled:opacity-30 transition-colors"
            style={{ background: 'var(--glass-bg)' }}
          >
            下一页
          </button>
        </div>
      )}
    </div>
  )
}
