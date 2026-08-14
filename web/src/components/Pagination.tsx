import type { KeyboardEvent } from 'react'
import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react'
import { Button, Input, Select } from '@/components/design-system'

interface PaginationProps {
  /** 当前页码（从1开始） */
  page: number
  /** 总页数 */
  totalPages: number
  /** 总记录数 */
  total?: number
  /** 每页数量 */
  pageSize?: number
  /** 每页数量可选项 */
  pageSizeOptions?: number[]
  /** 每页数量变化回调 */
  onPageSizeChange?: (size: number) => void
  /** 页码变化回调 */
  onPageChange: (page: number) => void
  /** 是否显示总数信息 */
  showTotal?: boolean
  /** 是否显示快速跳转 */
  showJumper?: boolean
  /** 最多显示的页码按钮数量 */
  maxButtons?: number
}

export default function Pagination({
  page,
  totalPages,
  total,
  pageSize,
  pageSizeOptions,
  onPageSizeChange,
  onPageChange,
  showTotal = true,
  showJumper = true,
  maxButtons = 7,
}: PaginationProps) {
  if (totalPages <= 1) return null

  const getPageNumbers = (): (number | 'ellipsis')[] => {
    if (totalPages <= maxButtons) {
      return Array.from({ length: totalPages }, (_, index) => index + 1)
    }

    const pages: (number | 'ellipsis')[] = []
    const half = Math.floor((maxButtons - 2) / 2)
    let start = Math.max(2, page - half)
    let end = Math.min(totalPages - 1, page + half)

    if (page - half < 2) end = Math.min(totalPages - 1, maxButtons - 1)
    if (page + half > totalPages - 1) start = Math.max(2, totalPages - maxButtons + 2)

    pages.push(1)
    if (start > 2) pages.push('ellipsis')
    for (let current = start; current <= end; current++) pages.push(current)
    if (end < totalPages - 1) pages.push('ellipsis')
    if (totalPages > 1) pages.push(totalPages)
    return pages
  }

  const handleJump = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter') return
    const value = parseInt(event.currentTarget.value, 10)
    if (!Number.isNaN(value) && value >= 1 && value <= totalPages) {
      onPageChange(value)
      event.currentTarget.value = ''
    }
  }

  return (
    <nav className="nv-pagination flex flex-wrap items-center justify-center gap-x-2 gap-y-3 py-6" aria-label="分页导航">
      {showTotal && total !== undefined && (
        <span className="nv-pagination-total mr-1 shrink-0 whitespace-nowrap text-xs text-[var(--nv-text-tertiary)]">
          共 <strong className="font-semibold text-[var(--nv-text-secondary)]">{total}</strong> 项
        </span>
      )}

      {pageSizeOptions && pageSizeOptions.length > 0 && onPageSizeChange && (
        <label className="nv-pagination-size mr-2 flex shrink-0 items-center gap-1.5 whitespace-nowrap text-xs text-[var(--nv-text-tertiary)]">
          <span className="shrink-0 whitespace-nowrap">每页</span>
          <Select
            value={pageSize}
            onChange={(event) => onPageSizeChange(Number(event.target.value))}
            className="h-8 w-[4.75rem] shrink-0 py-0"
            aria-label="每页数量"
          >
            {pageSizeOptions.map((size) => (
              <option key={size} value={size}>{size}</option>
            ))}
          </Select>
          <span className="shrink-0 whitespace-nowrap">条</span>
        </label>
      )}

      <Button variant="secondary" size="sm" iconOnly onClick={() => onPageChange(1)} disabled={page === 1} title="首页" aria-label="首页" className="shrink-0">
        <ChevronsLeft size={14} aria-hidden="true" />
      </Button>
      <Button variant="secondary" size="sm" iconOnly onClick={() => onPageChange(Math.max(1, page - 1))} disabled={page === 1} title="上一页" aria-label="上一页" className="shrink-0">
        <ChevronLeft size={14} aria-hidden="true" />
      </Button>

      {getPageNumbers().map((number, index) => number === 'ellipsis' ? (
        <span key={`ellipsis-${index}`} className="nv-pagination-ellipsis flex h-8 w-8 shrink-0 items-center justify-center text-xs text-[var(--nv-text-tertiary)]" aria-hidden="true">
          ···
        </span>
      ) : (
        <Button
          key={number}
          variant={page === number ? 'primary' : 'secondary'}
          size="sm"
          onClick={() => onPageChange(number)}
          className="nv-pagination-page min-w-8 shrink-0 px-2"
          aria-label={`第 ${number} 页`}
          aria-current={page === number ? 'page' : undefined}
        >
          {number}
        </Button>
      ))}

      <Button variant="secondary" size="sm" iconOnly onClick={() => onPageChange(Math.min(totalPages, page + 1))} disabled={page === totalPages} title="下一页" aria-label="下一页" className="shrink-0">
        <ChevronRight size={14} aria-hidden="true" />
      </Button>
      <Button variant="secondary" size="sm" iconOnly onClick={() => onPageChange(totalPages)} disabled={page === totalPages} title="末页" aria-label="末页" className="shrink-0">
        <ChevronsRight size={14} aria-hidden="true" />
      </Button>

      {showJumper && totalPages > 5 && (
        <label className="nv-pagination-jumper ml-2 flex shrink-0 items-center gap-1.5 whitespace-nowrap text-xs text-[var(--nv-text-tertiary)]">
          <span className="shrink-0 whitespace-nowrap">跳至</span>
          <Input
            type="number"
            min={1}
            max={totalPages}
            onKeyDown={handleJump}
            className="h-8 w-16 shrink-0 px-2 text-center"
            placeholder={`${page}`}
            aria-label="跳转页码"
          />
          <span className="shrink-0 whitespace-nowrap">页</span>
        </label>
      )}
    </nav>
  )
}
