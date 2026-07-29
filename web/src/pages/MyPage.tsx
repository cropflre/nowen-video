import { Link } from 'react-router-dom'
import { BarChart3, Clock, Heart, ListVideo, Settings, UserRound } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'

const entries = [
  {
    to: '/favorites',
    title: '我的收藏',
    description: '集中查看收藏的电影和剧集',
    icon: Heart,
  },
  {
    to: '/history',
    title: '观看记录',
    description: '继续上次观看，管理历史进度',
    icon: Clock,
  },
  {
    to: '/playlists',
    title: '播放列表',
    description: '整理想看、重温和家庭片单',
    icon: ListVideo,
  },
  {
    to: '/stats',
    title: '观影统计',
    description: '查看个人观影时间和内容偏好',
    icon: BarChart3,
  },
  {
    to: '/profile',
    title: '个人设置',
    description: '修改账号资料、密码和偏好',
    icon: Settings,
  },
]

export default function MyPage() {
  const user = useAuthStore((state) => state.user)

  return (
    <div className="space-y-6 animate-fade-in">
      <section
        className="overflow-hidden rounded-2xl border p-6 sm:p-8"
        style={{
          borderColor: 'var(--border-default)',
          background: 'linear-gradient(135deg, var(--card-bg), var(--bg-secondary))',
        }}
      >
        <div className="flex items-center gap-4">
          <div
            className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
              boxShadow: 'var(--shadow-neon)',
              color: 'var(--text-on-neon)',
            }}
          >
            <UserRound size={28} />
          </div>
          <div className="min-w-0">
            <p className="text-sm" style={{ color: 'var(--text-tertiary)' }}>我的影音空间</p>
            <h1 className="truncate text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
              {user?.nickname || user?.username || 'Nowen 用户'}
            </h1>
          </div>
        </div>
      </section>

      <section>
        <h2 className="mb-3 text-sm font-semibold uppercase tracking-[0.14em]" style={{ color: 'var(--text-tertiary)' }}>
          我的内容
        </h2>
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {entries.map(({ to, title, description, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className="group rounded-2xl border p-5 transition-all duration-200 hover:-translate-y-0.5"
              style={{
                borderColor: 'var(--border-default)',
                background: 'var(--card-bg)',
              }}
            >
              <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-xl bg-neon-blue/10 text-neon transition-transform group-hover:scale-105">
                <Icon size={20} />
              </div>
              <h3 className="font-semibold" style={{ color: 'var(--text-primary)' }}>{title}</h3>
              <p className="mt-1 text-sm leading-6" style={{ color: 'var(--text-tertiary)' }}>{description}</p>
            </Link>
          ))}
        </div>
      </section>
    </div>
  )
}
