import { useState, useEffect } from 'react'
import { offlineDownloadApi } from '@/api'
import type { DownloadTask, DownloadQueueInfo } from '@/types'
import {
  Download, Pause, Play, Trash2, X, Loader2,
  HardDrive, Clock, Zap, CheckCircle, AlertCircle,
} from 'lucide-react'
import clsx from 'clsx'

export default function DownloadManager() {
  const [tasks, setTasks] = useState<DownloadTask[]>([])
  const [queue, setQueue] = useState<DownloadQueueInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState('')
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const loadData = async () => {
    try {
      const [tasksRes, queueRes] = await Promise.all([
        offlineDownloadApi.list(filter || undefined),
        offlineDownloadApi.getQueue(),
      ])
      setTasks(tasksRes.data.data || [])
      setQueue(queueRes.data.data)
    } catch { /* ignore */ } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadData() }, [filter])

  // 定时刷新下载进度
  useEffect(() => {
    const timer = setInterval(loadData, 3000)
    return () => clearInterval(timer)
  }, [filter])

  const handleAction = async (action: 'pause' | 'resume' | 'cancel' | 'delete', taskId: string) => {
    try {
      switch (action) {
        case 'pause': await offlineDownloadApi.pause(taskId); break
        case 'resume': await offlineDownloadApi.resume(taskId); break
        case 'cancel': await offlineDownloadApi.cancel(taskId); break
        case 'delete': await offlineDownloadApi.delete(taskId); break
      }
      loadData()
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.error || '操作失败' })
      setTimeout(() => setMessage(null), 3000)
    }
  }

  const formatSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`
    if (bytes < 1048576) return `${(bytes / 1024).toFixed(1)} KB`
    if (bytes < 1073741824) return `${(bytes / 1048576).toFixed(1)} MB`
    return `${(bytes / 1073741824).toFixed(2)} GB`
  }

  const formatSpeed = (bytesPerSec: number) => {
    if (bytesPerSec < 1048576) return `${(bytesPerSec / 1024).toFixed(0)} KB/s`
    return `${(bytesPerSec / 1048576).toFixed(1)} MB/s`
  }

  const formatETA = (seconds: number) => {
    if (seconds <= 0) return '--'
    if (seconds < 60) return `${seconds}秒`
    if (seconds < 3600) return `${Math.floor(seconds / 60)}分${seconds % 60}秒`
    return `${Math.floor(seconds / 3600)}时${Math.floor((seconds % 3600) / 60)}分`
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'downloading': return <Loader2 className="h-4 w-4 animate-spin text-neon-blue" />
      case 'completed': return <CheckCircle className="h-4 w-4 text-green-400" />
      case 'failed': return <AlertCircle className="h-4 w-4 text-red-400" />
      case 'paused': return <Pause className="h-4 w-4 text-yellow-400" />
      default: return <Clock className="h-4 w-4 text-surface-400" />
    }
  }

  const getStatusLabel = (status: string) => {
    const labels: Record<string, string> = {
      pending: '等待中', downloading: '下载中', paused: '已暂停',
      completed: '已完成', failed: '失败', cancelled: '已取消',
    }
    return labels[status] || status
  }

  if (loading) {
    return (
      <div className="space-y-6 animate-fade-in">
        {/* 队列概览骨架 */}
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="rounded-xl p-4 text-center" style={{ background: 'var(--bg-card)', border: '1px solid var(--border-default)' }}>
              <div className="skeleton mx-auto h-7 w-12 rounded-lg" />
              <div className="skeleton mx-auto mt-2 h-3 w-10 rounded" />
            </div>
          ))}
        </div>
        {/* 标题区骨架 */}
        <div className="flex items-center gap-2">
          <div className="skeleton h-5 w-5 rounded" />
          <div className="skeleton h-6 w-24 rounded-lg flex-1" />
          <div className="skeleton h-9 w-24 rounded-lg" />
        </div>
        {/* 下载列表骨架 */}
        <div className="space-y-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="rounded-xl p-4" style={{ background: 'var(--bg-card)', border: '1px solid var(--border-default)' }}>
              <div className="flex items-center gap-3">
                <div className="skeleton h-4 w-4 rounded-full" />
                <div className="flex-1 space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="skeleton h-4 w-1/3 rounded" />
                    <div className="skeleton h-3 w-12 rounded" />
                  </div>
                  <div className="skeleton h-1.5 w-full rounded-full" />
                  <div className="flex items-center justify-between">
                    <div className="skeleton h-3 w-24 rounded" />
                    <div className="skeleton h-3 w-16 rounded" />
                  </div>
                </div>
                <div className="skeleton h-8 w-8 rounded-lg" />
              </div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {message && (
        <div className={clsx(
          'rounded-xl px-4 py-3 text-sm font-medium',
          message.type === 'success' ? 'bg-green-500/10 text-green-400 border border-green-500/20' : 'bg-red-500/10 text-red-400 border border-red-500/20'
        )}>{message.text}</div>
      )}

      {/* 队列概览 */}
      {queue && (
        <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
          {[
            { label: '总计', value: queue.total, color: 'text-white' },
            { label: '下载中', value: queue.downloading, color: 'text-neon-blue' },
            { label: '等待中', value: queue.pending, color: 'text-yellow-400' },
            { label: '已完成', value: queue.completed, color: 'text-green-400' },
            { label: '失败', value: queue.failed, color: 'text-red-400' },
          ].map(({ label, value, color }) => (
            <div key={label} className="card-glass rounded-xl p-4 text-center">
              <p className={clsx('font-display text-2xl font-bold', color)}>{value}</p>
              <p className="text-xs text-surface-400 mt-1">{label}</p>
            </div>
          ))}
        </div>
      )}

      {/* 筛选 */}
      <div className="flex items-center gap-2">
        <Download className="h-5 w-5 text-neon-blue" />
        <h2 className="font-display text-xl font-semibold text-white flex-1">离线下载</h2>
        <select value={filter} onChange={e => setFilter(e.target.value)}
          className="input-glass rounded-lg px-3 py-2 text-sm">
          <option value="">全部</option>
          <option value="downloading">下载中</option>
          <option value="pending">等待中</option>
          <option value="completed">已完成</option>
          <option value="failed">失败</option>
          <option value="paused">已暂停</option>
        </select>
      </div>

      {/* 下载列表 */}
      <div className="space-y-2">
        {tasks.map(task => (
          <div key={task.id} className="card-glass rounded-xl p-4">
            <div className="flex items-center gap-3">
              {getStatusIcon(task.status)}
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between">
                  <p className="text-sm font-medium text-white truncate">{task.title}</p>
                  <span className="text-xs text-surface-400 ml-2 shrink-0">{getStatusLabel(task.status)}</span>
                </div>

                {/* 进度条 */}
                {(task.status === 'downloading' || task.status === 'paused') && (
                  <div className="mt-2">
                    <div className="h-1.5 rounded-full bg-surface-700 overflow-hidden">
                      <div className="h-full rounded-full bg-gradient-to-r from-neon-blue to-neon-purple transition-all"
                        style={{ width: `${task.progress}%` }} />
                    </div>
                    <div className="flex items-center justify-between mt-1 text-xs text-surface-400">
                      <span>{formatSize(task.downloaded_size)} / {formatSize(task.file_size)}</span>
                      <div className="flex items-center gap-3">
                        {task.status === 'downloading' && (
                          <>
                            <span className="flex items-center gap-1"><Zap className="h-3 w-3" />{formatSpeed(task.speed)}</span>
                            <span className="flex items-center gap-1"><Clock className="h-3 w-3" />{formatETA(task.eta_seconds)}</span>
                          </>
                        )}
                        <span>{task.progress.toFixed(1)}%</span>
                      </div>
                    </div>
                  </div>
                )}

                {task.status === 'completed' && (
                  <div className="flex items-center gap-2 mt-1 text-xs text-surface-400">
                    <HardDrive className="h-3 w-3" />
                    <span>{formatSize(task.file_size)}</span>
                    <span>· {task.quality}</span>
                  </div>
                )}

                {task.status === 'failed' && task.error && (
                  <p className="text-xs text-red-400 mt-1">{task.error}</p>
                )}
              </div>

              {/* 操作按钮 */}
              <div className="flex items-center gap-1 shrink-0">
                {task.status === 'downloading' && (
                  <button onClick={() => handleAction('pause', task.id)}
                    className="btn-ghost rounded-lg p-2 text-xs" title="暂停">
                    <Pause className="h-4 w-4" />
                  </button>
                )}
                {task.status === 'paused' && (
                  <button onClick={() => handleAction('resume', task.id)}
                    className="btn-ghost rounded-lg p-2 text-xs" title="恢复">
                    <Play className="h-4 w-4" />
                  </button>
                )}
                {(task.status === 'downloading' || task.status === 'pending' || task.status === 'paused') && (
                  <button onClick={() => handleAction('cancel', task.id)}
                    className="text-red-400 hover:text-red-300 rounded-lg p-2 text-xs" title="取消">
                    <X className="h-4 w-4" />
                  </button>
                )}
                {(task.status === 'completed' || task.status === 'failed' || task.status === 'cancelled') && (
                  <button onClick={() => handleAction('delete', task.id)}
                    className="text-red-400 hover:text-red-300 rounded-lg p-2 text-xs" title="删除">
                    <Trash2 className="h-4 w-4" />
                  </button>
                )}
              </div>
            </div>
          </div>
        ))}

        {tasks.length === 0 && (
          <div className="text-center py-16 text-surface-500">
            <Download className="h-16 w-16 mx-auto mb-4 opacity-20" />
            <p className="text-lg">暂无下载任务</p>
            <p className="text-sm mt-1">在媒体详情页点击下载按钮开始离线下载</p>
          </div>
        )}
      </div>
    </div>
  )
}
