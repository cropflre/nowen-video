import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { type FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import Sidebar from './Sidebar'
import { PageContainer, SearchField } from './design-system'

const SCROLL_KEY_PREFIX = 'nowen_scroll_'
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

function ApplicationTopBar() {
  const location = useLocation()
  const navigate = useNavigate()
  const routeKeyword = location.pathname === '/search'
    ? new URLSearchParams(location.search).get('q') ?? ''
    : ''
  const [keyword, setKeyword] = useState(routeKeyword)
  const title = useMemo(() => resolveTitle(location.pathname), [location.pathname])

  useEffect(() => {
    setKeyword(routeKeyword)
  }, [location.pathname, routeKeyword])

  const submitSearch = (event: FormEvent) => {
    event.preventDefault()
    const value = keyword.trim()
    navigate(value ? `/search?q=${encodeURIComponent(value)}` : '/search')
  }

  return (
    <header className="nv-topbar pwa-safe-top" aria-label="页面工具栏" style={SAFE_INLINE_STYLE}>
      <h1 className="nv-topbar-title max-w-[20vw] sm:max-w-none">{title}</h1>
      <div className="nv-topbar-spacer" />
      <form onSubmit={submitSearch} role="search" className="flex min-w-0 flex-1 items-center justify-end sm:flex-initial">
        <SearchField
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          placeholder="搜索影片、剧集、演员"
          aria-label="全局搜索"
          wrapperClassName="max-w-full"
        />
      </form>
    </header>
  )
}

export default function Layout() {
  const location = useLocation()
  const mainRef = useRef<HTMLElement>(null)
  const isWidePage = WIDE_PAGE_PREFIXES.some((prefix) => location.pathname.startsWith(prefix))
  const usesLocalDetailChrome = location.pathname.startsWith('/media/')

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
    <div className="nv-app-shell relative flex h-full min-h-0 overflow-hidden">
      <Sidebar />
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
    </div>
  )
}
