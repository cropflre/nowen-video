import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { libraryApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useTranslation } from '@/i18n'
import type { Library } from '@/types'
import { BottomNavigation, NavigationRailLink, NavigationRailSection } from '@/ui'
import {
  ChevronLeft,
  ChevronRight,
  Film,
  Folder,
  Heart,
  Home,
  LogOut,
  Moon,
  Search,
  Settings,
  Sun,
  Tv,
  UserRound,
} from 'lucide-react'

interface SidebarProps {
  isMobileOpen?: boolean
  onMobileClose?: () => void
  collapsed?: boolean
  onCollapsedChange?: (collapsed: boolean) => void
}

function libraryIcon(type: Library['type']) {
  if (type === 'tvshow') return <Tv size={15} aria-hidden="true" />
  if (type === 'movie') return <Film size={15} aria-hidden="true" />
  return <Folder size={15} aria-hidden="true" />
}

export default function Sidebar({ collapsed = false, onCollapsedChange }: SidebarProps) {
  const { user, logout } = useAuthStore()
  const { theme, toggleTheme } = useThemeStore()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const [libraries, setLibraries] = useState<Library[]>([])
  const isDarkTheme = theme === 'dark'
  const themeActionLabel = isDarkTheme ? t('nav.switchToLight') : t('nav.switchToDark')
  const collapseActionLabel = collapsed ? '展开侧边栏' : '收起侧边栏'

  const displayName = user?.nickname?.trim() || user?.username || 'Admin'
  const initials = displayName.slice(0, 1).toUpperCase()
  const currentLibraryId = useMemo(() => {
    const match = location.pathname.match(/^\/library\/([^/]+)/)
    return match?.[1] || ''
  }, [location.pathname])

  useEffect(() => {
    let cancelled = false
    libraryApi.list()
      .then((response) => {
        if (!cancelled) setLibraries(response.data.data || [])
      })
      .catch(() => {
        if (!cancelled) setLibraries([])
      })
    return () => { cancelled = true }
  }, [])

  const mobileNavigationItems = [
    { to: '/', end: true, icon: <Home size={18} aria-hidden="true" />, label: t('nav.home') },
    {
      to: '/browse',
      icon: <Film size={18} aria-hidden="true" />,
      label: '影视库',
      activeOn: ['/media/', '/series/', '/collections/', '/library/'],
    },
    { to: '/search', icon: <Search size={18} aria-hidden="true" />, label: t('nav.search') },
    { to: '/my', icon: <UserRound size={18} aria-hidden="true" />, label: '我的' },
  ]

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <>
      <aside id="main-sidebar" className="nv-rail" aria-label="主导航" data-collapsed={collapsed ? 'true' : 'false'}>
        <div className="nv-rail-brand-row">
          <button type="button" className="nv-rail-wordmark" onClick={() => navigate('/')} aria-label="返回首页">
            <span className="nv-rail-brand" aria-hidden="true">N</span>
            <span className="nv-rail-brand-copy">
              <strong>NOWEN</strong>
              <span>Video</span>
            </span>
          </button>
          {onCollapsedChange && (
            <button
              type="button"
              className="nv-rail-collapse-toggle"
              onClick={() => onCollapsedChange(!collapsed)}
              aria-label={collapseActionLabel}
              aria-controls="main-sidebar"
              aria-expanded={!collapsed}
              title={collapseActionLabel}
            >
              {collapsed
                ? <ChevronRight size={14} aria-hidden="true" />
                : <ChevronLeft size={14} aria-hidden="true" />}
            </button>
          )}
        </div>

        <nav className="nv-rail-scroll">
          <NavigationRailSection>
            <NavigationRailLink to="/" end icon={<Home size={15} aria-hidden="true" />} label={t('nav.home')} />
            <NavigationRailLink to="/browse" icon={<Film size={15} aria-hidden="true" />} label="影视库" />
            <NavigationRailLink to="/favorites" icon={<Heart size={15} aria-hidden="true" />} label="收藏" />
            <NavigationRailLink to="/search" icon={<Search size={15} aria-hidden="true" />} label={t('nav.search')} />
          </NavigationRailSection>

          {libraries.length > 0 && (
            <NavigationRailSection title="媒体库">
              {libraries.map((library) => (
                <NavigationRailLink
                  key={library.id}
                  to={`/library/${library.id}`}
                  icon={libraryIcon(library.type)}
                  label={library.name}
                  meta={library.media_count !== undefined ? library.media_count : undefined}
                />
              ))}
              {currentLibraryId && !libraries.some((library) => library.id === currentLibraryId) && (
                <NavigationRailLink to={`/library/${currentLibraryId}`} icon={<Folder size={15} />} label="当前媒体库" />
              )}
            </NavigationRailSection>
          )}
        </nav>

        <div className="nv-rail-footer">
          <button
            type="button"
            className="nv-rail-profile"
            onClick={() => navigate('/profile')}
            aria-label="打开个人资料"
            title={collapsed ? displayName : undefined}
          >
            <span className="nv-rail-avatar" aria-hidden="true">{initials}</span>
            <span className="nv-rail-profile-copy">
              <strong>{displayName}</strong>
              <span>{user?.role === 'admin' ? '管理员' : '用户'}</span>
            </span>
          </button>

          <div className="nv-rail-footer-actions">
            {user?.role === 'admin' && (
              <button type="button" className="nv-rail-action" onClick={() => navigate('/admin')} aria-label="管理中心" title="管理中心">
                <Settings size={15} aria-hidden="true" />
              </button>
            )}
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
            <button type="button" className="nv-rail-action" onClick={handleLogout} aria-label={t('nav.logout')} title={t('nav.logout')}>
              <LogOut size={15} aria-hidden="true" />
            </button>
          </div>
        </div>
      </aside>

      <BottomNavigation items={mobileNavigationItems} />
    </>
  )
}
