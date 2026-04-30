import { useCallback, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router-dom'

export interface UsePaginationOptions {
  /** 初始页码，默认 1 */
  initialPage?: number
  /** 初始每页数量，默认 20 */
  initialSize?: number
  /** 可选的每页数量选项 */
  pageSizeOptions?: number[]
  /** 是否把分页状态同步到 URL (?page=&size=)，默认 false */
  syncToUrl?: boolean
  /** URL 模式下使用的查询参数 key，默认 page / size */
  pageKey?: string
  /** URL 模式下的每页数量参数 key */
  sizeKey?: string
  /** 当筛选条件变化时用于触发回到第一页的依赖，任何一项变化都会 reset 到 page=1 */
  resetOnChange?: unknown[]
}

export interface UsePaginationReturn {
  page: number
  size: number
  setPage: (page: number) => void
  setSize: (size: number) => void
  /** 计算总页数（传入 total） */
  totalPages: (total: number) => number
  /** 重置到第一页（保留 size） */
  reset: () => void
  /** 下一页（不会越界需要配合 totalPages） */
  nextPage: () => void
  /** 上一页 */
  prevPage: () => void
  /** 可选项：当前 size 是否来自默认值 */
  isDefaultSize: boolean
}

/**
 * 分页状态管理 Hook。
 *
 * 支持两种模式：
 * 1. 本地状态：`syncToUrl=false`（默认）
 * 2. URL 同步：`syncToUrl=true`，页码变化会写入 ?page=N&size=N，刷新/分享链接后仍保留
 *
 * 用法：
 * ```ts
 * const { page, size, setPage, setSize, totalPages } = usePagination({
 *   initialSize: 30,
 *   syncToUrl: true,
 *   resetOnChange: [keyword, statusFilter],
 * })
 * const pages = totalPages(totalFromApi)
 * ```
 */
export function usePagination(options: UsePaginationOptions = {}): UsePaginationReturn {
  const {
    initialPage = 1,
    initialSize = 20,
    syncToUrl = false,
    pageKey = 'page',
    sizeKey = 'size',
  } = options

  const [searchParams, setSearchParams] = useSearchParams()

  // 本地状态（仅在 syncToUrl=false 时使用）
  const [localPage, setLocalPage] = useState(initialPage)
  const [localSize, setLocalSize] = useState(initialSize)

  const page = useMemo(() => {
    if (syncToUrl) {
      const p = parseInt(searchParams.get(pageKey) || String(initialPage), 10)
      return Number.isFinite(p) && p >= 1 ? p : initialPage
    }
    return localPage
  }, [syncToUrl, searchParams, pageKey, initialPage, localPage])

  const size = useMemo(() => {
    if (syncToUrl) {
      const s = parseInt(searchParams.get(sizeKey) || String(initialSize), 10)
      return Number.isFinite(s) && s >= 1 ? s : initialSize
    }
    return localSize
  }, [syncToUrl, searchParams, sizeKey, initialSize, localSize])

  const setPage = useCallback(
    (next: number) => {
      if (syncToUrl) {
        setSearchParams(
          (prev) => {
            const p = new URLSearchParams(prev)
            if (next <= 1) p.delete(pageKey)
            else p.set(pageKey, String(next))
            return p
          },
          { replace: true },
        )
      } else {
        setLocalPage(Math.max(1, next))
      }
    },
    [syncToUrl, setSearchParams, pageKey],
  )

  const setSize = useCallback(
    (next: number) => {
      if (syncToUrl) {
        setSearchParams(
          (prev) => {
            const p = new URLSearchParams(prev)
            // 切换每页数量时回到第一页
            p.delete(pageKey)
            if (next === initialSize) p.delete(sizeKey)
            else p.set(sizeKey, String(next))
            return p
          },
          { replace: true },
        )
      } else {
        setLocalSize(Math.max(1, next))
        setLocalPage(1)
      }
    },
    [syncToUrl, setSearchParams, pageKey, sizeKey, initialSize],
  )

  const totalPages = useCallback(
    (total: number) => Math.max(1, Math.ceil(total / size)),
    [size],
  )

  const reset = useCallback(() => setPage(1), [setPage])
  const nextPage = useCallback(() => setPage(page + 1), [setPage, page])
  const prevPage = useCallback(() => setPage(Math.max(1, page - 1)), [setPage, page])

  return {
    page,
    size,
    setPage,
    setSize,
    totalPages,
    reset,
    nextPage,
    prevPage,
    isDefaultSize: size === initialSize,
  }
}

export default usePagination
