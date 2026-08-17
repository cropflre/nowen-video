import { userApi } from '@/api'
import { useToast } from '@/components/Toast'
import { EmptyState } from '@/components/design-system'
import { useTranslation } from '@/i18n'
import { usePageCache } from '@/hooks/usePageCache'
import { usePagination } from '@/hooks/usePagination'
import type { Favorite } from '@/types'
import MediaCard from '@/components/MediaCard'
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
  const media = favorites.map((favorite) => favorite.media).filter(Boolean)
  const pages = totalPages(total)

  return (
    <div className="nv-personal-workspace nv-favorites-page">
      <header className="nv-personal-workspace-header">
        <div className="nv-page-title-lockup">
          <div className="nv-page-title-icon" aria-hidden="true">
            <Heart size={20} />
          </div>
          <div className="min-w-0">
            <span className="nv-personal-workspace-eyebrow">MY LIBRARY</span>
            <h1 className="nv-page-title">{t('favorites.title')}</h1>
            <p className="nv-page-subtitle">把喜欢的电影与剧集留在一个更容易再次找到的位置。</p>
          </div>
        </div>
        <div className="nv-personal-workspace-stat" aria-label={`共 ${total} 个收藏`}>
          <strong>{total}</strong>
          <span>个收藏</span>
        </div>
      </header>

      <section className="nv-personal-workspace-panel" aria-labelledby="favorite-media-title">
        <div className="nv-personal-workspace-toolbar">
          <div>
            <h2 id="favorite-media-title">收藏内容</h2>
            <p>{total > 0 ? `当前共有 ${total} 个收藏，按最近收藏内容浏览。` : '收藏的内容会集中显示在这里。'}</p>
          </div>
          {total > 0 && <span className="nv-personal-workspace-count">{total} 项</span>}
        </div>

        {loading && (
          <div className="nv-personal-media-grid" aria-busy="true" aria-label="正在加载收藏内容">
            {Array.from({ length: 10 }).map((_, index) => (
              <div key={index}>
                <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
                <div className="skeleton mt-2 h-3 w-3/4" />
                <div className="skeleton mt-1.5 h-2.5 w-1/2" />
              </div>
            ))}
          </div>
        )}

        {!loading && media.length > 0 && (
          <div className="nv-personal-media-grid">
            {media.map((item) => <MediaCard key={item.id} media={item} />)}
          </div>
        )}

        {!loading && media.length === 0 && (
          <EmptyState
            className="nv-personal-workspace-empty"
            icon={<Heart size={26} aria-hidden="true" />}
            title={t('favorites.empty')}
            description={t('favorites.emptyHint')}
          />
        )}

        {total > 0 && (
          <div className="nv-personal-workspace-pagination">
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
        )}
      </section>
    </div>
  )
}
