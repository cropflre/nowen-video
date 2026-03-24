import { NavLink, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useEffect, useState, useCallback, useRef } from 'react'
import { libraryApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
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
  X,
} from 'lucide-react'
import clsx from 'clsx'

interface SidebarProps {
  /** 移动端是否展开 */
  isMobileOpen?: boolean
  /** 移动端关闭回调 */
  onMobileClose?: () => void
}

export default function Sidebar({ isMobileOpen = false, onMobileClose }: SidebarProps) {
  const { user, logout } = useAuthStore()
  const { theme, toggleTheme } = useThemeStore()
  const navigate = useNavigate()
  const [libraries, setLibraries] = useState<Library[]>([])
  const [collapsed, setCollapsed] = useState(false)
  const { on, off } = useWebSocket()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // 加载媒体库列表
  const fetchLibraries = useCallback(() => {
    libraryApi.list().then((res) => {
      setLibraries(res.data.data)
    }).catch(() => {})
  }, [])

  useEffect(() => {
    fetchLibraries()
  }, [fetchLibraries])

  // 监听 WebSocket 事件，实时更新媒体库列表
  useEffect(() => {
    const debouncedRefresh = () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(() => fetchLibraries(), 500)
    }

    on(WS_EVENTS.LIBRARY_DELETED, debouncedRefresh)
    on(WS_EVENTS.LIBRARY_UPDATED, debouncedRefresh)
    on(WS_EVENTS.SCAN_COMPLETED, debouncedRefresh)

    return () => {
      off(WS_EVENTS.LIBRARY_DELETED, debouncedRefresh)
      off(WS_EVENTS.LIBRARY_UPDATED, debouncedRefresh)
      off(WS_EVENTS.SCAN_COMPLETED, debouncedRefresh)
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
    }
  }, [on, off, fetchLibraries])

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

  // 桌面端用 collapsed，移动端始终展开全宽
  const sidebarContent = (
    <>
      {/* 右侧霓虹分割线 */}
      <div className="absolute right-0 top-0 bottom-0 w-px bg-gradient-to-b from-transparent via-neon-blue/20 to-transparent" />

      {/* Logo 区域 */}
      <div className="flex h-16 items-center justify-between px-4">
        {(!collapsed || isMobileOpen) && (
          <h1 className="font-display text-lg font-bold tracking-wider">
            <span className="text-neon text-neon-glow">N</span>
            <span style={{ color: 'var(--text-primary)' }}>OWEN</span>
          </h1>
        )}
        {collapsed && !isMobileOpen && (
          <div className="flex w-full justify-center">
            <Zap size={20} className="text-neon animate-neon-breathe" />
          </div>
        )}
        {/* 桌面端折叠按钮 */}
        {!collapsed && !isMobileOpen && (
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="rounded-lg p-1.5 text-surface-400 transition-all duration-200 hover:text-neon hover:bg-neon-blue/5 hidden md:block"
          >
            <ChevronLeft size={18} className="transition-transform" />
          </button>
        )}
        {/* 移动端关闭按钮 */}
        {isMobileOpen && onMobileClose && (
          <button
            onClick={onMobileClose}
            className="rounded-lg p-1.5 text-surface-400 transition-all duration-200 hover:text-neon hover:bg-neon-blue/5 md:hidden"
          >
            <X size={18} />
          </button>
        )}
      </div>

      {/* 主导航 */}
      <nav className="flex-1 space-y-0.5 overflow-y-auto px-2 py-4">
        <NavLink
          to="/"
          end
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Home size={18} />
          {(!collapsed || isMobileOpen) && <span>首页</span>}
        </NavLink>

        <NavLink
          to="/search"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Search size={18} />
          {(!collapsed || isMobileOpen) && <span>搜索</span>}
        </NavLink>

        <NavLink
          to="/favorites"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Heart size={18} />
          {(!collapsed || isMobileOpen) && <span>我的收藏</span>}
        </NavLink>

        <NavLink
          to="/history"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Clock size={18} />
          {(!collapsed || isMobileOpen) && <span>观看历史</span>}
        </NavLink>

        <NavLink
          to="/playlists"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <ListVideo size={18} />
          {(!collapsed || isMobileOpen) && <span>播放列表</span>}
        </NavLink>

        {/* 媒体库列表 */}
        {libraries.length > 0 && (
          <>
            {(!collapsed || isMobileOpen) && (
              <div className="px-3 pb-1 pt-6 text-[10px] font-bold uppercase tracking-[0.2em] text-neon/40">
                媒体库
              </div>
            )}
            {collapsed && !isMobileOpen && (
              <div className="my-3 mx-3 border-t border-neon-blue/10" />
            )}

            {libraries.map((lib) => (
              <NavLink
                key={lib.id}
                to={`/library/${lib.id}`}
                className={({ isActive }) => clsx('nav-item', isActive && 'active')}
                onClick={onMobileClose}
              >
                {iconForType(lib.type)}
                {(!collapsed || isMobileOpen) && <span>{lib.name}</span>}
              </NavLink>
            ))}
          </>
        )}

        {/* 管理入口 */}
        {user?.role === 'admin' && (
          <>
            {(!collapsed || isMobileOpen) && (
              <div className="px-3 pb-1 pt-6 text-[10px] font-bold uppercase tracking-[0.2em] text-neon/40">
                管理
              </div>
            )}
            {collapsed && !isMobileOpen && (
              <div className="my-3 mx-3 border-t border-neon-blue/10" />
            )}

            <NavLink
              to="/admin"
              className={({ isActive }) => clsx('nav-item', isActive && 'active')}
              onClick={onMobileClose}
            >
              <Settings size={18} />
              {(!collapsed || isMobileOpen) && <span>系统管理</span>}
            </NavLink>
          </>
        )}
      </nav>

      {/* 主题切换 + 用户信息 */}
      <div className="border-t p-3" style={{ borderColor: 'var(--border-default)' }}>
        {/* 主题切换按钮 */}
        <div className={clsx('mb-3', collapsed && !isMobileOpen && 'flex justify-center')}>
          <button
            onClick={toggleTheme}
            className={clsx(
              'group relative flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-300',
              'hover:bg-[var(--nav-hover-bg)]',
              (collapsed && !isMobileOpen) ? 'justify-center' : 'w-full'
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
            {(!collapsed || isMobileOpen) && (
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
          {(!collapsed || isMobileOpen) && (
            <div className="flex-1 min-w-0">
              <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                {user?.username}
              </p>
              <p className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                {user?.role === 'admin' ? '管理员' : '用户'}
              </p>
            </div>
          )}
          {(!collapsed || isMobileOpen) && (
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
        {collapsed && !isMobileOpen && (
          <button
            onClick={() => setCollapsed(false)}
            className="mt-3 flex w-full justify-center rounded-lg p-1.5 text-surface-500 transition-all hover:text-neon hover:bg-neon-blue/5"
          >
            <ChevronLeft size={16} className="rotate-180" />
          </button>
        )}
      </div>
    </>
  )

  return (
    <>
      {/* 桌面端侧边栏 */}
      <aside
        className={clsx(
          'glass-panel-strong relative z-20 hidden h-screen flex-col transition-all duration-300 md:flex',
          collapsed ? 'w-[68px]' : 'w-60'
        )}
      >
        {sidebarContent}
      </aside>

      {/* 移动端抽屉侧边栏 */}
      <aside
        className={clsx(
          'glass-panel-strong fixed inset-y-0 left-0 z-40 flex w-64 flex-col transition-transform duration-300 ease-out md:hidden',
          isMobileOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        {sidebarContent}
      </aside>
    </>
  )
}
