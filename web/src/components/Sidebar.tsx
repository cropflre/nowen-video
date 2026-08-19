import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useServerProfileStore } from '@/stores/serverProfile'
import { useEffect, useState, useCallback, useRef } from 'react'
import { libraryApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import type { Library } from '@/types'
import { useTranslation } from '@/i18n'
import LanguageSwitcher from '@/components/LanguageSwitcher'
import { BottomNavigation, NavigationRailLink, NavigationRailSection } from '@/ui'
import {
  Film,
  FolderOpen,
  Home,
  Layers,
  LogOut,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Search,
  Settings,
  Sun,
  Tv,
  UserRound,
  Video,
  Zap,
} from 'lucide-react'

interface SidebarProps {
  isMobileOpen?: boolean
  onMobileClose?: () => void
  collapsed?: boolean
  onCollapsedChange?: (collapsed: boolean) => void
}

export default function Sidebar({ collapsed = false, onCollapsedChange }: SidebarProps) {
  const { user, logout } = useAuthStore()
  const { theme, toggleTheme } = useThemeStore()
  const manifest = useServerProfileStore((state) => state.manifest)
  const isFullProfile = manifest?.profile === 'full'
  const preprocessAvailable = manifest?.capabilities.preprocess?.available === true
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [libraries, setLibraries] = useState<Library[]>([])
  const { on, off } = useWebSocket()
  const refreshTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isDarkTheme = theme === 'dark'
  const themeActionLabel = isDarkTheme ? t('nav.switchToLight') : t('nav.switchToDark')
  const collapseActionLabel = collapsed ? '展开侧边栏' : '收起侧边栏'

  const fetchLibraries = useCallback(() => {
    libraryApi.list().then((res) => setLibraries(res.data.data)).catch(() => {})
  }, [])

  useEffect(() => { fetchLibraries() }, [fetchLibraries])

  useEffect(() => {
    const debouncedRefresh = () => {
      if (refreshTimerRef.current) clearTimeout(refreshTimerRef.current)
      refreshTimerRef.current = setTimeout(fetchLibraries, 500)
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

  const iconForType = (type: string) => {
    switch (type) {
      case 'movie': return <Film size={16} aria-hidden="true" />
      case 'tvshow': return <Tv size={16} aria-hidden="true" />
      case 'mixed': return <Layers size={16} aria-hidden="true" />
      case 'other': return <Video size={16} aria-hidden="true" />
      default: return <FolderOpen size={16} aria-hidden="true" />
    }
  }

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const displayName = user?.nickname?.trim() || user?.username || 'Nowen'
  const initials = displayName.slice(0, 2).toUpperCase()
  const mobileNavigationItems = [
    { to: '/', end: true, icon: <Home size={18} aria-hidden="true" />, label: t('nav.home') },
    { to: '/browse', icon: <Film size={18} aria-hidden="true" />, label: '影视库' },
    { to: '/search', icon: <Search size={18} aria-hidden="true" />, label: t('nav.search') },
    { to: '/my', icon: <UserRound size={18} aria-hidden="true" />, label: '我的' },
  ]

  return (
    <>
      <aside id="main-sidebar" className="nv-rail" aria-label="主导航" data-collapsed={collapsed ? 'true' : 'false'}>
        <div className="nv-rail-brand-row">
          <div className="nv-rail-brand" aria-hidden="true">N</div>
          <div className="nv-rail-brand-copy">
            <strong>Nowen Video</strong>
            <span>MEDIA LIBRARY</span>
          </div>
        </div>

        <nav className="nv-rail-scroll">
          <NavigationRailSection title="浏览">
            <NavigationRailLink to="/" end icon={<Home size={16} aria-hidden="true" />} label={t('nav.home')} />
            <NavigationRailLink to="/browse" icon={<Film size={16} aria-hidden="true" />} label="影视库" />
            <NavigationRailLink to="/collections" icon={<Layers size={16} aria-hidden="true" />} label="合集" />
            <NavigationRailLink to="/search" icon={<Search size={16} aria-hidden="true" />} label={t('nav.search')} />
            <NavigationRailLink to="/my" icon={<UserRound size={16} aria-hidden="true" />} label="我的" />
          </NavigationRailSection>

          {libraries.length > 0 && (
            <NavigationRailSection title="媒体库">
              {libraries.map((library) => (
                <NavigationRailLink
                  key={library.id}
                  to={`/library/${library.id}`}
                  icon={iconForType(library.type)}
                  label={library.name}
                  meta={typeof library.media_count === 'number' ? library.media_count : undefined}
                />
              ))}
            </NavigationRailSection>
          )}

          {user?.role === 'admin' && (
            <NavigationRailSection title="管理">
              <NavigationRailLink to="/admin" icon={<Settings size={16} aria-hidden="true" />} label="管理中心" />
              {isFullProfile && <NavigationRailLink to="/files" icon={<FolderOpen size={16} aria-hidden="true" />} label="文件管理" />}
              {preprocessAvailable && <NavigationRailLink to="/preprocess" icon={<Zap size={16} aria-hidden="true" />} label="任务中心" />}
            </NavigationRailSection>
          )}
        </nav>

        <div className="nv-rail-footer">
          <div className="nv-rail-profile" title={collapsed ? `${displayName} · ${user?.role === 'admin' ? 'admin' : 'user'}` : undefined}>
            <div className="nv-rail-avatar" aria-hidden="true">{initials}</div>
            <div className="nv-rail-profile-copy">
              <strong>{displayName}</strong>
              <span>{user?.role === 'admin' ? 'admin' : 'user'}</span>
            </div>
          </div>
          <div className="nv-rail-footer-actions">
            {onCollapsedChange && (
              <button
                type="button"
                className="nv-rail-action nv-rail-collapse-toggle"
                onClick={() => onCollapsedChange(!collapsed)}
                aria-label={collapseActionLabel}
                aria-controls="main-sidebar"
                aria-expanded={!collapsed}
                title={collapseActionLabel}
              >
                {collapsed
                  ? <PanelLeftOpen size={15} aria-hidden="true" />
                  : <PanelLeftClose size={15} aria-hidden="true" />}
              </button>
            )}
            <LanguageSwitcher compact />
            <button
              type="button"
              className="nv-rail-action"
              onClick={toggleTheme}
              aria-label={themeActionLabel}
              aria-pressed={!isDarkTheme}
              title={themeActionLabel}
            >
              {isDarkTheme ? <Sun size={15} aria-hidden="true" /> : <Moon size={15} aria-hidden="true" />}
            </button>
            <button
              type="button"
              className="nv-rail-action"
              onClick={handleLogout}
              aria-label={t('nav.logout')}
              title={t('nav.logout')}
            >
              <LogOut size={15} aria-hidden="true" />
            </button>
          </div>
        </div>
      </aside>

      <BottomNavigation items={mobileNavigationItems} />
    </>
  )
}
