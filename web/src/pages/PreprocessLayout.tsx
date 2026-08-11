import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { Zap, Subtitles } from 'lucide-react'
import clsx from 'clsx'

/**
 * 预处理模块壳组件
 * - 顶部一级 Tab：视频预处理 / 字幕预处理
 * - 下挂 <Outlet />：分别渲染 PreprocessPage 与 SubtitlePreprocessPage
 *
 * 设计要点：
 * 1) 路由级 Tab，URL 与页面状态强一致，浏览器前进/后退、刷新、深链分享均工作正常
 * 2) 不嵌套大组件，避免 PreprocessPage 与 SubtitlePreprocessPage 同屏挂载
 * 3) 子页面保持独立，WS 订阅自然按 Tab 切换 mount/unmount
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
      <div className="sticky top-0 z-10 -mx-4 border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-elevated)] px-4 py-2 backdrop-blur-md sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8">
        <div className="flex flex-wrap items-center gap-1">
          {tabs.map((tab) => {
            const Icon = tab.icon
            return (
              <NavLink
                key={tab.to}
                to={tab.to}
                end={tab.end}
                className={clsx(
                  'inline-flex min-h-8 items-center gap-1.5 rounded-[var(--nv-radius-control)] border px-3 py-1.5 text-sm transition-[background-color,border-color,color] duration-200',
                  tab.active
                    ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] font-medium text-[var(--nv-action-primary)]'
                    : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] text-[var(--nv-text-secondary)] hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]',
                )}
              >
                <Icon
                  size={14}
                  className={tab.active ? 'text-[var(--nv-action-primary)]' : 'text-[var(--nv-text-tertiary)]'}
                  aria-hidden="true"
                />
                <span>{tab.label}</span>
              </NavLink>
            )
          })}
        </div>
      </div>

      <Outlet />
    </div>
  )
}
