import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useTranslation } from '@/i18n'
import { BottomNavigation, NavigationRailLink, NavigationRailSection } from '@/ui'
import {
  ChevronLeft,
  ChevronRight,
  Clock3,
  Film,
  Home,
  Layers,
  LogOut,
  Moon,
  Search,
  Settings,
  Sun,
  UserRound,
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
  const { t } = useTranslation()
  const navigate = useNavigate()
  const isDarkTheme = theme === 'dark'
  const themeActionLabel = isDarkTheme ? t('nav.switchToLight') : t('nav.switchToDark')
  const collapseActionLabel = collapsed ? '展开侧边栏' : '收起侧边栏'

  const displayName = user?.nickname?.trim() || user?.username || 'Admin'
  const initials = displayName.slice(0, 1).toUpperCase()
  const mobileNavigationItems = [
    { to: '/', end: true, icon: <Home size={18} aria-hidden="true" />, label: t('nav.home') },
    {
      to: '/browse',
      icon: <Film size={18} aria-hidden="true" />,
      label: '影视库',
      activeOn: ['/media/', '/series/', '/collections/'],
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
          <div className="nv-rail-brand" aria-hidden="true">N</div>
          <div className="nv-rail-brand-copy">
            <strong>NOWEN VIDEO</strong>
            <span>MEDIA LIBRARY</span>
          </div>
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
                ? <ChevronRight size={15} aria-hidden="true" />
                : <ChevronLeft size={15} aria-hidden="true" />}
            </button>
          )}
        </div>

        <nav className="nv-rail-scroll">
          <NavigationRailSection>
            <NavigationRailLink to="/" end icon={<Home size={17} aria-hidden="true" />} label={t('nav.home')} />
            <NavigationRailLink to="/browse" icon={<Film size={17} aria-hidden="true" />} label="影视库" />
            <NavigationRailLink to="/collections" icon={<Layers size={17} aria-hidden="true" />} label="合集" />
            <NavigationRailLink to="/search" icon={<Search size={17} aria-hidden="true" />} label={t('nav.search')} />
            <NavigationRailLink to="/my" icon={<UserRound size={17} aria-hidden="true" />} label="我的" />
          </NavigationRailSection>

          <NavigationRailSection title="我的列表">
            <NavigationRailLink to="/favorites" icon={<Clock3 size={17} aria-hidden="true" />} label="稍后观看" />
          </NavigationRailSection>

          {user?.role === 'admin' && (
            <NavigationRailSection title="更多">
              <NavigationRailLink to="/admin" icon={<Settings size={17} aria-hidden="true" />} label="管理中心" />
            </NavigationRailSection>
          )}
        </nav>

        <div className="nv-rail-footer">
          <button
            type="button"
            className="nv-rail-profile"
            onClick={() => navigate('/profile')}
            aria-label="打开个人资料"
            title={collapsed ? `${displayName} · ${user?.role === 'admin' ? 'admin' : 'user'}` : undefined}
          >
            <div className="nv-rail-avatar" aria-hidden="true">{initials}</div>
            <div className="nv-rail-profile-copy">
              <strong>{displayName}</strong>
              <span>{user?.role === 'admin' ? 'admin' : 'user'}</span>
            </div>
          </button>
          <div className="nv-rail-footer-actions">
            <button
              type="button"
              className="nv-rail-action"
              onClick={() => navigate(user?.role === 'admin' ? '/admin' : '/profile')}
              aria-label={user?.role === 'admin' ? '管理中心' : '个人资料'}
              title={user?.role === 'admin' ? '管理中心' : '个人资料'}
            >
              <Settings size={16} aria-hidden="true" />
            </button>
            <button
              type="button"
              className="nv-rail-action"
              onClick={toggleTheme}
              aria-label={themeActionLabel}
              aria-pressed={!isDarkTheme}
              title={themeActionLabel}
            >
              {isDarkTheme ? <Sun size={16} aria-hidden="true" /> : <Moon size={16} aria-hidden="true" />}
            </button>
            <button
              type="button"
              className="nv-rail-action"
              onClick={handleLogout}
              aria-label={t('nav.logout')}
              title={t('nav.logout')}
            >
              <LogOut size={16} aria-hidden="true" />
            </button>
          </div>
        </div>
      </aside>

      <BottomNavigation items={mobileNavigationItems} />
    </>
  )
}
