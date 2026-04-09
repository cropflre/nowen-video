import { Outlet, useLocation } from 'react-router-dom'
import { useState, useEffect, useRef, useCallback } from 'react'
import { AnimatePresence } from 'framer-motion'
import Sidebar from './Sidebar'
import PageTransition from './PageTransition'
import { Menu } from 'lucide-react'

// 滚动位置保存 key 前缀
const SCROLL_KEY_PREFIX = 'nowen_scroll_'

export default function Layout() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const location = useLocation()
  const mainRef = useRef<HTMLElement>(null)
  const prevPathRef = useRef(location.pathname + location.search)

  // 路由切换时自动关闭移动端侧边栏
  useEffect(() => {
    setMobileOpen(false)
  }, [location.pathname])

  // 路由切换前保存当前滚动位置，切换后恢复目标页面的滚动位置
  useEffect(() => {
    const mainEl = mainRef.current
    if (!mainEl) return

    // 保存离开页面的滚动位置
    const prevKey = SCROLL_KEY_PREFIX + prevPathRef.current
    // 注意：此时 prevPathRef 还是上一个路径
    // 在 useEffect 执行时，DOM 已经更新，但我们在 cleanup 中保存
    // 所以改用 ref 记录

    // 恢复当前页面的滚动位置
    const currentKey = SCROLL_KEY_PREFIX + location.pathname + location.search
    const savedPos = sessionStorage.getItem(currentKey)
    if (savedPos) {
      // 延迟恢复，等待页面内容渲染完成
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
    <div className="relative flex h-screen overflow-hidden" style={{ backgroundColor: 'var(--bg-base)' }}>
      {/* 深空背景光效 */}
      <div className="pointer-events-none fixed inset-0 z-0 bg-deep-space" />
      <div className="pointer-events-none fixed inset-0 z-0 noise-bg" />

      {/* 移动端遮罩 */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/60 backdrop-blur-sm transition-opacity duration-300 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* 侧边导航 */}
      <Sidebar isMobileOpen={mobileOpen} onMobileClose={() => setMobileOpen(false)} />

      {/* 主内容区 */}
      <main ref={mainRef} id="main-scroll-container" className="relative z-10 flex-1 min-w-0 overflow-y-auto">
        {/* 移动端顶部栏 */}
        <div className="sticky top-0 z-20 flex items-center gap-3 px-4 py-3 md:hidden"
          style={{
            background: 'var(--bg-base)',
            borderBottom: '1px solid var(--border-default)',
          }}
        >
          <button
            onClick={() => setMobileOpen(true)}
            className="rounded-lg p-2 transition-colors hover:bg-[var(--nav-hover-bg)]"
            style={{ color: 'var(--text-secondary)' }}
          >
            <Menu size={22} />
          </button>
          <h1 className="font-display text-base font-bold tracking-wider">
            <span className="text-neon text-neon-glow">N</span>
            <span style={{ color: 'var(--text-primary)' }}>OWEN</span>
          </h1>
        </div>

        <div className={`px-4 py-6 sm:px-6 lg:px-8 transition-all duration-300 ${
          // 需要全宽展示的页面（文件管理、预处理等）不限制最大宽度
          ['/files', '/preprocess', '/subtitle-preprocess', '/admin'].some(p => location.pathname.startsWith(p))
            ? 'w-full'
            : 'mx-auto max-w-7xl'
        }`}>
          <AnimatePresence mode="wait">
            <PageTransition key={location.pathname}>
              <Outlet />
            </PageTransition>
          </AnimatePresence>
        </div>
      </main>
    </div>
  )
}
