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
    <div className="nv-section-stack nv-library-page nv-favorites-page">
      <header className="nv-page-hero-header">
        <div className="nv-page-title-lockup">
          <div className="nv-page-title-icon" aria-hidden="true">
            <Heart size={20} />
          </div>
          <div className="min-w-0">
            <h1 className="nv-page-title">{t('favorites.title')}</h1>
            <p className="nv-page-subtitle">
              {total > 0 ? `共 ${total} 个收藏` : '集中浏览你收藏的电影与剧集。'}
            </p>
          </div>
        </div>
      </header>

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
