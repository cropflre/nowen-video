import { NavLink, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useEffect, useState, useCallback, useRef } from 'react'
import { libraryApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type { Library } from '@/types'
import LanguageSwitcher from './LanguageSwitcher'
import { useTranslation } from '@/i18n'
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
  BarChart3,
  FolderOpen as FolderOpenIcon,
  Activity,
  Subtitles,
  Library as LibraryIcon,
} from 'lucide-react'
import clsx from 'clsx'
import { motion, AnimatePresence } from 'framer-motion'
import { sidebarVariants, sidebarMobileVariants } from '@/lib/motion'

interface SidebarProps {
  /** 移动端是否展开 */
  isMobileOpen?: boolean
  /** 移动端关闭回调 */
  onMobileClose?: () => void
}

export default function Sidebar({ isMobileOpen = false, onMobileClose }: SidebarProps) {
  const { user, logout } = useAuthStore()
  const { theme, toggleTheme } = useThemeStore()
  const { t } = useTranslation()
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
          {(!collapsed || isMobileOpen) && <span>{t('nav.home')}</span>}
        </NavLink>

        <NavLink
          to="/search"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Search size={18} />
          {(!collapsed || isMobileOpen) && <span>{t('nav.search')}</span>}
        </NavLink>

        <NavLink
          to="/browse"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Layers size={18} />
          {(!collapsed || isMobileOpen) && <span>影视库</span>}
        </NavLink>

        <NavLink
          to="/collections"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <LibraryIcon size={18} />
          {(!collapsed || isMobileOpen) && <span>影视合集</span>}
        </NavLink>

        <NavLink
          to="/favorites"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Heart size={18} />
          {(!collapsed || isMobileOpen) && <span>{t('nav.favorites')}</span>}
        </NavLink>

        <NavLink
          to="/history"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Clock size={18} />
          {(!collapsed || isMobileOpen) && <span>{t('nav.history')}</span>}
        </NavLink>

        <NavLink
          to="/playlists"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <ListVideo size={18} />
          {(!collapsed || isMobileOpen) && <span>{t('nav.playlists')}</span>}
        </NavLink>

        <NavLink
          to="/stats"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <BarChart3 size={18} />
          {(!collapsed || isMobileOpen) && <span>{t('nav.stats')}</span>}
        </NavLink>

        <NavLink
          to="/profile"
          className={({ isActive }) => clsx('nav-item', isActive && 'active')}
          onClick={onMobileClose}
        >
          <Settings size={18} />
          {(!collapsed || isMobileOpen) && <span>{t('nav.profile')}</span>}
        </NavLink>

        {/* 媒体库列表 */}
        {libraries.length > 0 && (
          <>
            {(!collapsed || isMobileOpen) && (
              <div className="px-3 pb-1 pt-6 text-[10px] font-bold uppercase tracking-[0.2em] text-neon/40">
                {t('nav.libraries')}
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
                {t('nav.management')}
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
              {(!collapsed || isMobileOpen) && <span>{t('nav.admin')}</span>}
            </NavLink>

            <NavLink
              to="/files"
              className={({ isActive }) => clsx('nav-item', isActive && 'active')}
              onClick={onMobileClose}
            >
              <FolderOpenIcon size={18} />
              {(!collapsed || isMobileOpen) && <span>{t('nav.files')}</span>}
            </NavLink>

            <NavLink
              to="/pulse"
              className={({ isActive }) => clsx('nav-item', isActive && 'active')}
              onClick={onMobileClose}
            >
              <Activity size={18} />
              {(!collapsed || isMobileOpen) && <span>{t('nav.pulse')}</span>}
            </NavLink>

            <NavLink
              to="/preprocess"
              className={({ isActive }) => clsx('nav-item', isActive && 'active')}
              onClick={onMobileClose}
            >
              <Zap size={18} />
              {(!collapsed || isMobileOpen) && <span>预处理</span>}
            </NavLink>

            <NavLink
              to="/subtitle-preprocess"
              className={({ isActive }) => clsx('nav-item', isActive && 'active')}
              onClick={onMobileClose}
            >
              <Subtitles size={18} />
              {(!collapsed || isMobileOpen) && <span>字幕预处理</span>}
            </NavLink>
          </>
        )}
      </nav>

      {/* 主题切换 + 用户信息 */}
      <div className="border-t p-3 border-[var(--border-default)]">
        {/* 主题切换按钮 */}
        <div className={clsx('mb-3', collapsed && !isMobileOpen && 'flex justify-center')}>
          <button
            onClick={toggleTheme}
            onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleTheme() } }}
            className={clsx(
              'theme-toggle-btn group relative flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-300',
              (collapsed && !isMobileOpen) ? 'justify-center' : 'w-full'
            )}
            style={{
              color: 'var(--text-secondary)',
              background: theme === 'light' ? 'var(--nav-hover-bg)' : undefined,
              border: theme === 'light' ? '1px solid var(--border-default)' : '1px solid transparent',
            }}
            /* 注意：此处保留 style 因为需要根据 theme 状态动态切换 */
            title={theme === 'dark' ? t('nav.switchToLight') : t('nav.switchToDark')}
            aria-label={theme === 'dark' ? t('nav.switchToLight') : t('nav.switchToDark')}
            role="switch"
            aria-checked={theme === 'dark'}
          >
            {/* 图标容器 - 固定尺寸确保对齐 */}
            <div className="relative flex h-[18px] w-[18px] items-center justify-center flex-shrink-0">
              <Sun
                size={18}
                className={clsx(
                  'absolute transition-all duration-500',
                  theme === 'light'
                    ? 'rotate-0 scale-100 opacity-100 text-amber-500'
                    : 'rotate-90 scale-0 opacity-0'
                )}
                style={theme === 'light' ? { filter: 'drop-shadow(0 0 4px rgba(245, 158, 11, 0.4))' } : undefined}
              />
              <Moon
                size={18}
                className={clsx(
                  'absolute transition-all duration-500',
                  theme === 'dark'
                    ? 'rotate-0 scale-100 opacity-100 text-neon'
                    : '-rotate-90 scale-0 opacity-0'
                )}
                style={theme === 'dark' ? { filter: 'drop-shadow(0 0 4px var(--neon-blue-40))' } : undefined}
              />
            </div>
            {(!collapsed || isMobileOpen) && (
              <span className="transition-colors group-hover:text-[var(--text-primary)]">
                {theme === 'dark' ? t('nav.darkMode') : t('nav.lightMode')}
              </span>
            )}
            {/* 当前状态指示点 */}
            {(!collapsed || isMobileOpen) && (
              <span
                className="ml-auto h-1.5 w-1.5 rounded-full flex-shrink-0"
                style={{
                  background: theme === 'dark' ? 'var(--neon-blue)' : '#f59e0b',
                  boxShadow: theme === 'dark'
                    ? '0 0 6px var(--neon-blue-40)'
                    : '0 0 6px rgba(245, 158, 11, 0.4)',
                }}
              />
            )}
          </button>
        </div>

        {/* 语言切换 */}
        {(!collapsed || isMobileOpen) && (
          <LanguageSwitcher />
        )}

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
              <p className="truncate text-sm font-medium text-theme-primary">
                {user?.username}
              </p>
              <p className="text-xs text-theme-tertiary">
                {user?.role === 'admin' ? t('user.admin') : t('user.user')}
              </p>
            </div>
          )}
          {(!collapsed || isMobileOpen) && (
            <button
              onClick={handleLogout}
              className="rounded-lg p-1.5 text-surface-400 transition-all hover:text-red-400 hover:bg-red-400/5"
              title={t('nav.logout')}
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
      {/* 桌面端侧边栏 — 弹性宽度动画 */}
      <motion.aside
        className="glass-panel-strong relative z-20 hidden h-screen flex-col md:flex overflow-hidden flex-shrink-0"
        animate={collapsed ? 'collapsed' : 'expanded'}
        variants={sidebarVariants}
        style={{ willChange: 'width' }}
      >
        {sidebarContent}
      </motion.aside>

      {/* 移动端抽屉侧边栏 — 弹性滑入 */}
      <AnimatePresence>
        {isMobileOpen && (
          <motion.aside
            className="glass-panel-strong fixed inset-y-0 left-0 z-40 flex w-64 flex-col md:hidden"
            variants={sidebarMobileVariants}
            initial="hidden"
            animate="visible"
            exit="exit"
          >
            {sidebarContent}
          </motion.aside>
        )}
      </AnimatePresence>
    </>
  )
}
