import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Activity,
  CheckCircle2,
  CircleAlert,
  Clock3,
  Database,
  HardDrive,
  Loader2,
  RefreshCw,
  RotateCcw,
  ShieldCheck,
  X,
  XCircle,
} from 'lucide-react'
import { taskCenterApi } from '@/api'
import type {
  TaskCenterSnapshot,
  UnifiedTask,
  UnifiedTaskAction,
  UnifiedTaskKind,
  UnifiedTaskStatus,
} from '@/api'
import { Button, Surface, Tag, type TagTone } from '@/components/design-system'
import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'

const TASK_REFRESH_DEBOUNCE_MS = 250

const kindLabel: Record<UnifiedTaskKind, string> = {
  scan: '媒体库扫描',
  scrape: '元数据刮削',
  artifact_cleanup: '转码缓存清理',
  legacy_artifact_migration: '旧转码目录迁移',
  legacy_projection_migration: '旧转码历史登记',
  storage_incident: '转码存储告警',
}

const statusLabel: Record<UnifiedTaskStatus, string> = {
  queued: '等待中',
  running: '进行中',
  completed: '已完成',
  failed: '需要处理',
  cancelled: '已取消',
}

function taskIcon(kind: UnifiedTaskKind, status: UnifiedTaskStatus) {
  if (kind === 'storage_incident') return <CircleAlert size={17} aria-hidden="true" />
  if ((kind === 'artifact_cleanup' || kind === 'legacy_artifact_migration') && status === 'failed') return <CircleAlert size={17} aria-hidden="true" />
  if (status === 'failed') return <XCircle size={17} aria-hidden="true" />
  if (status === 'completed') return <CheckCircle2 size={17} aria-hidden="true" />
  if (status === 'queued') return <Clock3 size={17} aria-hidden="true" />
  if (kind === 'artifact_cleanup' || kind === 'legacy_artifact_migration') return <HardDrive size={17} aria-hidden="true" />
  if (kind === 'legacy_projection_migration' || kind === 'scan') return <Database size={17} aria-hidden="true" />
  return <Loader2 size={17} className="animate-spin" aria-hidden="true" />
}

function statusTone(status: UnifiedTaskStatus): TagTone {
  switch (status) {
    case 'running': return 'brand'
    case 'completed': return 'success'
    case 'failed': return 'danger'
    case 'cancelled': return 'neutral'
    default: return 'warning'
  }
}

function statusColor(status: UnifiedTaskStatus) {
  switch (status) {
    case 'running': return 'var(--nv-action-primary)'
    case 'completed': return 'var(--nv-status-success)'
    case 'failed': return 'var(--nv-status-danger)'
    case 'cancelled': return 'var(--nv-text-tertiary)'
    default: return 'var(--nv-status-warning)'
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

function taskRequestError(requestError: unknown, fallback: string) {
  if (typeof requestError === 'object' && requestError !== null) {
    const response = (requestError as { response?: { data?: { error?: unknown } } }).response
    if (typeof response?.data?.error === 'string' && response.data.error.trim() !== '') {
      return response.data.error
    }
  }
  return requestError instanceof Error ? requestError.message : fallback
}

function TaskRow({
  task,
  actionLoading,
  onAction,
}: {
  task: UnifiedTask
  actionLoading: string | null
  onAction: (task: UnifiedTask, action: UnifiedTaskAction) => void
}) {
  const active = task.status === 'queued' || task.status === 'running'
  const cleanupTask = task.kind === 'artifact_cleanup' || task.kind === 'legacy_artifact_migration'
  const projectionMigrationTask = task.kind === 'legacy_projection_migration'
  const migrationTask = task.kind === 'legacy_artifact_migration'
  const storageIncident = task.kind === 'storage_incident'
  const operationalIssue = (cleanupTask || storageIncident) && task.status === 'failed'
  const time = formatTime(task.updated_at || task.started_at || task.created_at)
  const actions = task.actions || []

  return (
    <Surface
      className="bg-[var(--nv-bg-surface-soft)] p-3 shadow-none"
      style={operationalIssue ? {
        borderColor: 'color-mix(in srgb, var(--nv-status-danger) 28%, var(--nv-border-default))',
        background: 'color-mix(in srgb, var(--nv-status-danger) 4%, var(--nv-bg-surface-soft))',
      } : undefined}
    >
      <div className="flex items-start gap-3">
        <div
          className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-hover)]"
          style={{ color: statusColor(task.status) }}
        >
          {taskIcon(task.kind, task.status)}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{task.title}</p>
              <p className="mt-0.5 text-xs text-[var(--nv-text-tertiary)]">
                {kindLabel[task.kind]}{task.subtitle ? ` · ${task.subtitle}` : ''}
              </p>
            </div>
            <Tag tone={statusTone(task.status)}>{statusLabel[task.status]}</Tag>
          </div>

          {active && (
            <div className="mt-3">
              <div className="mb-1 flex items-center justify-between text-[11px] text-[var(--nv-text-tertiary)]">
                <span className={cleanupTask || storageIncident ? 'min-w-0 break-all pr-3' : 'truncate pr-3'}>{task.message || '处理中'}</span>
                <span>{Math.round(task.progress)}%</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full bg-[var(--nv-bg-hover)]">
                <div
                  className="h-full rounded-full bg-[var(--nv-action-primary)] transition-[width] duration-300"
                  style={{ width: `${Math.max(2, task.progress)}%` }}
                />
              </div>
            </div>
          )}

          {!active && (task.message || time) && (
            <div className="mt-2 flex items-start justify-between gap-3 text-[11px] text-[var(--nv-text-tertiary)]">
              <span className={cleanupTask || storageIncident ? 'min-w-0 break-all' : 'truncate'}>{task.message}</span>
              <span className="shrink-0">{time}</span>
            </div>
          )}

          {cleanupTask && task.status === 'failed' && !migrationTask && (
            <p className="mt-2 text-[11px] leading-5 text-[var(--nv-text-tertiary)]">
              修复挂载、权限或路径配置后再重试。操作仍会经过 Cleanup Lease 与路径安全校验。
            </p>
          )}

          {storageIncident && (
            <p className="mt-2 text-[11px] leading-5 text-[var(--nv-text-tertiary)]">
              新转码 Claim 已暂停，正在运行的任务保持原 Lease。系统会持续执行真实写探针，确认存储恢复后自动解除告警。
            </p>
          )}

          {actions.length > 0 && (
            <div className="mt-3 flex justify-end gap-2">
              {actions.includes('retry') && (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => onAction(task, 'retry')}
                  disabled={actionLoading !== null}
                  loading={actionLoading === `${task.id}:retry`}
                >
                  {actionLoading !== `${task.id}:retry` && <RotateCcw size={13} aria-hidden="true" />}
                  {cleanupTask ? '立即重试' : projectionMigrationTask ? '重新登记' : '重试'}
                </Button>
              )}
              {actions.includes('rollback') && (
                <Button
                  type="button"
                  variant="secondary"
                  size="sm"
                  onClick={() => onAction(task, 'rollback')}
                  disabled={actionLoading !== null}
                  loading={actionLoading === `${task.id}:rollback`}
                  className="text-[var(--nv-status-warning)]"
                  style={{ borderColor: 'color-mix(in srgb, var(--nv-status-warning) 30%, var(--nv-border-default))' }}
                >
                  {actionLoading !== `${task.id}:rollback` && <ShieldCheck size={13} aria-hidden="true" />}
                  保留目录
                </Button>
              )}
            </div>
          )}
        </div>
      </div>
    </Surface>
  )
}

export default function TaskCenter() {
  const user = useAuthStore((state) => state.user)
  const manifest = useServerProfileStore((state) => state.manifest)
  const [open, setOpen] = useState(false)
  const [snapshot, setSnapshot] = useState<TaskCenterSnapshot | null>(null)
  const [loading, setLoading] = useState(false)
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const refreshInFlightRef = useRef<Promise<void> | null>(null)
  const refreshQueuedRef = useRef(false)
  const taskRefreshTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(null)
  const { on, off } = useWebSocket()

  const enabled = user?.role === 'admin' && manifest?.profile === 'lite'

  const performRefresh = useCallback(async () => {
    const response = await taskCenterApi.list({ limit: 50 })
    setSnapshot(response.data.data)
    setError(null)
  }, [])

  const refresh = useCallback(async (quiet = false) => {
    if (!enabled) return

    if (refreshInFlightRef.current) {
      refreshQueuedRef.current = true
      return refreshInFlightRef.current
    }

    if (!quiet) setLoading(true)
    const request = (async () => {
      do {
        refreshQueuedRef.current = false
        try {
          await performRefresh()
        } catch (requestError) {
          setError(taskRequestError(requestError, '无法读取任务状态'))
        }
      } while (refreshQueuedRef.current)
    })()

    refreshInFlightRef.current = request
    try {
      await request
    } finally {
      refreshInFlightRef.current = null
      if (!quiet) setLoading(false)
    }
  }, [enabled, performRefresh])

  const scheduleTaskRefresh = useCallback(() => {
    if (!enabled) return
    if (taskRefreshTimerRef.current !== null) {
      window.clearTimeout(taskRefreshTimerRef.current)
    }
    taskRefreshTimerRef.current = window.setTimeout(() => {
      taskRefreshTimerRef.current = null
      void refresh(true)
    }, TASK_REFRESH_DEBOUNCE_MS)
  }, [enabled, refresh])

  const handleAction = useCallback(async (task: UnifiedTask, action: UnifiedTaskAction) => {
    const sourceID = task.source_id || task.id.replace(`${task.kind}:`, '')
    const key = `${task.id}:${action}`
    setActionLoading(key)
    setError(null)
    try {
      await taskCenterApi.action(task.kind, sourceID, action)
      await refresh(true)
    } catch (requestError) {
      setError(taskRequestError(requestError, '任务操作失败'))
    } finally {
      setActionLoading(null)
    }
  }, [refresh])

  useEffect(() => {
    if (!enabled) return
    void refresh()
    const timer = window.setInterval(() => void refresh(true), 30_000)
    return () => window.clearInterval(timer)
  }, [enabled, refresh])

  useEffect(() => {
    if (!enabled) return
    const handleTaskEvent = () => scheduleTaskRefresh()
    on(WS_EVENTS.TASK_UPDATED, handleTaskEvent)
    return () => {
      off(WS_EVENTS.TASK_UPDATED, handleTaskEvent)
      if (taskRefreshTimerRef.current !== null) {
        window.clearTimeout(taskRefreshTimerRef.current)
        taskRefreshTimerRef.current = null
      }
    }
  }, [enabled, off, on, scheduleTaskRefresh])

  const tasks = snapshot?.tasks || []
  const activeCount = snapshot?.summary.active || 0
  const cleanupIssueCount = useMemo(
    () => tasks.filter((task) => task.kind === 'artifact_cleanup' && task.status === 'failed').length,
    [tasks],
  )
  const storageIssueCount = useMemo(
    () => tasks.filter((task) => task.kind === 'storage_incident' && task.status === 'failed').length,
    [tasks],
  )
  const operationalIssueCount = cleanupIssueCount + storageIssueCount
  const badgeCount = activeCount + operationalIssueCount
  const activeTasks = useMemo(() => tasks.filter((task) => task.status === 'queued' || task.status === 'running'), [tasks])
  const recentTasks = useMemo(() => tasks.filter((task) => task.status !== 'queued' && task.status !== 'running'), [tasks])

  if (!enabled) return null

  const renderTask = (task: UnifiedTask) => (
    <TaskRow key={task.id} task={task} actionLoading={actionLoading} onAction={handleAction} />
  )

  const issueLabel = storageIssueCount > 0
    ? `${storageIssueCount} 个存储故障`
    : `${cleanupIssueCount} 个清理问题`

  return (
    <>
      <Button
        type="button"
        variant={operationalIssueCount > 0 ? 'danger' : 'secondary'}
        size="sm"
        onClick={() => setOpen(true)}
        className="fixed right-4 top-14 z-40 shadow-[var(--nv-shadow-card)] backdrop-blur md:right-6"
        aria-label={operationalIssueCount > 0 ? `打开任务中心，${issueLabel}` : '打开任务中心'}
      >
        {operationalIssueCount > 0
          ? <CircleAlert size={18} className="animate-pulse" aria-hidden="true" />
          : <Activity size={18} className={activeCount > 0 ? 'animate-pulse text-[var(--nv-action-primary)]' : ''} aria-hidden="true" />}
        <span className="hidden sm:inline">任务</span>
        {badgeCount > 0 && (
          <span
            className="flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-[11px] font-bold"
            style={{
              background: operationalIssueCount > 0 ? 'var(--nv-status-danger)' : 'var(--nv-action-primary)',
              color: 'var(--nv-text-on-brand)',
            }}
          >
            {badgeCount > 99 ? '99+' : badgeCount}
          </span>
        )}
      </Button>

      {open && (
        <div className="fixed inset-0 z-[var(--nv-z-modal)]">
          <button
            type="button"
            className="absolute inset-0 bg-[var(--nv-bg-overlay)] backdrop-blur-sm"
            aria-label="关闭任务中心"
            onClick={() => setOpen(false)}
          />
          <aside className="absolute inset-y-0 right-0 flex w-full max-w-md flex-col border-l border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] shadow-[var(--nv-shadow-elevated)]">
            <header className="flex items-center justify-between border-b border-[var(--nv-border-subtle)] px-5 py-4">
              <div>
                <div className="flex items-center gap-2">
                  {operationalIssueCount > 0
                    ? <CircleAlert size={19} className="text-[var(--nv-status-danger)]" aria-hidden="true" />
                    : <Activity size={19} className="text-[var(--nv-action-primary)]" aria-hidden="true" />}
                  <h2 className="font-semibold text-[var(--nv-text-primary)]">任务中心</h2>
                  {operationalIssueCount > 0 && <Tag tone="danger">{issueLabel}</Tag>}
                </div>
                <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">扫描、刮削、转码、缓存清理和存储告警统一展示</p>
              </div>
              <div className="flex items-center gap-1">
                <Button variant="ghost" size="sm" iconOnly onClick={() => void refresh()} disabled={loading} aria-label="刷新任务">
                  <RefreshCw size={17} className={loading ? 'animate-spin' : ''} aria-hidden="true" />
                </Button>
                <Button variant="ghost" size="sm" iconOnly onClick={() => setOpen(false)} aria-label="关闭任务中心">
                  <X size={19} aria-hidden="true" />
                </Button>
              </div>
            </header>

            <div className="flex-1 overflow-y-auto p-4">
              {error && (
                <div className="mb-4 flex items-center gap-2 rounded-[var(--nv-radius-control)] border p-3 text-sm text-[var(--nv-status-danger)]" style={{ borderColor: 'color-mix(in srgb, var(--nv-status-danger) 25%, transparent)', background: 'color-mix(in srgb, var(--nv-status-danger) 6%, transparent)' }} role="alert">
                  <CircleAlert size={17} aria-hidden="true" />
                  {error}
                </div>
              )}

              {loading && !snapshot ? (
                <div className="flex min-h-60 items-center justify-center">
                  <Loader2 size={24} className="animate-spin text-[var(--nv-action-primary)]" aria-label="加载任务" />
                </div>
              ) : tasks.length === 0 ? (
                <div className="flex min-h-60 flex-col items-center justify-center text-center">
                  <CheckCircle2 size={34} className="mb-3 text-[var(--nv-status-success)]" aria-hidden="true" />
                  <p className="font-medium text-[var(--nv-text-primary)]">当前没有后台任务</p>
                  <p className="mt-1 text-sm text-[var(--nv-text-tertiary)]">扫描媒体库、开始转码或出现存储问题后会显示在这里。</p>
                </div>
              ) : (
                <div className="space-y-5">
                  {activeTasks.length > 0 && (
                    <section>
                      <div className="mb-2 flex items-center justify-between">
                        <h3 className="text-xs font-semibold uppercase tracking-[0.14em] text-[var(--nv-text-tertiary)]">进行中</h3>
                        <Tag tone="brand">{activeTasks.length} 项</Tag>
                      </div>
                      <div className="space-y-2">{activeTasks.map(renderTask)}</div>
                    </section>
                  )}
                  {recentTasks.length > 0 && (
                    <section>
                      <h3 className="mb-2 text-xs font-semibold uppercase tracking-[0.14em] text-[var(--nv-text-tertiary)]">最近任务与待处理问题</h3>
                      <div className="space-y-2">{recentTasks.map(renderTask)}</div>
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
