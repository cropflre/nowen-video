import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { collectionApi } from '@/api'
import { streamApi } from '@/api'
import type { MovieCollection } from '@/types'
import { Library, Search, Film, Loader2, X, Play, Merge, Trash2 } from 'lucide-react'

export default function CollectionsPage() {
  const navigate = useNavigate()
  const [collections, setCollections] = useState<MovieCollection[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [searchKeyword, setSearchKeyword] = useState('')
  const [searchResults, setSearchResults] = useState<MovieCollection[] | null>(null)
  const [operating, setOperating] = useState(false)
  const [operationMsg, setOperationMsg] = useState('')
  const pageSize = 24

  // 加载合集列表
  const fetchCollections = useCallback(async () => {
    setLoading(true)
    try {
      const res = await collectionApi.list({ page, size: pageSize })
      setCollections(res.data.data || [])
      setTotal(res.data.total || 0)
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [page])

  useEffect(() => {
    fetchCollections()
  }, [fetchCollections])

  // 搜索合集
  const handleSearch = useCallback(async () => {
    if (!searchKeyword.trim()) {
      setSearchResults(null)
      return
    }
    try {
      const res = await collectionApi.search(searchKeyword.trim(), 20)
      setSearchResults(res.data.data || [])
    } catch {
      setSearchResults([])
    }
  }, [searchKeyword])

  // 合并同名重复合集
  const handleMergeDuplicates = useCallback(async () => {
    if (operating) return
    setOperating(true)
    setOperationMsg('')
    try {
      const res = await collectionApi.mergeDuplicates()
      setOperationMsg(res.data.message || `已合并 ${res.data.merged} 组重复合集`)
      fetchCollections()
    } catch {
      setOperationMsg('合并失败，请重试')
    } finally {
      setOperating(false)
    }
  }, [operating, fetchCollections])

  // 清理空壳合集
  const handleCleanupEmpty = useCallback(async () => {
    if (operating) return
    setOperating(true)
    setOperationMsg('')
    try {
      const res = await collectionApi.cleanupEmpty()
      setOperationMsg(res.data.message || `已清理 ${res.data.cleaned} 个空壳合集`)
      fetchCollections()
    } catch {
      setOperationMsg('清理失败，请重试')
    } finally {
      setOperating(false)
    }
  }, [operating, fetchCollections])

  const displayList = searchResults !== null ? searchResults : collections
  const totalPages = Math.ceil(total / pageSize)

  return (
    <div className="space-y-6">
      {/* 页面标题 */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Library size={24} className="text-neon" />
          <h1 className="font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            影视合集
          </h1>
          <span className="rounded-full px-2.5 py-0.5 text-xs font-medium"
            style={{ background: 'var(--neon-blue-10)', color: 'var(--neon-blue)' }}>
            {total} 个合集
          </span>
        </div>
      </div>

      {/* 搜索栏 + 管理操作 */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative flex-1 max-w-md">
          <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-surface-400" />
          <input
            type="text"
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            placeholder="搜索合集名称..."
            className="input w-full pl-9 pr-8"
          />
          {searchKeyword && (
            <button
              onClick={() => { setSearchKeyword(''); setSearchResults(null) }}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-surface-400 hover:text-surface-300"
            >
              <X size={14} />
            </button>
          )}
        </div>
        <button onClick={handleSearch} className="btn-primary px-4 py-2 text-sm">
          搜索
        </button>
        <div className="flex gap-2 ml-auto">
          <button
            onClick={handleMergeDuplicates}
            disabled={operating}
            className="btn-ghost flex items-center gap-1.5 px-3 py-2 text-xs"
            title="合并所有同名重复合集，保留最早创建的合集并将电影迁移过来"
          >
            {operating ? <Loader2 size={14} className="animate-spin" /> : <Merge size={14} />}
            合并重复
          </button>
          <button
            onClick={handleCleanupEmpty}
            disabled={operating}
            className="btn-ghost flex items-center gap-1.5 px-3 py-2 text-xs"
            title="删除所有没有关联电影的空壳合集"
          >
            {operating ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
            清理空壳
          </button>
        </div>
      </div>

      {/* 操作结果提示 */}
      {operationMsg && (
        <div className="flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm"
          style={{ background: 'var(--neon-blue-10)', color: 'var(--neon-blue)', border: '1px solid var(--neon-blue-20)' }}>
          <span>{operationMsg}</span>
          <button onClick={() => setOperationMsg('')} className="ml-auto">
            <X size={14} />
          </button>
        </div>
      )}

      {/* 合集网格 */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <Loader2 size={24} className="animate-spin text-neon" />
        </div>
      ) : displayList.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-center">
          <Library size={48} className="mb-4 text-surface-600" />
          <p className="text-lg font-medium" style={{ color: 'var(--text-secondary)' }}>
            {searchResults !== null ? '未找到匹配的合集' : '暂无影视合集'}
          </p>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-tertiary)' }}>
            {searchResults !== null ? '请尝试其他关键词' : '扫描媒体库后系统会自动匹配电影系列合集'}
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {displayList.map((coll) => (
            <CollectionCard
              key={coll.id}
              collection={coll}
              onClick={() => navigate(`/collections/${coll.id}`)}
            />
          ))}
        </div>
      )}

      {/* 分页 */}
      {searchResults === null && totalPages > 1 && (
        <div className="flex items-center justify-center gap-2 pt-4">
          <button
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={page <= 1}
            className="btn-ghost px-3 py-1.5 text-sm disabled:opacity-30"
          >
            上一页
          </button>
          <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {page} / {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={page >= totalPages}
            className="btn-ghost px-3 py-1.5 text-sm disabled:opacity-30"
          >
            下一页
          </button>
        </div>
      )}
    </div>
  )
}

/** 合集卡片 — 点击跳转到合集详情页 */
function CollectionCard({ collection: coll, onClick }: { collection: MovieCollection; onClick: () => void }) {
  return (
    <div
      className="media-card group cursor-pointer overflow-hidden rounded-xl transition-all duration-300 hover:scale-[1.02]"
      style={{ border: '1px solid var(--border-default)' }}
      onClick={onClick}
    >
      {/* 海报 */}
      <div className="relative aspect-[2/3] overflow-hidden" style={{ background: 'var(--bg-surface)' }}>
        <img
          src={streamApi.getCollectionPosterUrl(coll.id)}
          alt={coll.name}
          className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        <div className="absolute inset-0 -z-10 flex items-center justify-center" style={{ color: 'var(--text-muted)' }}>
          <Film size={32} />
        </div>

        {/* 数量标签 */}
        <div className="absolute bottom-2 right-2 rounded-md px-2 py-0.5 text-xs font-bold backdrop-blur-md"
          style={{ background: 'rgba(0,0,0,0.7)', color: 'var(--neon-blue)' }}>
          {coll.media_count} 部
        </div>

        {/* 悬停遮罩 */}
        <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
          <div className="absolute bottom-2 left-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-full"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                boxShadow: '0 0 12px var(--neon-blue-40)',
              }}
            >
              <Play size={14} className="ml-0.5 text-white" fill="white" />
            </div>
          </div>
        </div>
      </div>

      {/* 信息 */}
      <div className="p-3">
        <h3 className="truncate text-sm font-semibold transition-colors group-hover:text-neon"
          style={{ color: 'var(--text-primary)' }}>
          {coll.name}
        </h3>
        {coll.overview && (
          <p className="mt-1 line-clamp-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {coll.overview}
          </p>
        )}
      </div>
    </div>
  )
}
