import type { KeyboardEvent } from 'react'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button, Input, Select } from '@/components/design-system'

interface PaginationProps {
  page: number
  totalPages: number
  total?: number
  pageSize?: number
  pageSizeOptions?: number[]
  onPageSizeChange?: (size: number) => void
  onPageChange: (page: number) => void
  showTotal?: boolean
  showJumper?: boolean
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

  const safePageSize = Math.max(pageSize || 0, 1)
  const firstItem = total && total > 0 ? (page - 1) * safePageSize + 1 : 0
  const lastItem = total && total > 0 ? Math.min(page * safePageSize, total) : 0

  return (
    <nav className="nv-pagination" aria-label="分页导航">
      {showTotal && total !== undefined && (
        <div className="nv-pagination-summary" aria-live="polite">
          {firstItem > 0 ? `第 ${firstItem}-${lastItem} 项 / 共 ${total}` : `共 ${total} 项`}
        </div>
      )}

      <div className="nv-pagination-pages" role="group" aria-label="页码">
        <Button
          variant="secondary"
          size="sm"
          iconOnly
          onClick={() => onPageChange(Math.max(1, page - 1))}
          disabled={page === 1}
          title="上一页"
          aria-label="上一页"
          className="nv-pagination-arrow"
        >
          <ChevronLeft size={14} aria-hidden="true" />
        </Button>

        {getPageNumbers().map((number, index) => number === 'ellipsis' ? (
          <span key={`ellipsis-${index}`} className="nv-pagination-ellipsis" aria-hidden="true">…</span>
        ) : (
          <Button
            key={number}
            variant={page === number ? 'primary' : 'ghost'}
            size="sm"
            onClick={() => onPageChange(number)}
            className="nv-pagination-page"
            aria-label={`第 ${number} 页`}
            aria-current={page === number ? 'page' : undefined}
          >
            {number}
          </Button>
        ))}

        <Button
          variant="secondary"
          size="sm"
          iconOnly
          onClick={() => onPageChange(Math.min(totalPages, page + 1))}
          disabled={page === totalPages}
          title="下一页"
          aria-label="下一页"
          className="nv-pagination-arrow"
        >
          <ChevronRight size={14} aria-hidden="true" />
        </Button>
      </div>

      <div className="nv-pagination-options">
        {pageSizeOptions && pageSizeOptions.length > 0 && onPageSizeChange && (
          <label className="nv-pagination-size">
            <span>每页</span>
            <Select
              value={pageSize}
              onChange={(event) => onPageSizeChange(Number(event.target.value))}
              style={{ width: '3.5rem', flex: '0 0 3.5rem' }}
              aria-label="每页数量"
            >
              {pageSizeOptions.map((size) => <option key={size} value={size}>{size}</option>)}
            </Select>
          </label>
        )}

        {showJumper && totalPages > 5 && (
          <label className="nv-pagination-jumper">
            <span>跳至</span>
            <Input
              type="number"
              min={1}
              max={totalPages}
              onKeyDown={handleJump}
              style={{ width: '2.75rem', flex: '0 0 2.75rem' }}
              placeholder={`${page}`}
              aria-label="跳转页码"
            />
            <span>/ {totalPages}</span>
          </label>
        )}
      </div>
    </nav>
  )
}
