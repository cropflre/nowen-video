import { userApi } from '@/api'
import { useToast } from '@/components/Toast'
import { EmptyState } from '@/components/design-system'
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

  const { data, loading, error } = usePageCache<FavoritesData>(
    `favorites:page=${page}:size=${size}`,
    async () => {
      const res = await userApi.favorites(page, size)
      return { list: res.data.data || [], total: res.data.total }
    },
    { ttl: 15_000 },
  )

  if (error) toast.error(t('favorites.loadFailed'))

  const favorites = data?.list ?? []
  const total = data?.total ?? 0
  const media = favorites.map((favorite) => favorite.media)
  const pages = totalPages(total)

  return (
    <div className="nv-section-stack">
      <div>
        <h1 className="flex items-center gap-2 text-2xl font-bold tracking-[-0.02em] text-[var(--nv-text-primary)]">
          <Heart size={22} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
          {t('favorites.title')}
        </h1>
        {total > 0 && <p className="mt-1 text-sm text-[var(--nv-text-tertiary)]">共 {total} 个收藏</p>}
      </div>

      <MediaGrid items={media} loading={loading} />

      {!loading && media.length === 0 && (
        <EmptyState
          icon={<Heart size={26} aria-hidden="true" />}
          title={t('favorites.empty')}
          description={t('favorites.emptyHint')}
        />
      )}

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
