import { useEffect, useState } from 'react'
import { statsApi, streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import type { UserStatsOverview } from '@/types'
import { Clock, Film, BarChart3, Heart } from 'lucide-react'

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
        // 静默失败
      } finally {
        setLoading(false)
      }
    }
    fetchStats()
  }, [])

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-transparent"
          style={{ borderTopColor: 'var(--neon-blue)' }} />
      </div>
    )
  }

  if (!stats) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <BarChart3 size={48} className="mb-4 text-surface-600" />
        <h3 className="text-lg font-semibold" style={{ color: 'var(--text-secondary)' }}>
          {t('stats.noData')}
        </h3>
        <p className="mt-2 text-sm" style={{ color: 'var(--text-muted)' }}>
          {t('stats.noDataHint')}
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-8 animate-fade-in">
      <h1 className="font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        {t('stats.title')}
      </h1>

      {/* 总览卡片 */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={<Clock size={24} />}
          label={t('stats.totalWatchTime')}
          value={t('stats.hours', { hours: stats.total_hours.toFixed(1) })}
          subValue={t('stats.minutes', { minutes: stats.total_minutes.toFixed(0) })}
          gradient="from-blue-500/10 to-cyan-500/10"
          iconColor="text-cyan-400"
        />
        <StatCard
          icon={<Film size={24} />}
          label={t('stats.watchedCount')}
          value={t('stats.countUnit', { count: String(stats.most_watched?.length || 0) })}
          subValue={t('stats.growing')}
          gradient="from-purple-500/10 to-pink-500/10"
          iconColor="text-purple-400"
        />
        <StatCard
          icon={<Heart size={24} />}
          label={t('stats.favoriteGenre')}
          value={stats.top_genres?.[0]?.genres?.split(',')[0] || t('stats.noGenre')}
          subValue={stats.top_genres?.[0] ? t('stats.minutes', { minutes: Number(stats.top_genres[0].total_minutes).toFixed(0) }) : ''}
          gradient="from-rose-500/10 to-orange-500/10"
          iconColor="text-rose-400"
        />
        <StatCard
          icon={<BarChart3 size={24} />}
          label={t('stats.dailyAvg')}
          value={stats.daily_stats?.length ? t('stats.dailyAvgMinutes', { minutes: (stats.total_minutes / Math.max(stats.daily_stats.length, 1)).toFixed(0) }) : t('stats.dailyAvgMinutes', { minutes: '0' })}
          subValue={t('stats.last30Days')}
          gradient="from-green-500/10 to-emerald-500/10"
          iconColor="text-green-400"
        />
      </div>

      {/* 每日观看趋势 */}
      {stats.daily_stats && stats.daily_stats.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <h2 className="mb-6 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            {t('stats.dailyTrend')}
          </h2>
          <div className="flex items-end gap-1 overflow-x-auto pb-2" style={{ minHeight: 120 }}>
            {stats.daily_stats.map((day) => {
              const maxMinutes = Math.max(...stats.daily_stats!.map((d) => Number(d.total_minutes) || 0))
              const height = maxMinutes > 0 ? (Number(day.total_minutes) / maxMinutes) * 100 : 0
              return (
                <div key={day.date} className="group flex flex-col items-center gap-1" style={{ minWidth: 24 }}>
                  <span
                    className="text-[10px] opacity-0 transition-opacity group-hover:opacity-100"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    {Number(day.total_minutes).toFixed(0)}min
                  </span>
                  <div
                    className="w-5 rounded-t-md transition-all group-hover:opacity-80"
                    style={{
                      height: `${Math.max(height, 4)}px`,
                      background: 'linear-gradient(to top, var(--neon-blue), var(--neon-purple))',
                      opacity: 0.7,
                    }}
                  />
                  <span className="text-[9px]" style={{ color: 'var(--text-muted)' }}>
                    {day.date.slice(5)}
                  </span>
                </div>
              )
            })}
          </div>
        </section>
      )}

      {/* 最爱类型 */}
      {stats.top_genres && stats.top_genres.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <h2 className="mb-4 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            {t('stats.topGenres')}
          </h2>
          <div className="space-y-3">
            {stats.top_genres.map((genre, i) => {
              const maxMin = Number(stats.top_genres![0].total_minutes) || 1
              const pct = (Number(genre.total_minutes) / maxMin) * 100
              return (
                <div key={i} className="flex items-center gap-3">
                  <span className="w-20 text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                    {(genre.genres as string).split(',')[0]}
                  </span>
                  <div className="flex-1 h-3 rounded-full overflow-hidden" style={{ background: 'var(--bg-surface)' }}>
                    <div
                      className="h-full rounded-full transition-all duration-700"
                      style={{
                        width: `${pct}%`,
                        background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                      }}
                    />
                  </div>
                  <span className="text-xs w-16 text-right" style={{ color: 'var(--text-tertiary)' }}>
                    {Number(genre.total_minutes).toFixed(0)}min
                  </span>
                </div>
              )
            })}
          </div>
        </section>
      )}

      {/* 看得最多 */}
      {stats.most_watched && stats.most_watched.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <h2 className="mb-4 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            {t('stats.mostWatched')}
          </h2>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
            {stats.most_watched.map((item) => (
              <div key={item.media_id} className="text-center">
                <div className="relative mx-auto aspect-[2/3] w-full overflow-hidden rounded-xl" style={{ background: 'var(--bg-surface)' }}>
                  {item.poster_path ? (
                    <img
                      src={item.media_type === 'series'
                        ? streamApi.getSeriesPosterUrl(item.media_id)
                        : streamApi.getPosterUrl(item.media_id)}
                      alt={item.title as string}
                      className="h-full w-full object-cover"
                      loading="lazy"
                    />
                  ) : (
                    <div className="flex h-full items-center justify-center">
                      <Film size={32} className="text-surface-700" />
                    </div>
                  )}
                </div>
                <h4 className="mt-2 truncate text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
                  {item.title as string}
                </h4>
                <p className="text-[10px]" style={{ color: 'var(--text-tertiary)' }}>
                  {t('stats.minutes', { minutes: Number(item.total_minutes).toFixed(0) })}
                </p>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  )
}

// 统计卡片组件
function StatCard({
  icon,
  label,
  value,
  subValue,
  gradient,
  iconColor,
}: {
  icon: React.ReactNode
  label: string
  value: string
  subValue: string
  gradient: string
  iconColor: string
}) {
  return (
    <div className={`glass-panel rounded-2xl p-5 bg-gradient-to-br ${gradient}`}>
      <div className={`mb-3 ${iconColor}`}>{icon}</div>
      <p className="text-xs font-medium uppercase tracking-wider" style={{ color: 'var(--text-tertiary)' }}>
        {label}
      </p>
      <p className="mt-1 font-display text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
        {value}
      </p>
      {subValue && (
        <p className="mt-0.5 text-xs" style={{ color: 'var(--text-muted)' }}>
          {subValue}
        </p>
      )}
    </div>
  )
}
