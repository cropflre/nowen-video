import { useMemo } from 'react'
import { Layers } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { collectionApi } from '@/api'
import { usePageCache } from '@/hooks/usePageCache'
import type { CollectionWithMedia } from '@/types'
import { groupByMovie } from '@/utils/collectionGroup'
import CollectionDetailHero from '@/components/media/CollectionDetailHero'
import CollectionMovieBrowser from '@/components/media/CollectionMovieBrowser'
import { Button, EmptyState, Surface } from '@/components/design-system'
import { MediaGrid } from '@/ui'

export default function CollectionDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const { data, loading, error } = usePageCache<CollectionWithMedia>(
    id ? `collection:detail:${id}` : null,
    async () => {
      const res = await collectionApi.getDetail(id!)
      return res.data.data
    },
    { ttl: 60_000 },
  )

  const movieCount = useMemo(() => groupByMovie(data?.media || []).length, [data?.media])
  const fileCount = data?.media?.length || 0

  if (loading && !data) {
    return (
      <div className="nv-collection-detail-page relative -mx-4 -mt-6 sm:-mx-6 lg:-mx-8">
        <div className="skeleton nv-collection-detail-hero-skeleton w-full" />
        <div className="nv-collection-detail-body">
          <MediaGrid aria-busy="true" aria-label="合集内容加载中">
            {Array.from({ length: 12 }).map((_, index) => <div key={index} className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />)}
          </MediaGrid>
        </div>
      </div>
    )
  }

  if (error || !data) {
    return (
      <Surface variant="raised" className="mx-auto max-w-3xl">
        <EmptyState
          icon={<Layers size={24} />}
          title={error ? '合集不存在或加载失败' : '合集不存在'}
          description="返回合集列表后可以继续浏览其他系列合集。"
          action={<Button type="button" variant="secondary" onClick={() => navigate('/collections')}>返回合集列表</Button>}
        />
      </Surface>
    )
  }

  return (
    <div className="nv-collection-detail-page relative -mx-4 -mt-6 sm:-mx-6 lg:-mx-8">
      <CollectionDetailHero
        data={data}
        movieCount={movieCount}
        fileCount={fileCount}
        onBack={() => {
          if (window.history.length > 1) navigate(-1)
          else navigate('/collections')
        }}
      />

      <div className="nv-collection-detail-body">
        <CollectionMovieBrowser media={data.media} />
      </div>
    </div>
  )
}
