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
  type LucideIcon,
} from 'lucide-react'
import type { TagTone } from '@/components/design-system'
import { formatFileSize } from './constants'

interface FileStatsBarProps {
  stats: FileManagerStats
}

interface StatItem {
  label: string
  value: string | number
  icon: LucideIcon
  tone?: TagTone
}

function toneClassName(tone: TagTone = 'neutral') {
  switch (tone) {
    case 'brand':
      return 'text-[var(--nv-text-secondary)]'
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
    { label: '近 7 天导入', value: stats.recent_imports, icon: Download },
    { label: '操作记录', value: stats.recent_operations, icon: FileText },
  ]

  if ((stats.partial_count ?? 0) > 0) {
    items.push({ label: '部分刮削', value: stats.partial_count!, icon: AlertTriangle, tone: 'warning' })
  }

  if ((stats.failed_count ?? 0) > 0) {
    items.push({ label: '刮削失败', value: stats.failed_count!, icon: XCircle, tone: 'danger' })
  }

  return (
    <section
      aria-label="文件统计"
      className="scrollbar-hide flex max-w-full overflow-x-auto border-y border-[var(--nv-border-subtle)] py-2"
    >
      {items.map((item) => {
        const Icon = item.icon
        return (
          <div
            key={item.label}
            className="flex min-w-[112px] shrink-0 items-center gap-2 border-r border-[var(--nv-border-subtle)] px-3 last:border-r-0 first:pl-1"
          >
            <Icon size={14} className={toneClassName(item.tone)} aria-hidden="true" />
            <div className="min-w-0">
              <div className="truncate text-[13px] font-semibold leading-5 text-[var(--nv-text-primary)]">{item.value}</div>
              <div className="truncate text-[10px] leading-4 text-[var(--nv-text-tertiary)]">{item.label}</div>
            </div>
          </div>
        )
      })}
    </section>
  )
}
