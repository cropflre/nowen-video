import { Link } from 'react-router-dom'
import { BarChart3, ChevronRight, Clock, Heart, ListVideo, Server, Settings, UserRound } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'

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

const optionalCapabilityLabels: Record<string, string> = {
  ai: 'AI',
  webdav: 'WebDAV',
  alist: 'Alist',
  s3: 'S3',
  preprocess: '预处理',
  emby_compat: 'Emby 兼容',
  cast: '投屏',
  music: '音乐',
  photos: '相册',
  federation: '联邦',
  plugins: '插件',
}

export default function MyPage() {
  const user = useAuthStore((state) => state.user)
  const manifest = useServerProfileStore((state) => state.manifest)
  const profileLoading = useServerProfileStore((state) => state.loading)

  const enabledOptional = manifest
    ? Object.entries(optionalCapabilityLabels).filter(([name]) => manifest.capabilities[name]?.enabled)
    : []
  const restartBound = manifest
    ? Object.entries(manifest.capabilities).filter(([, capability]) => capability.configurable && capability.requires_restart)
    : []

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

      {user?.role === 'admin' && (
        <section>
          <h2 className="mb-3 text-sm font-semibold uppercase tracking-[0.14em]" style={{ color: 'var(--text-tertiary)' }}>
            服务端
          </h2>
          <Link
            to="/admin"
            className="group block rounded-2xl border p-5 transition-all duration-200 hover:-translate-y-0.5"
            style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}
          >
            <div className="flex items-start gap-4">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-neon-blue/10 text-neon">
                <Server size={21} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="font-semibold" style={{ color: 'var(--text-primary)' }}>服务端模式</h3>
                  <span
                    className="rounded-full px-2.5 py-1 text-xs font-semibold uppercase tracking-wide"
                    style={{
                      color: manifest?.profile === 'lite' ? 'var(--neon-blue)' : 'var(--neon-purple)',
                      background: manifest?.profile === 'lite' ? 'var(--neon-blue-10)' : 'var(--neon-purple-10)',
                    }}
                  >
                    {profileLoading ? '检测中' : manifest?.profile === 'lite' ? 'Lite' : manifest?.profile === 'full' ? 'Full' : '未知'}
                  </span>
                </div>
                <p className="mt-1 text-sm leading-6" style={{ color: 'var(--text-tertiary)' }}>
                  {manifest?.profile === 'lite'
                    ? '面向 NAS 的影视核心模式，非核心服务按配置启用。'
                    : manifest?.profile === 'full'
                      ? '完整兼容模式，包含高级扩展与历史功能。'
                      : '正在读取服务端能力与运行状态。'}
                </p>

                {enabledOptional.length > 0 && (
                  <div className="mt-3 flex flex-wrap gap-2">
                    {enabledOptional.map(([name, label]) => (
                      <span
                        key={name}
                        className="rounded-lg px-2 py-1 text-xs"
                        style={{ background: 'var(--nav-hover-bg)', color: 'var(--text-secondary)' }}
                      >
                        {label}
                      </span>
                    ))}
                  </div>
                )}

                {restartBound.length > 0 && (
                  <p className="mt-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                    {restartBound.map(([name]) => optionalCapabilityLabels[name] || name).join('、')} 的启停在 Lite 模式下需重启服务生效。
                  </p>
                )}
              </div>
              <ChevronRight size={18} className="mt-1 shrink-0 text-surface-500 transition-transform group-hover:translate-x-0.5" />
            </div>
          </Link>
        </section>
      )}

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
