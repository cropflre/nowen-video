import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from 'lucide-react'
import clsx from 'clsx'

interface PaginationProps {
  /** 当前页码（从1开始） */
  page: number
  /** 总页数 */
  totalPages: number
  /** 总记录数 */
  total?: number
  /** 每页数量 */
  pageSize?: number
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
  onPageChange,
  showTotal = true,
  showJumper = true,
  maxButtons = 7,
}: PaginationProps) {
  if (totalPages <= 1) return null

  // 计算要显示的页码列表
  const getPageNumbers = (): (number | 'ellipsis')[] => {
    if (totalPages <= maxButtons) {
      return Array.from({ length: totalPages }, (_, i) => i + 1)
    }

    const pages: (number | 'ellipsis')[] = []
    const half = Math.floor((maxButtons - 2) / 2) // 减去首尾两页

    let start = Math.max(2, page - half)
    let end = Math.min(totalPages - 1, page + half)

    // 调整范围确保显示足够的页码
    if (page - half < 2) {
      end = Math.min(totalPages - 1, maxButtons - 1)
    }
    if (page + half > totalPages - 1) {
      start = Math.max(2, totalPages - maxButtons + 2)
    }

    // 第一页
    pages.push(1)

    // 左侧省略号
    if (start > 2) {
      pages.push('ellipsis')
    }

    // 中间页码
    for (let i = start; i <= end; i++) {
      pages.push(i)
    }

    // 右侧省略号
    if (end < totalPages - 1) {
      pages.push('ellipsis')
    }

    // 最后一页
    if (totalPages > 1) {
      pages.push(totalPages)
    }

    return pages
  }

  const handleJump = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      const val = parseInt((e.target as HTMLInputElement).value)
      if (!isNaN(val) && val >= 1 && val <= totalPages) {
        onPageChange(val)
        ;(e.target as HTMLInputElement).value = ''
      }
    }
  }

  const pageNumbers = getPageNumbers()

  return (
    <div className="flex flex-wrap items-center justify-center gap-2 py-6">
      {/* 总数信息 */}
      {showTotal && total !== undefined && (
        <span className="mr-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          共 <strong style={{ color: 'var(--text-secondary)' }}>{total}</strong> 项
          {pageSize && (
            <> · 每页 {pageSize} 项</>
          )}
        </span>
      )}

      {/* 首页 */}
      <button
        onClick={() => onPageChange(1)}
        disabled={page === 1}
        className="flex h-8 w-8 items-center justify-center rounded-lg transition-all disabled:opacity-30 disabled:cursor-not-allowed"
        style={{
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
        }}
        title="首页"
      >
        <ChevronsLeft size={14} />
      </button>

      {/* 上一页 */}
      <button
        onClick={() => onPageChange(Math.max(1, page - 1))}
        disabled={page === 1}
        className="flex h-8 w-8 items-center justify-center rounded-lg transition-all disabled:opacity-30 disabled:cursor-not-allowed"
        style={{
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
        }}
        title="上一页"
      >
        <ChevronLeft size={14} />
      </button>

      {/* 页码按钮 */}
      {pageNumbers.map((num, idx) =>
        num === 'ellipsis' ? (
          <span
            key={`ellipsis-${idx}`}
            className="flex h-8 w-8 items-center justify-center text-xs"
            style={{ color: 'var(--text-muted)' }}
          >
            ···
          </span>
        ) : (
          <button
            key={num}
            onClick={() => onPageChange(num)}
            className={clsx(
              'flex h-8 min-w-[2rem] items-center justify-center rounded-lg px-2 text-sm font-medium transition-all',
              page === num && 'text-neon'
            )}
            style={
              page === num
                ? {
                    background: 'var(--nav-active-bg)',
                    border: '1px solid var(--border-hover)',
                    color: 'var(--neon-blue)',
                    boxShadow: 'var(--shadow-neon)',
                  }
                : {
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-secondary)',
                  }
            }
          >
            {num}
          </button>
        )
      )}

      {/* 下一页 */}
      <button
        onClick={() => onPageChange(Math.min(totalPages, page + 1))}
        disabled={page === totalPages}
        className="flex h-8 w-8 items-center justify-center rounded-lg transition-all disabled:opacity-30 disabled:cursor-not-allowed"
        style={{
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
        }}
        title="下一页"
      >
        <ChevronRight size={14} />
      </button>

      {/* 末页 */}
      <button
        onClick={() => onPageChange(totalPages)}
        disabled={page === totalPages}
        className="flex h-8 w-8 items-center justify-center rounded-lg transition-all disabled:opacity-30 disabled:cursor-not-allowed"
        style={{
          border: '1px solid var(--border-default)',
          color: 'var(--text-secondary)',
        }}
        title="末页"
      >
        <ChevronsRight size={14} />
      </button>

      {/* 快速跳转 */}
      {showJumper && totalPages > 5 && (
        <div className="ml-2 flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          <span>跳至</span>
          <input
            type="number"
            min={1}
            max={totalPages}
            onKeyDown={handleJump}
            className="h-8 w-14 rounded-lg border px-2 text-center text-sm outline-none transition-all focus:border-[var(--border-hover)]"
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              color: 'var(--text-primary)',
            }}
            placeholder={`${page}`}
          />
          <span>页</span>
        </div>
      )}
    </div>
  )
}
