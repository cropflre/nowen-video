import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { Search } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import Sidebar from './Sidebar'
import { PageContainer } from './design-system'
import { AppShell } from '@/ui'

const SCROLL_KEY_PREFIX = 'nowen_scroll_'
const SIDEBAR_COLLAPSED_KEY = 'nowen_sidebar_collapsed'
const WIDE_PAGE_PREFIXES = ['/browse', '/library/', '/files', '/preprocess', '/admin', '/collections', '/media/', '/series/', '/person/']
const DETAIL_PAGE_PREFIXES = ['/media/', '/series/', '/person/']

const TITLE_BY_PREFIX: Array<[string, string]> = [
  ['/browse', '找片'],
  ['/search', '搜索'],
  ['/favorites', '想看'],
  ['/history', '观看历史'],
  ['/playlists', '播放列表'],
  ['/collections', '合集'],
  ['/files', '文件管理'],
  ['/preprocess', '任务中心'],
  ['/admin', '家庭管理'],
  ['/stats', '统计'],
  ['/profile', '个人资料'],
  ['/my', '我的'],
  ['/library/', '媒体库'],
  ['/series/', '剧集详情'],
  ['/media/', '媒体详情'],
  ['/person/', '人物详情'],
]

const SAFE_INLINE_STYLE = {
  paddingInlineStart: 'max(var(--nv-page-gutter), env(safe-area-inset-left, 0px))',
  paddingInlineEnd: 'max(var(--nv-page-gutter), env(safe-area-inset-right, 0px))',
} as const

function resolveTitle(pathname: string) {
  if (pathname === '/') return '首页'
  return TITLE_BY_PREFIX.find(([prefix]) => pathname.startsWith(prefix))?.[1] ?? 'Nowen Video'
}

function readInitialSidebarCollapsed() {
  try {
    return window.localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}

function MinimalToolbar() {
  const location = useLocation()
  const navigate = useNavigate()
  const title = useMemo(() => resolveTitle(location.pathname), [location.pathname])
  const isDetailRoute = DETAIL_PAGE_PREFIXES.some((prefix) => location.pathname.startsWith(prefix))
  const searchFromUrl = location.pathname === '/search' ? new URLSearchParams(location.search).get('q') || '' : ''
  const [query, setQuery] = useState(searchFromUrl)

  useEffect(() => {
    if (location.pathname === '/search') setQuery(searchFromUrl)
  }, [location.pathname, searchFromUrl])

  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const next = query.trim()
    navigate(next ? `/search?q=${encodeURIComponent(next)}` : '/search')
  }

  if (location.pathname === '/browse' || location.pathname === '/search') return null

  return (
    <header className="nv-minimal-toolbar" style={SAFE_INLINE_STYLE}>
      <div className="nv-minimal-toolbar-side">
        {!isDetailRoute && <span className="nv-toolbar-route-title">{title}</span>}
      </div>

      <form className="nv-toolbar-search" role="search" onSubmit={handleSearch}>
        <Search size={14} aria-hidden="true" />
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="搜索片名"
          aria-label="搜索片名"
        />
      </form>

      <div className="nv-minimal-toolbar-side nv-minimal-toolbar-side--end" aria-hidden="true" />
    </header>
  )
}

export default function Layout() {
  const location = useLocation()
  const mainRef = useRef<HTMLElement>(null)
  const viewerId = useAuthStore((state) => state.user?.id)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(readInitialSidebarCollapsed)
  const isWidePage = location.pathname === '/' || WIDE_PAGE_PREFIXES.some((prefix) => location.pathname.startsWith(prefix))
  const usesLocalDetailChrome = location.pathname.startsWith('/media/') || location.pathname.startsWith('/series/')

  useEffect(() => {
    try {
      window.localStorage.setItem(SIDEBAR_COLLAPSED_KEY, sidebarCollapsed ? '1' : '0')
    } catch {
      // Storage may be unavailable in privacy-restricted browser contexts.
    }
  }, [sidebarCollapsed])

  useEffect(() => {
    const mainEl = mainRef.current
    if (!mainEl) return
    const key = SCROLL_KEY_PREFIX + viewerId + location.pathname + location.search
    let target = 0
    try { target = Math.max(0, Number(sessionStorage.getItem(key)) || 0) } catch { /* Optional storage. */ }
    let restoring = target > 0
    let lastPosition = target
    let frame = 0
    const save = () => {
      if (restoring) return
      try { sessionStorage.setItem(key, String(lastPosition)) } catch { /* Optional storage. */ }
    }
    const restore = () => {
      if (!restoring) return
      mainEl.scrollTop = target
      if (Math.abs(mainEl.scrollTop - target) < 1) { restoring = false; lastPosition = mainEl.scrollTop }
    }
    // Lazy pages initially render a short loader. Restore again when their
    // content grows instead of overwriting the saved offset with a clamped zero.
    const observer = new ResizeObserver(restore)
    Array.from(mainEl.children).forEach((child) => observer.observe(child))
    mainEl.scrollTop = 0
    frame = requestAnimationFrame(restore)
    const stopRestoring = () => { restoring = false }
    const handleScroll = () => {
      if (!restoring) lastPosition = mainEl.scrollTop
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(save)
    }
    mainEl.addEventListener('scroll', handleScroll, { passive: true })
    mainEl.addEventListener('wheel', stopRestoring, { passive: true })
    mainEl.addEventListener('touchstart', stopRestoring, { passive: true })
    mainEl.addEventListener('pointerdown', stopRestoring)
    mainEl.addEventListener('keydown', stopRestoring)
    return () => {
      cancelAnimationFrame(frame)
      save()
      observer.disconnect()
      mainEl.removeEventListener('scroll', handleScroll)
      mainEl.removeEventListener('wheel', stopRestoring)
      mainEl.removeEventListener('touchstart', stopRestoring)
      mainEl.removeEventListener('pointerdown', stopRestoring)
      mainEl.removeEventListener('keydown', stopRestoring)
    }
  }, [location.pathname, location.search, viewerId])

  return (
    <AppShell
      sidebar={(
        <Sidebar
          collapsed={sidebarCollapsed}
          onCollapsedChange={setSidebarCollapsed}
        />
      )}
      sidebarCollapsed={sidebarCollapsed}
    >
      <main
        ref={mainRef}
        id="main-scroll-container"
        className="nv-main-scroll relative min-w-0 flex-1 overflow-y-auto overscroll-contain"
      >
        <MinimalToolbar />
        <PageContainer
          width={isWidePage ? 'wide' : 'content'}
          className={usesLocalDetailChrome ? 'nv-page-container--detail' : 'nv-page-container--minimal'}
          style={SAFE_INLINE_STYLE}
        >
          <Outlet />
        </PageContainer>
      </main>
    </AppShell>
  )
}
