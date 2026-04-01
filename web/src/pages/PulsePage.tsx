import { useEffect, useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { pulseApi, streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import type {
  PulseDashboardData,
  PulseAnalyticsData,
  PulseTopContentItem,
  PulseTopUserItem,
  PulseRecentPlayItem,
} from '@/api/pulse'
import {
  Activity,
  Film,
  Tv,
  Users,
  HardDrive,
  Clock,
  TrendingUp,
  BarChart3,
  Play,
  Eye,
  Flame,
  Loader2,
  PieChart,
  Monitor,
  Database,
  Calendar,
} from 'lucide-react'
import clsx from 'clsx'

// 格式化存储大小
function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

// 格式化分钟数
function formatMinutes(minutes: number): string {
  if (minutes < 60) return `${Math.round(minutes)} 分钟`
  if (minutes < 1440) return `${(minutes / 60).toFixed(1)} 小时`
  return `${(minutes / 1440).toFixed(1)} 天`
}

type TabKey = 'dashboard' | 'analytics'

export default function PulsePage() {
  const { user } = useAuthStore()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const [activeTab, setActiveTab] = useState<TabKey>('dashboard')
  const [dashboardData, setDashboardData] = useState<PulseDashboardData | null>(null)
  const [analyticsData, setAnalyticsData] = useState<PulseAnalyticsData | null>(null)
  const [loading, setLoading] = useState(true)
  const [trendDays, setTrendDays] = useState(30)

  // 非管理员重定向
  useEffect(() => {
    if (user?.role !== 'admin') {
      navigate('/')
    }
  }, [user, navigate])

  // 加载仪表盘数据
  const fetchDashboard = useCallback(async () => {
    try {
      const res = await pulseApi.getDashboard()
      setDashboardData(res.data.data)
    } catch {
      // 静默
    }
  }, [])

  // 加载分析数据
  const fetchAnalytics = useCallback(async () => {
    try {
      const res = await pulseApi.getAnalytics()
      setAnalyticsData(res.data.data)
    } catch {
      // 静默
    }
  }, [])

  useEffect(() => {
    const load = async () => {
      setLoading(true)
      if (activeTab === 'dashboard') {
        await fetchDashboard()
      } else {
        await fetchAnalytics()
      }
      setLoading(false)
    }
    load()
  }, [activeTab, fetchDashboard, fetchAnalytics])

  if (user?.role !== 'admin') return null

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 size={32} className="animate-spin" style={{ color: 'var(--neon-blue)' }} />
      </div>
    )
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Activity size={28} className="text-neon" />
          <h1 className="font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            {t('pulse.title')}
          </h1>
        </div>
      </div>

      {/* Tab 切换 */}
      <div className="flex gap-1 rounded-xl p-1" style={{ background: 'var(--bg-surface)' }}>
        {([
          { key: 'dashboard' as TabKey, label: t('pulse.tabDashboard'), icon: <BarChart3 size={16} /> },
          { key: 'analytics' as TabKey, label: t('pulse.tabAnalytics'), icon: <PieChart size={16} /> },
        ]).map((tab) => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={clsx(
              'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-all',
              activeTab === tab.key
                ? 'shadow-sm'
                : 'hover:opacity-80'
            )}
            style={{
              background: activeTab === tab.key ? 'var(--bg-card)' : 'transparent',
              color: activeTab === tab.key ? 'var(--neon-blue)' : 'var(--text-tertiary)',
            }}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* 仪表盘 Tab */}
      {activeTab === 'dashboard' && dashboardData && (
        <DashboardTab data={dashboardData} trendDays={trendDays} setTrendDays={setTrendDays} />
      )}

      {/* 媒体分析 Tab */}
      {activeTab === 'analytics' && analyticsData && (
        <AnalyticsTab data={analyticsData} />
      )}
    </div>
  )
}

// ==================== 仪表盘 Tab ====================
function DashboardTab({
  data,
  trendDays,
  setTrendDays,
}: {
  data: PulseDashboardData
  trendDays: number
  setTrendDays: (d: number) => void
}) {
  const { t } = useTranslation()
  const ov = data.overview

  return (
    <div className="space-y-6">
      {/* 概览卡片 */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-4">
        <OverviewCard
          icon={<Film size={22} />}
          label={t('pulse.totalMovies')}
          value={String(ov.total_movies)}
          gradient="from-blue-500/10 to-cyan-500/10"
          iconColor="text-cyan-400"
        />
        <OverviewCard
          icon={<Tv size={22} />}
          label={t('pulse.totalSeries')}
          value={String(ov.total_series)}
          sub={`${ov.total_episodes} ${t('pulse.episodes')}`}
          gradient="from-purple-500/10 to-pink-500/10"
          iconColor="text-purple-400"
        />
        <OverviewCard
          icon={<Users size={22} />}
          label={t('pulse.totalUsers')}
          value={String(ov.total_users)}
          sub={`${ov.active_users_7d} ${t('pulse.activeIn7d')}`}
          gradient="from-green-500/10 to-emerald-500/10"
          iconColor="text-green-400"
        />
        <OverviewCard
          icon={<HardDrive size={22} />}
          label={t('pulse.totalStorage')}
          value={formatBytes(ov.total_storage_bytes)}
          gradient="from-orange-500/10 to-amber-500/10"
          iconColor="text-orange-400"
        />
      </div>

      {/* 播放统计卡片 */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-2">
        <OverviewCard
          icon={<Clock size={22} />}
          label={t('pulse.totalPlayTime')}
          value={formatMinutes(ov.total_play_minutes)}
          gradient="from-rose-500/10 to-red-500/10"
          iconColor="text-rose-400"
        />
        <OverviewCard
          icon={<Play size={22} />}
          label={t('pulse.todayPlayTime')}
          value={formatMinutes(ov.today_play_minutes)}
          gradient="from-indigo-500/10 to-violet-500/10"
          iconColor="text-indigo-400"
        />
      </div>

      {/* 播放趋势 */}
      {data.trends && data.trends.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
              <TrendingUp size={20} className="text-neon" />
              {t('pulse.playTrends')}
            </h2>
            <div className="flex gap-1 rounded-lg p-0.5" style={{ background: 'var(--bg-surface)' }}>
              {[7, 30, 90].map((d) => (
                <button
                  key={d}
                  onClick={() => setTrendDays(d)}
                  className={clsx(
                    'rounded-md px-3 py-1 text-xs font-medium transition-all',
                    trendDays === d ? 'shadow-sm' : ''
                  )}
                  style={{
                    background: trendDays === d ? 'var(--bg-card)' : 'transparent',
                    color: trendDays === d ? 'var(--neon-blue)' : 'var(--text-muted)',
                  }}
                >
                  {d}{t('pulse.days')}
                </button>
              ))}
            </div>
          </div>
          <TrendChart data={data.trends} />
        </section>
      )}

      {/* 时段分布热力图 */}
      {data.hourly_distribution && data.hourly_distribution.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <Clock size={20} className="text-neon" />
            {t('pulse.hourlyDist')}
          </h2>
          <HourlyHeatmap data={data.hourly_distribution} />
        </section>
      )}

      {/* 热门内容 + 用户排行 */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* 热门内容 */}
        {data.top_content && data.top_content.length > 0 && (
          <section className="glass-panel rounded-2xl p-6">
            <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
              <Flame size={20} className="text-orange-400" />
              {t('pulse.topContent')}
            </h2>
            <TopContentList items={data.top_content} />
          </section>
        )}

        {/* 用户排行 */}
        {data.top_users && data.top_users.length > 0 && (
          <section className="glass-panel rounded-2xl p-6">
            <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
              <Users size={20} className="text-green-400" />
              {t('pulse.topUsers')}
            </h2>
            <TopUserList items={data.top_users} />
          </section>
        )}
      </div>

      {/* 最近播放 */}
      {data.recent_plays && data.recent_plays.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <Eye size={20} className="text-blue-400" />
            {t('pulse.recentPlays')}
          </h2>
          <RecentPlayList items={data.recent_plays} />
        </section>
      )}
    </div>
  )
}

// ==================== 媒体分析 Tab ====================
function AnalyticsTab({ data }: { data: PulseAnalyticsData }) {
  const { t } = useTranslation()

  return (
    <div className="space-y-6">
      {/* 媒体库统计 */}
      {data.library_stats && data.library_stats.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <Database size={20} className="text-neon" />
            {t('pulse.libraryStats')}
          </h2>
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {data.library_stats.map((lib) => (
              <div
                key={lib.library_id}
                className="rounded-xl p-4 transition-all hover:scale-[1.02]"
                style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)' }}
              >
                <div className="flex items-center gap-2 mb-2">
                  {lib.library_type === 'movie' ? <Film size={16} className="text-cyan-400" /> : <Tv size={16} className="text-purple-400" />}
                  <span className="font-medium text-sm" style={{ color: 'var(--text-primary)' }}>{lib.library_name}</span>
                </div>
                <div className="grid grid-cols-2 gap-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  <span>{t('pulse.mediaCount')}: <strong style={{ color: 'var(--text-secondary)' }}>{lib.media_count}</strong></span>
                  {lib.series_count > 0 && (
                    <span>{t('pulse.seriesCount')}: <strong style={{ color: 'var(--text-secondary)' }}>{lib.series_count}</strong></span>
                  )}
                  <span>{t('pulse.storage')}: <strong style={{ color: 'var(--text-secondary)' }}>{formatBytes(lib.total_size_bytes)}</strong></span>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* 画质分布 + 编码分布 */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {data.resolution_distribution && data.resolution_distribution.length > 0 && (
          <section className="glass-panel rounded-2xl p-6">
            <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
              <Monitor size={20} className="text-cyan-400" />
              {t('pulse.resolutionDist')}
            </h2>
            <DistributionBars
              items={data.resolution_distribution.map((d) => ({ label: d.resolution, value: d.count }))}
              color="var(--neon-blue)"
            />
          </section>
        )}

        {data.codec_distribution && data.codec_distribution.length > 0 && (
          <section className="glass-panel rounded-2xl p-6">
            <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
              <HardDrive size={20} className="text-purple-400" />
              {t('pulse.codecDist')}
            </h2>
            <DistributionBars
              items={data.codec_distribution.map((d) => ({ label: d.codec, value: d.count }))}
              color="var(--neon-purple)"
            />
          </section>
        )}
      </div>

      {/* 类型分布 */}
      {data.genre_distribution && data.genre_distribution.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <BarChart3 size={20} className="text-orange-400" />
            {t('pulse.genreDist')}
          </h2>
          <DistributionBars
            items={data.genre_distribution.slice(0, 15).map((d) => ({ label: d.genre.split(',')[0], value: d.count }))}
            color="var(--neon-blue)"
          />
        </section>
      )}

      {/* 媒体库增长趋势 */}
      {data.growth && data.growth.length > 0 && (
        <section className="glass-panel rounded-2xl p-6">
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <Calendar size={20} className="text-green-400" />
            {t('pulse.growthTrend')}
          </h2>
          <div className="flex items-end gap-2 overflow-x-auto pb-2" style={{ minHeight: 120 }}>
            {data.growth.map((point) => {
              const maxCount = Math.max(...data.growth.map((g) => g.count))
              const height = maxCount > 0 ? (point.count / maxCount) * 100 : 0
              return (
                <div key={point.date} className="group flex flex-col items-center gap-1" style={{ minWidth: 40 }}>
                  <span
                    className="text-[10px] opacity-0 transition-opacity group-hover:opacity-100"
                    style={{ color: 'var(--text-tertiary)' }}
                  >
                    +{point.count}
                  </span>
                  <div
                    className="w-7 rounded-t-md transition-all group-hover:opacity-80"
                    style={{
                      height: `${Math.max(height, 4)}px`,
                      background: 'linear-gradient(to top, var(--neon-blue), var(--neon-purple))',
                      opacity: 0.7,
                    }}
                  />
                  <span className="text-[9px]" style={{ color: 'var(--text-muted)' }}>
                    {point.date.slice(2)}
                  </span>
                </div>
              )
            })}
          </div>
        </section>
      )}
    </div>
  )
}

// ==================== 子组件 ====================

// 概览卡片
function OverviewCard({
  icon,
  label,
  value,
  sub,
  gradient,
  iconColor,
}: {
  icon: React.ReactNode
  label: string
  value: string
  sub?: string
  gradient: string
  iconColor: string
}) {
  return (
    <div className={`glass-panel rounded-2xl p-4 bg-gradient-to-br ${gradient}`}>
      <div className={`mb-2 ${iconColor}`}>{icon}</div>
      <p className="text-[11px] font-medium uppercase tracking-wider" style={{ color: 'var(--text-tertiary)' }}>
        {label}
      </p>
      <p className="mt-1 font-display text-xl font-bold" style={{ color: 'var(--text-primary)' }}>
        {value}
      </p>
      {sub && (
        <p className="mt-0.5 text-[11px]" style={{ color: 'var(--text-muted)' }}>
          {sub}
        </p>
      )}
    </div>
  )
}

// 播放趋势图
function TrendChart({ data }: { data: { date: string; total_minutes: number; play_count: number }[] }) {
  const maxMinutes = Math.max(...data.map((d) => d.total_minutes), 1)

  return (
    <div className="flex items-end gap-1 overflow-x-auto pb-2" style={{ minHeight: 140 }}>
      {data.map((point) => {
        const height = (point.total_minutes / maxMinutes) * 120
        return (
          <div key={point.date} className="group flex flex-col items-center gap-1" style={{ minWidth: 22 }}>
            <span
              className="text-[10px] opacity-0 transition-opacity group-hover:opacity-100 whitespace-nowrap"
              style={{ color: 'var(--text-tertiary)' }}
            >
              {Math.round(point.total_minutes)}min
            </span>
            <div
              className="w-4 rounded-t-md transition-all group-hover:opacity-80"
              style={{
                height: `${Math.max(height, 3)}px`,
                background: 'linear-gradient(to top, var(--neon-blue), var(--neon-purple))',
                opacity: 0.7,
              }}
            />
            <span className="text-[8px] -rotate-45 origin-top-left whitespace-nowrap" style={{ color: 'var(--text-muted)' }}>
              {point.date.slice(5)}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// 时段分布热力图
function HourlyHeatmap({ data }: { data: { hour: number; play_count: number; total_minutes: number }[] }) {
  const { t } = useTranslation()
  const maxCount = Math.max(...data.map((d) => d.play_count), 1)

  // 补全 24 小时
  const hours = Array.from({ length: 24 }, (_, i) => {
    const found = data.find((d) => d.hour === i)
    return { hour: i, play_count: found?.play_count || 0, total_minutes: found?.total_minutes || 0 }
  })

  return (
    <div>
      <div className="grid grid-cols-12 gap-1 sm:grid-cols-24">
        {hours.map((h) => {
          const intensity = maxCount > 0 ? h.play_count / maxCount : 0
          return (
            <div
              key={h.hour}
              className="group relative aspect-square rounded-md transition-all hover:scale-110 cursor-default"
              style={{
                background: intensity > 0
                  ? `rgba(var(--neon-blue-rgb, 59, 130, 246), ${0.1 + intensity * 0.8})`
                  : 'var(--bg-surface)',
                border: '1px solid var(--border-subtle)',
              }}
              title={`${h.hour}:00 - ${h.play_count} ${t('pulse.plays')}, ${Math.round(h.total_minutes)} min`}
            >
              <span className="absolute inset-0 flex items-center justify-center text-[9px] font-medium"
                style={{ color: intensity > 0.5 ? '#fff' : 'var(--text-muted)' }}>
                {h.hour}
              </span>
            </div>
          )
        })}
      </div>
      <div className="mt-2 flex items-center justify-end gap-2 text-[10px]" style={{ color: 'var(--text-muted)' }}>
        <span>{t('pulse.less')}</span>
        {[0.1, 0.3, 0.5, 0.7, 0.9].map((v) => (
          <div
            key={v}
            className="h-3 w-3 rounded-sm"
            style={{ background: `rgba(var(--neon-blue-rgb, 59, 130, 246), ${v})` }}
          />
        ))}
        <span>{t('pulse.more')}</span>
      </div>
    </div>
  )
}

// 热门内容列表
function TopContentList({ items }: { items: PulseTopContentItem[] }) {
  return (
    <div className="space-y-2">
      {items.slice(0, 10).map((item, i) => (
        <div
          key={item.media_id}
          className="flex items-center gap-3 rounded-xl p-2 transition-all hover:scale-[1.01]"
          style={{ background: 'var(--bg-surface)' }}
        >
          <span
            className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold"
            style={{
              background: i < 3 ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))' : 'var(--bg-card)',
              color: i < 3 ? '#fff' : 'var(--text-muted)',
            }}
          >
            {i + 1}
          </span>
          <div className="h-10 w-7 flex-shrink-0 overflow-hidden rounded-md" style={{ background: 'var(--bg-card)' }}>
            {item.poster_path ? (
              <img
                src={streamApi.getPosterUrl(item.media_id)}
                alt={item.title}
                className="h-full w-full object-cover"
                loading="lazy"
              />
            ) : (
              <div className="flex h-full items-center justify-center">
                <Film size={12} className="text-surface-700" />
              </div>
            )}
          </div>
          <div className="flex-1 min-w-0">
            <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{item.title}</p>
            <p className="text-[10px]" style={{ color: 'var(--text-muted)' }}>
              {item.media_type === 'episode' ? '📺' : '🎬'} {Math.round(item.total_minutes)} min · {item.unique_users} 人观看
            </p>
          </div>
        </div>
      ))}
    </div>
  )
}

// 用户排行列表
function TopUserList({ items }: { items: PulseTopUserItem[] }) {
  return (
    <div className="space-y-2">
      {items.slice(0, 10).map((item, i) => (
        <div
          key={item.user_id}
          className="flex items-center gap-3 rounded-xl p-2 transition-all hover:scale-[1.01]"
          style={{ background: 'var(--bg-surface)' }}
        >
          <span
            className="flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold"
            style={{
              background: i < 3 ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))' : 'var(--bg-card)',
              color: i < 3 ? '#fff' : 'var(--text-muted)',
            }}
          >
            {i + 1}
          </span>
          <div
            className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full text-xs font-bold"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
              color: 'var(--text-on-neon)',
            }}
          >
            {item.username?.charAt(0).toUpperCase()}
          </div>
          <div className="flex-1 min-w-0">
            <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{item.username}</p>
            <p className="text-[10px]" style={{ color: 'var(--text-muted)' }}>
              {formatMinutes(item.total_minutes)} · {item.media_count} 部作品
            </p>
          </div>
        </div>
      ))}
    </div>
  )
}

// 最近播放列表
function RecentPlayList({ items }: { items: PulseRecentPlayItem[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr style={{ color: 'var(--text-muted)' }}>
            <th className="pb-2 text-left text-xs font-medium">用户</th>
            <th className="pb-2 text-left text-xs font-medium">内容</th>
            <th className="pb-2 text-right text-xs font-medium">时长</th>
            <th className="pb-2 text-right text-xs font-medium">时间</th>
          </tr>
        </thead>
        <tbody>
          {items.slice(0, 15).map((item, i) => (
            <tr key={i} className="border-t" style={{ borderColor: 'var(--border-subtle)' }}>
              <td className="py-2">
                <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>{item.username}</span>
              </td>
              <td className="py-2">
                <div className="flex items-center gap-2">
                  <span className="text-[10px]">{item.media_type === 'episode' ? '📺' : '🎬'}</span>
                  <span className="truncate text-xs" style={{ color: 'var(--text-primary)', maxWidth: 200 }}>{item.title}</span>
                </div>
              </td>
              <td className="py-2 text-right">
                <span className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{Math.round(item.watch_minutes)} min</span>
              </td>
              <td className="py-2 text-right">
                <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>{item.date}</span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// 分布条形图（通用）
function DistributionBars({ items, color }: { items: { label: string; value: number }[]; color: string }) {
  const maxVal = Math.max(...items.map((d) => d.value), 1)
  const total = items.reduce((sum, d) => sum + d.value, 0)

  return (
    <div className="space-y-2">
      {items.map((item, i) => {
        const pct = (item.value / maxVal) * 100
        const share = total > 0 ? ((item.value / total) * 100).toFixed(1) : '0'
        return (
          <div key={i} className="flex items-center gap-3">
            <span className="w-24 truncate text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
              {item.label || '未知'}
            </span>
            <div className="flex-1 h-4 rounded-full overflow-hidden" style={{ background: 'var(--bg-surface)' }}>
              <div
                className="h-full rounded-full transition-all duration-700"
                style={{ width: `${pct}%`, background: color, opacity: 0.7 }}
              />
            </div>
            <span className="w-16 text-right text-[11px]" style={{ color: 'var(--text-tertiary)' }}>
              {item.value} ({share}%)
            </span>
          </div>
        )
      })}
    </div>
  )
}
