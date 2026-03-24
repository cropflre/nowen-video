import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { userApi, streamApi } from '@/api'
import type { WatchHistory } from '@/types'
import { Clock, Play, Trash2, X } from 'lucide-react'

export default function HistoryPage() {
  const [histories, setHistories] = useState<WatchHistory[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const size = 20

  const fetchHistory = async (p: number) => {
    setLoading(true)
    try {
      const res = await userApi.history(p, size)
      setHistories(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      // 静默处理
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchHistory(page)
  }, [page])

  const handleDelete = async (mediaId: string) => {
    try {
      await userApi.deleteHistory(mediaId)
      setHistories((h) => h.filter((item) => item.media_id !== mediaId))
      setTotal((t) => t - 1)
    } catch {
      // 静默处理
    }
  }

  const handleClear = async () => {
    if (!confirm('确定清空所有观看历史？')) return
    try {
      await userApi.clearHistory()
      setHistories([])
      setTotal(0)
    } catch {
      // 静默处理
    }
  }

  const formatProgress = (position: number, duration: number) => {
    if (!duration) return 0
    return Math.round((position / duration) * 100)
  }

  const formatTime = (seconds: number) => {
    if (!seconds) return '0:00'
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    const s = Math.floor(seconds % 60)
    if (h > 0) {
      return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
    }
    return `${m}:${s.toString().padStart(2, '0')}`
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffHours = diffMs / (1000 * 60 * 60)

    if (diffHours < 1) return '刚刚'
    if (diffHours < 24) return `${Math.floor(diffHours)} 小时前`
    const diffDays = Math.floor(diffHours / 24)
    if (diffDays < 7) return `${diffDays} 天前`
    return date.toLocaleDateString('zh-CN')
  }

  const totalPages = Math.ceil(total / size)

  return (
    <div>
      {/* 标题栏 */}
      <div className="mb-6 flex items-center justify-between">
        <h1 className="flex items-center gap-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Clock size={24} className="text-neon" />
          观看历史
        </h1>
        {histories.length > 0 && (
          <button
            onClick={handleClear}
            className="btn-ghost gap-1.5 text-sm text-red-400 hover:text-red-300"
          >
            <Trash2 size={14} />
            清空历史
          </button>
        )}
      </div>

      {/* 加载状态 */}
      {loading && (
        <div className="space-y-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="flex gap-4 rounded-xl p-4" style={{
              background: 'var(--bg-card)',
              border: '1px solid var(--border-default)',
            }}>
              <div className="skeleton h-20 w-36 rounded-lg" />
              <div className="flex-1 space-y-2">
                <div className="skeleton h-5 w-1/3 rounded" />
                <div className="skeleton h-4 w-1/4 rounded" />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 历史列表 */}
      {!loading && (
        <div className="space-y-3">
          {histories.map((item) => (
            <div
              key={item.id}
              className="glass-panel-subtle group flex gap-4 rounded-xl p-4 transition-all duration-300 hover:border-neon-blue/20 hover:shadow-card-hover"
            >
              {/* 缩略图 */}
              <Link
                to={`/play/${item.media_id}`}
                className="relative h-20 w-36 flex-shrink-0 overflow-hidden rounded-lg" style={{ background: 'var(--bg-surface)' }}
              >
                <img
                  src={streamApi.getPosterUrl(item.media_id)}
                  alt={item.media?.title}
                  className="h-full w-full object-cover"
                  onError={(e) => {
                    (e.target as HTMLImageElement).style.display = 'none'
                  }}
                />
                {/* 播放图标 */}
                <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
                  <Play size={24} className="text-white" fill="white" />
                </div>
                {/* 霓虹进度条 */}
                <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-white/10">
                  <div
                    className="h-full"
                    style={{
                      width: `${formatProgress(item.position, item.duration)}%`,
                      background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                      boxShadow: '0 0 6px rgba(0, 240, 255, 0.3)',
                    }}
                  />
                </div>
              </Link>

              {/* 信息 */}
              <div className="flex flex-1 flex-col justify-center">
                <Link
                  to={`/media/${item.media_id}`}
                  className="text-sm font-medium transition-colors hover:text-neon"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {item.media?.title || '未知媒体'}
                </Link>
                <div className="mt-1 flex items-center gap-3 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  <span>
                    观看至 {formatTime(item.position)} / {formatTime(item.duration)}
                  </span>
                  <span className="text-neon-blue/20">·</span>
                  <span>
                    {item.completed ? '✅ 已看完' : `${formatProgress(item.position, item.duration)}%`}
                  </span>
                  <span className="text-neon-blue/20">·</span>
                  <span>{formatDate(item.updated_at)}</span>
                </div>
              </div>

              {/* 删除按钮 */}
              <button
                onClick={() => handleDelete(item.media_id)}
                className="self-center rounded-lg p-2 text-surface-500 opacity-0 transition-all hover:text-red-400 hover:bg-red-400/5 group-hover:opacity-100"
                title="删除此记录"
              >
                <X size={16} />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* 空状态 */}
      {!loading && histories.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div
            className="mb-6 flex h-20 w-20 items-center justify-center rounded-2xl animate-float"
            style={{
              background: 'rgba(0, 240, 255, 0.05)',
              border: '1px solid rgba(0, 240, 255, 0.08)',
            }}
          >
            <Clock size={36} className="text-surface-600" />
          </div>
          <p className="font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-secondary)' }}>暂无观看历史</p>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-muted)' }}>
            开始播放视频后，观看记录将出现在这里
          </p>
        </div>
      )}

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="mt-8 flex items-center justify-center gap-3">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
            className="btn-ghost rounded-xl border border-neon-blue/10 px-4 py-2 text-sm disabled:opacity-30"
          >
            上一页
          </button>
          <span className="font-display text-sm tracking-wide text-neon">
            {page} / {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page === totalPages}
            className="btn-ghost rounded-xl border border-neon-blue/10 px-4 py-2 text-sm disabled:opacity-30"
          >
            下一页
          </button>
        </div>
      )}
    </div>
  )
}
