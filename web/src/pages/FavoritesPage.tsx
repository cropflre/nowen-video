import { useEffect, useState } from 'react'
import { userApi } from '@/api'
import type { Favorite } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import { Heart } from 'lucide-react'

export default function FavoritesPage() {
  const [favorites, setFavorites] = useState<Favorite[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const size = 30

  useEffect(() => {
    setLoading(true)
    userApi
      .favorites(page, size)
      .then((res) => {
        setFavorites(res.data.data || [])
        setTotal(res.data.total)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [page])

  const media = favorites.map((f) => f.media)
  const totalPages = Math.ceil(total / size)

  return (
    <div>
      <h1 className="mb-6 flex items-center gap-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        <Heart size={24} className="text-red-400" />
        我的收藏
      </h1>

      <MediaGrid items={media} loading={loading} />

      {/* 空状态 */}
      {!loading && media.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div
            className="mb-6 flex h-20 w-20 items-center justify-center rounded-2xl"
            style={{
              background: 'rgba(239, 68, 68, 0.05)',
              border: '1px solid rgba(239, 68, 68, 0.1)',
            }}
          >
            <Heart size={36} className="text-surface-600" />
          </div>
          <p className="font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-secondary)' }}>还没有收藏的内容</p>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-muted)' }}>
            浏览媒体库，点击收藏按钮添加到这里
          </p>
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
