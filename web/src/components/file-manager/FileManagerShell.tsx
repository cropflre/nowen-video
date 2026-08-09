import type { ReactNode } from 'react'
import { FolderOpen, Globe, History, RefreshCw, ShieldAlert } from 'lucide-react'
import { Button, PageContainer } from '@/components/design-system'
import type { TabType } from './constants'

interface FileManagerShellProps {
  activeTab: TabType
  onTabChange: (tab: TabType) => void
  onOpenLogs: () => void
  onRefresh: () => void
  children: ReactNode
}

const tabs: Array<{
  value: TabType
  label: string
  icon: ReactNode
}> = [
  { value: 'files', label: '文件列表', icon: <FolderOpen size={16} aria-hidden="true" /> },
  { value: 'scrape', label: '刮削任务', icon: <Globe size={16} aria-hidden="true" /> },
  { value: 'adult', label: '成人刮削', icon: <ShieldAlert size={16} aria-hidden="true" /> },
]

export default function FileManagerShell({
  activeTab,
  onTabChange,
  onOpenLogs,
  onRefresh,
  children,
}: FileManagerShellProps) {
  return (
    <PageContainer width="wide" className="min-h-screen">
      <div className="space-y-6">
        <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
                <FolderOpen size={20} aria-hidden="true" />
              </div>
              <div className="min-w-0">
                <h1 className="text-xl font-semibold tracking-[-0.02em] text-[var(--nv-text-primary)] sm:text-2xl">
                  影视文件管理
                </h1>
                <p className="mt-1 text-sm leading-6 text-[var(--nv-text-tertiary)]">
                  管理媒体文件、刮削元数据、批量重命名与目录结构。
                </p>
              </div>
            </div>
          </div>

          {activeTab === 'files' && (
            <div className="flex shrink-0 flex-wrap items-center gap-2 sm:justify-end">
              <Button type="button" variant="secondary" size="sm" onClick={onOpenLogs}>
                <History size={15} aria-hidden="true" />
                操作日志
              </Button>
              <Button type="button" variant="ghost" size="sm" onClick={onRefresh}>
                <RefreshCw size={15} aria-hidden="true" />
                刷新
              </Button>
            </div>
          )}
        </header>

        <nav
          className="inline-flex max-w-full items-center gap-1 overflow-x-auto rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-1"
          aria-label="文件管理功能"
        >
          {tabs.map((tab) => {
            const selected = activeTab === tab.value
            return (
              <button
                key={tab.value}
                type="button"
                onClick={() => onTabChange(tab.value)}
                aria-current={selected ? 'page' : undefined}
                className="flex shrink-0 items-center gap-2 rounded-[var(--nv-radius-control)] px-3 py-2 text-sm font-medium outline-none transition-[background-color,color,box-shadow] duration-200 hover:bg-[var(--nv-bg-hover)] focus-visible:shadow-[var(--nv-shadow-focus)] sm:px-4"
                style={{
                  background: selected ? 'var(--nv-bg-active)' : 'transparent',
                  color: selected ? 'var(--nv-action-primary)' : 'var(--nv-text-secondary)',
                }}
              >
                {tab.icon}
                {tab.label}
              </button>
            )
          })}
        </nav>

        {children}
      </div>
    </PageContainer>
  )
}
