import { NavLink, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useServerProfileStore } from '@/stores/serverProfile'
import { useEffect, useState, useCallback, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { libraryApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import type { Library } from '@/types'
import LanguageSwitcher from './LanguageSwitcher'
import { useTranslation } from '@/i18n'
import {
  ChevronLeft,
  Film,
  FolderOpen,
  Home,
  Layers,
  LogOut,
  Moon,
  Search,
  Settings,
  Sun,
  Tv,
  UserRound,
  Video,
  X,
  Zap,
} from 'lucide-react'
import clsx from 'clsx'
import { motion, AnimatePresence } from 'framer-motion'
import { sidebarVariants, sidebarMobileVariants } from '@/lib/motion'

interface SidebarProps {
  isMobileOpen?: boolean
  onMobileClose?: () => void
}

export default function Sidebar({ isMobileOpen = false, onMobileClose }: SidebarProps) {
  const { user, logout } = useAuthStore()
  const { theme, toggleTheme } = useThemeStore()
  const manifest = useServerProfileStore((state) => state.manifest)
  const isFullProfile = manifest?.profile === 'full'
  const preprocessAvailable = manifest?.capabilities.preprocess?.available === true
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [libraries, setLibraries] = useState<Library[]>([])
  const [collapsed, setCollapsed] = useState(false)
  const { on, off } = useWebSocket()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const fetchLibraries = useCallback(() => {
    libraryApi.list().then((res) => {
      setLibraries(res.data.data)
    }).catch(() => {})
  }, [])

  useEffect(() => {
    fetchLibraries()
  }, [fetchLibraries])

  useEffect(() => {
    const debouncedRefresh = () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(() => fetchLibraries(), 500)
    }
    const bumpOnMediaChange = () => bumpPosterVersion()

    on(WS_EVENTS.LIBRARY_DELETED, debouncedRefresh)
    on(WS_EVENTS.LIBRARY_UPDATED, debouncedRefresh)
    on(WS_EVENTS.SCAN_COMPLETED, debouncedRefresh)
    on(WS_EVENTS.SCAN_COMPLETED, bumpOnMediaChange)
    on(WS_EVENTS.SCRAPE_COMPLETED, bumpOnMediaChange)

    return () => {
      off(WS_EVENTS.LIBRARY_DELETED, debouncedRefresh)
      off(WS_EVENTS.LIBRARY_UPDATED, debouncedRefresh)
      off(WS_EVENTS.SCAN_COMPLETED, debouncedRefresh)
      off(WS_EVENTS.SCAN_COMPLETED, bumpOnMediaChange)
      off(WS_EVENTS.SCRAPE_COMPLETED, bumpOnMediaChange)
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

  const showText = !collapsed || isMobileOpen
  const navItem = (to: string, icon: ReactNode, label: string, end = false) => (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) => clsx('nav-item', isActive && 'active')}
      onClick={onMobileClose}
    >
      {icon}
      {showText && <span>{label}</span>}
    </NavLink>
  )

  const sidebarContent = (
    <>
      <div className="absolute right-0 top-0 bottom-0 w-px bg-gradient-to-b from-transparent via-neon-blue/20 to-transparent" />

      <div className="flex h-16 items-center justify-between px-4">
        {showText ? (
          <h1 className="font-display text-lg font-bold tracking-wider">
            <span className="text-neon text-neon-glow">N</span>
            <span style={{ color: 'var(--text-primary)' }}>OWEN</span>
          </h1>
        ) : (
          <div className="flex w-full justify-center">
            <Zap size={20} className="text-neon animate-neon-breathe" />
          </div>
        )}

        {!collapsed && !isMobileOpen && (
          <button
            onClick={() => setCollapsed(true)}
            className="hidden rounded-lg p-1.5 text-surface-400 transition-all hover:bg-neon-blue/5 hover:text-neon md:block"
            aria-label="收起侧边栏"
          >
            <ChevronLeft size={18} />
          </button>
        )}
        {isMobileOpen && onMobileClose && (
          <button
            onClick={(event) => { event.stopPropagation(); onMobileClose() }}
            className="rounded-lg p-2 text-surface-400 transition-all hover:bg-neon-blue/5 hover:text-neon md:hidden"
            aria-label="关闭菜单"
          >
            <X size={20} />
          </button>
        )}
      </div>

      <nav className="flex-1 space-y-0.5 overflow-y-auto px-2 py-4">
        {navItem('/', <Home size={18} />, t('nav.home'), true)}
        {navItem('/browse', <Layers size={18} />, '影视库')}
        {navItem('/search', <Search size={18} />, t('nav.search'))}
        {navItem('/my', <UserRound size={18} />, '我的')}

        {libraries.length > 0 && (
          <>
            {showText ? (
              <div className="px-3 pb-1 pt-6 text-[10px] font-bold uppercase tracking-[0.2em] text-neon/40">
                {t('nav.libraries')}
              </div>
            ) : (
              <div className="my-3 mx-3 border-t border-neon-blue/10" />
            )}
            {libraries.map((library) => (
              <NavLink
                key={library.id}
                to={`/library/${library.id}`}
                className={({ isActive }) => clsx('nav-item', isActive && 'active')}
                onClick={onMobileClose}
              >
                {iconForType(library.type)}
                {showText && <span className="truncate">{library.name}</span>}
              </NavLink>
            ))}
          </>
        )}

        {user?.role === 'admin' && (
          <>
            {showText ? (
              <div className="px-3 pb-1 pt-6 text-[10px] font-bold uppercase tracking-[0.2em] text-neon/40">
                {t('nav.management')}
              </div>
            ) : (
              <div className="my-3 mx-3 border-t border-neon-blue/10" />
            )}
            {navItem('/admin', <Settings size={18} />, '管理中心')}
            {isFullProfile && navItem('/files', <FolderOpen size={18} />, '文件管理')}
            {preprocessAvailable && navItem('/preprocess', <Zap size={18} />, '预处理')}
          </>
        )}
      </nav>

      <div className="border-t p-3 border-[var(--border-default)]">
        <div className={clsx('mb-3', !showText && 'flex justify-center')}>
          <button
            onClick={toggleTheme}
            onKeyDown={(event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                toggleTheme()
              }
            }}
            className={clsx(
              'theme-toggle-btn group relative flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-all duration-300',
              showText ? 'w-full' : 'justify-center'
            )}
            style={{
              color: 'var(--text-secondary)',
              background: theme === 'light' ? 'var(--nav-hover-bg)' : undefined,
              border: theme === 'light' ? '1px solid var(--border-default)' : '1px solid transparent',
            }}
            title={theme === 'dark' ? t('nav.switchToLight') : t('nav.switchToDark')}
            aria-label={theme === 'dark' ? t('nav.switchToLight') : t('nav.switchToDark')}
            role="switch"
            aria-checked={theme === 'dark'}
          >
            <div className="relative flex h-[18px] w-[18px] shrink-0 items-center justify-center">
              <Sun
                size={18}
                className={clsx(
                  'absolute transition-all duration-500',
                  theme === 'light' ? 'rotate-0 scale-100 opacity-100 text-amber-500' : 'rotate-90 scale-0 opacity-0'
                )}
              />
              <Moon
                size={18}
                className={clsx(
                  'absolute transition-all duration-500',
                  theme === 'dark' ? 'rotate-0 scale-100 opacity-100 text-neon' : '-rotate-90 scale-0 opacity-0'
                )}
              />
            </div>
            {showText && (
              <span className="transition-colors group-hover:text-[var(--text-primary)]">
                {theme === 'dark' ? t('nav.darkMode') : t('nav.lightMode')}
              </span>
            )}
          </button>
        </div>

        {showText && <LanguageSwitcher />}

        <div className="mt-3 flex items-center gap-3">
          <div
            className="relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-bold"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
              boxShadow: 'var(--shadow-neon)',
              color: 'var(--text-on-neon)',
            }}
          >
            {user?.username?.charAt(0).toUpperCase()}
          </div>
          {showText && (
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-theme-primary">{user?.username}</p>
              <p className="text-xs text-theme-tertiary">
                {user?.role === 'admin' ? t('user.admin') : t('user.user')}
              </p>
            </div>
          )}
          {showText && (
            <button
              onClick={handleLogout}
              className="rounded-lg p-1.5 text-surface-400 transition-all hover:bg-red-400/5 hover:text-red-400"
              title={t('nav.logout')}
              aria-label={t('nav.logout')}
            >
              <LogOut size={16} />
            </button>
          )}
        </div>

        {collapsed && !isMobileOpen && (
          <button
            onClick={() => setCollapsed(false)}
            className="mt-3 flex w-full justify-center rounded-lg p-1.5 text-surface-500 transition-all hover:bg-neon-blue/5 hover:text-neon"
            aria-label="展开侧边栏"
          >
            <ChevronLeft size={16} className="rotate-180" />
          </button>
        )}
      </div>
    </>
  )

  return (
    <>
      <motion.aside
        className="glass-panel-strong relative z-20 hidden h-screen flex-shrink-0 flex-col overflow-hidden md:flex"
        animate={collapsed ? 'collapsed' : 'expanded'}
        variants={sidebarVariants}
        style={{ willChange: 'width' }}
      >
        {sidebarContent}
      </motion.aside>

      {createPortal(
        <>
          <AnimatePresence>
            {isMobileOpen && (
              <motion.div
                key="sidebar-overlay"
                className="fixed inset-0 z-[9998] bg-black/60 md:hidden"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.2 }}
                onClick={onMobileClose}
                aria-hidden="true"
              />
            )}
          </AnimatePresence>
          <AnimatePresence>
            {isMobileOpen && (
              <motion.aside
                key="sidebar-drawer"
                className="glass-panel-strong fixed inset-y-0 left-0 z-[9999] flex w-64 flex-col md:hidden"
                variants={sidebarMobileVariants}
                initial="hidden"
                animate="visible"
                exit="exit"
              >
                {sidebarContent}
              </motion.aside>
            )}
          </AnimatePresence>
        </>,
        document.body
      )}
    </>
  )
}
