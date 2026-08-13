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
  { value: 'files', label: '文件列表', icon: <FolderOpen size={15} aria-hidden="true" /> },
  { value: 'scrape', label: '刮削任务', icon: <Globe size={15} aria-hidden="true" /> },
  { value: 'adult', label: '\u6210\u4eba\u522e\u524a', icon: <ShieldAlert size={15} aria-hidden="true" /> },
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
      <div className="space-y-4">
        <div className="flex min-w-0 items-end gap-3 border-b border-[var(--nv-border-subtle)]">
          <nav
            className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto"
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
                  className={`relative flex h-10 shrink-0 items-center gap-1.5 px-2.5 text-[13px] font-medium outline-none transition-[color,background-color] duration-150 focus-visible:shadow-[var(--nv-shadow-focus)] ${
                    selected
                      ? 'text-[var(--nv-text-primary)] after:absolute after:inset-x-2 after:bottom-[-1px] after:h-px after:bg-[var(--nv-text-primary)]'
                      : 'text-[var(--nv-text-tertiary)] hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-secondary)]'
                  }`}
                >
                  {tab.icon}
                  {tab.label}
                </button>
              )
            })}
          </nav>

          {activeTab === 'files' && (
            <div className="mb-1 flex shrink-0 items-center gap-1">
              <Button type="button" variant="ghost" size="sm" onClick={onOpenLogs} title="操作日志">
                <History size={14} aria-hidden="true" />
                <span className="hidden sm:inline">日志</span>
              </Button>
              <Button type="button" variant="ghost" size="sm" iconOnly onClick={onRefresh} aria-label="刷新" title="刷新">
                <RefreshCw size={14} aria-hidden="true" />
              </Button>
            </div>
          )}
        </div>

        {children}
      </div>
    </PageContainer>
  )
}
