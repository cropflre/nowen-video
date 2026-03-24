import { Outlet } from 'react-router-dom'
import Sidebar from './Sidebar'
import PageTransition from './PageTransition'

export default function Layout() {
  return (
    <div className="relative flex h-screen overflow-hidden" style={{ backgroundColor: 'var(--bg-base)' }}>
      {/* 深空背景光效 */}
      <div className="pointer-events-none fixed inset-0 z-0 bg-deep-space" />
      <div className="pointer-events-none fixed inset-0 z-0 noise-bg" />

      {/* 侧边导航 */}
      <Sidebar />

      {/* 主内容区 */}
      <main className="relative z-10 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
          <PageTransition>
            <Outlet />
          </PageTransition>
        </div>
      </main>
    </div>
  )
}
