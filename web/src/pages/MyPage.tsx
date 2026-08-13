import { Link } from 'react-router-dom'
import { BarChart3, ChevronRight, Clock, Heart, ListVideo, Moon, Server, Settings, Sun, UserRound } from 'lucide-react'
import { Button, Section, Tag } from '@/components/design-system'
import LanguageSwitcher from '@/components/LanguageSwitcher'
import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'
import { useThemeStore } from '@/stores/theme'

const entries = [
  { to: '/favorites', title: '我的收藏', description: '收藏的电影和剧集', icon: Heart },
  { to: '/history', title: '观看记录', description: '继续观看与历史进度', icon: Clock },
  { to: '/playlists', title: '播放列表', description: '整理个人片单', icon: ListVideo },
  { to: '/stats', title: '观影统计', description: '观看时间与内容偏好', icon: BarChart3 },
  { to: '/profile', title: '个人设置', description: '账号资料、密码和偏好', icon: Settings },
]

const capabilityLabels: Record<string, string> = {
  ai: 'AI', webdav: 'WebDAV', alist: 'Alist', s3: 'S3', preprocess: '预处理',
  emby_compat: 'Emby 兼容', cast: '投屏', music: '音乐', photos: '相册', federation: '联邦', plugins: '插件',
}

export default function MyPage() {
  const user = useAuthStore((state) => state.user)
  const manifest = useServerProfileStore((state) => state.manifest)
  const profileLoading = useServerProfileStore((state) => state.loading)
  const { theme, toggleTheme } = useThemeStore()
  const enabledCapabilities = manifest ? Object.entries(capabilityLabels).filter(([name]) => manifest.capabilities[name]?.enabled) : []
  const pendingRestart = manifest ? Object.values(manifest.capabilities).filter((capability) => capability.pending_restart) : []

  const profileLabel = profileLoading
    ? '检测中'
    : manifest?.profile === 'lite'
      ? '正式版'
      : manifest?.profile === 'full'
        ? '旧版兼容'
        : '未知'

  const profileDescription = manifest?.profile === 'lite'
    ? 'Nowen Video 正式服务端，面向 NAS 与家庭影音场景优化，扩展能力按配置启用。'
    : manifest?.profile === 'full'
      ? '旧版兼容运行模式，仅用于迁移、回滚或历史能力验证。'
      : '查看服务端能力和管理设置。'

  return (
    <div className="nv-section-stack">
      <section className="flex items-center gap-3 border-b border-[var(--nv-border-subtle)] pb-5">
        <div className="grid h-10 w-10 shrink-0 place-items-center rounded-[var(--nv-radius-control)] bg-[var(--nv-fill-active)] text-[var(--nv-text-secondary)]">
          <UserRound size={20} aria-hidden="true" />
        </div>
        <div className="min-w-0">
          <p className="text-[11px] text-[var(--nv-text-tertiary)]">我的影音空间</p>
          <h1 className="truncate text-lg font-semibold tracking-[-0.015em] text-[var(--nv-text-primary)]">{user?.nickname || user?.username || 'Nowen 用户'}</h1>
        </div>
      </section>

      <Section title="我的内容">
        <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
          {entries.map(({ to, title, description, icon: Icon }) => (
            <Link key={to} to={to} className="group flex min-h-14 items-center gap-3 px-1 py-2.5 transition-colors duration-150 hover:bg-[var(--nv-fill-hover)]">
              <span className="grid h-8 w-8 shrink-0 place-items-center rounded-[9px] bg-[var(--nv-fill-hover)] text-[var(--nv-text-tertiary)]"><Icon size={16} aria-hidden="true" /></span>
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium text-[var(--nv-text-primary)]">{title}</span>
                <span className="mt-0.5 block truncate text-[11px] text-[var(--nv-text-tertiary)]">{description}</span>
              </span>
              <ChevronRight size={15} className="shrink-0 text-[var(--nv-text-tertiary)] transition-transform duration-150 group-hover:translate-x-0.5" aria-hidden="true" />
            </Link>
          ))}
        </div>
      </Section>

      <Section title="界面">
        <div className="grid gap-2 sm:grid-cols-2">
          <LanguageSwitcher />
          <Button type="button" variant="ghost" onClick={toggleTheme} className="!justify-start">
            {theme === 'dark' ? <Moon size={16} aria-hidden="true" /> : <Sun size={16} aria-hidden="true" />}
            {theme === 'dark' ? '切换浅色模式' : '切换深色模式'}
          </Button>
        </div>
      </Section>

      {user?.role === 'admin' && (
        <Section title="服务端">
          <Link to="/admin" className="group flex items-start gap-3 border-y border-[var(--nv-border-subtle)] px-1 py-3 transition-colors duration-150 hover:bg-[var(--nv-fill-hover)]">
            <span className="grid h-8 w-8 shrink-0 place-items-center rounded-[9px] bg-[var(--nv-fill-hover)] text-[var(--nv-text-tertiary)]"><Server size={16} aria-hidden="true" /></span>
            <span className="min-w-0 flex-1">
              <span className="flex flex-wrap items-center gap-1.5">
                <span className="text-sm font-medium text-[var(--nv-text-primary)]">服务端模式</span>
                <Tag>{profileLabel}</Tag>
                {pendingRestart.length > 0 && <Tag tone="warning">待重启</Tag>}
              </span>
              <span className="mt-1 block text-[11px] leading-5 text-[var(--nv-text-tertiary)]">{profileDescription}</span>
              {enabledCapabilities.length > 0 && <span className="mt-2 flex flex-wrap gap-1.5">{enabledCapabilities.map(([name, label]) => <Tag key={name}>{label}</Tag>)}</span>}
            </span>
            <ChevronRight size={15} className="mt-2 shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
          </Link>
        </Section>
      )}
    </div>
  )
}
