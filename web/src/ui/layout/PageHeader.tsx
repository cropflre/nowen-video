import { useEffect, useRef, type CSSProperties, type FormEvent, type ReactNode } from 'react'
import clsx from 'clsx'
import { SearchField } from '@/components/design-system'

export interface PageHeaderProps {
  title: ReactNode
  subtitle?: ReactNode
  className?: string
  style?: CSSProperties
  searchValue?: string
  searchPlaceholder?: string
  searchAriaLabel?: string
  showSearch?: boolean
  showSearchShortcut?: boolean
  actions?: ReactNode
  onSearchValueChange?: (value: string) => void
  onSearchSubmit?: (value: string) => void
}

export function PageHeader({
  title,
  subtitle,
  className,
  style,
  searchValue = '',
  searchPlaceholder = '搜索影片、剧集、演员、导演...',
  searchAriaLabel = '全局搜索',
  showSearch = true,
  showSearchShortcut = true,
  actions,
  onSearchValueChange,
  onSearchSubmit,
}: PageHeaderProps) {
  const searchRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!showSearch || !showSearchShortcut) return
    const handleShortcut = (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== 'k') return
      event.preventDefault()
      searchRef.current?.focus()
      searchRef.current?.select()
    }
    window.addEventListener('keydown', handleShortcut)
    return () => window.removeEventListener('keydown', handleShortcut)
  }, [showSearch, showSearchShortcut])

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    onSearchSubmit?.(searchValue.trim())
  }

  return (
    <header
      className={clsx('nv-page-header nv-topbar pwa-safe-top', className)}
      aria-label="页面工具栏"
      style={style}
    >
      <div className="nv-page-header-title-slot">
        <h1 className="nv-topbar-title">{title}</h1>
        {subtitle && <div className="nv-topbar-subtitle">{subtitle}</div>}
      </div>

      <div className="nv-page-header-search-slot">
        {showSearch && (
          <form
            onSubmit={handleSubmit}
            role="search"
            className="nv-page-header-search"
          >
            <SearchField
              ref={searchRef}
              value={searchValue}
              onChange={(event) => onSearchValueChange?.(event.target.value)}
              placeholder={searchPlaceholder}
              aria-label={searchAriaLabel}
              wrapperClassName="max-w-full"
            />
            {showSearchShortcut && <kbd className="nv-page-header-search-shortcut" aria-hidden="true">⌘ K</kbd>}
          </form>
        )}
      </div>

      <div className="nv-page-header-actions-slot">
        {actions && <div className="nv-page-header-actions">{actions}</div>}
      </div>
    </header>
  )
}

export default PageHeader
