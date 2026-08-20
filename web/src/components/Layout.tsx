import { Link, Outlet, useLocation } from 'react-router-dom'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Clock3, Heart } from 'lucide-react'
import Sidebar from './Sidebar'
import { PageContainer } from './design-system'
import { AppShell, PageHeader } from '@/ui'

const SCROLL_KEY_PREFIX = 'nowen_scroll_'
const SIDEBAR_COLLAPSED_KEY = 'nowen_sidebar_collapsed'
const WIDE_PAGE_PREFIXES = ['/files', '/preprocess', '/admin', '/collections', '/media/', '/series/', '/person/']

const TITLE_BY_PREFIX: Array<[string, string]> = [
  ['/browse', '影视库'],
  ['/search', '搜索'],
  ['/favorites', '收藏'],
  ['/history', '观看历史'],
  ['/playlists', '播放列表'],
  ['/collections', '合集'],
  ['/files', '文件管理'],
  ['/preprocess', '任务中心'],
  ['/admin', '管理中心'],
  ['/stats', '统计'],
  ['/profile', '个人资料'],
  ['/my', '我的'],
  ['/library/', '媒体库'],
  ['/series/', '剧集详情'],
  ['/media/', '媒体详情'],
  ['/person/', '演员详情'],
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

function ApplicationTopBar() {
  const location = useLocation()
  const isHomeRoute = location.pathname === '/'
  const title = useMemo(() => resolveTitle(location.pathname), [location.pathname])

  const actions = (
    <>
      <Link to="/history" className="nv-page-header-action nv-page-header-action--label" aria-label="观看历史" title="观看历史">
        <Clock3 size={15} aria-hidden="true" />
        <span>观看历史</span>
      </Link>
      <Link to="/favorites" className="nv-page-header-action nv-page-header-action--label" aria-label="我的收藏" title="我的收藏">
        <Heart size={15} aria-hidden="true" />
        <span>我的收藏</span>
      </Link>
    </>
  )

  return (
    <PageHeader
      title={title}
      subtitle={isHomeRoute ? '精选推荐 · 精彩不断' : undefined}
      showSearch={false}
      showSearchShortcut={false}
      actions={actions}
      className="nv-topbar--navigation-only"
      style={SAFE_INLINE_STYLE}
    />
  )
}

export default function Layout() {
  const location = useLocation()
  const mainRef = useRef<HTMLElement>(null)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(readInitialSidebarCollapsed)
  const isWidePage = WIDE_PAGE_PREFIXES.some((prefix) => location.pathname.startsWith(prefix))
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
    const currentKey = SCROLL_KEY_PREFIX + location.pathname + location.search
    const savedPos = sessionStorage.getItem(currentKey)
    requestAnimationFrame(() => {
      mainEl.scrollTop = savedPos ? parseInt(savedPos, 10) : 0
    })
  }, [location.pathname, location.search])

  useEffect(() => {
    const mainEl = mainRef.current
    if (!mainEl) return
    let ticking = false
    const handleScroll = () => {
      if (ticking) return
      ticking = true
      requestAnimationFrame(() => {
        sessionStorage.setItem(
          SCROLL_KEY_PREFIX + location.pathname + location.search,
          String(mainEl.scrollTop),
        )
        ticking = false
      })
    }
    mainEl.addEventListener('scroll', handleScroll, { passive: true })
    return () => mainEl.removeEventListener('scroll', handleScroll)
  }, [location.pathname, location.search])

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
        {!usesLocalDetailChrome && <ApplicationTopBar />}
        <PageContainer
          width={isWidePage ? 'wide' : 'content'}
          className={usesLocalDetailChrome ? 'nv-page-container--detail' : undefined}
          style={SAFE_INLINE_STYLE}
        >
          <Outlet />
        </PageContainer>
      </main>
    </AppShell>
  )
}
