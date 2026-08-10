import type { ComponentType } from 'react'
import type { FileManagerStats } from '@/types'
import {
  AlertCircle,
  AlertTriangle,
  Check,
  Download,
  FileText,
  FileVideo,
  Film,
  HardDrive,
  Tv,
  XCircle,
} from 'lucide-react'
import { Surface, type TagTone } from '@/components/design-system'
import { formatFileSize } from './constants'

interface FileStatsBarProps {
  stats: FileManagerStats
}

interface StatItem {
  label: string
  value: string | number
  icon: ComponentType<{ size?: number; className?: string; 'aria-hidden'?: boolean }>
  tone?: TagTone
}

function toneClassName(tone: TagTone = 'neutral') {
  switch (tone) {
    case 'brand':
      return 'text-[var(--nv-action-primary)]'
    case 'success':
      return 'text-[var(--nv-status-success)]'
    case 'warning':
    case 'rating':
      return 'text-[var(--nv-status-warning)]'
    case 'danger':
      return 'text-[var(--nv-status-danger)]'
    default:
      return 'text-[var(--nv-text-tertiary)]'
  }
}

export default function FileStatsBar({ stats }: FileStatsBarProps) {
  const items: StatItem[] = [
    { label: '总文件', value: stats.total_files, icon: FileVideo, tone: 'brand' },
    { label: '电影', value: stats.movie_count, icon: Film },
    { label: '剧集', value: stats.episode_count, icon: Tv },
    { label: '已刮削', value: stats.scraped_count, icon: Check, tone: 'success' },
    { label: '未刮削', value: stats.unscraped_count, icon: AlertCircle, tone: 'warning' },
    { label: '总大小', value: formatFileSize(stats.total_size_bytes), icon: HardDrive },
    { label: '近 7 天导入', value: stats.recent_imports, icon: Download, tone: 'brand' },
    { label: '操作记录', value: stats.recent_operations, icon: FileText },
  ]

  if ((stats.partial_count ?? 0) > 0) {
    items.push({
      label: '部分刮削',
      value: stats.partial_count!,
      icon: AlertTriangle,
      tone: 'warning',
    })
  }

  if ((stats.failed_count ?? 0) > 0) {
    items.push({
      label: '刮削失败',
      value: stats.failed_count!,
      icon: XCircle,
      tone: 'danger',
    })
  }

  return (
    <Surface
      as="section"
      aria-label="文件统计"
      className="grid overflow-hidden p-1 sm:grid-cols-2 lg:grid-cols-4 2xl:grid-cols-8"
    >
      {items.map((item) => {
        const Icon = item.icon
        return (
          <div
            key={item.label}
            className="flex min-w-0 items-center gap-3 rounded-[var(--nv-radius-control)] px-3 py-2.5 transition-colors hover:bg-[var(--nv-bg-hover)]"
          >
            <div
              className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] ${toneClassName(item.tone)}`}
            >
              <Icon size={15} aria-hidden="true" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="truncate text-sm font-semibold leading-5 text-[var(--nv-text-primary)]">
                {item.value}
              </div>
              <div className="truncate text-[11px] leading-4 text-[var(--nv-text-tertiary)]">
                {item.label}
              </div>
            </div>
          </div>
        )
      })}
    </Surface>
  )
}
