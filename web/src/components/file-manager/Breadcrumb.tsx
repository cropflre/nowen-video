import { ChevronRight, Home } from 'lucide-react'

interface BreadcrumbProps {
  folderPath: string
  onNavigate: (path: string) => void
  onGoHome: () => void
}

export default function Breadcrumb({ folderPath, onNavigate, onGoHome }: BreadcrumbProps) {
  if (!folderPath) return null

  const normalized = folderPath.replace(/\\/g, '/')
  const parts = normalized.split('/').filter(Boolean)
  const items = parts.map((name, index) => {
    const path = parts.slice(0, index + 1).join('/')
    return {
      name,
      path: normalized.startsWith('/') ? `/${path}` : path,
    }
  })

  return (
    <nav
      aria-label="当前文件夹路径"
      className="max-w-full overflow-x-auto rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-1.5 py-1"
    >
      <ol className="flex min-w-max items-center gap-0.5 text-sm">
        <li>
          <button
            type="button"
            onClick={onGoHome}
            className="flex items-center gap-1.5 rounded-[var(--nv-radius-control)] px-2 py-1.5 text-[var(--nv-text-secondary)] outline-none transition-[background-color,color,box-shadow] duration-200 hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:shadow-[var(--nv-shadow-focus)]"
          >
            <Home size={14} aria-hidden="true" />
            <span>全部</span>
          </button>
        </li>

        {items.map((item, index) => {
          const isCurrent = index === items.length - 1
          return (
            <li key={item.path} className="flex items-center gap-0.5">
              <ChevronRight size={14} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
              {isCurrent ? (
                <span
                  aria-current="page"
                  className="max-w-52 truncate rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-active)] px-2 py-1.5 font-medium text-[var(--nv-action-primary)] sm:max-w-72"
                  title={item.name}
                >
                  {item.name}
                </span>
              ) : (
                <button
                  type="button"
                  onClick={() => onNavigate(item.path)}
                  className="max-w-44 truncate rounded-[var(--nv-radius-control)] px-2 py-1.5 text-[var(--nv-text-secondary)] outline-none transition-[background-color,color,box-shadow] duration-200 hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:shadow-[var(--nv-shadow-focus)] sm:max-w-60"
                  title={item.name}
                >
                  {item.name}
                </button>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
