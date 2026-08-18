import type { CSSProperties, FormEvent, ReactNode } from 'react'
import clsx from 'clsx'
import { SearchField } from '@/components/design-system'

export interface PageHeaderProps {
  title: ReactNode
  className?: string
  style?: CSSProperties
  searchValue?: string
  searchPlaceholder?: string
  searchAriaLabel?: string
  showSearch?: boolean
  actions?: ReactNode
  onSearchValueChange?: (value: string) => void
  onSearchSubmit?: (value: string) => void
}

export function PageHeader({
  title,
  className,
  style,
  searchValue = '',
  searchPlaceholder = '搜索影片、剧集、演员',
  searchAriaLabel = '全局搜索',
  showSearch = true,
  actions,
  onSearchValueChange,
  onSearchSubmit,
}: PageHeaderProps) {
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
      <h1 className="nv-topbar-title max-w-[20vw] sm:max-w-none">{title}</h1>
      <div className="nv-topbar-spacer" />

      {showSearch && (
        <form
          onSubmit={handleSubmit}
          role="search"
          className="nv-page-header-search flex min-w-0 flex-1 items-center justify-end sm:flex-initial"
        >
          <SearchField
            value={searchValue}
            onChange={(event) => onSearchValueChange?.(event.target.value)}
            placeholder={searchPlaceholder}
            aria-label={searchAriaLabel}
            wrapperClassName="max-w-full"
          />
        </form>
      )}

      {actions && <div className="nv-page-header-actions">{actions}</div>}
    </header>
  )
}

export default PageHeader
