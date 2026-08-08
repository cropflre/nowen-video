import { useEffect, useMemo, useState } from 'react'
import { BarChart3, Clock, Film, Heart } from 'lucide-react'
import { statsApi, streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import type { UserStatsOverview } from '@/types'
import { EmptyState, PageContainer, Section, Surface, Tag } from '@/components/design-system'

export default function StatsPage() {
  const [stats, setStats] = useState<UserStatsOverview | null>(null)
  const [loading, setLoading] = useState(true)
  const { t } = useTranslation()

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const res = await statsApi.getMyStats()
        setStats(res.data.data)
      } catch {
        // 保持原有静默失败行为。
      } finally {
        setLoading(false)
      }
    }
    fetchStats()
  }, [])

  const dailyMax = useMemo(() => {
    if (!stats?.daily_stats?.length) return 0
    return Math.max(...stats.daily_stats.map((day) => Number(day.total_minutes) || 0))
  }, [stats?.daily_stats])

  if (loading) {
    return (
      <PageContainer>
        <div className="flex min-h-[50vh] items-center justify-center">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-[var(--nv-border-default)] border-t-[var(--nv-action-primary)]" aria-label="Loading" />
        </div>
      </PageContainer>
    )
  }

  if (!stats) {
    return (
      <PageContainer>
        <EmptyState icon={<BarChart3 size={28} />} title={t('stats.noData')} description={t('stats.noDataHint')} />
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <div className="space-y-8">
        <div>
          <div className="mb-2 flex items-center gap-2 text-sm font-medium text-[var(--nv-action-primary)]">
            <BarChart3 size={17} aria-hidden="true" />
            {t('stats.title')}
          </div>
          <h1 className="text-2xl font-semibold tracking-[-0.02em] text-[var(--nv-text-primary)]">{t('stats.title')}</h1>
          <p className="mt-2 text-sm text-[var(--nv-text-tertiary)]">观看时长、内容偏好与最近观看趋势。</p>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          <StatCard
            icon={<Clock size={21} />}
            label={t('stats.totalWatchTime')}
            value={t('stats.hours', { hours: stats.total_hours.toFixed(1) })}
            subValue={t('stats.minutes', { minutes: stats.total_minutes.toFixed(0) })}
          />
          <StatCard
            icon={<Film size={21} />}
            label={t('stats.watchedCount')}
            value={t('stats.countUnit', { count: String(stats.most_watched?.length || 0) })}
            subValue={t('stats.growing')}
          />
          <StatCard
            icon={<Heart size={21} />}
            label={t('stats.favoriteGenre')}
            value={stats.top_genres?.[0]?.genres?.split(',')[0] || t('stats.noGenre')}
            subValue={stats.top_genres?.[0] ? t('stats.minutes', { minutes: Number(stats.top_genres[0].total_minutes).toFixed(0) }) : ''}
          />
          <StatCard
            icon={<BarChart3 size={21} />}
            label={t('stats.dailyAvg')}
            value={stats.daily_stats?.length
              ? t('stats.dailyAvgMinutes', { minutes: (stats.total_minutes / Math.max(stats.daily_stats.length, 1)).toFixed(0) })
              : t('stats.dailyAvgMinutes', { minutes: '0' })}
            subValue={t('stats.last30Days')}
          />
        </div>

        {stats.daily_stats && stats.daily_stats.length > 0 && (
          <Section title={t('stats.dailyTrend')} description={t('stats.last30Days')}>
            <Surface className="p-5 sm:p-6">
              <div className="flex min-h-40 items-end gap-2 overflow-x-auto pb-2">
                {stats.daily_stats.map((day) => {
                  const minutes = Number(day.total_minutes) || 0
                  const height = dailyMax > 0 ? (minutes / dailyMax) * 112 : 0
                  return (
                    <div key={day.date} className="group flex min-w-7 flex-1 flex-col items-center justify-end gap-2" title={`${day.date}: ${minutes.toFixed(0)} min`}>
                      <span className="text-[10px] text-[var(--nv-text-tertiary)] opacity-0 transition-opacity group-hover:opacity-100">{minutes.toFixed(0)}m</span>
                      <div
                        className="w-full max-w-7 rounded-t-[var(--nv-radius-sm)] bg-[var(--nv-action-primary)] opacity-75 transition-[height,opacity] group-hover:opacity-100"
                        style={{ height: `${Math.max(height, 4)}px` }}
                      />
                      <span className="text-[10px] text-[var(--nv-text-tertiary)]">{day.date.slice(5)}</span>
                    </div>
                  )
                })}
              </div>
            </Surface>
          </Section>
        )}

        {stats.top_genres && stats.top_genres.length > 0 && (
          <Section title={t('stats.topGenres')}>
            <Surface className="p-5 sm:p-6">
              <div className="space-y-4">
                {stats.top_genres.map((genre, index) => {
                  const maxMinutes = Number(stats.top_genres?.[0]?.total_minutes) || 1
                  const minutes = Number(genre.total_minutes) || 0
                  const percentage = Math.min(100, (minutes / maxMinutes) * 100)
                  const name = String(genre.genres || '').split(',')[0]
                  return (
                    <div key={`${name}-${index}`} className="grid grid-cols-[minmax(5rem,8rem)_1fr_auto] items-center gap-3">
                      <span className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{name}</span>
                      <div className="h-2 overflow-hidden rounded-full bg-[var(--nv-bg-surface-soft)]">
                        <div className="h-full rounded-full bg-[var(--nv-action-primary)] transition-[width] duration-700" style={{ width: `${percentage}%` }} />
                      </div>
                      <span className="min-w-16 text-right text-xs text-[var(--nv-text-tertiary)]">{minutes.toFixed(0)}min</span>
                    </div>
                  )
                })}
              </div>
            </Surface>
          </Section>
        )}

        {stats.most_watched && stats.most_watched.length > 0 && (
          <Section title={t('stats.mostWatched')}>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
              {stats.most_watched.map((item) => (
                <Surface key={item.media_id} as="article" className="group overflow-hidden p-0">
                  <div className="relative aspect-[2/3] overflow-hidden bg-[var(--nv-bg-surface-soft)]">
                    {item.poster_path ? (
                      <img
                        src={item.media_type === 'series'
                          ? streamApi.getSeriesPosterUrl(item.media_id)
                          : streamApi.getPosterUrl(item.media_id)}
                        alt={String(item.title)}
                        className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-[1.025]"
                        loading="lazy"
                      />
                    ) : (
                      <div className="flex h-full items-center justify-center text-[var(--nv-text-tertiary)]"><Film size={30} /></div>
                    )}
                    <div className="absolute bottom-2 right-2">
                      <Tag tone="brand">{t('stats.minutes', { minutes: Number(item.total_minutes).toFixed(0) })}</Tag>
                    </div>
                  </div>
                  <div className="p-3">
                    <h3 className="truncate text-sm font-medium text-[var(--nv-text-primary)]" title={String(item.title)}>{String(item.title)}</h3>
                  </div>
                </Surface>
              ))}
            </div>
          </Section>
        )}
      </div>
    </PageContainer>
  )
}

function StatCard({ icon, label, value, subValue }: { icon: React.ReactNode; label: string; value: string; subValue: string }) {
  return (
    <Surface className="p-5">
      <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
        {icon}
      </div>
      <p className="text-xs font-semibold uppercase tracking-[0.08em] text-[var(--nv-text-tertiary)]">{label}</p>
      <p className="mt-2 text-2xl font-semibold tracking-[-0.02em] text-[var(--nv-text-primary)]">{value}</p>
      {subValue && <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{subValue}</p>}
    </Surface>
  )
}
