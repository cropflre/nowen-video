import { userApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { usePageCache } from '@/hooks/usePageCache'
import { usePagination } from '@/hooks/usePagination'
import type { Favorite } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import Pagination from '@/components/Pagination'
import { Heart } from 'lucide-react'

interface FavoritesData {
  list: Favorite[]
  total: number
}

export default function FavoritesPage() {
  const { page, size, setPage, setSize, totalPages } = usePagination({
    initialSize: 30,
    syncToUrl: true,
  })
  const toast = useToast()
  const { t } = useTranslation()

  // 按 page 分键缓存：切换分页时如果命中缓存则零 loading
  const { data, loading, error } = usePageCache<FavoritesData>(
    `favorites:page=${page}:size=${size}`,
    async () => {
      const res = await userApi.favorites(page, size)
      return { list: res.data.data || [], total: res.data.total }
    },
    { ttl: 15_000 },
  )

  if (error) {
    toast.error(t('favorites.loadFailed'))
  }

  const favorites = data?.list ?? []
  const total = data?.total ?? 0
  const media = favorites.map((f) => f.media)
  const pages = totalPages(total)

  return (
    <div>
      <h1 className="mb-6 flex items-center gap-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        <Heart size={24} className="text-red-400" />
        {t('favorites.title')}
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
          <p className="font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-secondary)' }}>{t('favorites.empty')}</p>
          <p className="mt-1 text-sm" style={{ color: 'var(--text-muted)' }}>
            {t('favorites.emptyHint')}
          </p>
        </div>
      )}

      {/* 分页 */}
      <Pagination
        page={page}
        totalPages={pages}
        total={total}
        pageSize={size}
        pageSizeOptions={[20, 30, 50, 100]}
        onPageChange={setPage}
        onPageSizeChange={setSize}
      />
    </div>
  )
}
