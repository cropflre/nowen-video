import { ChevronRight, Home } from 'lucide-react'

interface BreadcrumbProps {
  folderPath: string
  onNavigate: (path: string) => void
  onGoHome: () => void
}

export default function Breadcrumb({ folderPath, onNavigate, onGoHome }: BreadcrumbProps) {
  if (!folderPath) return null

  const normalized = folderPath.replace(/\\/g, '/')
  const isAbsoluteUnix = normalized.startsWith('/') && !normalized.startsWith('//')
  const isUNC = normalized.startsWith('//')
  const isWindowsDrive = /^[A-Za-z]:\//.test(normalized)
  const parts = normalized.split('/').filter(Boolean)
  const items = parts.map((name, index) => {
    const joined = parts.slice(0, index + 1).join('/')
    let path = joined
    if (isUNC) path = `//${joined}`
    else if (isAbsoluteUnix) path = `/${joined}`
    else if (isWindowsDrive && index === 0) path = `${joined}/`

    return { name, path }
  })

  return (
    <nav aria-label="当前文件夹路径" className="max-w-full overflow-x-auto">
      <ol className="flex min-w-max items-center gap-0.5 text-[12px]">
        <li>
          <button
            type="button"
            onClick={onGoHome}
            className="flex h-7 items-center gap-1 rounded-[var(--nv-radius-control)] px-1.5 text-[var(--nv-text-tertiary)] outline-none transition-[background-color,color] duration-150 hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-secondary)] focus-visible:shadow-[var(--nv-shadow-focus)]"
          >
            <Home size={13} aria-hidden="true" />
            <span>全部</span>
          </button>
        </li>

        {items.map((item, index) => {
          const isCurrent = index === items.length - 1
          return (
            <li key={item.path} className="flex items-center gap-0.5">
              <ChevronRight size={12} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
              {isCurrent ? (
                <span
                  aria-current="page"
                  className="max-w-52 truncate px-1.5 py-1 font-medium text-[var(--nv-text-primary)] sm:max-w-72"
                  title={item.name}
                >
                  {item.name}
                </span>
              ) : (
                <button
                  type="button"
                  onClick={() => onNavigate(item.path)}
                  className="max-w-44 truncate rounded-[var(--nv-radius-control)] px-1.5 py-1 text-[var(--nv-text-tertiary)] outline-none transition-[background-color,color] duration-150 hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-secondary)] focus-visible:shadow-[var(--nv-shadow-focus)] sm:max-w-60"
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
