// 番号刮削 - 批量刮削与运营中心
// 批量任务、文件夹刮削、Cookie、镜像、缓存、定时调度和分析报表统一入口。
import { useCallback, useEffect, useState } from 'react'
import { adultScraperApi } from '@/api'
import type {
  AdultBatchTask,
  AdultBatchProgressEvent,
  AdultSchedulerConfig,
  AdultScrapeReport,
  MirrorStatus,
} from '@/api/adultScraper'
import AdultFolderScraperPanel from './AdultFolderScraperPanel'
import AdultCookieLoginPanel from './AdultCookieLoginPanel'
import {
  Play,
  Pause,
  Square,
  RefreshCw,
  Trash2,
  Clock,
  BarChart3,
  Globe,
  Database,
  AlertTriangle,
  CheckCircle2,
  Loader2,
  Wifi,
  WifiOff,
  TrendingUp,
  History,
  RotateCw,
  FolderSearch,
  Cookie,
} from 'lucide-react'
import { useDialog } from '@/components/Dialog'
import { Button, Input, Select, Surface, Tag, type TagTone } from '@/components/design-system'

type Tab = 'batch' | 'folder' | 'cookies' | 'mirrors' | 'cache' | 'scheduler' | 'report'

const tabs: Array<{ key: Tab; label: string; icon: React.ReactNode }> = [
  { key: 'batch', label: '批量刮削', icon: <Play className="h-4 w-4" aria-hidden="true" /> },
  { key: 'folder', label: '文件夹刮削', icon: <FolderSearch className="h-4 w-4" aria-hidden="true" /> },
  { key: 'cookies', label: 'Cookie 登录', icon: <Cookie className="h-4 w-4" aria-hidden="true" /> },
  { key: 'mirrors', label: '镜像管理', icon: <Globe className="h-4 w-4" aria-hidden="true" /> },
  { key: 'cache', label: '缓存管理', icon: <Database className="h-4 w-4" aria-hidden="true" /> },
  { key: 'scheduler', label: '定时调度', icon: <Clock className="h-4 w-4" aria-hidden="true" /> },
  { key: 'report', label: '分析报表', icon: <TrendingUp className="h-4 w-4" aria-hidden="true" /> },
]

export default function AdultScraperProSection() {
  const [tab, setTab] = useState<Tab>('batch')

  return (
    <Surface className="p-4 md:p-5">
      <div className="mb-4 flex items-center gap-2">
        <div className="flex h-9 w-9 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
          <BarChart3 className="h-5 w-5" aria-hidden="true" />
        </div>
        <div>
          <h3 className="text-lg font-semibold text-[var(--nv-text-primary)]">番号刮削运营中心</h3>
          <p className="text-xs text-[var(--nv-text-tertiary)]">统一管理批量刮削、镜像、缓存、调度与失败分析</p>
        </div>
      </div>

      <div className="mb-4 flex flex-wrap gap-2 border-b border-[var(--nv-border-subtle)] pb-3" role="tablist" aria-label="番号刮削运营中心">
        {tabs.map((item) => (
          <Button
            key={item.key}
            type="button"
            variant={tab === item.key ? 'primary' : 'ghost'}
            size="sm"
            role="tab"
            aria-selected={tab === item.key}
            onClick={() => setTab(item.key)}
          >
            {item.icon}{item.label}
          </Button>
        ))}
      </div>

      {tab === 'batch' && <BatchPanel />}
      {tab === 'folder' && <AdultFolderScraperPanel />}
      {tab === 'cookies' && <AdultCookieLoginPanel />}
      {tab === 'mirrors' && <MirrorsPanel />}
      {tab === 'cache' && <CachePanel />}
      {tab === 'scheduler' && <SchedulerPanel />}
      {tab === 'report' && <ReportPanel />}
    </Surface>
  )
}

function BatchPanel() {
  const dialog = useDialog()
  const [tasks, setTasks] = useState<AdultBatchTask[]>([])
  const [history, setHistory] = useState<AdultBatchTask[]>([])
  const [loading, setLoading] = useState(false)
  const [starting, setStarting] = useState(false)
  const [opts, setOpts] = useState({
    library_id: '',
    only_unscraped: true,
    dry_run: false,
    concurrency: 2,
    aggregated: false,
  })
  const [progressByTask, setProgressByTask] = useState<Record<string, AdultBatchProgressEvent>>({})

  const loadTasks = useCallback(async () => {
    setLoading(true)
    try {
      const response = await adultScraperApi.listBatchTasks()
      setTasks(response.data.data.active || [])
      setHistory(response.data.data.history || [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadTasks()
    const timer = window.setInterval(() => void loadTasks(), 3000)
    return () => window.clearInterval(timer)
  }, [loadTasks])

  useEffect(() => {
    try {
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${proto}//${window.location.host}/ws`)
      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          if (data?.type === 'adult_batch_progress' && data?.data?.task_id) {
            setProgressByTask((current) => ({ ...current, [data.data.task_id]: data.data }))
          }
          if (data?.type === 'adult_batch_completed') void loadTasks()
        } catch {
          // Ignore unrelated/invalid websocket payloads.
        }
      }
      return () => ws.close()
    } catch {
      return undefined
    }
  }, [loadTasks])

  const handleStart = async () => {
    setStarting(true)
    try {
      await adultScraperApi.startBatch(opts)
      await loadTasks()
    } catch (error: any) {
      await dialog.alert({ title: '启动失败', message: error?.response?.data?.error || error?.message, variant: 'error' })
    } finally {
      setStarting(false)
    }
  }

  return (
    <div className="space-y-4">
      <Surface className="bg-[var(--nv-bg-surface-soft)] p-4 shadow-none">
        <div className="mb-3 text-sm font-medium text-[var(--nv-text-primary)]">启动新任务</div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <LabelCheck label="只处理未刮削" checked={opts.only_unscraped} onChange={(value) => setOpts({ ...opts, only_unscraped: value })} />
          <LabelCheck label="预览模式（仅识别）" checked={opts.dry_run} onChange={(value) => setOpts({ ...opts, dry_run: value })} />
          <LabelCheck label="聚合模式（精刮，较慢）" checked={opts.aggregated} onChange={(value) => setOpts({ ...opts, aggregated: value })} />
          <LabelNumber label="并发度（1-8）" value={opts.concurrency} onChange={(value) => setOpts({ ...opts, concurrency: value })} min={1} max={8} />
        </div>
        <div className="mt-3 flex items-center gap-2">
          <Button type="button" variant="primary" onClick={handleStart} disabled={starting} loading={starting}>
            {!starting && <Play className="h-4 w-4" aria-hidden="true" />}启动全库批量刮削
          </Button>
          <Button type="button" variant="secondary" iconOnly onClick={() => void loadTasks()} title="刷新任务" aria-label="刷新批量刮削任务">
            <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} aria-hidden="true" />
          </Button>
        </div>
      </Surface>

      <TaskSection
        title={`进行中的任务（${tasks.length}）`}
        icon={tasks.length > 0 ? <Loader2 className="h-4 w-4 animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" /> : <CheckCircle2 className="h-4 w-4 text-[var(--nv-status-success)]" aria-hidden="true" />}
        empty="暂无进行中的任务"
      >
        {tasks.map((task) => (
          <TaskRow
            key={task.id}
            task={task}
            live={progressByTask[task.id]}
            onPause={async () => { await adultScraperApi.pauseBatch(task.id); await loadTasks() }}
            onResume={async () => { await adultScraperApi.resumeBatch(task.id); await loadTasks() }}
            onCancel={async () => { await adultScraperApi.cancelBatch(task.id); await loadTasks() }}
          />
        ))}
      </TaskSection>

      <TaskSection title={`历史任务（最近 ${history.length} 个）`} icon={<History className="h-4 w-4 text-[var(--nv-text-tertiary)]" aria-hidden="true" />} empty="暂无历史">
        {history.slice(0, 10).map((task) => <TaskRow key={task.id} task={task} />)}
      </TaskSection>
    </div>
  )
}

function TaskSection({ title, icon, empty, children }: { title: string; icon: React.ReactNode; empty: string; children: React.ReactNode }) {
  const hasChildren = Array.isArray(children) ? children.length > 0 : Boolean(children)
  return (
    <div>
      <div className="mb-2 flex items-center gap-2 text-sm font-medium text-[var(--nv-text-primary)]">{icon}{title}</div>
      {!hasChildren ? (
        <Surface className="bg-[var(--nv-bg-surface-soft)] p-4 text-center text-sm text-[var(--nv-text-tertiary)] shadow-none">{empty}</Surface>
      ) : (
        <div className="space-y-2">{children}</div>
      )}
    </div>
  )
}

function taskTone(status: string): TagTone {
  if (status === 'completed') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'paused') return 'warning'
  if (status === 'cancelled') return 'neutral'
  return 'brand'
}

function TaskRow({
  task, live, onPause, onResume, onCancel,
}: {
  task: AdultBatchTask
  live?: AdultBatchProgressEvent
  onPause?: () => void
  onResume?: () => void
  onCancel?: () => void
}) {
  const current = live?.current ?? task.current
  const success = live?.success ?? task.success
  const failed = live?.failed ?? task.failed
  const skipped = live?.skipped ?? task.skipped
  const total = task.total
  const percent = total > 0 ? Math.min(100, Math.round((current / total) * 100)) : 0
  const progressColor = task.status === 'completed'
    ? 'var(--nv-status-success)'
    : task.status === 'failed'
      ? 'var(--nv-status-danger)'
      : task.status === 'paused'
        ? 'var(--nv-status-warning)'
        : task.status === 'cancelled'
          ? 'var(--nv-text-tertiary)'
          : 'var(--nv-action-primary)'

  return (
    <Surface className="bg-[var(--nv-bg-surface-soft)] p-3 shadow-none">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <Tag tone={taskTone(task.status)}>{task.status.toUpperCase()}</Tag>
            <span className="font-mono text-[var(--nv-text-tertiary)]">#{task.id.slice(0, 8)}</span>
            {task.aggregated && <Tag tone="brand">聚合</Tag>}
            {task.dry_run && <Tag tone="neutral">预览</Tag>}
          </div>
          {live?.media_title && (
            <div className="mt-1 truncate text-xs text-[var(--nv-text-tertiary)]">
              {live.status === 'failed' ? '❌' : live.status === 'success' ? '✅' : '⏳'} {live.code} · {live.media_title}
              {live.err_msg && <span className="ml-2 text-[var(--nv-status-danger)]">{live.err_msg}</span>}
            </div>
          )}
        </div>
        <div className="flex items-center gap-1">
          {task.status === 'running' && onPause && <Button type="button" variant="ghost" size="sm" iconOnly onClick={onPause} title="暂停"><Pause className="h-4 w-4" aria-hidden="true" /></Button>}
          {task.status === 'paused' && onResume && <Button type="button" variant="ghost" size="sm" iconOnly onClick={onResume} title="恢复"><Play className="h-4 w-4" aria-hidden="true" /></Button>}
          {(task.status === 'running' || task.status === 'paused') && onCancel && <Button type="button" variant="danger" size="sm" iconOnly onClick={onCancel} title="取消"><Square className="h-4 w-4" aria-hidden="true" /></Button>}
        </div>
      </div>

      <div className="mt-2">
        <div className="h-1.5 overflow-hidden rounded-full bg-[var(--nv-bg-control)]">
          <div className="h-full rounded-full transition-[width] duration-300" style={{ width: `${percent}%`, background: progressColor }} />
        </div>
        <div className="mt-1 flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--nv-text-tertiary)]">
          <span>{current} / {total} ({percent}%)</span>
          <span><span className="text-[var(--nv-status-success)]">✓ {success}</span>{' · '}<span className="text-[var(--nv-status-danger)]">✗ {failed}</span>{' · '}<span>跳过 {skipped}</span></span>
        </div>
      </div>
    </Surface>
  )
}

function MirrorsPanel() {
  const [data, setData] = useState<Record<string, { mirrors: MirrorStatus[]; preferred: string }> | null>(null)
  const [lastHealthAt, setLastHealthAt] = useState('')
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const response = await adultScraperApi.listMirrors()
      setData(response.data.data.sources)
      setLastHealthAt(response.data.data.last_health_at || '')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const handleHealthCheck = async () => {
    setChecking(true)
    try {
      await adultScraperApi.healthCheckMirrors()
      await load()
    } finally {
      setChecking(false)
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-sm text-[var(--nv-text-tertiary)]">最近健康检查：{lastHealthAt ? new Date(lastHealthAt).toLocaleString() : '从未'}</div>
        <Button type="button" variant="primary" size="sm" onClick={handleHealthCheck} disabled={checking} loading={checking}>
          {!checking && <RefreshCw className="h-4 w-4" aria-hidden="true" />}全部健康检查
        </Button>
      </div>

      {loading ? (
        <div className="flex justify-center py-6"><Loader2 className="h-5 w-5 animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" /></div>
      ) : (
        <div className="space-y-3">
          {data && Object.entries(data).map(([source, value]) => (
            <Surface key={source} className="bg-[var(--nv-bg-surface-soft)] p-3 shadow-none">
              <div className="mb-2 flex items-center justify-between gap-3">
                <div className="font-medium uppercase text-[var(--nv-text-primary)]">{source}</div>
                <div className="text-xs text-[var(--nv-text-tertiary)]">首选：{value.preferred}</div>
              </div>
              <div className="space-y-1">
                {value.mirrors.map((mirror, index) => (
                  <div key={`${mirror.url}-${index}`} className="flex flex-wrap items-center justify-between gap-2 rounded-[var(--nv-radius-sm)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] px-2 py-1.5 text-xs">
                    <div className="flex min-w-0 items-center gap-2">
                      {mirror.healthy ? <Wifi className="h-3.5 w-3.5 shrink-0 text-[var(--nv-status-success)]" aria-hidden="true" /> : <WifiOff className="h-3.5 w-3.5 shrink-0 text-[var(--nv-status-danger)]" aria-hidden="true" />}
                      <span className="truncate font-mono text-[var(--nv-text-primary)]">{mirror.url}</span>
                    </div>
                    <div className="text-[var(--nv-text-tertiary)]">
                      {mirror.latency_ms > 0 && <span>{mirror.latency_ms}ms</span>}
                      {mirror.fail_count > 0 && <span className="ml-2 text-[var(--nv-status-danger)]">失败 {mirror.fail_count}次</span>}
                    </div>
                  </div>
                ))}
              </div>
            </Surface>
          ))}
        </div>
      )}
    </div>
  )
}

function CachePanel() {
  const dialog = useDialog()
  const [stats, setStats] = useState<{ size: number; max_size: number; total_hit: number; ttl: string } | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const response = await adultScraperApi.getCacheStats()
      setStats(response.data.data)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const handleClear = async () => {
    const ok = await dialog.confirm({ title: '清空番号缓存', message: '确定清空所有番号元数据缓存？', confirmText: '清空', variant: 'danger' })
    if (!ok) return
    await adultScraperApi.clearCache()
    await load()
  }

  return (
    <div className="space-y-3">
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <StatCard label="缓存条目" value={stats?.size ?? 0} />
        <StatCard label="最大容量" value={stats?.max_size ?? 0} />
        <StatCard label="累计命中" value={stats?.total_hit ?? 0} />
        <StatCard label="过期时间" value={stats?.ttl ?? '-'} />
      </div>
      <div className="flex items-center gap-2">
        <Button type="button" variant="secondary" size="sm" onClick={() => void load()} loading={loading}>{!loading && <RefreshCw className="h-4 w-4" aria-hidden="true" />}刷新</Button>
        <Button type="button" variant="danger" size="sm" onClick={handleClear}><Trash2 className="h-4 w-4" aria-hidden="true" />清空缓存</Button>
      </div>
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <Surface className="bg-[var(--nv-bg-surface-soft)] p-3 shadow-none">
      <div className="text-xs text-[var(--nv-text-tertiary)]">{label}</div>
      <div className="mt-1 text-lg font-semibold text-[var(--nv-text-primary)]">{value}</div>
    </Surface>
  )
}

function SchedulerPanel() {
  const dialog = useDialog()
  const [cfg, setCfg] = useState<AdultSchedulerConfig>({ Enabled: false, DailyHour: 3, DailyMinute: 30, Concurrency: 2, OnlyUnscraped: true, Aggregated: false })
  const [lastRunAt, setLastRunAt] = useState('')
  const [lastTaskID, setLastTaskID] = useState('')
  const [saving, setSaving] = useState(false)
  const [running, setRunning] = useState(false)

  const load = useCallback(async () => {
    const response = await adultScraperApi.getScheduler()
    if (response.data.data.config) setCfg(response.data.data.config)
    setLastRunAt(response.data.data.last_run_at || '')
    setLastTaskID(response.data.data.last_task_id || '')
  }, [])

  useEffect(() => { void load() }, [load])

  const handleSave = async () => {
    setSaving(true)
    try {
      await adultScraperApi.updateScheduler(cfg)
      await dialog.alert({ title: '调度器配置已保存', variant: 'success' })
    } finally {
      setSaving(false)
    }
  }

  const handleRunNow = async () => {
    setRunning(true)
    try {
      const response = await adultScraperApi.triggerScheduler()
      await dialog.alert({ title: '任务已启动', message: response.data.data.task_id, variant: 'success' })
      await load()
    } catch (error: any) {
      await dialog.alert({ title: '启动失败', message: error?.response?.data?.error || error?.message, variant: 'error' })
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className="space-y-4">
      <Surface className="bg-[var(--nv-bg-surface-soft)] p-4 shadow-none">
        <div className="mb-3 grid grid-cols-1 gap-3 md:grid-cols-2">
          <LabelCheck label="启用每日定时刮削" checked={cfg.Enabled} onChange={(value) => setCfg({ ...cfg, Enabled: value })} />
          <LabelCheck label="只刮削未成功的媒体" checked={cfg.OnlyUnscraped} onChange={(value) => setCfg({ ...cfg, OnlyUnscraped: value })} />
          <LabelNumber label="执行小时 (0-23)" value={cfg.DailyHour} onChange={(value) => setCfg({ ...cfg, DailyHour: value })} min={0} max={23} />
          <LabelNumber label="执行分钟 (0-59)" value={cfg.DailyMinute} onChange={(value) => setCfg({ ...cfg, DailyMinute: value })} min={0} max={59} />
          <LabelNumber label="并发度" value={cfg.Concurrency} onChange={(value) => setCfg({ ...cfg, Concurrency: value })} min={1} max={8} />
          <LabelCheck label="使用聚合模式" checked={cfg.Aggregated} onChange={(value) => setCfg({ ...cfg, Aggregated: value })} />
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button type="button" variant="primary" size="sm" onClick={handleSave} loading={saving} disabled={saving}>保存配置</Button>
          <Button type="button" variant="secondary" size="sm" onClick={handleRunNow} loading={running} disabled={running}>{!running && <Play className="h-4 w-4" aria-hidden="true" />}立即触发一次</Button>
        </div>
      </Surface>

      <Surface className="bg-[var(--nv-bg-surface-soft)] p-3 text-sm text-[var(--nv-text-primary)] shadow-none">
        <div>最近运行时间: {lastRunAt ? new Date(lastRunAt).toLocaleString() : '从未执行'}</div>
        {lastTaskID && <div className="text-[var(--nv-text-tertiary)]">最近任务 ID: {lastTaskID}</div>}
      </Surface>
    </div>
  )
}

function ReportPanel() {
  const dialog = useDialog()
  const [days, setDays] = useState(7)
  const [report, setReport] = useState<AdultScrapeReport | null>(null)
  const [loading, setLoading] = useState(false)
  const [failedCount, setFailedCount] = useState(0)
  const [retrying, setRetrying] = useState(false)
  const [loadErr, setLoadErr] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setLoadErr('')
    try {
      const [reportResponse, failedResponse] = await Promise.all([
        adultScraperApi.getReport(days),
        adultScraperApi.getFailedItems(days),
      ])
      setReport(reportResponse.data.data)
      setFailedCount(failedResponse.data.count || 0)
    } catch (error: any) {
      setLoadErr(error?.response?.data?.error || error?.message || '加载报表失败')
    } finally {
      setLoading(false)
    }
  }, [days])

  useEffect(() => { void load() }, [load])

  const handleRetry = async () => {
    const ok = await dialog.confirm({ title: '重试失败记录', message: `确定重试最近 ${days} 天内的 ${failedCount} 条失败记录？`, confirmText: '重试', variant: 'warning' })
    if (!ok) return
    setRetrying(true)
    try {
      const response = await adultScraperApi.retryFailed({ days, concurrency: 2 })
      await dialog.alert({ title: '重试任务已启动', message: `task_id: ${response.data.data.task_id}（${response.data.data.retry_count} 条）`, variant: 'success' })
    } catch (error: any) {
      await dialog.alert({ title: '重试失败', message: error?.response?.data?.error || error?.message, variant: 'error' })
    } finally {
      setRetrying(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="text-[var(--nv-text-tertiary)]">时间范围：</span>
        <Select value={days} onChange={(event) => setDays(parseInt(event.target.value, 10))} className="w-auto min-w-32">
          <option value={1}>最近 1 天</option>
          <option value={7}>最近 7 天</option>
          <option value={30}>最近 30 天</option>
          <option value={0}>全部历史</option>
        </Select>
        <Button type="button" variant="secondary" size="sm" iconOnly onClick={() => void load()} title="刷新报表" aria-label="刷新番号刮削报表">
          {loading ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : <RefreshCw className="h-4 w-4" aria-hidden="true" />}
        </Button>
        {failedCount > 0 && (
          <Button type="button" variant="secondary" size="sm" className="ml-auto" onClick={handleRetry} disabled={retrying} loading={retrying}>
            {!retrying && <RotateCw className="h-4 w-4" aria-hidden="true" />}重试失败 {failedCount} 条
          </Button>
        )}
      </div>

      {loadErr && (
        <div className="rounded-[var(--nv-radius-control)] border px-3 py-2 text-xs text-[var(--nv-status-danger)]" style={{ borderColor: 'color-mix(in srgb, var(--nv-status-danger) 28%, transparent)', background: 'color-mix(in srgb, var(--nv-status-danger) 8%, transparent)' }}>
          ⚠️ {loadErr}
        </div>
      )}

      {!report && !loadErr && loading && (
        <div className="flex items-center justify-center gap-2 py-8 text-sm text-[var(--nv-text-tertiary)]"><Loader2 className="h-4 w-4 animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" />正在生成报表...</div>
      )}

      {report && (
        <>
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            <StatCard label="总处理数" value={report.total_processed ?? 0} />
            <StatCard label="成功" value={report.total_success ?? 0} />
            <StatCard label="失败" value={report.total_failed ?? 0} />
            <StatCard label="总成功率" value={`${((report.overall_rate ?? 0) * 100).toFixed(1)}%`} />
          </div>

          <div>
            <div className="mb-2 text-sm font-medium text-[var(--nv-text-primary)]">各数据源成功率</div>
            {(report.by_source ?? []).length === 0 ? (
              <Surface className="bg-[var(--nv-bg-surface-soft)] p-3 text-center text-xs text-[var(--nv-text-tertiary)] shadow-none">暂无数据（请先执行批量刮削或文件夹刮削生成历史记录）</Surface>
            ) : (
              <div className="space-y-1.5">
                {(report.by_source ?? []).map((source) => (
                  <Surface key={source.source} className="bg-[var(--nv-bg-surface-soft)] p-2.5 text-xs shadow-none">
                    <div className="mb-1 flex items-center justify-between gap-3">
                      <span className="font-mono text-[var(--nv-text-primary)]">{source.source}</span>
                      <span className="text-[var(--nv-text-tertiary)]">{source.success}/{source.total} ({(source.success_rate * 100).toFixed(1)}%)</span>
                    </div>
                    <div className="h-1 overflow-hidden rounded-full bg-[var(--nv-bg-control)]"><div className="h-full rounded-full bg-[var(--nv-status-success)]" style={{ width: `${source.success_rate * 100}%` }} /></div>
                  </Surface>
                ))}
              </div>
            )}
          </div>

          {(report.by_prefix ?? []).length > 0 && (
            <div>
              <div className="mb-2 text-sm font-medium text-[var(--nv-text-primary)]">TOP 番号前缀</div>
              <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
                {(report.by_prefix ?? []).slice(0, 8).map((prefix) => (
                  <Surface key={prefix.prefix} className="bg-[var(--nv-bg-surface-soft)] px-2 py-1.5 text-xs shadow-none">
                    <div className="font-mono text-[var(--nv-text-primary)]">{prefix.prefix}</div>
                    <div className="text-[var(--nv-text-tertiary)]">{prefix.success}/{prefix.total} · {(prefix.success_rate * 100).toFixed(0)}%</div>
                  </Surface>
                ))}
              </div>
            </div>
          )}

          {(report.top_failures ?? []).length > 0 && (
            <div>
              <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-[var(--nv-status-danger)]"><AlertTriangle className="h-4 w-4" aria-hidden="true" />最常失败番号</div>
              <div className="flex flex-wrap gap-1.5">{(report.top_failures ?? []).map((code) => <Tag key={code} tone="danger" className="font-mono">{code}</Tag>)}</div>
            </div>
          )}

          {(report.best_hours ?? []).length > 0 && (
            <div>
              <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-[var(--nv-status-success)]"><CheckCircle2 className="h-4 w-4" aria-hidden="true" />成功率最高时段（可用于调整定时执行时间）</div>
              <div className="flex flex-wrap gap-2">{(report.best_hours ?? []).map((hour) => <Tag key={hour} tone="success">{hour}:00</Tag>)}</div>
            </div>
          )}

          {(report.total_processed ?? 0) === 0 && (
            <div className="rounded-[var(--nv-radius-control)] border border-dashed border-[var(--nv-border-default)] p-4 text-center text-xs text-[var(--nv-text-tertiary)]">所选时间范围内没有刮削记录。开始一次批量刮削后，报表会自动填充。</div>
          )}
        </>
      )}
    </div>
  )
}

function LabelCheck({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] p-2 text-sm text-[var(--nv-text-primary)]">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} className="h-4 w-4 accent-[var(--nv-action-primary)]" />
      <span>{label}</span>
    </label>
  )
}

function LabelNumber({ label, value, onChange, min, max }: { label: string; value: number; onChange: (value: number) => void; min?: number; max?: number }) {
  return (
    <label className="flex items-center gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] p-2 text-sm">
      <span className="min-w-0 flex-1 text-[var(--nv-text-secondary)]">{label}</span>
      <Input type="number" value={value} onChange={(event) => onChange(parseInt(event.target.value, 10) || 0)} min={min} max={max} className="h-8 w-20 text-right" />
    </label>
  )
}
