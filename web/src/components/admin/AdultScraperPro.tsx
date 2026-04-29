// 番号刮削 - 批量刮削与运营中心
// 功能：
//   - 批量刮削任务启动 / 暂停 / 恢复 / 取消 / 进度实时跟踪
//   - 镜像状态查看 / 健康检查 / 自定义切换
//   - 元数据缓存管理
//   - 定时调度配置
//   - 失败分析报表 + 一键重试失败
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
import clsx from 'clsx'

type Tab = 'batch' | 'folder' | 'cookies' | 'mirrors' | 'cache' | 'scheduler' | 'report'

export default function AdultScraperProSection() {
  const [tab, setTab] = useState<Tab>('batch')

  return (
    <div className="rounded-xl p-5" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
      <div className="mb-4 flex items-center gap-2">
        <BarChart3 className="h-5 w-5 text-purple-400" />
        <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>番号刮削运营中心</h3>
      </div>

      <div className="mb-4 flex flex-wrap gap-2 pb-2" style={{ borderBottom: '1px solid var(--border-default)' }}>
        <TabButton active={tab === 'batch'} onClick={() => setTab('batch')} icon={<Play className="h-4 w-4" />} label="批量刮削" />
        <TabButton active={tab === 'folder'} onClick={() => setTab('folder')} icon={<FolderSearch className="h-4 w-4" />} label="文件夹刮削" />
        <TabButton active={tab === 'cookies'} onClick={() => setTab('cookies')} icon={<Cookie className="h-4 w-4" />} label="Cookie 登录" />
        <TabButton active={tab === 'mirrors'} onClick={() => setTab('mirrors')} icon={<Globe className="h-4 w-4" />} label="镜像管理" />
        <TabButton active={tab === 'cache'} onClick={() => setTab('cache')} icon={<Database className="h-4 w-4" />} label="缓存管理" />
        <TabButton active={tab === 'scheduler'} onClick={() => setTab('scheduler')} icon={<Clock className="h-4 w-4" />} label="定时调度" />
        <TabButton active={tab === 'report'} onClick={() => setTab('report')} icon={<TrendingUp className="h-4 w-4" />} label="分析报表" />
      </div>

      {tab === 'batch' && <BatchPanel />}
      {tab === 'folder' && <AdultFolderScraperPanel />}
      {tab === 'cookies' && <AdultCookieLoginPanel />}
      {tab === 'mirrors' && <MirrorsPanel />}
      {tab === 'cache' && <CachePanel />}
      {tab === 'scheduler' && <SchedulerPanel />}
      {tab === 'report' && <ReportPanel />}
    </div>
  )
}

function TabButton({
  active, onClick, icon, label,
}: {
  active: boolean; onClick: () => void; icon: React.ReactNode; label: string
}) {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
        active ? 'bg-purple-500/20 text-purple-500 dark:text-purple-300' : 'hover:bg-[var(--nav-hover-bg)]',
      )}
      style={!active ? { color: 'var(--text-secondary)' } : undefined}
    >
      {icon}
      {label}
    </button>
  )
}

// ==================== 批量刮削面板 ====================

function BatchPanel() {
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
      const res = await adultScraperApi.listBatchTasks()
      setTasks(res.data.data.active || [])
      setHistory(res.data.data.history || [])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadTasks()
    const timer = setInterval(loadTasks, 3000)
    return () => clearInterval(timer)
  }, [loadTasks])

  // WebSocket 订阅进度事件
  useEffect(() => {
    try {
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${proto}//${window.location.host}/ws`)
      ws.onmessage = (evt) => {
        try {
          const data = JSON.parse(evt.data)
          if (data?.type === 'adult_batch_progress' && data?.data?.task_id) {
            setProgressByTask((prev) => ({ ...prev, [data.data.task_id]: data.data }))
          }
          if (data?.type === 'adult_batch_completed') {
            loadTasks()
          }
        } catch {}
      }
      return () => ws.close()
    } catch {}
  }, [loadTasks])

  const handleStart = async () => {
    setStarting(true)
    try {
      await adultScraperApi.startBatch(opts)
      await loadTasks()
    } catch (err: any) {
      alert('启动失败: ' + (err?.response?.data?.error || err?.message))
    } finally {
      setStarting(false)
    }
  }

  return (
    <div className="space-y-4">
      {/* 启动控制 */}
      <div className="rounded-lg p-4" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
        <div className="mb-3 text-sm font-medium" style={{ color: 'var(--text-primary)' }}>启动新任务</div>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <LabelCheck label="只处理未刮削" checked={opts.only_unscraped} onChange={(v) => setOpts({ ...opts, only_unscraped: v })} />
          <LabelCheck label="预览模式（仅识别）" checked={opts.dry_run} onChange={(v) => setOpts({ ...opts, dry_run: v })} />
          <LabelCheck label="聚合模式（精刮，较慢）" checked={opts.aggregated} onChange={(v) => setOpts({ ...opts, aggregated: v })} />
          <LabelNumber label="并发度（1-8）" value={opts.concurrency} onChange={(v) => setOpts({ ...opts, concurrency: v })} min={1} max={8} />
        </div>
        <div className="mt-3 flex items-center gap-2">
          <button
            onClick={handleStart}
            disabled={starting}
            className="flex items-center gap-1.5 rounded-lg bg-purple-500 px-4 py-2 text-sm font-medium text-white hover:bg-purple-600 disabled:opacity-60"
          >
            {starting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
            启动全库批量刮削
          </button>
          <button onClick={loadTasks} className="rounded-lg p-2 hover:bg-[var(--nav-hover-bg)]" style={{ border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}>
            <RefreshCw className={clsx('h-4 w-4', loading && 'animate-spin')} />
          </button>
        </div>
      </div>

      {/* 活跃任务 */}
      <div>
        <div className="mb-2 flex items-center gap-2 text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
          <Loader2 className="h-4 w-4 animate-spin text-green-500" />
          进行中的任务（{tasks.length}）
        </div>
        {tasks.length === 0 ? (
          <div className="rounded-lg p-4 text-center text-sm" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}>暂无进行中的任务</div>
        ) : (
          <div className="space-y-2">
            {tasks.map((t) => (
              <TaskRow
                key={t.id}
                task={t}
                live={progressByTask[t.id]}
                onPause={async () => { await adultScraperApi.pauseBatch(t.id); loadTasks() }}
                onResume={async () => { await adultScraperApi.resumeBatch(t.id); loadTasks() }}
                onCancel={async () => { await adultScraperApi.cancelBatch(t.id); loadTasks() }}
              />
            ))}
          </div>
        )}
      </div>

      {/* 历史任务 */}
      <div>
        <div className="mb-2 flex items-center gap-2 text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
          <History className="h-4 w-4" style={{ color: 'var(--text-secondary)' }} />
          历史任务（最近 {history.length} 个）
        </div>
        {history.length === 0 ? (
          <div className="rounded-lg p-4 text-center text-sm" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}>暂无历史</div>
        ) : (
          <div className="space-y-2">
            {history.slice(0, 10).map((t) => (
              <TaskRow key={t.id} task={t} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
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

  const statusColor =
    task.status === 'completed' ? 'text-green-500'
      : task.status === 'failed' ? 'text-red-500'
        : task.status === 'cancelled' ? 'text-gray-400'
          : task.status === 'paused' ? 'text-yellow-500'
            : 'text-blue-500'

  return (
    <div className="rounded-lg p-3" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 text-sm">
            <span className={clsx('font-mono font-semibold', statusColor)}>{task.status.toUpperCase()}</span>
            <span style={{ color: 'var(--text-secondary)' }}>#{task.id.slice(0, 8)}</span>
            {task.aggregated && <span className="rounded bg-purple-500/20 px-1.5 text-xs text-purple-500 dark:text-purple-300">聚合</span>}
            {task.dry_run && <span className="rounded bg-gray-500/20 px-1.5 text-xs" style={{ color: 'var(--text-secondary)' }}>预览</span>}
          </div>
          {live?.media_title && (
            <div className="mt-1 truncate text-xs" style={{ color: 'var(--text-secondary)' }}>
              {live.status === 'failed' ? '❌' : live.status === 'success' ? '✅' : '⏳'} {live.code} · {live.media_title}
              {live.err_msg && <span className="ml-2 text-red-500">{live.err_msg}</span>}
            </div>
          )}
        </div>
        <div className="flex items-center gap-1">
          {task.status === 'running' && onPause && (
            <button onClick={onPause} className="rounded p-1.5 hover:bg-[var(--nav-hover-bg)]" style={{ color: 'var(--text-secondary)' }} title="暂停">
              <Pause className="h-4 w-4" />
            </button>
          )}
          {task.status === 'paused' && onResume && (
            <button onClick={onResume} className="rounded p-1.5 hover:bg-[var(--nav-hover-bg)]" style={{ color: 'var(--text-secondary)' }} title="恢复">
              <Play className="h-4 w-4" />
            </button>
          )}
          {(task.status === 'running' || task.status === 'paused') && onCancel && (
            <button onClick={onCancel} className="rounded p-1.5 text-red-500 hover:bg-red-500/10" title="取消">
              <Square className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      {/* 进度条 */}
      <div className="mt-2">
        <div className="h-1.5 overflow-hidden rounded-full" style={{ background: 'var(--border-default)' }}>
          <div className="h-full bg-gradient-to-r from-blue-400 to-purple-400 transition-all" style={{ width: `${percent}%` }} />
        </div>
        <div className="mt-1 flex items-center justify-between text-xs" style={{ color: 'var(--text-secondary)' }}>
          <span>{current} / {total} ({percent}%)</span>
          <span>
            <span className="text-green-500">✓ {success}</span>
            {' · '}
            <span className="text-red-500">✗ {failed}</span>
            {' · '}
            <span style={{ color: 'var(--text-secondary)' }}>跳过 {skipped}</span>
          </span>
        </div>
      </div>
    </div>
  )
}

// ==================== 镜像管理面板 ====================

function MirrorsPanel() {
  const [data, setData] = useState<Record<string, { mirrors: MirrorStatus[]; preferred: string }> | null>(null)
  const [lastHealthAt, setLastHealthAt] = useState('')
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await adultScraperApi.listMirrors()
      setData(res.data.data.sources)
      setLastHealthAt(res.data.data.last_health_at || '')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

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
      <div className="flex items-center justify-between">
        <div className="text-sm" style={{ color: 'var(--text-secondary)' }}>
          最近健康检查：{lastHealthAt ? new Date(lastHealthAt).toLocaleString() : '从未'}
        </div>
        <button
          onClick={handleHealthCheck}
          disabled={checking}
          className="flex items-center gap-1.5 rounded-lg bg-blue-500 px-3 py-1.5 text-sm text-white hover:bg-blue-600 disabled:opacity-60"
        >
          {checking ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          全部健康检查
        </button>
      </div>

      {loading ? (
        <div className="flex justify-center py-6"><Loader2 className="h-5 w-5 animate-spin" style={{ color: 'var(--text-secondary)' }} /></div>
      ) : (
        <div className="space-y-3">
          {data && Object.entries(data).map(([src, v]) => (
            <div key={src} className="rounded-lg p-3" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
              <div className="mb-2 flex items-center justify-between">
                <div className="font-medium uppercase" style={{ color: 'var(--text-primary)' }}>{src}</div>
                <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>首选：{v.preferred}</div>
              </div>
              <div className="space-y-1">
                {v.mirrors.map((m, i) => (
                  <div key={i} className="flex items-center justify-between rounded px-2 py-1 text-xs" style={{ background: 'var(--nav-hover-bg)' }}>
                    <div className="flex items-center gap-2">
                      {m.healthy ? <Wifi className="h-3.5 w-3.5 text-green-500" /> : <WifiOff className="h-3.5 w-3.5 text-red-500" />}
                      <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{m.url}</span>
                    </div>
                    <div style={{ color: 'var(--text-secondary)' }}>
                      {m.latency_ms > 0 && <span>{m.latency_ms}ms</span>}
                      {m.fail_count > 0 && <span className="ml-2 text-red-500">失败 {m.fail_count}次</span>}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ==================== 缓存管理面板 ====================

function CachePanel() {
  const [stats, setStats] = useState<{ size: number; max_size: number; total_hit: number; ttl: string } | null>(null)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await adultScraperApi.getCacheStats()
      setStats(res.data.data)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const handleClear = async () => {
    if (!confirm('确定清空所有番号元数据缓存？')) return
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
        <button onClick={load} className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm hover:bg-[var(--nav-hover-bg)]" style={{ border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          刷新
        </button>
        <button onClick={handleClear} className="flex items-center gap-1.5 rounded-lg bg-red-500/80 px-3 py-1.5 text-sm text-white hover:bg-red-600">
          <Trash2 className="h-4 w-4" />
          清空缓存
        </button>
      </div>
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg p-3" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
      <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>{label}</div>
      <div className="mt-1 text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>{value}</div>
    </div>
  )
}

// ==================== 定时调度面板 ====================

function SchedulerPanel() {
  const [cfg, setCfg] = useState<AdultSchedulerConfig>({
    Enabled: false,
    DailyHour: 3,
    DailyMinute: 30,
    Concurrency: 2,
    OnlyUnscraped: true,
    Aggregated: false,
  })
  const [lastRunAt, setLastRunAt] = useState('')
  const [lastTaskID, setLastTaskID] = useState('')
  const [saving, setSaving] = useState(false)
  const [running, setRunning] = useState(false)

  const load = useCallback(async () => {
    const res = await adultScraperApi.getScheduler()
    if (res.data.data.config) {
      setCfg(res.data.data.config)
    }
    setLastRunAt(res.data.data.last_run_at || '')
    setLastTaskID(res.data.data.last_task_id || '')
  }, [])

  useEffect(() => { load() }, [load])

  const handleSave = async () => {
    setSaving(true)
    try {
      await adultScraperApi.updateScheduler(cfg)
      alert('调度器配置已保存')
    } finally {
      setSaving(false)
    }
  }

  const handleRunNow = async () => {
    setRunning(true)
    try {
      const res = await adultScraperApi.triggerScheduler()
      alert('任务已启动：' + res.data.data.task_id)
      await load()
    } catch (e: any) {
      alert(e?.response?.data?.error || e?.message)
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="rounded-lg p-4" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
        <div className="mb-3 grid grid-cols-1 gap-3 md:grid-cols-2">
          <LabelCheck label="启用每日定时刮削" checked={cfg.Enabled} onChange={(v) => setCfg({ ...cfg, Enabled: v })} />
          <LabelCheck label="只刮削未成功的媒体" checked={cfg.OnlyUnscraped} onChange={(v) => setCfg({ ...cfg, OnlyUnscraped: v })} />
          <LabelNumber label="执行小时 (0-23)" value={cfg.DailyHour} onChange={(v) => setCfg({ ...cfg, DailyHour: v })} min={0} max={23} />
          <LabelNumber label="执行分钟 (0-59)" value={cfg.DailyMinute} onChange={(v) => setCfg({ ...cfg, DailyMinute: v })} min={0} max={59} />
          <LabelNumber label="并发度" value={cfg.Concurrency} onChange={(v) => setCfg({ ...cfg, Concurrency: v })} min={1} max={8} />
          <LabelCheck label="使用聚合模式" checked={cfg.Aggregated} onChange={(v) => setCfg({ ...cfg, Aggregated: v })} />
        </div>
        <div className="flex items-center gap-2">
          <button onClick={handleSave} disabled={saving} className="rounded-lg bg-blue-500 px-4 py-1.5 text-sm text-white hover:bg-blue-600 disabled:opacity-60">
            {saving ? '保存中...' : '保存配置'}
          </button>
          <button onClick={handleRunNow} disabled={running} className="flex items-center gap-1.5 rounded-lg bg-purple-500 px-3 py-1.5 text-sm text-white hover:bg-purple-600 disabled:opacity-60">
            {running ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
            立即触发一次
          </button>
        </div>
      </div>

      <div className="rounded-lg p-3 text-sm" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}>
        <div>最近运行时间: {lastRunAt ? new Date(lastRunAt).toLocaleString() : '从未执行'}</div>
        {lastTaskID && <div style={{ color: 'var(--text-secondary)' }}>最近任务 ID: {lastTaskID}</div>}
      </div>
    </div>
  )
}

// ==================== 分析报表面板 ====================

function ReportPanel() {
  const [days, setDays] = useState(7)
  const [report, setReport] = useState<AdultScrapeReport | null>(null)
  const [loading, setLoading] = useState(false)
  const [failedCount, setFailedCount] = useState(0)
  const [retrying, setRetrying] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [r1, r2] = await Promise.all([
        adultScraperApi.getReport(days),
        adultScraperApi.getFailedItems(days),
      ])
      setReport(r1.data.data)
      setFailedCount(r2.data.count || 0)
    } finally {
      setLoading(false)
    }
  }, [days])

  useEffect(() => { load() }, [load])

  const handleRetry = async () => {
    if (!confirm(`确定重试最近 ${days} 天内的 ${failedCount} 条失败记录？`)) return
    setRetrying(true)
    try {
      const res = await adultScraperApi.retryFailed({ days, concurrency: 2 })
      alert(`已启动重试任务: ${res.data.data.task_id}（${res.data.data.retry_count} 条）`)
    } catch (e: any) {
      alert(e?.response?.data?.error || e?.message)
    } finally {
      setRetrying(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 text-sm">
        <span style={{ color: 'var(--text-secondary)' }}>时间范围：</span>
        <select
          value={days}
          onChange={(e) => setDays(parseInt(e.target.value, 10))}
          className="rounded-lg px-2 py-1"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
        >
          <option value={1}>最近 1 天</option>
          <option value={7}>最近 7 天</option>
          <option value={30}>最近 30 天</option>
          <option value={0}>全部历史</option>
        </select>
        <button onClick={load} className="rounded-lg p-1.5 hover:bg-[var(--nav-hover-bg)]" style={{ border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}>
          {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
        </button>
        {failedCount > 0 && (
          <button onClick={handleRetry} disabled={retrying} className="ml-auto flex items-center gap-1.5 rounded-lg bg-orange-500/80 px-3 py-1.5 text-sm text-white hover:bg-orange-600 disabled:opacity-60">
            {retrying ? <Loader2 className="h-4 w-4 animate-spin" /> : <RotateCw className="h-4 w-4" />}
            重试失败 {failedCount} 条
          </button>
        )}
      </div>

      {report && (
        <>
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            <StatCard label="总处理数" value={report.total_processed} />
            <StatCard label="成功" value={report.total_success} />
            <StatCard label="失败" value={report.total_failed} />
            <StatCard label="总成功率" value={`${(report.overall_rate * 100).toFixed(1)}%`} />
          </div>

          <div>
            <div className="mb-2 text-sm font-medium" style={{ color: 'var(--text-primary)' }}>各数据源成功率</div>
            <div className="space-y-1">
              {report.by_source.map((s) => (
                <div key={s.source} className="rounded p-2 text-xs" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
                  <div className="mb-1 flex items-center justify-between">
                    <span className="font-mono" style={{ color: 'var(--text-primary)' }}>{s.source}</span>
                    <span style={{ color: 'var(--text-secondary)' }}>{s.success}/{s.total} ({(s.success_rate * 100).toFixed(1)}%)</span>
                  </div>
                  <div className="h-1 overflow-hidden rounded-full" style={{ background: 'var(--border-default)' }}>
                    <div className="h-full bg-green-500" style={{ width: `${s.success_rate * 100}%` }} />
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div>
            <div className="mb-2 text-sm font-medium" style={{ color: 'var(--text-primary)' }}>TOP 番号前缀</div>
            <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
              {report.by_prefix.slice(0, 8).map((p) => (
                <div key={p.prefix} className="rounded px-2 py-1 text-xs" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
                  <div className="font-mono" style={{ color: 'var(--text-primary)' }}>{p.prefix}</div>
                  <div style={{ color: 'var(--text-secondary)' }}>{p.success}/{p.total} · {(p.success_rate * 100).toFixed(0)}%</div>
                </div>
              ))}
            </div>
          </div>

          {report.top_failures.length > 0 && (
            <div>
              <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-red-500">
                <AlertTriangle className="h-4 w-4" />
                最常失败番号
              </div>
              <div className="flex flex-wrap gap-1">
                {report.top_failures.map((c) => (
                  <span key={c} className="rounded bg-red-500/15 px-2 py-0.5 text-xs font-mono text-red-500 dark:text-red-300">{c}</span>
                ))}
              </div>
            </div>
          )}

          {report.best_hours.length > 0 && (
            <div>
              <div className="mb-2 flex items-center gap-1.5 text-sm font-medium text-green-600 dark:text-green-300">
                <CheckCircle2 className="h-4 w-4" />
                成功率最高时段（可用于调整定时执行时间）
              </div>
              <div className="flex gap-2">
                {report.best_hours.map((h) => (
                  <span key={h} className="rounded bg-green-500/15 px-2 py-0.5 text-xs text-green-600 dark:text-green-300">{h}:00</span>
                ))}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}

// ==================== 公共表单组件 ====================

function LabelCheck({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center gap-2 rounded p-2 text-sm" style={{ background: 'var(--nav-hover-bg)', color: 'var(--text-primary)' }}>
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} className="h-4 w-4 accent-purple-500" />
      <span>{label}</span>
    </label>
  )
}

function LabelNumber({ label, value, onChange, min, max }: { label: string; value: number; onChange: (v: number) => void; min?: number; max?: number }) {
  return (
    <label className="flex items-center gap-2 rounded p-2 text-sm" style={{ background: 'var(--nav-hover-bg)' }}>
      <span className="min-w-0 flex-1" style={{ color: 'var(--text-secondary)' }}>{label}</span>
      <input
        type="number"
        value={value}
        onChange={(e) => onChange(parseInt(e.target.value, 10) || 0)}
        min={min}
        max={max}
        className="w-20 rounded px-2 py-1 text-right"
        style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
      />
    </label>
  )
}
