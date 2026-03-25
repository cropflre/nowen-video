import { useState, useEffect, useCallback } from 'react'
import type { AccessLog } from '@/types'
import { adminApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  FileText,
  Loader2,
  ChevronLeft,
  ChevronRight,
  Filter,
  Globe,
  Clock,
  Search,
} from 'lucide-react'
import clsx from 'clsx'

// 操作类型颜色映射
const ACTION_COLORS: Record<string, string> = {
  login: 'text-green-400',
  register: 'text-blue-400',
  play: 'text-neon',
  scrape: 'text-purple-400',
  scan: 'text-yellow-400',
  delete: 'text-red-400',
  update: 'text-amber-400',
}

export default function AccessLogsTab() {
  const toast = useToast()
  const [logs, setLogs] = useState<AccessLog[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [filterAction, setFilterAction] = useState('')
  const [filterUser, setFilterUser] = useState('')
  const size = 15

  const fetchLogs = useCallback(async (p: number) => {
    setLoading(true)
    try {
      const res = await adminApi.listAccessLogs({
        page: p,
        size,
        action: filterAction || undefined,
        user_id: filterUser || undefined,
      })
      setLogs(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      toast.error('加载访问日志失败')
    } finally {
      setLoading(false)
    }
  }, [filterAction, filterUser])

  useEffect(() => {
    fetchLogs(page)
  }, [page, fetchLogs])

  // 重置筛选时回到第一页
  useEffect(() => {
    setPage(1)
  }, [filterAction, filterUser])

  const totalPages = Math.ceil(total / size)

  const formatTime = (dateStr: string) => {
    const d = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - d.getTime()
    const diffMin = Math.floor(diffMs / 60000)
    if (diffMin < 1) return '刚刚'
    if (diffMin < 60) return `${diffMin}分钟前`
    const diffH = Math.floor(diffMin / 60)
    if (diffH < 24) return `${diffH}小时前`
    return d.toLocaleString('zh-CN', {
      month: 'numeric',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <FileText size={20} className="text-neon/60" />
          访问日志
        </h2>
        <span className="text-sm text-surface-400">共 {total} 条记录</span>
      </div>

      {/* 筛选栏 */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          <Filter size={12} />
          筛选:
        </div>
        <select
          value={filterAction}
          onChange={(e) => setFilterAction(e.target.value)}
          className="input py-1.5 px-3 text-xs"
        >
          <option value="">全部操作</option>
          <option value="login">登录</option>
          <option value="play">播放</option>
          <option value="scrape">刮削</option>
          <option value="scan">扫描</option>
          <option value="delete">删除</option>
        </select>
        <div className="relative">
          <Search size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-surface-500" />
          <input
            type="text"
            value={filterUser}
            onChange={(e) => setFilterUser(e.target.value)}
            placeholder="用户ID筛选"
            className="input py-1.5 pl-7 pr-3 text-xs w-40"
          />
        </div>
      </div>

      {/* 日志列表 */}
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Loader2 size={24} className="animate-spin text-neon/40" />
        </div>
      ) : logs.length > 0 ? (
        <div className="space-y-1.5">
          {logs.map((log) => (
            <div
              key={log.id}
              className="glass-panel-subtle group flex items-start gap-3 rounded-xl px-4 py-3 transition-all hover:border-neon-blue/10"
            >
              {/* 操作类型标签 */}
              <span className={clsx(
                'mt-0.5 flex-shrink-0 rounded-md px-2 py-0.5 text-[10px] font-bold uppercase',
                ACTION_COLORS[log.action] || 'text-surface-400'
              )} style={{ background: 'var(--nav-hover-bg)' }}>
                {log.action}
              </span>

              {/* 详情 */}
              <div className="min-w-0 flex-1">
                <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
                  <span className="font-medium text-neon/80">{log.username}</span>
                  {' '}
                  <span style={{ color: 'var(--text-tertiary)' }}>{log.detail || log.resource}</span>
                </p>
                <div className="mt-1 flex flex-wrap items-center gap-3 text-[11px]" style={{ color: 'var(--text-muted)' }}>
                  <span className="flex items-center gap-1">
                    <Clock size={10} />
                    {formatTime(log.created_at)}
                  </span>
                  <span className="flex items-center gap-1">
                    <Globe size={10} />
                    {log.ip}
                  </span>
                  {log.user_agent && (
                    <span className="truncate max-w-[200px]" title={log.user_agent}>
                      {log.user_agent.split(' ')[0]}
                    </span>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-16 text-center">
          <div>
            <FileText size={32} className="mx-auto mb-2 text-surface-600" />
            <p className="text-sm text-surface-500">暂无访问日志</p>
          </div>
        </div>
      )}

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-3">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className="btn-ghost rounded-xl border border-neon-blue/10 px-3 py-1.5 text-sm disabled:opacity-30"
          >
            <ChevronLeft size={16} />
          </button>
          <span className="font-display text-sm tracking-wide text-neon">
            {page} / {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page === totalPages}
            className="btn-ghost rounded-xl border border-neon-blue/10 px-3 py-1.5 text-sm disabled:opacity-30"
          >
            <ChevronRight size={16} />
          </button>
        </div>
      )}
    </div>
  )
}
