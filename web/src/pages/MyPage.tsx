import { Link } from 'react-router-dom'
import { BarChart3, ChevronRight, Clock, Heart, ListVideo, Moon, Server, Settings, Sun, UserRound } from 'lucide-react'
import { Button, Section } from '@/components/design-system'
import FamilyInvite from '@/components/FamilyInvite'
import ViewingPreferences from '@/components/ViewingPreferences'
import LanguageSwitcher from '@/components/LanguageSwitcher'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'

const entries = [
  { to: '/favorites', title: '想看', description: '留到下次看的电影和剧集', icon: Heart },
  { to: '/history', title: '观看记录', description: '继续观看与历史进度', icon: Clock },
  { to: '/playlists', title: '播放列表', description: '整理个人片单', icon: ListVideo },
  { to: '/stats', title: '观影统计', description: '观看时间与内容偏好', icon: BarChart3 },
  { to: '/profile', title: '个人设置', description: '账号资料、密码和偏好', icon: Settings },
]

export default function MyPage() {
  const user = useAuthStore((state) => state.user)
  const { theme, toggleTheme } = useThemeStore()

  return (
    <div className="nv-section-stack">
      <section className="flex items-center gap-3 border-b border-[var(--nv-border-subtle)] pb-5">
        <div className="grid h-10 w-10 shrink-0 place-items-center rounded-[var(--nv-radius-control)] bg-[var(--nv-fill-active)] text-[var(--nv-text-secondary)]">
          <UserRound size={20} aria-hidden="true" />
        </div>
        <div className="min-w-0">
          <p className="text-[11px] text-[var(--nv-text-tertiary)]">{user?.child_mode ? '儿童空间' : '我的影音空间'}</p>
          <h1 className="truncate text-lg font-semibold tracking-[-0.015em] text-[var(--nv-text-primary)]">{user?.nickname || user?.username || 'Nowen 用户'}</h1>
        </div>
      </section>

      <Section title="我的内容">
        <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
          {entries.filter((entry) => !user?.child_mode || !['/playlists','/stats'].includes(entry.to)).map(({ to, title, description, icon: Icon }) => (
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

      <ViewingPreferences />

      <Section title="界面">
        <div className="grid gap-2 sm:grid-cols-2">
          <LanguageSwitcher />
          <Button type="button" variant="ghost" onClick={toggleTheme} className="!justify-start">
            {theme === 'dark' ? <Moon size={16} aria-hidden="true" /> : <Sun size={16} aria-hidden="true" />}
            {theme === 'dark' ? '切换浅色模式' : '切换深色模式'}
          </Button>
        </div>
      </Section>

      {user?.role === 'admin' && <FamilyInvite />}

      {user?.role === 'admin' && (
        <Section title="家庭管理">
          <Link to="/admin" className="group flex min-h-14 items-center gap-3 border-y border-[var(--nv-border-subtle)] py-3">
            <Server size={20} aria-hidden="true" />
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-medium">管理影片与家人</span>
              <span className="mt-1 block text-sm text-[var(--nv-text-tertiary)]">添加影片文件夹、管理账号和处理播放问题</span>
            </span>
            <ChevronRight size={18} aria-hidden="true" />
          </Link>
        </Section>
      )}
    </div>
  )
}
