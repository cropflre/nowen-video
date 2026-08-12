import { Outlet, useLocation } from 'react-router-dom'
import { useState, useEffect, useRef } from 'react'
import { AnimatePresence } from 'framer-motion'
import Sidebar from './Sidebar'
import PageTransition from './PageTransition'
import { Button, PageContainer } from './design-system'
import { Menu } from 'lucide-react'

// 滚动位置保存 key 前缀
const SCROLL_KEY_PREFIX = 'nowen_scroll_'

const WIDE_PAGE_PREFIXES = ['/files', '/preprocess', '/admin', '/collections']

export default function Layout() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const location = useLocation()
  const mainRef = useRef<HTMLElement>(null)
  const prevPathRef = useRef(location.pathname + location.search)
  const isWidePage = WIDE_PAGE_PREFIXES.some((prefix) => location.pathname.startsWith(prefix))

  // 路由切换时自动关闭移动端侧边栏
  useEffect(() => {
    setMobileOpen(false)
  }, [location.pathname])

  // 路由切换前保存当前滚动位置，切换后恢复目标页面的滚动位置
  useEffect(() => {
    const mainEl = mainRef.current
    if (!mainEl) return

    const currentKey = SCROLL_KEY_PREFIX + location.pathname + location.search
    const savedPos = sessionStorage.getItem(currentKey)
    if (savedPos) {
      requestAnimationFrame(() => {
        mainEl.scrollTop = parseInt(savedPos, 10)
      })
    } else {
      mainEl.scrollTop = 0
    }

    prevPathRef.current = location.pathname + location.search
  }, [location.pathname, location.search])

  // 持续保存滚动位置（节流）
  useEffect(() => {
    const mainEl = mainRef.current
    if (!mainEl) return

    let ticking = false
    const handleScroll = () => {
      if (!ticking) {
        ticking = true
        requestAnimationFrame(() => {
          const key = SCROLL_KEY_PREFIX + location.pathname + location.search
          sessionStorage.setItem(key, String(mainEl.scrollTop))
          ticking = false
        })
      }
    }

    mainEl.addEventListener('scroll', handleScroll, { passive: true })
    return () => mainEl.removeEventListener('scroll', handleScroll)
  }, [location.pathname, location.search])

  return (
    <div className="nv-app-shell relative flex h-full flex-col overflow-hidden">
      <div className="relative flex min-h-0 flex-1 overflow-hidden">
        <Sidebar isMobileOpen={mobileOpen} onMobileClose={() => setMobileOpen(false)} />

        <main
          ref={mainRef}
          id="main-scroll-container"
          className="nv-main-scroll relative z-10 min-w-0 flex-1 overflow-y-auto"
        >
          <div
            className="sticky top-0 z-20 flex items-center gap-3 border-b px-4 py-3 md:hidden"
            style={{
              background: 'color-mix(in srgb, var(--nv-bg-canvas) 92%, transparent)',
              borderColor: 'var(--nv-border-subtle)',
              backdropFilter: 'blur(16px)',
            }}
          >
            <Button
              variant="ghost"
              size="sm"
              iconOnly
              onClick={() => setMobileOpen(true)}
              aria-label="打开导航菜单"
            >
              <Menu size={20} />
            </Button>
            <h1 className="font-display text-base font-bold tracking-[0.12em] text-[var(--nv-text-primary)]">
              <span className="text-[var(--nv-action-primary)]">N</span>OWEN
            </h1>
          </div>

          <PageContainer width={isWidePage ? 'wide' : 'content'}>
            <AnimatePresence mode="wait">
              <PageTransition key={location.pathname}>
                <Outlet />
              </PageTransition>
            </AnimatePresence>
          </PageContainer>
        </main>
      </div>
    </div>
  )
}
