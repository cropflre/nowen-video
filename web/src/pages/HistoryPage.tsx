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
import {
  MediaArtwork,
  PersonalWorkspace,
  PersonalWorkspaceHeader,
  PersonalWorkspacePanel,
} from '@/ui'
import { Clock, Film, Play, Trash2, X } from 'lucide-react'

interface HistoryData {
  list: WatchHistory[]
  total: number
}

interface HistoryGroup {
  label: string
  items: WatchHistory[]
}

function getHistoryBucket(dateStr: string) {
  const date = new Date(dateStr)
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const target = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
  const diffDays = Math.floor((today - target) / (24 * 60 * 60 * 1000))

  if (diffDays <= 0) return '今天'
  if (diffDays < 7) return '最近 7 天'
  return '更早'
}

function groupHistory(items: WatchHistory[]): HistoryGroup[] {
  const order = ['今天', '最近 7 天', '更早']
  const buckets = new Map<string, WatchHistory[]>()

  items.forEach((item) => {
    const label = getHistoryBucket(item.updated_at)
    const group = buckets.get(label) ?? []
    group.push(item)
    buckets.set(label, group)
  })

  return order
    .map((label) => ({ label, items: buckets.get(label) ?? [] }))
    .filter((group) => group.items.length > 0)
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
  const historyGroups = groupHistory(histories)

  return (
    <PersonalWorkspace className="nv-history-page">
      <PersonalWorkspaceHeader
        icon={<Clock size={20} />}
        eyebrow="WATCH HISTORY"
        title={t('history.title')}
        description="回到最近播放过的内容，并从上次位置继续观看。"
        statValue={total}
        statLabel="条记录"
        statAriaLabel={`共 ${total} 条观看记录`}
        actions={histories.length > 0 ? (
          <Button variant="danger" size="sm" onClick={handleClear}>
            <Trash2 size={14} aria-hidden="true" />
            {t('history.clearAll')}
          </Button>
        ) : undefined}
      />

      <PersonalWorkspacePanel
        titleId="recent-history-title"
        title="最近观看"
        description={total > 0 ? '按最近播放时间整理，点击即可继续观看。' : '最近播放过的媒体会出现在这里。'}
        count={total > 0 ? `${total} 项` : undefined}
      >
        {loading && (
          <div className="nv-history-grid" aria-busy="true" aria-label="正在加载观看历史">
            {Array.from({ length: 6 }).map((_, index) => (
              <div key={index} className="nv-history-card nv-history-card--loading">
                <div className="skeleton nv-history-thumb" />
                <div className="flex-1 space-y-3 py-2">
                  <div className="skeleton h-5 w-2/3" />
                  <div className="skeleton h-4 w-1/2" />
                  <div className="skeleton h-8 w-28" />
                </div>
              </div>
            ))}
          </div>
        )}

        {!loading && historyGroups.map((group) => (
          <section key={group.label} className="nv-history-group" aria-labelledby={`history-${group.label}`}>
            <div className="nv-history-group-header">
              <h3 id={`history-${group.label}`}>{group.label}</h3>
              <span>{group.items.length}</span>
            </div>

            <div className="nv-history-grid">
              {group.items.map((item) => {
                const progress = formatProgress(item.position, item.duration)
                const displayTitle = item.media?.media_type === 'episode' && item.media?.series
                  ? `${item.media.series.title} S${String(item.media.season_num || 0).padStart(2, '0')}E${String(item.media.episode_num || 0).padStart(2, '0')}`
                  : (item.media?.title || t('history.unknownMedia'))
                const historyArtwork = item.media?.media_type === 'episode' && item.media?.series?.backdrop_path
                  ? streamApi.getSeriesBackdropUrl(item.media.series.id)
                  : streamApi.getBackdropUrl(item.media_id)
                const fallbackPoster = streamApi.getPosterUrl(item.media_id)

                return (
                  <article key={item.id} className="nv-history-card group">
                    <Link
                      to={`/play/${item.media_id}`}
                      className="nv-history-thumb"
                      aria-label={`继续播放 ${displayTitle}`}
                    >
                      <MediaArtwork
                        src={historyArtwork}
                        fallbackSrc={fallbackPoster}
                        alt=""
                        ratio="landscape"
                        className="absolute inset-0 !rounded-none !border-0 !shadow-none"
                        imageClassName="transition-[filter,transform] duration-300 group-hover:scale-[1.02] group-hover:brightness-[.84]"
                        fallback={<Film size={22} aria-hidden="true" />}
                      />
                      <div className="nv-history-play-overlay" aria-hidden="true">
                        <span className="nv-history-play-button">
                          <Play size={16} fill="currentColor" />
                        </span>
                      </div>
                      <div className="nv-history-progress-track">
                        <div className="nv-history-progress" style={{ width: `${progress}%` }} />
                      </div>
                    </Link>

                    <div className="nv-history-card-body">
                      <div className="nv-history-card-heading">
                        <div className="min-w-0">
                          <Link
                            to={`/media/${item.media_id}`}
                            className="nv-history-title"
                            title={displayTitle}
                          >
                            {displayTitle}
                          </Link>
                          {item.media?.media_type === 'episode' && item.media?.episode_title && (
                            <p className="nv-history-episode-title">{item.media.episode_title}</p>
                          )}
                        </div>
                        <Button
                          variant="ghost"
                          size="sm"
                          iconOnly
                          onClick={() => handleDelete(item.media_id)}
                          className="nv-history-delete"
                          title={t('history.deleteRecord')}
                          aria-label={`${t('history.deleteRecord')}：${displayTitle}`}
                        >
                          <X size={15} aria-hidden="true" />
                        </Button>
                      </div>

                      <div className="nv-history-meta">
                        <span>{t('history.watchedTo', { position: formatTime(item.position), duration: formatTime(item.duration) })}</span>
                        <span aria-hidden="true">·</span>
                        <span>{item.completed ? t('history.completed') : `${progress}%`}</span>
                      </div>

                      <div className="nv-history-card-footer">
                        <Link to={`/play/${item.media_id}`} className="nv-history-continue-action">
                          <Play size={13} fill="currentColor" aria-hidden="true" />
                          继续播放
                        </Link>
                        <span>{formatDate(item.updated_at)}</span>
                      </div>
                    </div>
                  </article>
                )
              })}
            </div>
          </section>
        ))}

        {!loading && histories.length === 0 && (
          <EmptyState
            className="nv-personal-workspace-empty"
            icon={<Clock size={26} aria-hidden="true" />}
            title={t('history.empty')}
            description={t('history.emptyHint')}
          />
        )}

        {total > 0 && (
          <div className="nv-personal-workspace-pagination">
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
        )}
      </PersonalWorkspacePanel>
    </PersonalWorkspace>
  )
}
