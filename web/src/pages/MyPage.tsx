import { Link } from 'react-router-dom'
import { BarChart3, ChevronRight, Clock, Heart, ListVideo, Server, Settings, UserRound } from 'lucide-react'
import { Section, Surface, Tag } from '@/components/design-system'
import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'

const entries = [
  { to: '/favorites', title: '我的收藏', description: '集中查看收藏的电影和剧集', icon: Heart },
  { to: '/history', title: '观看记录', description: '继续上次观看，管理历史进度', icon: Clock },
  { to: '/playlists', title: '播放列表', description: '整理想看、重温和家庭片单', icon: ListVideo },
  { to: '/stats', title: '观影统计', description: '查看个人观影时间和内容偏好', icon: BarChart3 },
  { to: '/profile', title: '个人设置', description: '修改账号资料、密码和偏好', icon: Settings },
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
  const pendingRestart = manifest
    ? Object.entries(manifest.capabilities).filter(([, capability]) => capability.pending_restart)
    : []

  return (
    <div className="nv-section-stack">
      <Surface as="section" className="overflow-hidden p-6 sm:p-8">
        <div className="flex items-center gap-4">
          <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-[var(--nv-radius-container)] bg-[var(--nv-action-primary)] text-[var(--nv-text-on-brand)]">
            <UserRound size={27} aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <p className="text-sm text-[var(--nv-text-tertiary)]">我的影音空间</p>
            <h1 className="truncate text-2xl font-bold tracking-[-0.02em] text-[var(--nv-text-primary)]">
              {user?.nickname || user?.username || 'Nowen 用户'}
            </h1>
          </div>
        </div>
      </Surface>

      {user?.role === 'admin' && (
        <Section title="服务端">
          <Link
            to="/admin"
            className="group block rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] p-5 transition-[background-color,border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-elevated)] hover:shadow-[var(--nv-shadow-card)]"
          >
            <div className="flex items-start gap-4">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[var(--nv-radius-card)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
                <Server size={21} aria-hidden="true" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="font-semibold text-[var(--nv-text-primary)]">服务端版本</h3>
                  <Tag tone="brand">
                    {profileLoading ? '检测中' : manifest?.profile === 'lite' ? '正式版' : manifest?.profile === 'full' ? '旧版兼容' : '未知'}
                  </Tag>
                  {pendingRestart.length > 0 && <Tag tone="warning">待重启</Tag>}
                </div>
                <p className="mt-1 max-w-3xl text-sm leading-6 text-[var(--nv-text-tertiary)]">
                  {manifest?.profile === 'lite'
                    ? 'Nowen Video 正式服务端，面向 NAS 与家庭影音场景优化，扩展能力按配置启用。'
                    : manifest?.profile === 'full'
                      ? '旧版兼容运行模式，仅用于迁移、回滚或历史能力验证。'
                      : '正在读取服务端能力与运行状态。'}
                </p>

                {enabledOptional.length > 0 && (
                  <div className="mt-3 flex flex-wrap gap-2">
                    {enabledOptional.map(([name, label]) => <Tag key={name}>{label}</Tag>)}
                  </div>
                )}

                {pendingRestart.length > 0 && (
                  <p className="mt-3 text-xs text-[var(--nv-status-warning)]">
                    {pendingRestart.map(([name]) => optionalCapabilityLabels[name] || name).join('、')} 配置已更改，重启服务后生效。
                  </p>
                )}
              </div>
              <ChevronRight size={18} className="mt-1 shrink-0 text-[var(--nv-text-tertiary)] transition-transform duration-200 group-hover:translate-x-0.5" aria-hidden="true" />
            </div>
          </Link>
        </Section>
      )}

      <Section title="我的内容">
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {entries.map(({ to, title, description, icon: Icon }) => (
            <Link
              key={to}
              to={to}
              className="group rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] p-5 transition-[background-color,border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-elevated)] hover:shadow-[var(--nv-shadow-card)]"
            >
              <div className="mb-4 flex h-10 w-10 items-center justify-center rounded-[var(--nv-radius-card)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
                <Icon size={20} aria-hidden="true" />
              </div>
              <h3 className="font-semibold text-[var(--nv-text-primary)]">{title}</h3>
              <p className="mt-1 text-sm leading-6 text-[var(--nv-text-tertiary)]">{description}</p>
            </Link>
          ))}
        </div>
      </Section>
    </div>
  )
}
