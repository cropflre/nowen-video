import { ChevronRight, Home } from 'lucide-react'

interface BreadcrumbProps {
  /** 当前选中的文件夹路径 */
  folderPath: string
  /** 点击面包屑项时的回调 */
  onNavigate: (path: string) => void
  /** 点击"全部"回到根目录 */
  onGoHome: () => void
}

export default function Breadcrumb({ folderPath, onNavigate, onGoHome }: BreadcrumbProps) {
  if (!folderPath) return null

  // 标准化路径分隔符
  const normalized = folderPath.replace(/\\/g, '/')
  const parts = normalized.split('/').filter(Boolean)

  // 构建面包屑项
  const items: { name: string; path: string }[] = []
  for (let i = 0; i < parts.length; i++) {
    const path = parts.slice(0, i + 1).join('/')
    // 如果原始路径以 / 开头（Linux），需要加回去
    const fullPath = normalized.startsWith('/') ? '/' + path : path
    items.push({ name: parts[i], path: fullPath })
  }

  return (
    <div className="flex items-center gap-1 text-sm flex-wrap">
      {/* 根目录 */}
      <button
        onClick={onGoHome}
        className="flex items-center gap-1 px-2 py-1 rounded-md transition-colors hover:bg-white/[0.06]"
        style={{ color: 'var(--text-secondary)' }}
      >
        <Home size={14} />
        <span>全部</span>
      </button>

      {/* 路径项 */}
      {items.map((item, idx) => (
        <div key={item.path} className="flex items-center gap-1">
          <ChevronRight size={14} className="text-surface-500 flex-shrink-0" />
          {idx === items.length - 1 ? (
            // 最后一项（当前目录）不可点击
            <span
              className="px-2 py-1 rounded-md font-medium"
              style={{ color: 'var(--text-primary)' }}
            >
              {item.name}
            </span>
          ) : (
            <button
              onClick={() => onNavigate(item.path)}
              className="px-2 py-1 rounded-md transition-colors hover:bg-white/[0.06]"
              style={{ color: 'var(--text-secondary)' }}
            >
              {item.name}
            </button>
          )}
        </div>
      ))}
    </div>
  )
}
