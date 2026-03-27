import type { Media, Library, FileManagerStats } from '@/types'

// 数据源选项
export const SOURCE_OPTIONS = [
  { value: '', label: '自动 (TMDb)' },
  { value: 'tmdb', label: 'TMDb' },
  { value: 'bangumi', label: 'Bangumi' },
  { value: 'ai', label: 'AI增强' },
]

// 语言选项
export const LANGUAGE_OPTIONS = [
  { value: '', label: '不翻译（保持原语言）', flag: '🌐' },
  { value: 'zh', label: '中文', flag: '🇨🇳' },
  { value: 'en', label: 'English', flag: '🇺🇸' },
  { value: 'ja', label: '日本語', flag: '🇯🇵' },
  { value: 'ko', label: '한국어', flag: '🇰🇷' },
  { value: 'fr', label: 'Français', flag: '🇫🇷' },
  { value: 'de', label: 'Deutsch', flag: '🇩🇪' },
  { value: 'es', label: 'Español', flag: '🇪🇸' },
  { value: 'pt', label: 'Português', flag: '🇧🇷' },
  { value: 'ru', label: 'Русский', flag: '🇷🇺' },
  { value: 'it', label: 'Italiano', flag: '🇮🇹' },
  { value: 'th', label: 'ไทย', flag: '🇹🇭' },
  { value: 'vi', label: 'Tiếng Việt', flag: '🇻🇳' },
]

// 排序选项
export const SORT_OPTIONS = [
  { value: 'created_at', label: '导入时间' },
  { value: 'title', label: '标题' },
  { value: 'year', label: '年份' },
  { value: 'rating', label: '评分' },
  { value: 'file_size', label: '文件大小' },
  { value: 'updated_at', label: '更新时间' },
]

// 格式化文件大小
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// Tab 类型
export type TabType = 'files' | 'scrape'

// 对话框类型
export type DialogType = 'none' | 'import' | 'batchImport' | 'scanDir' | 'edit' | 'detail' | 'rename' | 'logs'

// 文件管理器共享状态接口
export interface FileManagerState {
  // 数据
  files: Media[]
  total: number
  page: number
  loading: boolean
  stats: FileManagerStats | null
  libraries: Library[]
  // 筛选
  keyword: string
  filterLibrary: string
  filterMediaType: string
  filterScraped: string
  sortBy: string
  sortOrder: string
  showFilters: boolean
  // 选择
  selectedIds: Set<string>
  // 视图
  viewMode: 'table' | 'grid'
  // 刮削源
  scrapeSource: string
}
