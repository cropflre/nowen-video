import { useState, useEffect, useCallback } from 'react'
import { adminApi } from '@/api'
import type { SystemLog, SystemLogStats } from '@/types'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import {
  FileText,
  AlertTriangle,
  AlertCircle,
  Info,
  Bug,
  Download,
  Trash2,
  RefreshCw,
  Search,
  Filter,
  ChevronDown,
  ChevronUp,
  Clock,
  Globe,
  User,
  Loader2,
  Activity,
  Server,
  Play,
  X,
} from 'lucide-react'
import clsx from 'clsx'

// 日志级别配置
const LEVEL_CONFIG: Record<string, { label: string; color: string; bg: string; icon: typeof Info }> = {
  error: { label: '错误', color: 'text-red-400', bg: 'bg-red-500/10', icon: AlertCircle },
  warn: { label: '警告', color: 'text-yellow-400', bg: 'bg-yellow-500/10', icon: AlertTriangle },
  info: { label: '信息', color: 'text-blue-400', bg: 'bg-blue-500/10', icon: Info },
  debug: { label: '调试', color: 'text-surface-400', bg: 'bg-surface-500/10', icon: Bug },
}

// 日志类型配置
const TYPE_CONFIG: Record<string, { label: string; icon: typeof Globe }> = {
  api: { label: 'API 请求', icon: Globe },
  playback: { label: '播放错误', icon: Play },
  system: { label: '系统事件', icon: Server },
}

// HTTP 方法颜色
const METHOD_COLORS: Record<string, string> = {
  GET: 'text-green-400',
  POST: 'text-blue-400',
  PUT: 'text-yellow-400',
  DELETE: 'text-red-400',
  PATCH: 'text-purple-400',
}

export default function LogsTab() {
  const [logs, setLogs] = useState<SystemLog[]>([])
  const [stats, setStats] = useState<SystemLogStats | null>(null)
  const [total, setTotal] = useState(0)
  const { page, size: pageSize, setPage, setSize, totalPages: calcTotalPages } = usePagination({ initialSize: 30 })
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  // 过滤条件
  const [filterType, setFilterType] = useState('')
  const [filterLevel, setFilterLevel] = useState('')
  const [filterKeyword, setFilterKeyword] = useState('')
  const [filterMethod, setFilterMethod] = useState('')
  const [showFilters, setShowFilters] = useState(false)

  // 清理对话框
  const [showCleanDialog, setShowCleanDialog] = useState(false)
  const [cleanDays, setCleanDays] = useState(30)
  const [cleaning, setCleaning] = useState(false)

  // 加载日志
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
      // 静默处理
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, filterType, filterLevel, filterKeyword, filterMethod])

  // 加载统计
  const loadStats = useCallback(async () => {
    try {
      const res = await adminApi.getSystemLogStats()
      setStats(res.data.data)
    } catch {
      // 静默处理
    }
  }, [])

  useEffect(() => {
    loadLogs()
  }, [loadLogs])

  useEffect(() => {
    loadStats()
  }, [loadStats])

  // 搜索防抖
  const [searchInput, setSearchInput] = useState('')
  useEffect(() => {
    const timer = setTimeout(() => {
      setFilterKeyword(searchInput)
      setPage(1)
    }, 400)
    return () => clearTimeout(timer)
  }, [searchInput])

  // 导出
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
      const a = document.createElement('a')
      a.href = url
      a.download = `system-logs-${new Date().toISOString().slice(0, 10)}.csv`
      a.click()
      URL.revokeObjectURL(url)
    } catch {
      // 静默处理
    } finally {
      setExporting(false)
    }
  }

  // 清理
  const handleClean = async () => {
    setCleaning(true)
    try {
      await adminApi.cleanSystemLogs(cleanDays)
      setShowCleanDialog(false)
      loadLogs()
      loadStats()
    } catch {
      // 静默处理
    } finally {
      setCleaning(false)
    }
  }

  const totalPages = calcTotalPages(total)

  // 格式化时间
  const formatTime = (t: string) => {
    const d = new Date(t)
    const now = new Date()
    const diff = now.getTime() - d.getTime()
    if (diff < 60000) return '刚刚'
    if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`
    if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`
    return d.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  return (
    <div className="space-y-6">
      {/* ===== 统计卡片 ===== */}
      {stats && (
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <div className="glass-panel-subtle rounded-xl p-4">
            <div className="flex items-center gap-2 text-sm text-surface-400">
              <FileText size={14} className="text-neon/60" />
              总日志数
            </div>
            <p className="mt-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
              {stats.total.toLocaleString()}
            </p>
            <p className="mt-1 text-xs text-surface-500">
              今日 {stats.today_count.toLocaleString()} 条
            </p>
          </div>

          <div className="glass-panel-subtle rounded-xl p-4">
            <div className="flex items-center gap-2 text-sm text-surface-400">
              <AlertCircle size={14} className="text-red-400/60" />
              今日错误
            </div>
            <p className={clsx('mt-2 font-display text-2xl font-bold tracking-wide', stats.today_errors > 0 ? 'text-red-400' : '')} style={stats.today_errors === 0 ? { color: 'var(--text-primary)' } : undefined}>
              {stats.today_errors}
            </p>
            <p className="mt-1 text-xs text-surface-500">
              总错误 {(stats.level_counts?.error || 0).toLocaleString()} 条
            </p>
          </div>

          <div className="glass-panel-subtle rounded-xl p-4">
            <div className="flex items-center gap-2 text-sm text-surface-400">
              <Globe size={14} className="text-neon/60" />
              API 请求
            </div>
            <p className="mt-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
              {(stats.type_counts?.api || 0).toLocaleString()}
            </p>
            <p className="mt-1 text-xs text-surface-500">
              播放错误 {(stats.type_counts?.playback || 0).toLocaleString()} 条
            </p>
          </div>

          <div className="glass-panel-subtle rounded-xl p-4">
            <div className="flex items-center gap-2 text-sm text-surface-400">
              <Server size={14} className="text-neon/60" />
              系统事件
            </div>
            <p className="mt-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
              {(stats.type_counts?.system || 0).toLocaleString()}
            </p>
            <p className="mt-1 text-xs text-surface-500">
              警告 {(stats.level_counts?.warn || 0).toLocaleString()} 条
            </p>
          </div>
        </div>
      )}

      {/* ===== 工具栏 ===== */}
      <div className="flex flex-wrap items-center gap-3">
        {/* 搜索框 */}
        <div className="relative flex-1 min-w-[200px]">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-surface-500" />
          <input
            type="text"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="input pl-9 pr-3 py-2 text-sm w-full"
            placeholder="搜索日志内容、路径、详情..."
          />
        </div>

        {/* 快捷过滤按钮 */}
        <div className="flex items-center gap-1.5">
          <button
            onClick={() => { setFilterLevel(filterLevel === 'error' ? '' : 'error'); setPage(1) }}
            className={clsx('btn-ghost px-3 py-2 text-xs gap-1.5 rounded-lg', filterLevel === 'error' && 'bg-red-500/10 text-red-400')}
          >
            <AlertCircle size={13} />
            仅错误
          </button>
          <button
            onClick={() => setShowFilters(!showFilters)}
            className={clsx('btn-ghost px-3 py-2 text-xs gap-1.5 rounded-lg', showFilters && 'text-neon')}
          >
            <Filter size={13} />
            筛选
          </button>
        </div>

        {/* 操作按钮 */}
        <div className="flex items-center gap-1.5">
          <button onClick={() => { loadLogs(); loadStats() }} className="btn-ghost px-3 py-2 text-xs gap-1.5 rounded-lg" disabled={loading}>
            <RefreshCw size={13} className={loading ? 'animate-spin' : ''} />
            刷新
          </button>
          <button onClick={handleExport} className="btn-ghost px-3 py-2 text-xs gap-1.5 rounded-lg" disabled={exporting}>
            {exporting ? <Loader2 size={13} className="animate-spin" /> : <Download size={13} />}
            导出
          </button>
          <button onClick={() => setShowCleanDialog(true)} className="btn-ghost px-3 py-2 text-xs gap-1.5 rounded-lg text-red-400 hover:text-red-300">
            <Trash2 size={13} />
            清理
          </button>
        </div>
      </div>

      {/* ===== 展开的筛选面板 ===== */}
      {showFilters && (
        <div className="glass-panel-subtle rounded-xl p-4 flex flex-wrap gap-4 animate-slide-up">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">日志类型</label>
            <select
              value={filterType}
              onChange={(e) => { setFilterType(e.target.value); setPage(1) }}
              className="input text-sm py-1.5 px-3 min-w-[120px]"
            >
              <option value="">全部类型</option>
              <option value="api">API 请求</option>
              <option value="playback">播放错误</option>
              <option value="system">系统事件</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">日志级别</label>
            <select
              value={filterLevel}
              onChange={(e) => { setFilterLevel(e.target.value); setPage(1) }}
              className="input text-sm py-1.5 px-3 min-w-[120px]"
            >
              <option value="">全部级别</option>
              <option value="error">错误</option>
              <option value="warn">警告</option>
              <option value="info">信息</option>
              <option value="debug">调试</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">HTTP 方法</label>
            <select
              value={filterMethod}
              onChange={(e) => { setFilterMethod(e.target.value); setPage(1) }}
              className="input text-sm py-1.5 px-3 min-w-[120px]"
            >
              <option value="">全部方法</option>
              <option value="GET">GET</option>
              <option value="POST">POST</option>
              <option value="PUT">PUT</option>
              <option value="DELETE">DELETE</option>
            </select>
          </div>
          {(filterType || filterLevel || filterMethod || filterKeyword) && (
            <div className="flex items-end">
              <button
                onClick={() => { setFilterType(''); setFilterLevel(''); setFilterMethod(''); setSearchInput(''); setFilterKeyword(''); setPage(1) }}
                className="btn-ghost px-3 py-1.5 text-xs gap-1 text-surface-400 hover:text-surface-200"
              >
                <X size={12} />
                清除筛选
              </button>
            </div>
          )}
        </div>
      )}

      {/* ===== 日志列表 ===== */}
      <div className="glass-panel rounded-xl overflow-hidden">
        {/* 表头 */}
        <div className="hidden md:grid grid-cols-[80px_70px_1fr_100px_80px_140px] gap-3 px-4 py-2.5 text-xs font-medium text-surface-400" style={{ borderBottom: '1px solid var(--border-default)' }}>
          <span>级别</span>
          <span>类型</span>
          <span>消息</span>
          <span>状态码</span>
          <span>耗时</span>
          <span>时间</span>
        </div>

        {/* 日志条目 */}
        {loading && logs.length === 0 ? (
          <div className="flex items-center justify-center py-16">
            <Loader2 size={24} className="animate-spin text-neon" />
            <span className="ml-3 text-sm text-surface-400">加载中...</span>
          </div>
        ) : logs.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <Activity size={40} className="text-surface-600 mb-3" />
            <p className="text-sm text-surface-500">暂无日志记录</p>
            <p className="text-xs text-surface-600 mt-1">系统运行后将自动记录日志</p>
          </div>
        ) : (
          <div className="divide-y" style={{ borderColor: 'var(--border-default)' }}>
            {logs.map((log) => {
              const levelCfg = LEVEL_CONFIG[log.level] || LEVEL_CONFIG.info
              const typeCfg = TYPE_CONFIG[log.type] || TYPE_CONFIG.system
              const LevelIcon = levelCfg.icon
              const isExpanded = expandedId === log.id

              return (
                <div key={log.id}>
                  {/* 主行 */}
                  <div
                    className="grid grid-cols-1 md:grid-cols-[80px_70px_1fr_100px_80px_140px] gap-1 md:gap-3 px-4 py-2.5 cursor-pointer transition-colors hover:bg-[var(--nav-hover-bg)]"
                    onClick={() => setExpandedId(isExpanded ? null : log.id)}
                  >
                    {/* 级别 */}
                    <div className="flex items-center gap-1.5">
                      <span className={clsx('inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full', levelCfg.bg, levelCfg.color)}>
                        <LevelIcon size={11} />
                        <span className="hidden sm:inline">{levelCfg.label}</span>
                      </span>
                    </div>

                    {/* 类型 */}
                    <div className="flex items-center">
                      <span className="text-xs text-surface-400">{typeCfg.label}</span>
                    </div>

                    {/* 消息 */}
                    <div className="flex items-center gap-2 min-w-0">
                      {log.method && (
                        <span className={clsx('text-xs font-mono font-semibold flex-shrink-0', METHOD_COLORS[log.method] || 'text-surface-400')}>
                          {log.method}
                        </span>
                      )}
                      <span className="text-sm truncate" style={{ color: 'var(--text-secondary)' }}>
                        {log.path || log.message}
                      </span>
                      {isExpanded ? <ChevronUp size={12} className="flex-shrink-0 text-surface-500" /> : <ChevronDown size={12} className="flex-shrink-0 text-surface-500" />}
                    </div>

                    {/* 状态码 */}
                    <div className="flex items-center">
                      {log.status_code > 0 && (
                        <span className={clsx(
                          'text-xs font-mono font-medium px-2 py-0.5 rounded',
                          log.status_code >= 500 ? 'bg-red-500/10 text-red-400' :
                          log.status_code >= 400 ? 'bg-yellow-500/10 text-yellow-400' :
                          log.status_code >= 300 ? 'bg-blue-500/10 text-blue-400' :
                          'bg-green-500/10 text-green-400'
                        )}>
                          {log.status_code}
                        </span>
                      )}
                    </div>

                    {/* 耗时 */}
                    <div className="flex items-center">
                      {log.latency_ms > 0 && (
                        <span className={clsx(
                          'text-xs font-mono',
                          log.latency_ms > 1000 ? 'text-red-400' :
                          log.latency_ms > 300 ? 'text-yellow-400' :
                          'text-surface-400'
                        )}>
                          {log.latency_ms}ms
                        </span>
                      )}
                    </div>

                    {/* 时间 */}
                    <div className="flex items-center gap-1.5 text-xs text-surface-500">
                      <Clock size={11} />
                      {formatTime(log.created_at)}
                    </div>
                  </div>

                  {/* 展开详情 */}
                  {isExpanded && (
                    <div className="px-4 pb-3 pt-1 animate-slide-up" style={{ background: 'var(--nav-hover-bg)' }}>
                      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 text-xs">
                        {log.message && (
                          <div>
                            <span className="text-surface-500">消息：</span>
                            <span className="text-surface-300 ml-1">{log.message}</span>
                          </div>
                        )}
                        {log.path && (
                          <div>
                            <span className="text-surface-500">路径：</span>
                            <span className="text-surface-300 ml-1 font-mono">{log.path}</span>
                          </div>
                        )}
                        {log.client_ip && (
                          <div className="flex items-center gap-1">
                            <Globe size={11} className="text-surface-500" />
                            <span className="text-surface-500">IP：</span>
                            <span className="text-surface-300">{log.client_ip}</span>
                          </div>
                        )}
                        {log.username && (
                          <div className="flex items-center gap-1">
                            <User size={11} className="text-surface-500" />
                            <span className="text-surface-500">用户：</span>
                            <span className="text-surface-300">{log.username}</span>
                          </div>
                        )}
                        {log.media_title && (
                          <div>
                            <span className="text-surface-500">媒体：</span>
                            <span className="text-surface-300 ml-1">{log.media_title}</span>
                          </div>
                        )}
                        {log.source && (
                          <div>
                            <span className="text-surface-500">来源：</span>
                            <span className="text-surface-300 ml-1">{log.source}</span>
                          </div>
                        )}
                        {log.user_agent && (
                          <div className="sm:col-span-2 lg:col-span-3">
                            <span className="text-surface-500">User-Agent：</span>
                            <span className="text-surface-300 ml-1 break-all">{log.user_agent}</span>
                          </div>
                        )}
                        {log.detail && (
                          <div className="sm:col-span-2 lg:col-span-3">
                            <span className="text-surface-500">详情：</span>
                            <pre className="mt-1 p-2 rounded bg-black/30 text-surface-300 text-[11px] font-mono whitespace-pre-wrap break-all max-h-40 overflow-auto">
                              {log.detail}
                            </pre>
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}

        {/* 分页 */}
        {total > 0 && (
          <div className="px-4 py-3" style={{ borderTop: '1px solid var(--border-default)' }}>
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
      </div>

      {/* ===== 清理对话框 ===== */}
      {showCleanDialog && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          style={{ background: 'rgba(0, 0, 0, 0.6)', backdropFilter: 'blur(8px)' }}
          onClick={() => setShowCleanDialog(false)}
        >
          <div
            className="glass-panel w-full max-w-md rounded-2xl p-6"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="text-lg font-semibold mb-4" style={{ color: 'var(--text-primary)' }}>
              清理旧日志
            </h3>
            <p className="text-sm text-surface-400 mb-4">
              将删除指定天数之前的所有系统日志记录。此操作不可撤销。
            </p>
            <div className="mb-5">
              <label className="block text-sm font-medium mb-1.5" style={{ color: 'var(--text-secondary)' }}>
                保留最近
              </label>
              <div className="flex items-center gap-3">
                <input
                  type="number"
                  value={cleanDays}
                  onChange={(e) => setCleanDays(Math.max(1, parseInt(e.target.value) || 1))}
                  className="input text-sm py-2 px-3 w-24"
                  min={1}
                />
                <span className="text-sm text-surface-400">天的日志</span>
              </div>
            </div>
            <div className="flex items-center gap-2 justify-end">
              <button onClick={() => setShowCleanDialog(false)} className="btn-ghost px-4 py-2 text-sm">
                取消
              </button>
              <button
                onClick={handleClean}
                disabled={cleaning}
                className="btn-primary px-4 py-2 text-sm gap-1.5"
                style={{ background: 'linear-gradient(135deg, #ef4444 0%, #dc2626 100%)' }}
              >
                {cleaning ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
                确认清理
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
