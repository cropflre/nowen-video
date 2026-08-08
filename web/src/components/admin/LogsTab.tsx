import { useCallback, useEffect, useState } from 'react'
import {
  Activity,
  AlertCircle,
  AlertTriangle,
  Bug,
  ChevronDown,
  ChevronUp,
  Clock,
  Download,
  FileText,
  Filter,
  Globe,
  Info,
  Loader2,
  Play,
  RefreshCw,
  Search,
  Server,
  Trash2,
  User,
  X,
} from 'lucide-react'
import { adminApi } from '@/api'
import type { SystemLog, SystemLogStats } from '@/types'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import {
  Button,
  EmptyState,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Select,
  Surface,
  Tag,
  type TagTone,
} from '@/components/design-system'
import { AdminPanel, AdminSectionTitle, AdminStatus, type AdminStatusTone } from './AdminPrimitives'

const LEVEL_CONFIG: Record<string, { label: string; tone: TagTone; icon: typeof Info }> = {
  error: { label: '错误', tone: 'danger', icon: AlertCircle },
  warn: { label: '警告', tone: 'warning', icon: AlertTriangle },
  info: { label: '信息', tone: 'brand', icon: Info },
  debug: { label: '调试', tone: 'neutral', icon: Bug },
}

const TYPE_CONFIG: Record<string, { label: string; icon: typeof Globe }> = {
  api: { label: 'API 请求', icon: Globe },
  playback: { label: '播放错误', icon: Play },
  system: { label: '系统事件', icon: Server },
}

function statusTone(statusCode: number): TagTone {
  if (statusCode >= 500) return 'danger'
  if (statusCode >= 400) return 'warning'
  if (statusCode >= 200 && statusCode < 300) return 'success'
  return 'neutral'
}

function latencyTone(latencyMs: number): AdminStatusTone {
  if (latencyMs > 1000) return 'danger'
  if (latencyMs > 300) return 'warning'
  return 'neutral'
}

export default function LogsTab() {
  const [logs, setLogs] = useState<SystemLog[]>([])
  const [stats, setStats] = useState<SystemLogStats | null>(null)
  const [total, setTotal] = useState(0)
  const { page, size: pageSize, setPage, setSize, totalPages: calcTotalPages } = usePagination({ initialSize: 30 })
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const [filterType, setFilterType] = useState('')
  const [filterLevel, setFilterLevel] = useState('')
  const [filterKeyword, setFilterKeyword] = useState('')
  const [filterMethod, setFilterMethod] = useState('')
  const [showFilters, setShowFilters] = useState(false)
  const [searchInput, setSearchInput] = useState('')

  const [showCleanDialog, setShowCleanDialog] = useState(false)
  const [cleanDays, setCleanDays] = useState(30)
  const [cleaning, setCleaning] = useState(false)

  const loadLogs = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, any> = { page, size: pageSize }
      if (filterType) params.type = filterType
      if (filterLevel) params.level = filterLevel
      if (filterKeyword) params.keyword = filterKeyword
      if (filterMethod) params.method = filterMethod

      const res = await adminApi.listSystemLogs(params)
      setLogs(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      // Preserve the existing silent-failure behavior for admin log loading.
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, filterType, filterLevel, filterKeyword, filterMethod])

  const loadStats = useCallback(async () => {
    try {
      const res = await adminApi.getSystemLogStats()
      setStats(res.data.data)
    } catch {
      // Preserve the existing silent-failure behavior for log statistics.
    }
  }, [])

  useEffect(() => {
    loadLogs()
  }, [loadLogs])

  useEffect(() => {
    loadStats()
  }, [loadStats])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setFilterKeyword(searchInput)
      setPage(1)
    }, 400)
    return () => window.clearTimeout(timer)
  }, [searchInput, setPage])

  const handleExport = async () => {
    setExporting(true)
    try {
      const params: Record<string, any> = {}
      if (filterType) params.type = filterType
      if (filterLevel) params.level = filterLevel
      if (filterKeyword) params.keyword = filterKeyword
      if (filterMethod) params.method = filterMethod

      const res = await adminApi.exportSystemLogs(params)
      const blob = new Blob([res.data as any], { type: 'text/csv;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `system-logs-${new Date().toISOString().slice(0, 10)}.csv`
      anchor.click()
      URL.revokeObjectURL(url)
    } catch {
      // Preserve the existing silent-failure behavior for export.
    } finally {
      setExporting(false)
    }
  }

  const handleClean = async () => {
    setCleaning(true)
    try {
      await adminApi.cleanSystemLogs(cleanDays)
      setShowCleanDialog(false)
      loadLogs()
      loadStats()
    } catch {
      // Preserve the existing silent-failure behavior for cleanup.
    } finally {
      setCleaning(false)
    }
  }

  const clearFilters = () => {
    setFilterType('')
    setFilterLevel('')
    setFilterMethod('')
    setSearchInput('')
    setFilterKeyword('')
    setPage(1)
  }

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp)
    const diff = Date.now() - date.getTime()
    if (diff < 60000) return '刚刚'
    if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`
    if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  }

  const totalPages = calcTotalPages(total)
  const hasFilters = Boolean(filterType || filterLevel || filterMethod || filterKeyword)

  return (
    <div className="space-y-6">
      {stats && (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <StatCard
            icon={<FileText size={16} />}
            label="总日志数"
            value={stats.total.toLocaleString()}
            detail={`今日 ${stats.today_count.toLocaleString()} 条`}
          />
          <StatCard
            icon={<AlertCircle size={16} />}
            label="今日错误"
            value={stats.today_errors.toLocaleString()}
            detail={`总错误 ${(stats.level_counts?.error || 0).toLocaleString()} 条`}
            tone={stats.today_errors > 0 ? 'danger' : 'neutral'}
          />
          <StatCard
            icon={<Globe size={16} />}
            label="API 请求"
            value={(stats.type_counts?.api || 0).toLocaleString()}
            detail={`播放错误 ${(stats.type_counts?.playback || 0).toLocaleString()} 条`}
          />
          <StatCard
            icon={<Server size={16} />}
            label="系统事件"
            value={(stats.type_counts?.system || 0).toLocaleString()}
            detail={`警告 ${(stats.level_counts?.warn || 0).toLocaleString()} 条`}
          />
        </div>
      )}

      <AdminPanel
        title="系统日志"
        description="查看 API、播放与系统事件。搜索采用 400ms 防抖，刷新仍保持手动触发。"
        icon={<FileText size={17} />}
        bodyClassName="space-y-4"
      >
        <div className="flex flex-wrap items-center gap-2">
          <div className="relative min-w-[220px] flex-1">
            <Search
              size={15}
              className="pointer-events-none absolute left-3 top-1/2 z-10 -translate-y-1/2 text-[var(--nv-text-tertiary)]"
            />
            <Input
              type="search"
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              className="pl-9"
              placeholder="搜索日志内容、路径、详情..."
              aria-label="搜索系统日志"
            />
          </div>

          <Button
            size="sm"
            variant={filterLevel === 'error' ? 'danger' : 'secondary'}
            onClick={() => {
              setFilterLevel(filterLevel === 'error' ? '' : 'error')
              setPage(1)
            }}
          >
            <AlertCircle size={14} />
            仅错误
          </Button>
          <Button
            size="sm"
            variant={showFilters ? 'primary' : 'secondary'}
            onClick={() => setShowFilters((value) => !value)}
          >
            <Filter size={14} />
            筛选
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => {
              loadLogs()
              loadStats()
            }}
            disabled={loading}
          >
            <RefreshCw size={14} className={loading ? 'animate-spin' : undefined} />
            刷新
          </Button>
          <Button size="sm" variant="secondary" onClick={handleExport} loading={exporting}>
            {!exporting && <Download size={14} />}
            导出
          </Button>
          <Button size="sm" variant="danger" onClick={() => setShowCleanDialog(true)}>
            <Trash2 size={14} />
            清理
          </Button>
        </div>

        {showFilters && (
          <Surface className="grid gap-3 p-4 sm:grid-cols-2 xl:grid-cols-[repeat(3,minmax(0,180px))_auto]">
            <FilterField label="日志类型">
              <Select
                value={filterType}
                onChange={(event) => {
                  setFilterType(event.target.value)
                  setPage(1)
                }}
                className="w-full"
              >
                <option value="">全部类型</option>
                <option value="api">API 请求</option>
                <option value="playback">播放错误</option>
                <option value="system">系统事件</option>
              </Select>
            </FilterField>
            <FilterField label="日志级别">
              <Select
                value={filterLevel}
                onChange={(event) => {
                  setFilterLevel(event.target.value)
                  setPage(1)
                }}
                className="w-full"
              >
                <option value="">全部级别</option>
                <option value="error">错误</option>
                <option value="warn">警告</option>
                <option value="info">信息</option>
                <option value="debug">调试</option>
              </Select>
            </FilterField>
            <FilterField label="HTTP 方法">
              <Select
                value={filterMethod}
                onChange={(event) => {
                  setFilterMethod(event.target.value)
                  setPage(1)
                }}
                className="w-full"
              >
                <option value="">全部方法</option>
                <option value="GET">GET</option>
                <option value="POST">POST</option>
                <option value="PUT">PUT</option>
                <option value="DELETE">DELETE</option>
              </Select>
            </FilterField>
            {hasFilters && (
              <div className="flex items-end sm:col-span-2 xl:col-span-1">
                <Button size="sm" variant="ghost" onClick={clearFilters}>
                  <X size={13} />
                  清除筛选
                </Button>
              </div>
            )}
          </Surface>
        )}

        <Surface className="overflow-hidden p-0">
          <div className="hidden grid-cols-[90px_90px_minmax(0,1fr)_90px_90px_150px] gap-3 border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3 text-xs font-medium text-[var(--nv-text-tertiary)] md:grid">
            <span>级别</span>
            <span>类型</span>
            <span>消息</span>
            <span>状态码</span>
            <span>耗时</span>
            <span>时间</span>
          </div>

          {loading && logs.length === 0 ? (
            <div className="flex min-h-64 items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
              <Loader2 size={22} className="animate-spin text-[var(--nv-action-primary)]" />
              加载日志中...
            </div>
          ) : logs.length === 0 ? (
            <EmptyState
              icon={<Activity size={24} />}
              title={hasFilters ? '没有匹配的日志' : '暂无日志记录'}
              description={hasFilters ? '调整搜索或筛选条件后再试。' : '系统运行后将自动记录日志。'}
              action={hasFilters ? (
                <Button size="sm" variant="secondary" onClick={clearFilters}>
                  清除筛选
                </Button>
              ) : undefined}
            />
          ) : (
            <div className="divide-y divide-[var(--nv-border-subtle)]">
              {logs.map((log) => (
                <LogRow
                  key={log.id}
                  log={log}
                  expanded={expandedId === log.id}
                  onToggle={() => setExpandedId(expandedId === log.id ? null : log.id)}
                  formatTime={formatTime}
                />
              ))}
            </div>
          )}

          {total > 0 && (
            <div className="border-t border-[var(--nv-border-subtle)] px-4 py-3">
              <Pagination
                page={page}
                totalPages={totalPages}
                total={total}
                pageSize={pageSize}
                pageSizeOptions={[20, 30, 50, 100, 200]}
                onPageChange={setPage}
                onPageSizeChange={setSize}
              />
            </div>
          )}
        </Surface>
      </AdminPanel>

      <Modal open={showCleanDialog} onClose={() => setShowCleanDialog(false)} size="sm" ariaLabel="清理旧日志">
        <ModalHeader
          title="清理旧日志"
          description="删除指定天数之前的系统日志记录，此操作不可撤销。"
          icon={<Trash2 size={18} />}
          onClose={() => setShowCleanDialog(false)}
        />
        <ModalBody>
          <label className="mb-2 block text-sm font-medium text-[var(--nv-text-secondary)]">保留最近</label>
          <div className="flex items-center gap-3">
            <Input
              type="number"
              value={cleanDays}
              onChange={(event) => setCleanDays(Math.max(1, Number.parseInt(event.target.value, 10) || 1))}
              min={1}
              className="w-28"
            />
            <span className="text-sm text-[var(--nv-text-tertiary)]">天的日志</span>
          </div>
          <div className="mt-4 rounded-[var(--nv-radius-control)] border border-[color:var(--nv-status-warning)]/30 bg-[var(--nv-bg-surface-soft)] p-3 text-xs leading-5 text-[var(--nv-text-tertiary)]">
            清理只影响系统日志记录，不会修改媒体库、用户、播放历史或转码任务。
          </div>
        </ModalBody>
        <ModalFooter>
          <Button variant="ghost" onClick={() => setShowCleanDialog(false)}>取消</Button>
          <Button variant="danger" onClick={handleClean} loading={cleaning}>
            {!cleaning && <Trash2 size={14} />}
            确认清理
          </Button>
        </ModalFooter>
      </Modal>
    </div>
  )
}

function StatCard({
  icon,
  label,
  value,
  detail,
  tone = 'neutral',
}: {
  icon: React.ReactNode
  label: string
  value: string
  detail: string
  tone?: AdminStatusTone
}) {
  return (
    <Surface className="p-4">
      <div className="flex items-center gap-2 text-sm text-[var(--nv-text-tertiary)]">
        <span className={tone === 'danger' ? 'text-[var(--nv-status-danger)]' : 'text-[var(--nv-action-primary)]'}>{icon}</span>
        {label}
      </div>
      <div className="mt-2 flex items-end justify-between gap-3">
        <p className="text-2xl font-semibold tracking-tight text-[var(--nv-text-primary)]">{value}</p>
        {tone !== 'neutral' && <AdminStatus tone={tone}>{label}</AdminStatus>}
      </div>
      <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{detail}</p>
    </Surface>
  )
}

function FilterField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-medium text-[var(--nv-text-tertiary)]">{label}</span>
      {children}
    </label>
  )
}

function LogRow({
  log,
  expanded,
  onToggle,
  formatTime,
}: {
  log: SystemLog
  expanded: boolean
  onToggle: () => void
  formatTime: (timestamp: string) => string
}) {
  const levelConfig = LEVEL_CONFIG[log.level] || LEVEL_CONFIG.info
  const typeConfig = TYPE_CONFIG[log.type] || TYPE_CONFIG.system
  const LevelIcon = levelConfig.icon
  const TypeIcon = typeConfig.icon

  return (
    <div>
      <button
        type="button"
        onClick={onToggle}
        className="grid w-full grid-cols-1 gap-2 px-4 py-3 text-left transition-colors hover:bg-[var(--nv-bg-hover)] md:grid-cols-[90px_90px_minmax(0,1fr)_90px_90px_150px] md:items-center md:gap-3"
        aria-expanded={expanded}
      >
        <div className="flex items-center gap-2">
          <Tag tone={levelConfig.tone} className="gap-1">
            <LevelIcon size={11} />
            {levelConfig.label}
          </Tag>
          <span className="text-xs text-[var(--nv-text-tertiary)] md:hidden">{typeConfig.label}</span>
        </div>

        <div className="hidden items-center gap-1.5 text-xs text-[var(--nv-text-tertiary)] md:flex">
          <TypeIcon size={12} />
          {typeConfig.label}
        </div>

        <div className="flex min-w-0 items-center gap-2">
          {log.method && <Tag tone={log.method === 'DELETE' ? 'danger' : 'neutral'}>{log.method}</Tag>}
          <span className="min-w-0 flex-1 truncate text-sm text-[var(--nv-text-secondary)]">{log.path || log.message}</span>
          {expanded ? (
            <ChevronUp size={14} className="shrink-0 text-[var(--nv-text-tertiary)]" />
          ) : (
            <ChevronDown size={14} className="shrink-0 text-[var(--nv-text-tertiary)]" />
          )}
        </div>

        <div className="flex items-center gap-2 text-xs md:block">
          <span className="text-[var(--nv-text-tertiary)] md:hidden">状态</span>
          {log.status_code > 0 ? <Tag tone={statusTone(log.status_code)}>{log.status_code}</Tag> : <span className="text-[var(--nv-text-tertiary)]">—</span>}
        </div>

        <div className="flex items-center gap-2 text-xs md:block">
          <span className="text-[var(--nv-text-tertiary)] md:hidden">耗时</span>
          {log.latency_ms > 0 ? (
            <AdminStatus tone={latencyTone(log.latency_ms)}>{log.latency_ms}ms</AdminStatus>
          ) : (
            <span className="text-[var(--nv-text-tertiary)]">—</span>
          )}
        </div>

        <div className="flex items-center gap-1.5 text-xs text-[var(--nv-text-tertiary)]">
          <Clock size={12} />
          {formatTime(log.created_at)}
        </div>
      </button>

      {expanded && (
        <div className="border-t border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-4">
          <AdminSectionTitle icon={<Info size={14} />}>日志详情</AdminSectionTitle>
          <div className="mt-3 grid gap-3 text-xs sm:grid-cols-2 lg:grid-cols-3">
            {log.message && <DetailItem label="消息" value={log.message} />}
            {log.path && <DetailItem label="路径" value={log.path} mono />}
            {log.client_ip && <DetailItem label="IP" value={log.client_ip} icon={<Globe size={11} />} />}
            {log.username && <DetailItem label="用户" value={log.username} icon={<User size={11} />} />}
            {log.media_title && <DetailItem label="媒体" value={log.media_title} />}
            {log.source && <DetailItem label="来源" value={log.source} />}
            {log.user_agent && (
              <div className="sm:col-span-2 lg:col-span-3">
                <DetailItem label="User-Agent" value={log.user_agent} breakAll />
              </div>
            )}
            {log.detail && (
              <div className="sm:col-span-2 lg:col-span-3">
                <p className="mb-1.5 text-[var(--nv-text-tertiary)]">详情</p>
                <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] p-3 font-mono text-[11px] leading-5 text-[var(--nv-text-secondary)]">
                  {log.detail}
                </pre>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}

function DetailItem({
  label,
  value,
  mono = false,
  breakAll = false,
  icon,
}: {
  label: string
  value: string
  mono?: boolean
  breakAll?: boolean
  icon?: React.ReactNode
}) {
  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center gap-1 text-[var(--nv-text-tertiary)]">
        {icon}
        {label}
      </div>
      <div className={`${mono ? 'font-mono' : ''} ${breakAll ? 'break-all' : 'break-words'} text-[var(--nv-text-secondary)]`}>{value}</div>
    </div>
  )
}
