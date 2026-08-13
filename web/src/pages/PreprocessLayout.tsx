import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { Subtitles, Zap } from 'lucide-react'
import clsx from 'clsx'

/**
 * 预处理模块壳组件
 * - 顶部一级 Tab：视频预处理 / 字幕预处理
 * - 下挂 <Outlet />：分别渲染 PreprocessPage 与 SubtitlePreprocessPage
 *
 * 路由、前进后退和子页面 mount/unmount 行为保持不变；这里只负责统一 Navi 风格导航。
 */
export default function PreprocessLayout() {
  const location = useLocation()
  const isSubtitle = location.pathname.startsWith('/preprocess/subtitle')
  const isVideo = !isSubtitle

  const tabs = [
    { to: '/preprocess', label: '视频预处理', icon: Zap, active: isVideo, end: true },
    { to: '/preprocess/subtitle', label: '字幕预处理', icon: Subtitles, active: isSubtitle, end: false },
  ] as const

  return (
    <div className="space-y-3">
      <div className="sticky top-0 z-10 -mx-4 border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)] px-4 sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8">
        <nav className="flex min-w-0 items-center gap-1 overflow-x-auto" aria-label="预处理功能">
          {tabs.map((tab) => {
            const Icon = tab.icon
            return (
              <NavLink
                key={tab.to}
                to={tab.to}
                end={tab.end}
                aria-current={tab.active ? 'page' : undefined}
                className={clsx(
                  'relative flex h-10 shrink-0 items-center gap-1.5 px-2.5 text-[13px] font-medium outline-none transition-[background-color,color] duration-150 focus-visible:shadow-[var(--nv-shadow-focus)]',
                  tab.active
                    ? 'text-[var(--nv-text-primary)] after:absolute after:inset-x-2 after:bottom-[-1px] after:h-px after:bg-[var(--nv-text-primary)]'
                    : 'text-[var(--nv-text-tertiary)] hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-secondary)]',
                )}
              >
                <Icon size={14} aria-hidden="true" />
                <span>{tab.label}</span>
              </NavLink>
            )
          })}
        </nav>
      </div>

      <Outlet />
    </div>
  )
}
