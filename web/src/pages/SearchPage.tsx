import { useState, useEffect, useCallback } from 'react'
import { mediaApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { Media } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import { Search as SearchIcon, X } from 'lucide-react'

export default function SearchPage() {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<Media[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)
  const size = 30
  const toast = useToast()

  const doSearch = useCallback(async (q: string, p: number) => {
    if (!q.trim()) return
    setLoading(true)
    setSearched(true)
    try {
      const res = await mediaApi.search(q.trim(), p, size)
      setResults(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      toast.error('搜索失败，请稍后重试')
    } finally {
      setLoading(false)
    }
  }, [])

  // 防抖搜索
  useEffect(() => {
    if (!query.trim()) {
      setResults([])
      setTotal(0)
      setSearched(false)
      return
    }

    const timer = setTimeout(() => {
      setPage(1)
      doSearch(query, 1)
    }, 400)

    return () => clearTimeout(timer)
  }, [query, doSearch])

  // 翻页时搜索
  useEffect(() => {
    if (page > 1 && query.trim()) {
      doSearch(query, page)
    }
  }, [page, query, doSearch])

  const totalPages = Math.ceil(total / size)

  return (
    <div>
      {/* 搜索框 - 霓虹风格 */}
      <div className="relative mb-8">
        <SearchIcon
          size={20}
          className="absolute left-4 top-1/2 -translate-y-1/2 text-neon/40"
        />
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="input pl-12 pr-12 py-3.5 text-base"
          placeholder="搜索电影、剧集..."
          autoFocus
        />
        {query && (
          <button
            onClick={() => setQuery('')}
            className="absolute right-4 top-1/2 -translate-y-1/2 text-surface-500 transition-colors hover:text-neon"
          >
            <X size={18} />
          </button>
        )}
      </div>

      {/* 搜索结果 */}
      {searched && (
        <div className="mb-4 text-sm text-surface-400">
          找到 <span className="font-semibold text-neon">{total}</span> 个结果
        </div>
      )}

      <MediaGrid items={results} loading={loading} />

      {/* 空状态 */}
      {searched && !loading && results.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div
            className="mb-6 flex h-20 w-20 items-center justify-center rounded-2xl"
            style={{
              background: 'rgba(0, 240, 255, 0.05)',
              border: '1px solid rgba(0, 240, 255, 0.08)',
            }}
          >
            <SearchIcon size={36} className="text-surface-600" />
          </div>
          <p className="font-display text-base font-semibold tracking-wide text-surface-300">未找到匹配的内容</p>
          <p className="mt-1 text-sm text-surface-600">试试其他关键词</p>
        </div>
      )}

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="mt-8 flex items-center justify-center gap-3">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page === 1}
className="btn-ghost rounded-xl border border-neon-blue/10 px-4 py-2 text-sm disabled:opacity-30"
          >
            上一页
          </button>
          <span className="font-display text-sm tracking-wide text-neon">
            {page} / {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page === totalPages}
className="btn-ghost rounded-xl border border-neon-blue/10 px-4 py-2 text-sm disabled:opacity-30"
          >
            下一页
          </button>
        </div>
      )}
    </div>
  )
}
