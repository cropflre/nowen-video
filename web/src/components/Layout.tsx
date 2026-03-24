import { Outlet, useLocation } from 'react-router-dom'
import { useState, useEffect } from 'react'
import Sidebar from './Sidebar'
import PageTransition from './PageTransition'
import { Menu } from 'lucide-react'

export default function Layout() {
  const [mobileOpen, setMobileOpen] = useState(false)
  const location = useLocation()

  // 路由切换时自动关闭移动端侧边栏
  useEffect(() => {
    setMobileOpen(false)
  }, [location.pathname])

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
      <main className="relative z-10 flex-1 overflow-y-auto">
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

        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <PageTransition>
            <Outlet />
          </PageTransition>
        </div>
      </main>
    </div>
  )
}
