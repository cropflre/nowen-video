import { FileManagerStats } from '@/types'
import {
  FileVideo,
  Film,
  Tv,
  Check,
  AlertCircle,
  HardDrive,
  Download,
  FileText,
} from 'lucide-react'
import clsx from 'clsx'
import { formatFileSize } from './constants'

interface FileStatsBarProps {
  stats: FileManagerStats
}

// 统计卡片组件
export default function FileStatsBar({ stats }: FileStatsBarProps) {
  const items = [
    { label: '总文件', value: stats.total_files, icon: FileVideo, color: 'text-blue-400' },
    { label: '电影', value: stats.movie_count, icon: Film, color: 'text-purple-400' },
    { label: '剧集', value: stats.episode_count, icon: Tv, color: 'text-green-400' },
    { label: '已刮削', value: stats.scraped_count, icon: Check, color: 'text-emerald-400' },
    { label: '未刮削', value: stats.unscraped_count, icon: AlertCircle, color: 'text-amber-400' },
    { label: '总大小', value: formatFileSize(stats.total_size_bytes), icon: HardDrive, color: 'text-cyan-400' },
    { label: '近7天导入', value: stats.recent_imports, icon: Download, color: 'text-indigo-400' },
    { label: '操作记录', value: stats.recent_operations, icon: FileText, color: 'text-pink-400' },
  ]

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-8 gap-3">
      {items.map((item, i) => (
        <div key={i} className="glass-panel rounded-xl p-3 text-center">
          <item.icon size={18} className={clsx('mx-auto mb-1', item.color)} />
          <div className="text-lg font-bold" style={{ color: 'var(--text-primary)' }}>{item.value}</div>
          <div className="text-xs" style={{ color: 'var(--text-tertiary)' }}>{item.label}</div>
        </div>
      ))}
    </div>
  )
}
