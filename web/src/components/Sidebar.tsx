import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useTranslation } from '@/i18n'
import { BottomNavigation, NavigationRailLink, NavigationRailSection } from '@/ui'
import { ChevronLeft, ChevronRight, Film, Heart, Home, LogOut, Moon, Sun, UserRound } from 'lucide-react'

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
  const navigationItems = [
    { to: '/', end: true, icon: <Home size={18} aria-hidden="true" />, label: t('nav.home') },
    {
      to: '/browse', icon: <Film size={18} aria-hidden="true" />, label: t('nav.browse'),
      activeOn: ['/search', '/media/', '/series/', '/collections', '/library/', '/person/'],
    },
    { to: '/favorites', icon: <Heart size={18} aria-hidden="true" />, label: t('nav.favorites') },
    {
      to: '/my', icon: <UserRound size={18} aria-hidden="true" />, label: t('nav.my'),
      activeOn: ['/profile', '/history', '/playlists', '/stats', '/admin', '/files', '/preprocess'],
    },
  ]

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <>
      <aside id="main-sidebar" className="nv-rail" aria-label="主导航" data-collapsed={collapsed ? 'true' : 'false'}>
        <div className="nv-rail-brand-row">
          {!collapsed && (
            <button type="button" className="nv-rail-wordmark" onClick={() => navigate('/')} aria-label="返回首页">
              <span className="nv-rail-brand-copy">
                <strong>NOWEN</strong>
                <span>Video</span>
              </span>
            </button>
          )}

          {onCollapsedChange && (
            <button
              type="button"
              className="nv-rail-collapse-toggle"
              style={{
                position: 'static',
                marginInlineStart: collapsed ? 0 : 'auto',
                width: 28,
                minWidth: 28,
                height: 28,
                border: 0,
                background: 'transparent',
                boxShadow: 'none',
              }}
              onClick={() => onCollapsedChange(!collapsed)}
              aria-label={collapseActionLabel}
              aria-controls="main-sidebar"
              aria-expanded={!collapsed}
              title={collapseActionLabel}
            >
              {collapsed
                ? <ChevronRight size={16} aria-hidden="true" />
                : <ChevronLeft size={16} aria-hidden="true" />}
            </button>
          )}
        </div>

        <nav className="nv-rail-scroll">
          <NavigationRailSection>
            {navigationItems.map((item) => <NavigationRailLink key={item.to} {...item} />)}
          </NavigationRailSection>
        </nav>

        <div className="nv-rail-footer">
          <button
            type="button"
            className="nv-rail-profile"
            onClick={() => navigate('/my')}
            aria-label={t('nav.my')}
            title={collapsed ? displayName : undefined}
          >
            <span className="nv-rail-avatar" aria-hidden="true">{initials}</span>
            <span className="nv-rail-profile-copy">
              <strong>{displayName}</strong>
              <span>{user?.child_mode ? '儿童空间' : user?.role === 'admin' ? '管理员' : '用户'}</span>
            </span>
          </button>

          <div className="nv-rail-footer-actions">
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

      <BottomNavigation items={navigationItems} />
    </>
  )
}
