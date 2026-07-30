import { useCallback, useEffect, useMemo, useState } from 'react'
import { Activity, CheckCircle2, CircleAlert, Clock3, Database, Film, Loader2, RefreshCw, X, XCircle } from 'lucide-react'
import { taskCenterApi } from '@/api'
import type { TaskCenterSnapshot, UnifiedTask, UnifiedTaskKind, UnifiedTaskStatus } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'

const TASK_EVENTS = [
  WS_EVENTS.SCAN_STARTED,
  WS_EVENTS.SCAN_PROGRESS,
  WS_EVENTS.SCAN_PHASE,
  WS_EVENTS.SCAN_COMPLETED,
  WS_EVENTS.SCAN_FAILED,
  WS_EVENTS.SCRAPE_STARTED,
  WS_EVENTS.SCRAPE_PROGRESS,
  WS_EVENTS.SCRAPE_COMPLETED,
  WS_EVENTS.TRANSCODE_STARTED,
  WS_EVENTS.TRANSCODE_PROGRESS,
  WS_EVENTS.TRANSCODE_COMPLETED,
  WS_EVENTS.TRANSCODE_FAILED,
] as const

const kindLabel: Record<UnifiedTaskKind, string> = {
  scan: '媒体库扫描',
  scrape: '元数据刮削',
  transcode: '视频转码',
}

const statusLabel: Record<UnifiedTaskStatus, string> = {
  queued: '等待中',
  running: '进行中',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

function taskIcon(kind: UnifiedTaskKind, status: UnifiedTaskStatus) {
  if (status === 'failed') return <XCircle size={17} />
  if (status === 'completed') return <CheckCircle2 size={17} />
  if (status === 'queued') return <Clock3 size={17} />
  if (kind === 'scan') return <Database size={17} />
  if (kind === 'transcode') return <Film size={17} />
  return <Loader2 size={17} className="animate-spin" />
}

function statusColor(status: UnifiedTaskStatus) {
  switch (status) {
    case 'running': return 'var(--neon-blue)'
    case 'completed': return '#16A34A'
    case 'failed': return '#DC2626'
    case 'cancelled': return 'var(--text-muted)'
    default: return '#CA8A04'
  }
}

function formatTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function TaskRow({ task }: { task: UnifiedTask }) {
  const active = task.status === 'queued' || task.status === 'running'
  const time = formatTime(task.updated_at || task.started_at || task.created_at)

  return (
    <div className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}>
      <div className="flex items-start gap-3">
        <div
          className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg"
          style={{ color: statusColor(task.status), background: 'var(--nav-hover-bg)' }}
        >
          {taskIcon(task.kind, task.status)}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{task.title}</p>
              <p className="mt-0.5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                {kindLabel[task.kind]}{task.subtitle ? ` · ${task.subtitle}` : ''}
              </p>
            </div>
            <span className="shrink-0 text-xs font-medium" style={{ color: statusColor(task.status) }}>
              {statusLabel[task.status]}
            </span>
          </div>

          {active && (
            <div className="mt-3">
              <div className="mb-1 flex items-center justify-between text-[11px]" style={{ color: 'var(--text-muted)' }}>
                <span className="truncate pr-3">{task.message || '处理中'}</span>
                <span>{Math.round(task.progress)}%</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full" style={{ background: 'var(--nav-hover-bg)' }}>
                <div
                  className="h-full rounded-full transition-all duration-300"
                  style={{ width: `${Math.max(2, task.progress)}%`, background: 'var(--neon-blue)' }}
                />
              </div>
            </div>
          )}

          {!active && (task.message || time) && (
            <div className="mt-2 flex items-center justify-between gap-3 text-[11px]" style={{ color: 'var(--text-muted)' }}>
              <span className="truncate">{task.message}</span>
              <span className="shrink-0">{time}</span>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

export default function TaskCenter() {
  const user = useAuthStore((state) => state.user)
  const manifest = useServerProfileStore((state) => state.manifest)
  const [open, setOpen] = useState(false)
  const [snapshot, setSnapshot] = useState<TaskCenterSnapshot | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { on, off } = useWebSocket()

  const enabled = user?.role === 'admin' && manifest?.profile === 'lite'

  const refresh = useCallback(async (quiet = false) => {
    if (!enabled) return
    if (!quiet) setLoading(true)
    try {
      const response = await taskCenterApi.list({ limit: 50 })
      setSnapshot(response.data.data)
      setError(null)
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : '无法读取任务状态')
    } finally {
      if (!quiet) setLoading(false)
    }
  }, [enabled])

  useEffect(() => {
    if (!enabled) return
    void refresh()
    const timer = window.setInterval(() => void refresh(true), 30_000)
    return () => window.clearInterval(timer)
  }, [enabled, refresh])

  useEffect(() => {
    if (!enabled) return
    const handleTaskEvent = () => void refresh(true)
    TASK_EVENTS.forEach((event) => on(event, handleTaskEvent))
    return () => TASK_EVENTS.forEach((event) => off(event, handleTaskEvent))
  }, [enabled, off, on, refresh])

  const tasks = snapshot?.tasks || []
  const activeCount = snapshot?.summary.active || 0
  const activeTasks = useMemo(() => tasks.filter((task) => task.status === 'queued' || task.status === 'running'), [tasks])
  const recentTasks = useMemo(() => tasks.filter((task) => task.status !== 'queued' && task.status !== 'running'), [tasks])

  if (!enabled) return null

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="fixed right-4 top-14 z-40 flex h-10 items-center gap-2 rounded-xl border px-3 text-sm font-medium shadow-lg backdrop-blur md:right-6"
        style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)', color: 'var(--text-secondary)' }}
        aria-label="打开任务中心"
      >
        <Activity size={18} className={activeCount > 0 ? 'animate-pulse text-neon' : ''} />
        <span className="hidden sm:inline">任务</span>
        {activeCount > 0 && (
          <span className="flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-[11px] font-bold text-white" style={{ background: 'var(--neon-blue)' }}>
            {activeCount > 99 ? '99+' : activeCount}
          </span>
        )}
      </button>

      {open && (
        <div className="fixed inset-0 z-[100]">
          <button
            type="button"
            className="absolute inset-0 bg-black/50 backdrop-blur-sm"
            aria-label="关闭任务中心"
            onClick={() => setOpen(false)}
          />
          <aside
            className="absolute inset-y-0 right-0 flex w-full max-w-md flex-col border-l shadow-2xl"
            style={{ borderColor: 'var(--border-default)', background: 'var(--bg-base)' }}
          >
            <header className="flex items-center justify-between border-b px-5 py-4" style={{ borderColor: 'var(--border-default)' }}>
              <div>
                <div className="flex items-center gap-2">
                  <Activity size={19} className="text-neon" />
                  <h2 className="font-semibold" style={{ color: 'var(--text-primary)' }}>任务中心</h2>
                </div>
                <p className="mt-1 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  扫描、刮削和转码统一进度
                </p>
              </div>
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  onClick={() => void refresh()}
                  disabled={loading}
                  className="rounded-lg p-2 transition-colors hover:bg-[var(--nav-hover-bg)] disabled:opacity-50"
                  style={{ color: 'var(--text-secondary)' }}
                  aria-label="刷新任务"
                >
                  <RefreshCw size={17} className={loading ? 'animate-spin' : ''} />
                </button>
                <button
                  type="button"
                  onClick={() => setOpen(false)}
                  className="rounded-lg p-2 transition-colors hover:bg-[var(--nav-hover-bg)]"
                  style={{ color: 'var(--text-secondary)' }}
                  aria-label="关闭任务中心"
                >
                  <X size={19} />
                </button>
              </div>
            </header>

            <div className="flex-1 overflow-y-auto p-4">
              {error && (
                <div className="mb-4 flex items-center gap-2 rounded-xl border p-3 text-sm" style={{ borderColor: 'rgba(220,38,38,.25)', background: 'rgba(220,38,38,.06)', color: '#DC2626' }}>
                  <CircleAlert size={17} />
                  {error}
                </div>
              )}

              {loading && !snapshot ? (
                <div className="flex min-h-60 items-center justify-center">
                  <Loader2 size={24} className="animate-spin text-neon" />
                </div>
              ) : tasks.length === 0 ? (
                <div className="flex min-h-60 flex-col items-center justify-center text-center">
                  <CheckCircle2 size={34} className="mb-3 text-green-500" />
                  <p className="font-medium" style={{ color: 'var(--text-primary)' }}>当前没有后台任务</p>
                  <p className="mt-1 text-sm" style={{ color: 'var(--text-tertiary)' }}>扫描媒体库或开始播放转码后会显示在这里。</p>
                </div>
              ) : (
                <div className="space-y-5">
                  {activeTasks.length > 0 && (
                    <section>
                      <div className="mb-2 flex items-center justify-between">
                        <h3 className="text-xs font-semibold uppercase tracking-[0.14em]" style={{ color: 'var(--text-tertiary)' }}>进行中</h3>
                        <span className="text-xs text-neon">{activeTasks.length} 项</span>
                      </div>
                      <div className="space-y-2">{activeTasks.map((task) => <TaskRow key={task.id} task={task} />)}</div>
                    </section>
                  )}

                  {recentTasks.length > 0 && (
                    <section>
                      <h3 className="mb-2 text-xs font-semibold uppercase tracking-[0.14em]" style={{ color: 'var(--text-tertiary)' }}>最近任务</h3>
                      <div className="space-y-2">{recentTasks.map((task) => <TaskRow key={task.id} task={task} />)}</div>
                    </section>
                  )}
                </div>
              )}
            </div>
          </aside>
        </div>
      )}
    </>
  )
}
