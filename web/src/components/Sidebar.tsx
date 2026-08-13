import { NavLink, useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useServerProfileStore } from '@/stores/serverProfile'
import { useEffect, useState, useCallback, useRef, type ReactNode } from 'react'
import { libraryApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import type { Library } from '@/types'
import { useTranslation } from '@/i18n'
import LanguageSwitcher from '@/components/LanguageSwitcher'
import {
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
  Zap,
} from 'lucide-react'

interface SidebarProps {
  isMobileOpen?: boolean
  onMobileClose?: () => void
}

interface RailLinkProps {
  to: string
  icon: ReactNode
  label: string
  end?: boolean
}

function RailLink({ to, icon, label, end = false }: RailLinkProps) {
  return (
    <NavLink to={to} end={end} className="nv-rail-item" aria-label={label} title={label}>
      <span className="nv-rail-icon">{icon}</span>
      <span className="nv-rail-label">{label}</span>
    </NavLink>
  )
}

export default function Sidebar(_props: SidebarProps) {
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
      case 'movie': return <Film size={18} aria-hidden="true" />
      case 'tvshow': return <Tv size={18} aria-hidden="true" />
      case 'mixed': return <Layers size={18} aria-hidden="true" />
      case 'other': return <Video size={18} aria-hidden="true" />
      default: return <FolderOpen size={18} aria-hidden="true" />
    }
  }

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <>
      <aside className="nv-rail" aria-label="主导航">
        <div className="nv-rail-brand" aria-label="Nowen Video" title="Nowen Video">NV</div>

        <nav className="nv-rail-scroll">
          <RailLink to="/" end icon={<Home size={18} aria-hidden="true" />} label={t('nav.home')} />
          <RailLink to="/browse" icon={<Layers size={18} aria-hidden="true" />} label="影视库" />
          <RailLink to="/search" icon={<Search size={18} aria-hidden="true" />} label={t('nav.search')} />
          <RailLink to="/my" icon={<UserRound size={18} aria-hidden="true" />} label="我的" />

          {libraries.length > 0 && <div className="nv-rail-divider" aria-hidden="true" />}
          {libraries.map((library) => (
            <RailLink
              key={library.id}
              to={`/library/${library.id}`}
              icon={iconForType(library.type)}
              label={library.name}
            />
          ))}

          {user?.role === 'admin' && (
            <>
              <div className="nv-rail-divider" aria-hidden="true" />
              <RailLink to="/admin" icon={<Settings size={18} aria-hidden="true" />} label="管理" />
              {isFullProfile && <RailLink to="/files" icon={<FolderOpen size={18} aria-hidden="true" />} label="文件" />}
              {preprocessAvailable && <RailLink to="/preprocess" icon={<Zap size={18} aria-hidden="true" />} label="预处理" />}
            </>
          )}
        </nav>

        <div className="nv-rail-footer">
          <LanguageSwitcher compact />
          <button
            type="button"
            className="nv-rail-item"
            onClick={toggleTheme}
            aria-label={theme === 'dark' ? t('nav.switchToLight') : t('nav.switchToDark')}
            title={theme === 'dark' ? t('nav.switchToLight') : t('nav.switchToDark')}
          >
            <span className="nv-rail-icon">
              {theme === 'dark' ? <Moon size={17} aria-hidden="true" /> : <Sun size={17} aria-hidden="true" />}
            </span>
            <span className="nv-rail-label">主题</span>
          </button>
          <button
            type="button"
            className="nv-rail-item"
            onClick={handleLogout}
            aria-label={t('nav.logout')}
            title={t('nav.logout')}
          >
            <span className="nv-rail-icon"><LogOut size={17} aria-hidden="true" /></span>
            <span className="nv-rail-label">退出</span>
          </button>
        </div>
      </aside>

      <nav className="nv-mobile-nav" aria-label="移动端主导航">
        <RailLink to="/" end icon={<Home size={18} aria-hidden="true" />} label={t('nav.home')} />
        <RailLink to="/browse" icon={<Layers size={18} aria-hidden="true" />} label="影视库" />
        <RailLink to="/search" icon={<Search size={18} aria-hidden="true" />} label={t('nav.search')} />
        <RailLink to="/my" icon={<UserRound size={18} aria-hidden="true" />} label="我的" />
      </nav>
    </>
  )
}
