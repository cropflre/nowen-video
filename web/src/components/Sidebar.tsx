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
import { Button } from './design-system'
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
      className={({ isActive }) => clsx('nav-item flex items-center gap-3 px-3 text-sm font-medium', isActive && 'active')}
      onClick={onMobileClose}
    >
      <span className="flex h-5 w-5 shrink-0 items-center justify-center">{icon}</span>
      {showText && <span className="truncate">{label}</span>}
    </NavLink>
  )

  const sectionLabel = (label: string) => showText ? (
    <div className="px-3 pb-1 pt-6 text-[10px] font-semibold uppercase tracking-[0.16em] text-[var(--nv-text-tertiary)]">
      {label}
    </div>
  ) : (
    <div className="mx-3 my-3 border-t border-[var(--nv-border-subtle)]" />
  )

  const sidebarContent = (
    <>
      <div className="flex h-16 shrink-0 items-center justify-between px-4">
        {showText ? (
          <h1 className="font-display text-lg font-bold tracking-[0.12em] text-[var(--nv-text-primary)]">
            <span className="text-[var(--nv-action-primary)]">N</span>OWEN
          </h1>
        ) : (
          <div className="flex w-full justify-center">
            <Zap size={20} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
          </div>
        )}

        {!collapsed && !isMobileOpen && (
          <Button
            variant="ghost"
            size="sm"
            iconOnly
            onClick={() => setCollapsed(true)}
            className="hidden md:inline-flex"
            aria-label="收起侧边栏"
          >
            <ChevronLeft size={17} />
          </Button>
        )}
        {isMobileOpen && onMobileClose && (
          <Button
            variant="ghost"
            size="sm"
            iconOnly
            onClick={(event) => { event.stopPropagation(); onMobileClose() }}
            className="md:hidden"
            aria-label="关闭菜单"
          >
            <X size={19} />
          </Button>
        )}
      </div>

      <nav className="flex-1 space-y-1 overflow-y-auto px-2 py-3" aria-label="主导航">
        {navItem('/', <Home size={18} />, t('nav.home'), true)}
        {navItem('/browse', <Layers size={18} />, '影视库')}
        {navItem('/search', <Search size={18} />, t('nav.search'))}
        {navItem('/my', <UserRound size={18} />, '我的')}

        {libraries.length > 0 && (
          <>
            {sectionLabel(t('nav.libraries'))}
            {libraries.map((library) => (
              <NavLink
                key={library.id}
                to={`/library/${library.id}`}
                className={({ isActive }) => clsx('nav-item flex items-center gap-3 px-3 text-sm font-medium', isActive && 'active')}
                onClick={onMobileClose}
              >
                <span className="flex h-5 w-5 shrink-0 items-center justify-center">{iconForType(library.type)}</span>
                {showText && <span className="truncate">{library.name}</span>}
              </NavLink>
            ))}
          </>
        )}

        {user?.role === 'admin' && (
          <>
            {sectionLabel(t('nav.management'))}
            {navItem('/admin', <Settings size={18} />, '管理中心')}
            {isFullProfile && navItem('/files', <FolderOpen size={18} />, '文件管理')}
            {preprocessAvailable && navItem('/preprocess', <Zap size={18} />, '预处理')}
          </>
        )}
      </nav>

      <div className="shrink-0 border-t border-[var(--nv-border-subtle)] p-3">
        <div className={clsx('mb-3', !showText && 'flex justify-center')}>
          <button
            onClick={toggleTheme}
            className={clsx(
              'nv-button nv-button--ghost nv-button--md group relative flex items-center gap-3',
              showText ? 'w-full justify-start' : 'nv-button--icon-only',
            )}
            data-variant="ghost"
            data-size="md"
            data-icon-only={!showText || undefined}
            title={theme === 'dark' ? t('nav.switchToLight') : t('nav.switchToDark')}
            aria-label={theme === 'dark' ? t('nav.switchToLight') : t('nav.switchToDark')}
            role="switch"
            aria-checked={theme === 'dark'}
          >
            <div className="relative flex h-[18px] w-[18px] shrink-0 items-center justify-center">
              <Sun
                size={18}
                className={clsx(
                  'absolute transition-all duration-200',
                  theme === 'light' ? 'rotate-0 scale-100 opacity-100 text-amber-500' : 'rotate-90 scale-0 opacity-0',
                )}
              />
              <Moon
                size={18}
                className={clsx(
                  'absolute transition-all duration-200',
                  theme === 'dark' ? 'rotate-0 scale-100 opacity-100 text-[var(--nv-action-primary)]' : '-rotate-90 scale-0 opacity-0',
                )}
              />
            </div>
            {showText && <span>{theme === 'dark' ? t('nav.darkMode') : t('nav.lightMode')}</span>}
          </button>
        </div>

        {showText && <LanguageSwitcher />}

        <div className="mt-3 flex items-center gap-3">
          <div
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-sm font-bold"
            style={{
              background: 'var(--nv-action-primary)',
              color: 'var(--nv-text-on-brand)',
            }}
            aria-hidden="true"
          >
            {user?.username?.charAt(0).toUpperCase()}
          </div>
          {showText && (
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{user?.username}</p>
              <p className="text-xs text-[var(--nv-text-tertiary)]">
                {user?.role === 'admin' ? t('user.admin') : t('user.user')}
              </p>
            </div>
          )}
          {showText && (
            <Button
              variant="ghost"
              size="sm"
              iconOnly
              onClick={handleLogout}
              title={t('nav.logout')}
              aria-label={t('nav.logout')}
            >
              <LogOut size={16} />
            </Button>
          )}
        </div>

        {collapsed && !isMobileOpen && (
          <Button
            variant="ghost"
            size="sm"
            iconOnly
            onClick={() => setCollapsed(false)}
            className="mx-auto mt-3 flex"
            aria-label="展开侧边栏"
          >
            <ChevronLeft size={16} className="rotate-180" />
          </Button>
        )}
      </div>
    </>
  )

  return (
    <>
      <motion.aside
        className="glass-panel-strong relative z-20 hidden h-screen flex-shrink-0 flex-col overflow-hidden border-r border-[var(--nv-border-subtle)] md:flex"
        animate={collapsed ? 'collapsed' : 'expanded'}
        variants={sidebarVariants}
        style={{ willChange: 'width' }}
        aria-label="侧边导航"
      >
        {sidebarContent}
      </motion.aside>

      {createPortal(
        <>
          <AnimatePresence>
            {isMobileOpen && (
              <motion.div
                key="sidebar-overlay"
                className="fixed inset-0 z-[9998] bg-black/55 md:hidden"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.16 }}
                onClick={onMobileClose}
                aria-hidden="true"
              />
            )}
          </AnimatePresence>
          <AnimatePresence>
            {isMobileOpen && (
              <motion.aside
                key="sidebar-drawer"
                className="glass-panel-strong fixed inset-y-0 left-0 z-[9999] flex w-64 flex-col border-r border-[var(--nv-border-subtle)] md:hidden"
                variants={sidebarMobileVariants}
                initial="hidden"
                animate="visible"
                exit="exit"
                aria-label="移动端侧边导航"
              >
                {sidebarContent}
              </motion.aside>
            )}
          </AnimatePresence>
        </>,
        document.body,
      )}
    </>
  )
}
