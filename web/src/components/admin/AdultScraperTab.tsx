import { useCallback, useEffect, useMemo, useState } from 'react'
import { adultScraperApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type {
  AdultBatchItemResult,
  AdultBatchTask,
  AdultCacheStats,
  AdultLazyResultView,
  AdultLazyStatus,
  AdultLazyTaskView,
  AdultSchedulerStatus,
  AdultScrapeReport,
  AdultScraperConfig,
  FolderBatchTask,
  FolderScanResult,
} from '@/api/adultScraper'
import {
  AlertTriangle,
  BarChart3,
  CheckCircle2,
  ChevronDown,
  Clock,
  Database,
  FolderOpen,
  Loader2,
  Play,
  RefreshCw,
  RotateCcw,
  Search,
  Settings,
  Shield,
  Square,
  Trash2,
  XCircle,
  Zap,
} from 'lucide-react'
import clsx from 'clsx'
import FileBrowser from '@/components/FileBrowser'
import { Button, Input, Surface, Tag, type TagTone } from '@/components/design-system'
import { AdminStatus, type AdminStatusTone } from '@/components/admin/AdminPrimitives'

type Notice = { type: 'success' | 'error' | 'info'; text: string }

const LAZY_DEFAULTS = {
  minRequestInterval: 1500,
  maxRequestInterval: 3000,
  concurrency: 2,
  aggregated: true,
}

const taskStatusLabel: Record<string, string> = {
  running: '正在懒人刮削',
  paused: '已暂停',
  cancelled: '已停止',
  completed: '已完成',
  failed: '失败',
}

function formatTime(value?: string) {
  if (!value) return '--'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '--' : date.toLocaleString()
}

function progressOf(task?: AdultLazyTaskView | null) {
  if (!task || task.total <= 0) return 0
  return Math.min(100, Math.round((task.current / task.total) * 100))
}

function isScraperUsable(config: AdultScraperConfig | null) {
  if (!config?.enabled) return false
  return (config.sources || []).some((source) => source.enabled)
}

function resultTone(status: string): TagTone {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  return 'warning'
}

function taskTone(status?: string): TagTone {
  if (status === 'completed') return 'success'
  if (status === 'failed' || status === 'cancelled') return 'danger'
  if (status === 'running') return 'brand'
  return 'warning'
}

export default function AdultScraperSection() {
  const [config, setConfig] = useState<AdultScraperConfig | null>(null)
  const [folderPath, setFolderPath] = useState('')
  const [folderScan, setFolderScan] = useState<FolderScanResult | null>(null)
  const [tasks, setTasks] = useState<FolderBatchTask[]>([])
  const [mediaTasks, setMediaTasks] = useState<AdultBatchTask[]>([])
  const [failedItems, setFailedItems] = useState<AdultBatchItemResult[]>([])
  const [report, setReport] = useState<AdultScrapeReport | null>(null)
  const [schedulerInfo, setSchedulerInfo] = useState<AdultSchedulerStatus | null>(null)
  const [cacheStats, setCacheStats] = useState<AdultCacheStats | null>(null)
  const [currentTaskView, setCurrentTaskView] = useState<AdultLazyTaskView | null>(null)
  const [recentResultViews, setRecentResultViews] = useState<AdultLazyResultView[]>([])
  const [folderFailedCount, setFolderFailedCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const [scanning, setScanning] = useState(false)
  const [starting, setStarting] = useState(false)
  const [stopping, setStopping] = useState(false)
  const [opsLoading, setOpsLoading] = useState(false)
  const [retryingFailed, setRetryingFailed] = useState(false)
  const [triggeringSchedule, setTriggeringSchedule] = useState(false)
  const [clearingCache, setClearingCache] = useState(false)
  const [showFolderBrowser, setShowFolderBrowser] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [showDiagnostics, setShowDiagnostics] = useState(false)
  const [notice, setNotice] = useState<Notice | null>(null)
  const { on, off } = useWebSocket()

  const showNotice = useCallback((type: Notice['type'], text: string) => {
    setNotice({ type, text })
    window.setTimeout(() => setNotice(null), 4500)
  }, [])

  const loadConfig = useCallback(async () => {
    try {
      const res = await adultScraperApi.getConfig()
      setConfig(res.data.data)
    } catch {
      showNotice('error', '加载刮削配置失败')
    }
  }, [showNotice])

  const applyLazyStatus = useCallback((data: AdultLazyStatus) => {
    setTasks([...(data.folder_tasks?.active || []), ...(data.folder_tasks?.history || [])])
    setMediaTasks([...(data.media_tasks?.active || []), ...(data.media_tasks?.history || [])])
    setReport(data.report || null)
    setFailedItems(data.failed_items || [])
    setSchedulerInfo(data.scheduler || null)
    setCacheStats(data.cache?.stats || null)
    setCurrentTaskView(data.current_task || null)
    setRecentResultViews(data.recent_results || [])
    setFolderFailedCount(data.folder_failed_count || 0)
  }, [])

  const loadTasks = useCallback(async () => {
    try {
      const res = await adultScraperApi.getLazyStatus(7)
      applyLazyStatus(res.data.data)
    } catch {
      // 轮询失败不打扰用户
    }
  }, [applyLazyStatus])

  const loadOperations = useCallback(async () => {
    setOpsLoading(true)
    try {
      const res = await adultScraperApi.getLazyStatus(7)
      applyLazyStatus(res.data.data)
    } finally {
      setOpsLoading(false)
    }
  }, [applyLazyStatus])

  useEffect(() => {
    let mounted = true
    const init = async () => {
      setLoading(true)
      await Promise.all([loadConfig(), loadTasks()])
      if (mounted) setLoading(false)
    }
    void init()
    return () => { mounted = false }
  }, [loadConfig, loadTasks])

  useEffect(() => {
    const timer = window.setInterval(loadTasks, 10000)
    return () => window.clearInterval(timer)
  }, [loadTasks])

  useEffect(() => {
    let refreshTimer: number | null = null
    const refresh = () => {
      if (refreshTimer !== null) window.clearTimeout(refreshTimer)
      refreshTimer = window.setTimeout(() => {
        void loadTasks()
      }, 250)
    }
    const events = [
      WS_EVENTS.ADULT_BATCH_STARTED,
      WS_EVENTS.ADULT_BATCH_PROGRESS,
      WS_EVENTS.ADULT_BATCH_COMPLETED,
      WS_EVENTS.ADULT_BATCH_FAILED,
      WS_EVENTS.ADULT_BATCH_CANCELLED,
      WS_EVENTS.ADULT_FOLDER_BATCH_STARTED,
      WS_EVENTS.ADULT_FOLDER_BATCH_PROGRESS,
      WS_EVENTS.ADULT_FOLDER_BATCH_COMPLETED,
      WS_EVENTS.ADULT_FOLDER_BATCH_FAILED,
      WS_EVENTS.ADULT_FOLDER_BATCH_CANCELLED,
    ]
    events.forEach((event) => on(event, refresh))
    return () => {
      if (refreshTimer !== null) window.clearTimeout(refreshTimer)
      events.forEach((event) => off(event, refresh))
    }
  }, [loadTasks, off, on])

  useEffect(() => {
    if (showAdvanced) void loadOperations()
  }, [loadOperations, showAdvanced])

  const unifiedTasks = useMemo<AdultLazyTaskView[]>(() => {
    const folder = tasks.map((task) => ({
      id: task.id,
      kind: 'folder' as const,
      status: task.status,
      total: task.total,
      current: task.current,
      success: task.success,
      failed: task.failed,
      skipped: task.skipped,
      started_at: task.started_at,
      finished_at: task.finished_at,
      aggregated: task.aggregated,
      concurrency: task.concurrency,
    }))
    const media = mediaTasks.map((task) => ({
      id: task.id,
      kind: 'library' as const,
      status: task.status,
      total: task.total,
      current: task.current,
      success: task.success,
      failed: task.failed,
      skipped: task.skipped,
      started_at: task.started_at,
      finished_at: task.finished_at,
      aggregated: task.aggregated,
      concurrency: task.concurrency,
    }))
    return [...folder, ...media].sort((a, b) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime())
  }, [mediaTasks, tasks])

  const runningTask = currentTaskView?.status === 'running'
    ? currentTaskView
    : unifiedTasks.find((task) => task.status === 'running') || null
  const currentTask = currentTaskView || runningTask || unifiedTasks[0] || null
  const results = recentResultViews
  const successCount = results.filter((item) => item.status === 'success').length
  const failedCount = results.filter((item) => item.status === 'failed').length
  const skippedCount = results.filter((item) => item.status === 'skipped').length
  const taskProgress = progressOf(currentTask)
  const usable = isScraperUsable(config)

  const scanCurrentFolder = async (silent = false) => {
    const path = folderPath.trim()
    if (!path) {
      if (!silent) showNotice('error', '请先选择或输入要刮削的目录')
      return null
    }
    setScanning(true)
    try {
      const res = await adultScraperApi.scanFolder(path, true, 0)
      setFolderScan(res.data.data)
      if (!silent) {
        showNotice('success', `扫描完成：识别到 ${res.data.data.with_code} / ${res.data.data.total} 个可处理视频`)
      }
      return res.data.data
    } catch (err: any) {
      showNotice('error', err?.response?.data?.error || '目录扫描失败')
      return null
    } finally {
      setScanning(false)
    }
  }

  const startLazyScrape = async () => {
    const path = folderPath.trim()
    if (!path) {
      showNotice('error', '请先选择或输入要刮削的目录')
      return
    }
    if (runningTask) {
      showNotice('info', '已有刮削任务正在运行')
      return
    }

    setStarting(true)
    try {
      const res = await adultScraperApi.startLazy({
        path,
        recursive: true,
        max_depth: 0,
        aggregated: LAZY_DEFAULTS.aggregated,
        concurrency: LAZY_DEFAULTS.concurrency,
      })
      setFolderScan(res.data.data.scan)
      if (res.data.data.queued === 0) {
        showNotice('info', '没有发现需要刮削的新视频，已有 NFO 的文件会自动跳过')
      } else {
        showNotice('success', `已启动懒人刮削：${res.data.data.queued} 个视频，后台自动处理`)
      }
      await Promise.all([loadConfig(), loadTasks()])
    } catch (err: any) {
      showNotice('error', err?.response?.data?.error || '启动懒人刮削失败')
    } finally {
      setStarting(false)
    }
  }

  const stopRunningTask = async () => {
    if (!runningTask) return
    setStopping(true)
    try {
      if (runningTask.kind === 'folder') {
        await adultScraperApi.cancelFolderBatch(runningTask.id)
      } else {
        await adultScraperApi.cancelBatch(runningTask.id)
      }
      showNotice('info', '已请求停止当前刮削任务')
      await loadTasks()
    } catch (err: any) {
      showNotice('error', err?.response?.data?.error || '停止任务失败')
    } finally {
      setStopping(false)
    }
  }

  const retryFailedItems = async () => {
    setRetryingFailed(true)
    try {
      const res = await adultScraperApi.retryLazyFailed({
        days: 7,
        concurrency: LAZY_DEFAULTS.concurrency,
        aggregated: LAZY_DEFAULTS.aggregated,
      })
      const data = res.data.data
      if (data.retry_count === 0) {
        showNotice('info', '没有可重试的失败项')
      } else {
        showNotice('success', `已提交失败重试：${data.retry_count} 个条目（目录 ${data.folder_retry_count} / 媒体库 ${data.media_retry_count}）`)
      }
      await Promise.all([loadTasks(), loadOperations()])
    } catch (err: any) {
      showNotice('error', err?.response?.data?.error || '提交失败重试失败')
    } finally {
      setRetryingFailed(false)
    }
  }

  const triggerSchedulerNow = async () => {
    setTriggeringSchedule(true)
    try {
      await adultScraperApi.triggerScheduler()
      showNotice('success', '已触发一次运营调度任务')
      await Promise.all([loadTasks(), loadOperations()])
    } catch (err: any) {
      showNotice('error', err?.response?.data?.error || '触发调度失败')
    } finally {
      setTriggeringSchedule(false)
    }
  }

  const clearMetadataCache = async () => {
    setClearingCache(true)
    try {
      await adultScraperApi.clearCache()
      showNotice('success', '刮削缓存已清空')
      await loadOperations()
    } catch (err: any) {
      showNotice('error', err?.response?.data?.error || '清空缓存失败')
    } finally {
      setClearingCache(false)
    }
  }

  const resetLazyDefaults = async () => {
    try {
      await adultScraperApi.updateConfig({
        enabled: true,
        enable_javbus: true,
        enable_javdb: true,
        min_request_interval: LAZY_DEFAULTS.minRequestInterval,
        max_request_interval: LAZY_DEFAULTS.maxRequestInterval,
      })
      await loadConfig()
      showNotice('success', '已恢复懒人模式默认配置')
    } catch {
      showNotice('error', '恢复默认配置失败')
    }
  }

  if (loading) {
    return (
      <Surface className="flex min-h-40 items-center justify-center p-8">
        <div className="flex items-center gap-2 text-sm text-[var(--nv-text-tertiary)]">
          <Loader2 size={18} className="animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" />
          加载懒人刮削模块中...
        </div>
      </Surface>
    )
  }

  const noticeTone: AdminStatusTone = notice?.type === 'success'
    ? 'success'
    : notice?.type === 'error'
      ? 'danger'
      : 'active'

  return (
    <>
      <section className="space-y-4">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h2 className="flex items-center gap-2 text-lg font-semibold text-[var(--nv-text-primary)]">
              <Shield size={20} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
              懒人刮削
            </h2>
            <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
              选择目录并开始后，系统会自动识别番号、跳过已完成文件，并在后台写入 NFO、图片与元数据。
            </p>
          </div>
          <Tag tone={usable ? 'success' : 'warning'}>
            {usable ? '开箱即用已就绪' : '首次开始时自动初始化'}
          </Tag>
        </div>

        {notice && (
          <AdminStatus tone={noticeTone} className="w-full justify-start px-3.5 py-2.5">
            {notice.type === 'success'
              ? <CheckCircle2 size={15} aria-hidden="true" />
              : notice.type === 'error'
                ? <XCircle size={15} aria-hidden="true" />
                : <AlertTriangle size={15} aria-hidden="true" />}
            {notice.text}
          </AdminStatus>
        )}

        <Surface className="overflow-hidden p-0">
          <div className="border-b border-[var(--nv-border-subtle)] p-4 sm:p-5">
            <div className="flex flex-col gap-3 lg:flex-row">
              <div className="relative min-w-0 flex-1">
                <FolderOpen
                  size={16}
                  className="pointer-events-none absolute left-3 top-1/2 z-10 -translate-y-1/2 text-[var(--nv-text-tertiary)]"
                  aria-hidden="true"
                />
                <Input
                  value={folderPath}
                  onChange={(event) => {
                    setFolderPath(event.target.value)
                    setFolderScan(null)
                  }}
                  className="pl-9"
                  placeholder="选择或输入待刮削目录，例如 D:\\video"
                />
              </div>
              <div className="flex flex-wrap gap-2">
                <Button type="button" variant="secondary" onClick={() => setShowFolderBrowser(true)}>
                  <FolderOpen size={15} aria-hidden="true" />
                  选择目录
                </Button>
                <Button
                  type="button"
                  variant="secondary"
                  onClick={() => void scanCurrentFolder(false)}
                  disabled={scanning || !folderPath.trim()}
                >
                  {scanning ? <Loader2 size={15} className="animate-spin" /> : <Search size={15} />}
                  预扫描
                </Button>
                <Button
                  type="button"
                  variant="primary"
                  onClick={() => void startLazyScrape()}
                  disabled={starting || scanning || Boolean(runningTask)}
                >
                  {starting ? <Loader2 size={16} className="animate-spin" /> : <Play size={16} />}
                  一键开始
                </Button>
                {runningTask && (
                  <Button type="button" variant="danger" onClick={() => void stopRunningTask()} disabled={stopping}>
                    {stopping ? <Loader2 size={14} className="animate-spin" /> : <Square size={14} />}
                    停止
                  </Button>
                )}
              </div>
            </div>
          </div>

          <div className="grid gap-4 p-4 sm:p-5 xl:grid-cols-[minmax(0,1fr)_340px]">
            <div className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <StatCard label="视频总数" value={folderScan?.total ?? currentTask?.total ?? 0} tone="brand" />
                <StatCard label="识别番号" value={folderScan?.with_code ?? 0} tone="success" />
                <StatCard label="已跳过" value={currentTask?.skipped ?? skippedCount} tone="warning" />
                <StatCard label="失败" value={currentTask?.failed ?? failedCount} tone="danger" />
              </div>

              <div className="rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
                <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                  <div className="flex items-center gap-2">
                    <div className={clsx(
                      'flex h-9 w-9 items-center justify-center rounded-full border',
                      runningTask
                        ? 'border-[var(--nv-border-default)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]'
                        : 'border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] text-[var(--nv-text-tertiary)]',
                    )}>
                      {runningTask ? <Loader2 size={18} className="animate-spin" /> : <Database size={17} />}
                    </div>
                    <div>
                      <p className="text-sm font-semibold text-[var(--nv-text-primary)]">
                        {currentTask ? taskStatusLabel[currentTask.status] || currentTask.status : '等待开始'}
                      </p>
                      <p className="text-xs text-[var(--nv-text-tertiary)]">
                        {currentTask ? `开始时间：${formatTime(currentTask.started_at)}` : '选择目录后一键开始，后续自动后台运行'}
                      </p>
                    </div>
                  </div>
                  <Tag tone={taskTone(currentTask?.status)}>{taskProgress}%</Tag>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-[var(--nv-bg-control)]">
                  <div
                    className="h-full rounded-full bg-[var(--nv-action-primary)] transition-[width] duration-300 motion-reduce:transition-none"
                    style={{ width: `${taskProgress}%` }}
                  />
                </div>
                <div className="mt-3 grid grid-cols-3 gap-2 text-center text-xs text-[var(--nv-text-secondary)]">
                  <span>成功 <b className="font-mono text-[var(--nv-status-success)]">{currentTask?.success ?? successCount}</b></span>
                  <span>失败 <b className="font-mono text-[var(--nv-status-danger)]">{currentTask?.failed ?? failedCount}</b></span>
                  <span>跳过 <b className="font-mono text-[var(--nv-status-warning)]">{currentTask?.skipped ?? skippedCount}</b></span>
                </div>
              </div>

              <div className="rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
                <div className="mb-3 flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
                  <Zap size={15} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                  默认自动化策略
                </div>
                <div className="grid gap-2 text-xs text-[var(--nv-text-secondary)] md:grid-cols-2">
                  <LazyRule text="自动启用通用数据源，无需手动配置镜像" />
                  <LazyRule text="递归扫描目录，自动识别番号文件" />
                  <LazyRule text="已有 NFO 的视频自动跳过，避免重复刮削" />
                  <LazyRule text="聚合优先，失败自动降级串行多源" />
                  <LazyRule text="默认并发 2，自动控制请求间隔" />
                  <LazyRule text="刮削完成后自动写入 NFO、封面与背景图" />
                </div>
              </div>
            </div>

            <div className="rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
                  <Clock size={15} aria-hidden="true" />
                  最近结果
                </div>
                <Button type="button" variant="ghost" size="sm" onClick={() => void loadTasks()}>
                  <RefreshCw size={13} aria-hidden="true" />
                  刷新
                </Button>
              </div>
              <div className="max-h-[520px] space-y-1 overflow-auto pr-1 text-xs">
                {results.length === 0 ? (
                  <div className="rounded-[var(--nv-radius-control)] border border-dashed border-[var(--nv-border-default)] p-8 text-center text-[var(--nv-text-tertiary)]">
                    暂无刮削记录
                  </div>
                ) : results.slice(0, 100).map((item, index) => (
                  <div
                    key={`${item.code}-${item.path}-${index}`}
                    className="flex items-start gap-2 rounded-[var(--nv-radius-control)] px-2 py-2 transition-colors hover:bg-[var(--nv-bg-hover)]"
                    title={item.message || item.path}
                  >
                    <span
                      className="mt-1.5 h-2 w-2 shrink-0 rounded-full"
                      style={{
                        background: item.status === 'success'
                          ? 'var(--nv-status-success)'
                          : item.status === 'failed'
                            ? 'var(--nv-status-danger)'
                            : 'var(--nv-status-warning)',
                      }}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-mono font-medium text-[var(--nv-text-primary)]">
                          {item.code || 'UNKNOWN'}
                        </span>
                        {item.source && <Tag tone="neutral" className="text-[10px]">{item.source}</Tag>}
                      </div>
                      <div className="truncate text-[var(--nv-text-tertiary)]">
                        {item.status === 'failed' ? (item.message || item.title || item.path) : (item.title || item.message || item.path)}
                      </div>
                    </div>
                    <Tag tone={resultTone(item.status)} className="shrink-0 text-[10px]">
                      {item.status === 'success' ? '成功' : item.status === 'failed' ? '失败' : '跳过'}
                    </Tag>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </Surface>

        <CollapsibleSection
          open={showAdvanced}
          onToggle={() => setShowAdvanced((value) => !value)}
          icon={<Settings size={15} />}
          title="进阶运营中心"
          hint="默认隐藏；用于失败重试、报表、调度、缓存和诊断"
        >
          {opsLoading && (
            <AdminStatus tone="active" className="mb-3 w-full justify-start px-3 py-2">
              <Loader2 size={13} className="animate-spin" />
              正在加载运营数据...
            </AdminStatus>
          )}

          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <OperationCard icon={<BarChart3 size={15} />} title="7 天报表">
              <div className="space-y-1 text-xs text-[var(--nv-text-secondary)]">
                <p>处理：<b className="font-mono text-[var(--nv-action-primary)]">{report?.total_processed ?? 0}</b></p>
                <p>成功：<b className="font-mono text-[var(--nv-status-success)]">{report?.total_success ?? 0}</b></p>
                <p>失败：<b className="font-mono text-[var(--nv-status-danger)]">{report?.total_failed ?? 0}</b></p>
                <p>成功率：<b className="font-mono text-[var(--nv-text-primary)]">{Math.round((report?.overall_rate || 0) * 100)}%</b></p>
              </div>
            </OperationCard>

            <OperationCard icon={<RotateCcw size={15} />} title="失败重试">
              <p className="text-xs text-[var(--nv-text-tertiary)]">
                最近 7 天失败项：<b className="font-mono text-[var(--nv-status-danger)]">{failedItems.length + folderFailedCount}</b>
                <span className="ml-1 text-[10px]">目录 {folderFailedCount} / 媒体库 {failedItems.length}</span>
              </p>
              <Button
                type="button"
                variant="primary"
                size="sm"
                className="mt-3"
                onClick={() => void retryFailedItems()}
                disabled={retryingFailed || failedItems.length + folderFailedCount === 0}
              >
                {retryingFailed ? <Loader2 size={12} className="animate-spin" /> : <RotateCcw size={12} />}
                一键重试失败
              </Button>
            </OperationCard>

            <OperationCard icon={<Clock size={15} />} title="定时调度">
              <div className="space-y-1 text-xs text-[var(--nv-text-secondary)]">
                <p>状态：<b className={schedulerInfo?.config?.Enabled ? 'text-[var(--nv-status-success)]' : 'text-[var(--nv-text-tertiary)]'}>{schedulerInfo?.config?.Enabled ? '启用' : '关闭'}</b></p>
                <p>上次运行：{formatTime(schedulerInfo?.last_run_at)}</p>
              </div>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                className="mt-3"
                onClick={() => void triggerSchedulerNow()}
                disabled={triggeringSchedule}
              >
                {triggeringSchedule ? <Loader2 size={12} className="animate-spin" /> : <Play size={12} />}
                立即运行一次
              </Button>
            </OperationCard>

            <OperationCard icon={<Database size={15} />} title="缓存">
              <div className="space-y-1 text-xs text-[var(--nv-text-secondary)]">
                <p>条目：<b className="font-mono text-[var(--nv-action-primary)]">{cacheStats?.size ?? 0}</b> / {cacheStats?.max_size ?? 0}</p>
                <p>命中：<b className="font-mono text-[var(--nv-status-success)]">{cacheStats?.total_hit ?? 0}</b></p>
              </div>
              <Button
                type="button"
                variant="danger"
                size="sm"
                className="mt-3"
                onClick={() => void clearMetadataCache()}
                disabled={clearingCache}
              >
                {clearingCache ? <Loader2 size={12} className="animate-spin" /> : <Trash2 size={12} />}
                清空缓存
              </Button>
            </OperationCard>
          </div>

          <div className="mt-3 grid gap-3 xl:grid-cols-2">
            <OperationCard title="历史任务" icon={<Clock size={15} />}>
              <div className="mb-3 flex justify-end">
                <Button type="button" variant="ghost" size="sm" onClick={() => void Promise.all([loadTasks(), loadOperations()])}>
                  <RefreshCw size={12} aria-hidden="true" />
                  刷新
                </Button>
              </div>
              <div className="max-h-56 space-y-2 overflow-auto text-xs">
                {unifiedTasks.length === 0 ? (
                  <p className="text-[var(--nv-text-tertiary)]">暂无历史任务</p>
                ) : unifiedTasks.slice(0, 12).map((task) => (
                  <div key={`${task.kind}-${task.id}`} className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] px-3 py-2">
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-[var(--nv-text-secondary)]">{task.kind === 'folder' ? '目录刮削' : '媒体库批量'}</span>
                      <Tag tone={taskTone(task.status)}>{taskStatusLabel[task.status] || task.status}</Tag>
                    </div>
                    <div className="mt-1 flex flex-wrap gap-3 font-mono text-[var(--nv-text-tertiary)]">
                      <span>{task.current}/{task.total}</span>
                      <span>成功 {task.success}</span>
                      <span>失败 {task.failed}</span>
                    </div>
                  </div>
                ))}
              </div>
            </OperationCard>

            <OperationCard title="配置与数据源诊断" icon={<Settings size={15} />}>
              <p className="text-xs leading-5 text-[var(--nv-text-tertiary)]">
                普通用户无需配置；遇到连接失败、数据源失效时再展开查看。
              </p>
              <div className="mt-3 flex flex-wrap gap-2">
                <Button type="button" variant="primary" size="sm" onClick={() => void resetLazyDefaults()}>
                  <RefreshCw size={12} />
                  恢复懒人默认
                </Button>
                <Button type="button" variant="secondary" size="sm" onClick={() => void Promise.all([loadConfig(), loadOperations()])}>
                  <RefreshCw size={12} />
                  重新加载
                </Button>
                <Button type="button" variant="ghost" size="sm" onClick={() => setShowDiagnostics((value) => !value)}>
                  {showDiagnostics ? '收起诊断' : '展开诊断'}
                </Button>
              </div>
              {showDiagnostics && (
                <div className="mt-3 space-y-2">
                  {(config?.sources || []).map((source) => (
                    <div key={source.id} className="flex items-center justify-between gap-3 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] px-3 py-2 text-xs">
                      <div className="min-w-0">
                        <p className="font-medium text-[var(--nv-text-secondary)]">{source.name}</p>
                        <p className="truncate text-[var(--nv-text-tertiary)]">{source.url || '未配置'}</p>
                      </div>
                      <Tag tone={source.enabled ? 'success' : 'neutral'}>{source.enabled ? '启用' : '关闭'}</Tag>
                    </div>
                  ))}
                </div>
              )}
            </OperationCard>
          </div>
        </CollapsibleSection>
      </section>

      <FileBrowser
        open={showFolderBrowser}
        onClose={() => setShowFolderBrowser(false)}
        onSelect={(selectedPath) => {
          setFolderPath(selectedPath)
          setFolderScan(null)
          setShowFolderBrowser(false)
        }}
        initialPath={folderPath || undefined}
      />
    </>
  )
}

function StatCard({ label, value, tone = 'neutral' }: { label: string; value: number; tone?: TagTone }) {
  const valueColor = tone === 'brand'
    ? 'var(--nv-action-primary)'
    : tone === 'success'
      ? 'var(--nv-status-success)'
      : tone === 'warning'
        ? 'var(--nv-status-warning)'
        : tone === 'danger'
          ? 'var(--nv-status-danger)'
          : 'var(--nv-text-primary)'

  return (
    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3">
      <p className="text-xs text-[var(--nv-text-tertiary)]">{label}</p>
      <p className="mt-1 font-mono text-2xl font-semibold" style={{ color: valueColor }}>{value}</p>
    </div>
  )
}

function LazyRule({ text }: { text: string }) {
  return (
    <div className="flex items-center gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] px-3 py-2">
      <CheckCircle2 size={13} className="shrink-0 text-[var(--nv-status-success)]" aria-hidden="true" />
      <span>{text}</span>
    </div>
  )
}

function OperationCard({ icon, title, children }: { icon: React.ReactNode; title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
      <div className="mb-3 flex items-center gap-2 text-sm font-medium text-[var(--nv-text-primary)]">
        <span className="text-[var(--nv-action-primary)]">{icon}</span>
        {title}
      </div>
      {children}
    </div>
  )
}

function CollapsibleSection({
  open,
  onToggle,
  icon,
  title,
  hint,
  children,
}: {
  open: boolean
  onToggle: () => void
  icon: React.ReactNode
  title: string
  hint: string
  children: React.ReactNode
}) {
  return (
    <Surface className="p-4">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center justify-between gap-3 rounded-[var(--nv-radius-control)] text-left outline-none focus-visible:shadow-[var(--nv-shadow-focus)]"
        aria-expanded={open}
      >
        <span className="flex min-w-0 items-center gap-2 text-sm font-medium text-[var(--nv-text-secondary)]">
          <span className="text-[var(--nv-action-primary)]">{icon}</span>
          <span>{title}</span>
          <span className="hidden text-[10px] font-normal text-[var(--nv-text-tertiary)] sm:inline">{hint}</span>
        </span>
        <ChevronDown
          size={15}
          className={clsx('shrink-0 text-[var(--nv-text-tertiary)] transition-transform duration-200 motion-reduce:transition-none', open && 'rotate-180')}
          aria-hidden="true"
        />
      </button>
      {open && <div className="mt-4">{children}</div>}
    </Surface>
  )
}
