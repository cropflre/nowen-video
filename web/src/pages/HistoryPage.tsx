import { Link } from 'react-router-dom'
import { userApi, streamApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useDialog } from '@/components/Dialog'
import { Button, EmptyState } from '@/components/design-system'
import { useTranslation } from '@/i18n'
import { usePageCache, invalidatePageCachePrefix } from '@/hooks/usePageCache'
import { usePagination } from '@/hooks/usePagination'
import { formatProgress, formatTime } from '@/utils/format'
import type { WatchHistory } from '@/types'
import Pagination from '@/components/Pagination'
import { Clock, Play, Trash2, X } from 'lucide-react'

interface HistoryData {
  list: WatchHistory[]
  total: number
}

export default function HistoryPage() {
  const { page, size, setPage, setSize, totalPages } = usePagination({
    initialSize: 20,
    syncToUrl: true,
  })
  const toast = useToast()
  const { t } = useTranslation()
  const dialog = useDialog()

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
      mutate((previous) => ({
        list: (previous?.list ?? []).filter((item) => item.media_id !== mediaId),
        total: Math.max(0, (previous?.total ?? 0) - 1),
      }))
      invalidatePageCachePrefix('history:')
    } catch {
      toast.error(t('history.deleteFailed'))
    }
  }

  const handleClear = async () => {
    const ok = await dialog.confirm({
      title: t('history.clearConfirmTitle') || '清空观看历史',
      message: t('history.clearConfirm'),
      confirmText: t('history.clear') || '清空',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await userApi.clearHistory()
      mutate({ list: [], total: 0 })
      invalidatePageCachePrefix('history:')
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

  const pages = totalPages(total)

  return (
    <div className="nv-section-stack">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="flex items-center gap-2 text-2xl font-bold tracking-[-0.02em] text-[var(--nv-text-primary)]">
            <Clock size={22} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
            {t('history.title')}
          </h1>
          {total > 0 && <p className="mt-1 text-sm text-[var(--nv-text-tertiary)]">共 {total} 条观看记录</p>}
        </div>
        {histories.length > 0 && (
          <Button variant="danger" size="sm" onClick={handleClear}>
            <Trash2 size={14} aria-hidden="true" />
            {t('history.clearAll')}
          </Button>
        )}
      </div>

      {loading && (
        <div className="space-y-3" aria-busy="true" aria-label="正在加载观看历史">
          {Array.from({ length: 6 }).map((_, index) => (
            <div key={index} className="flex gap-4 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] p-3 sm:p-4">
              <div className="skeleton h-20 w-32 shrink-0 rounded-[var(--nv-radius-control)] sm:w-36" />
              <div className="flex-1 space-y-2 py-1">
                <div className="skeleton h-5 w-1/3" />
                <div className="skeleton h-4 w-1/4" />
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && histories.length > 0 && (
        <div className="space-y-3">
          {histories.map((item) => {
            const progress = formatProgress(item.position, item.duration)
            const displayTitle = item.media?.media_type === 'episode' && item.media?.series
              ? `${item.media.series.title} S${String(item.media.season_num || 0).padStart(2, '0')}E${String(item.media.episode_num || 0).padStart(2, '0')}`
              : (item.media?.title || t('history.unknownMedia'))

            return (
              <article
                key={item.id}
                className="group flex gap-3 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] p-3 transition-[background-color,border-color,box-shadow] duration-200 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-elevated)] hover:shadow-[var(--nv-shadow-card)] sm:gap-4 sm:p-4"
              >
                <Link
                  to={`/play/${item.media_id}`}
                  className="relative h-20 w-28 shrink-0 overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)] sm:w-36"
                  aria-label={`继续播放 ${displayTitle}`}
                >
                  <img
                    src={streamApi.getPosterUrl(item.media_id)}
                    alt=""
                    className="h-full w-full object-cover transition-[transform,filter] duration-300 group-hover:scale-[1.025] group-hover:brightness-90"
                    onError={(event) => { event.currentTarget.style.display = 'none' }}
                  />
                  <div className="absolute inset-0 flex items-center justify-center bg-black/35 opacity-0 transition-opacity duration-200 group-hover:opacity-100 group-focus-within:opacity-100">
                    <span className="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]">
                      <Play size={14} fill="currentColor" aria-hidden="true" />
                    </span>
                  </div>
                  <div className="absolute bottom-0 left-0 right-0 h-1 bg-black/25">
                    <div className="h-full bg-[var(--nv-action-primary)]" style={{ width: `${progress}%` }} />
                  </div>
                </Link>

                <div className="flex min-w-0 flex-1 flex-col justify-center">
                  <Link
                    to={`/media/${item.media_id}`}
                    className="truncate text-sm font-semibold text-[var(--nv-text-primary)] transition-colors hover:text-[var(--nv-action-primary)]"
                  >
                    {displayTitle}
                  </Link>
                  {item.media?.media_type === 'episode' && item.media?.episode_title && (
                    <p className="mt-0.5 truncate text-xs text-[var(--nv-text-secondary)]">{item.media.episode_title}</p>
                  )}
                  <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-[var(--nv-text-tertiary)]">
                    <span>{t('history.watchedTo', { position: formatTime(item.position), duration: formatTime(item.duration) })}</span>
                    <span aria-hidden="true">·</span>
                    <span>{item.completed ? t('history.completed') : `${progress}%`}</span>
                    <span aria-hidden="true">·</span>
                    <span>{formatDate(item.updated_at)}</span>
                  </div>
                </div>

                <Button
                  variant="ghost"
                  size="sm"
                  iconOnly
                  onClick={() => handleDelete(item.media_id)}
                  className="self-center opacity-70 sm:opacity-0 sm:group-hover:opacity-100 sm:focus:opacity-100"
                  title={t('history.deleteRecord')}
                  aria-label={`${t('history.deleteRecord')}：${displayTitle}`}
                >
                  <X size={15} aria-hidden="true" />
                </Button>
              </article>
            )
          })}
        </div>
      )}

      {!loading && histories.length === 0 && (
        <EmptyState
          icon={<Clock size={26} aria-hidden="true" />}
          title={t('history.empty')}
          description={t('history.emptyHint')}
        />
      )}

      <Pagination
        page={page}
        totalPages={pages}
        total={total}
        pageSize={size}
        pageSizeOptions={[10, 20, 50, 100]}
        onPageChange={setPage}
        onPageSizeChange={setSize}
      />
    </div>
  )
}
