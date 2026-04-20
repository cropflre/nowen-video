import { useState } from 'react'
import { Link } from 'react-router-dom'
import { userApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { usePageCache, invalidatePageCachePrefix } from '@/hooks/usePageCache'
import { formatProgress, formatTime } from '@/utils/format'
import type { WatchHistory } from '@/types'
import { Clock, Play, Trash2, X } from 'lucide-react'

interface HistoryData {
  list: WatchHistory[]
  total: number
}

export default function HistoryPage() {
  const [page, setPage] = useState(1)
  const size = 20
  const toast = useToast()
  const { t } = useTranslation()

  const { data, loading, mutate, refetch } = usePageCache<HistoryData>(
    `history:page=${page}:size=${size}`,
    async () => {
      const res = await userApi.history(page, size)
      return { list: res.data.data || [], total: res.data.total }
    },
    { ttl: 15_000 },
  )

  const histories = data?.list ?? []
  const total = data?.total ?? 0

  const handleDelete = async (mediaId: string) => {
    try {
      await userApi.deleteHistory(mediaId)
      // 乐观更新当前页；同时使其他分页缓存失效，确保重新访问时数据一致
      mutate((prev) => ({
        list: (prev?.list ?? []).filter((item) => item.media_id !== mediaId),
        total: Math.max(0, (prev?.total ?? 0) - 1),
      }))
      invalidatePageCachePrefix('history:')
    } catch {
      toast.error(t('history.deleteFailed'))
    }
  }

  const handleClear = async () => {
    if (!confirm(t('history.clearConfirm'))) return
    try {
      await userApi.clearHistory()
      mutate({ list: [], total: 0 })
      invalidatePageCachePrefix('history:')
      // 同时清除首页的"继续观看"缓存，让首页刷新时不显示已清理的记录
      invalidatePageCachePrefix('home:')
      refetch(true)
    } catch {
      toast.error(t('history.clearFailed'))
    }
  }

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffHours = diffMs / (1000 * 60 * 60)

    if (diffHours < 1) return t('history.justNow')
    if (diffHours < 24) return t('history.hoursAgo', { hours: String(Math.floor(diffHours)) })
    const diffDays = Math.floor(diffHours / 24)
    if (diffDays < 7) return t('history.daysAgo', { days: String(diffDays) })
    return date.toLocaleDateString('zh-CN')
  }

  const totalPages = Math.ceil(total / size)

  return (
    <div>
      {/* 标题栏 */}
      <div className="mb-6 flex items-center justify-between">
        <h1 className="flex items-center gap-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Clock size={24} className="text-neon" />
          {t('history.title')}
        </h1>
        {histories.length > 0 && (
          <button
            onClick={handleClear}
            className="btn-ghost gap-1.5 text-sm text-red-400 hover:text-red-300"
          >
            <Trash2 size={14} />
            {t('history.clearAll')}
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
                      boxShadow: '0 0 6px var(--neon-blue-30)',
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
                  {item.media?.media_type === 'episode' && item.media?.series
                    ? `${item.media.series.title} S${String(item.media.season_num || 0).padStart(2, '0')}E${String(item.media.episode_num || 0).padStart(2, '0')}`
                    : (item.media?.title || t('history.unknownMedia'))
                  }
                </Link>
                {item.media?.media_type === 'episode' && item.media?.episode_title && (
                  <p className="mt-0.5 truncate text-xs" style={{ color: 'var(--text-secondary)' }}>
                    {item.media.episode_title}
                  </p>
                )}
                <div className="mt-1 flex items-center gap-3 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  <span>
                    {t('history.watchedTo', { position: formatTime(item.position), duration: formatTime(item.duration) })}
                  </span>
                  <span className="text-neon-blue/20">·</span>
                  <span>
                    {item.completed ? t('history.completed') : `${formatProgress(item.position, item.duration)}%`}
                  </span>
                  <span className="text-neon-blue/20">·</span>
                  <span>{formatDate(item.updated_at)}</span>
                </div>
              </div>

              {/* 删除按钮 */}
              <button
                onClick={() => handleDelete(item.media_id)}
                className="self-center rounded-lg p-2 text-surface-500 opacity-0 transition-all hover:text-red-400 hover:bg-red-400/5 group-hover:opacity-100"
                title={t('history.deleteRecord')}
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
              background: 'var(--neon-blue-5)',
              border: '1px solid var(--neon-blue-8)',
            }}
          >
            <Clock size={36} className="text-surface-600" />
          </div>
          <p className="font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-secondary)' }}>{t('history.empty')}</p>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-muted)' }}>
            {t('history.emptyHint')}
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
            {t('pagination.prev')}
          </button>
          <span className="font-display text-sm tracking-wide text-neon">
            {page} / {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page === totalPages}
            className="btn-ghost rounded-xl border border-neon-blue/10 px-4 py-2 text-sm disabled:opacity-30"
          >
            {t('pagination.next')}
          </button>
        </div>
      )}
    </div>
  )
}
