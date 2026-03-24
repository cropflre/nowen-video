import { NavLink, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useEffect, useState } from 'react'
import { libraryApi } from '@/api'
import type { Library } from '@/types'
import {
  Home,
  Search,
  Heart,
  Clock,
  ListVideo,
  Settings,
  LogOut,
  Film,
  Tv,
  FolderOpen,
  ChevronLeft,
  Zap,
  Sun,
  Moon,
  Layers,
  Video,
} from 'lucide-react'
import clsx from 'clsx'

export default function Sidebar() {
  const { user, logout } = useAuthStore()
  const { theme, toggleTheme } = useThemeStore()
  const navigate = useNavigate()
  const [libraries, setLibraries] = useState<Library[]>([])
  const [collapsed, setCollapsed] = useState(false)

  useEffect(() => {
    libraryApi.list().then((res) => {
      setLibraries(res.data.data)
    }).catch(() => {})
  }, [])

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const iconForType = (type: string) => {
    switch (type) {
      case 'movie': return <Film size={18} />
      case 'tvshow': return <Tv size={18} />
      case 'mixed': return <Layers size={18} />
      case 'other': return <Video size={18} />
      default: return <FolderOpen size={18} />
    }
  }

  return (
    <aside
      className={clsx(
        'glass-panel-strong relative z-20 flex h-screen flex-col transition-all duration-300',
        collapsed ? 'w-[68px]' : 'w-60'
      )}
    >
      {/* 右侧霓虹分割线 */}
      <div className="absolute right-0 top-0 bottom-0 w-px bg-gradient-to-b from-transparent via-neon-blue/20 to-transparent" />

      {/* Logo 区域 */}
      <div className="flex h-16 items-center justify-between px-4">
        {!collapsed && (
          <h1 className="font-display text-lg font-bold tracking-wider">
            <span className="text-neon text-neon-glow">N</span>
            <span style={{ color: 'var(--text-primary)' }}>OWEN</span>
          </h1>
        )}
        {collapsed && (
          <div className="flex w-full justify-center">
            <Zap size={20} className="text-neon animate-neon-breathe" />
          </div>
        )}
        <button
          onClick={() => setCollapsed(!collapsed)}
          className={clsx(
            'rounded-lg p-1.5 text-surface-400 transition-all duration-200',
            'hover:text-neon hover:bg-neon-blue/5',
            collapsed && 'hidden'
          )}
        >
          <ChevronLeft size={18} className="transition-transform" />
        </button>
      </div>

      {/* 主导航 */}
      <nav className="flex-1 space-y-0.5 overflow-y-auto px-2 py-4">
        <NavLink
          to="/"
          end
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
        >
          <Home size={18} />
          {!collapsed && <span>首页</span>}
        </NavLink>

        <NavLink
          to="/search"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
        >
          <Search size={18} />
          {!collapsed && <span>搜索</span>}
        </NavLink>

        <NavLink
          to="/favorites"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
        >
          <Heart size={18} />
          {!collapsed && <span>我的收藏</span>}
        </NavLink>

        <NavLink
          to="/history"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
        >
          <Clock size={18} />
          {!collapsed && <span>观看历史</span>}
        </NavLink>

        <NavLink
          to="/playlists"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
        >
          <ListVideo size={18} />
          {!collapsed && <span>播放列表</span>}
        </NavLink>

        {/* 媒体库列表 */}
        {libraries.length > 0 && (
          <>
            {!collapsed && (
              <div className="px-3 pb-1 pt-6 text-[10px] font-bold uppercase tracking-[0.2em] text-neon/40">
                媒体库
              </div>
            )}
            {collapsed && (
              <div className="my-3 mx-3 border-t border-neon-blue/10" />
            )}

            {libraries.map((lib) => (
              <NavLink
                key={lib.id}
                to={`/library/${lib.id}`}
                className={({ isActive }) => clsx('nav-item', isActive && 'active')}
              >
                {iconForType(lib.type)}
                {!collapsed && <span>{lib.name}</span>}
              </NavLink>
            ))}
          </>
        )}

        {/* 管理入口 */}
        {user?.role === 'admin' && (
          <>
            {!collapsed && (
              <div className="px-3 pb-1 pt-6 text-[10px] font-bold uppercase tracking-[0.2em] text-neon/40">
                管理
              </div>
            )}
            {collapsed && (
              <div className="my-3 mx-3 border-t border-neon-blue/10" />
            )}

            <NavLink
              to="/admin"
              className={({ isActive }) => clsx('nav-item', isActive && 'active')}
            >
              <Settings size={18} />
              {!collapsed && <span>系统管理</span>}
            </NavLink>
          </>
        )}
      </nav>

      {/* 主题切换 + 用户信息 */}
      <div className="border-t p-3" style={{ borderColor: 'var(--border-default)' }}>
        {/* 主题切换按钮 */}
        <div className={clsx('mb-3', collapsed && 'flex justify-center')}>
          <button
            onClick={toggleTheme}
            className={clsx(
              'group relative flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-300',
              'hover:bg-[var(--nav-hover-bg)]',
              collapsed ? 'justify-center' : 'w-full'
            )}
            style={{ color: 'var(--text-secondary)' }}
            title={theme === 'dark' ? '切换到日间模式' : '切换到夜间模式'}
            aria-label={theme === 'dark' ? '切换到日间模式' : '切换到夜间模式'}
          >
            <div className="relative h-[18px] w-[18px]">
              <Sun
                size={18}
                className={clsx(
                  'absolute inset-0 transition-all duration-500',
                  theme === 'light'
                    ? 'rotate-0 scale-100 opacity-100 text-amber-500'
                    : 'rotate-90 scale-0 opacity-0'
                )}
              />
              <Moon
                size={18}
                className={clsx(
                  'absolute inset-0 transition-all duration-500',
                  theme === 'dark'
                    ? 'rotate-0 scale-100 opacity-100 text-neon'
                    : '-rotate-90 scale-0 opacity-0'
                )}
              />
            </div>
            {!collapsed && (
              <span className="transition-colors group-hover:text-[var(--text-primary)]">
                {theme === 'dark' ? '夜间模式' : '日间模式'}
              </span>
            )}
          </button>
        </div>

        <div className="flex items-center gap-3">
          {/* 霓虹头像 */}
          <div className="relative flex h-8 w-8 items-center justify-center rounded-full text-sm font-bold"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
              boxShadow: 'var(--shadow-neon)',
              color: 'var(--text-on-neon)',
            }}
          >
            {user?.username?.charAt(0).toUpperCase()}
          </div>
          {!collapsed && (
            <div className="flex-1 min-w-0">
              <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                {user?.username}
              </p>
              <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                {user?.role === 'admin' ? '管理员' : '用户'}
              </p>
            </div>
          )}
          {!collapsed && (
            <button
              onClick={handleLogout}
              className="rounded-lg p-1.5 text-surface-400 transition-all hover:text-red-400 hover:bg-red-400/5"
              title="退出登录"
            >
              <LogOut size={16} />
            </button>
          )}
        </div>

        {/* 折叠/展开按钮（折叠模式下显示在底部） */}
        {collapsed && (
          <button
            onClick={() => setCollapsed(false)}
            className="mt-3 flex w-full justify-center rounded-lg p-1.5 text-surface-500 transition-all hover:text-neon hover:bg-neon-blue/5"
          >
            <ChevronLeft size={16} className="rotate-180" />
          </button>
        )}
      </div>
    </aside>
  )
}
