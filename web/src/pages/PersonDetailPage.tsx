import { useEffect, useState } from 'react'
import { Film, User } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { personApi } from '@/api'
import type { Media, Person, Series } from '@/types'
import { useTranslation } from '@/i18n'
import MediaCard from '@/components/MediaCard'
import PersonHero from '@/components/media/PersonHero'
import { Button, EmptyState, PageContainer } from '@/components/design-system'
import { MediaRail } from '@/ui'

export default function PersonDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()

  const [person, setPerson] = useState<Person | null>(null)
  const [mediaList, setMediaList] = useState<Media[]>([])
  const [seriesList, setSeriesList] = useState<Series[]>([])
  const [loading, setLoading] = useState(true)
  const [worksLoading, setWorksLoading] = useState(true)

  useEffect(() => {
    if (!id) {
      setPerson(null)
      setMediaList([])
      setSeriesList([])
      setLoading(false)
      setWorksLoading(false)
      return
    }

    const abortController = new AbortController()
    setLoading(true)
    setWorksLoading(true)

    personApi.getDetail(id)
      .then((res) => {
        if (!abortController.signal.aborted) setPerson(res.data.data)
      })
      .catch(() => {
        if (!abortController.signal.aborted) setPerson(null)
      })
      .finally(() => {
        if (!abortController.signal.aborted) setLoading(false)
      })

    personApi.getMedia(id)
      .then((res) => {
        if (!abortController.signal.aborted) {
          setMediaList(res.data.media || [])
          setSeriesList(res.data.series || [])
        }
      })
      .catch(() => {
        if (!abortController.signal.aborted) {
          setMediaList([])
          setSeriesList([])
        }
      })
      .finally(() => {
        if (!abortController.signal.aborted) setWorksLoading(false)
      })

    return () => abortController.abort()
  }, [id])

  if (loading) return <PersonDetailSkeleton />

  if (!person || !id) {
    return (
      <PageContainer className="py-12">
        <EmptyState
          icon={<User size={28} aria-hidden="true" />}
          title={t('personDetail.notFound')}
          action={<Button type="button" variant="secondary" onClick={() => navigate(-1)}>{t('personDetail.goBack')}</Button>}
        />
      </PageContainer>
    )
  }

  const totalWorks = mediaList.length + seriesList.length

  return (
    <div className="nv-person-detail-page">
      <PersonHero
        person={person}
        personId={id}
        movieCount={mediaList.length}
        seriesCount={seriesList.length}
        worksLoading={worksLoading}
        onBack={() => navigate(-1)}
      />

      <div className="nv-person-detail-content">
        {worksLoading ? (
          <WorksSkeleton />
        ) : totalWorks === 0 ? (
          <EmptyState
            icon={<Film size={26} aria-hidden="true" />}
            title={t('personDetail.noWorks')}
            description={person.orig_name && person.orig_name !== person.name ? person.orig_name : undefined}
          />
        ) : (
          <div className="nv-person-work-sections">
            {mediaList.length > 0 && (
              <MediaRail title={t('personDetail.movies')} ariaLabel={t('personDetail.movies')} itemCount={mediaList.length}>
                {mediaList.map((media) => (
                  <div key={media.id} className="nv-person-work-card flex-shrink-0">
                    <MediaCard media={media} showBadges={false} />
                  </div>
                ))}
              </MediaRail>
            )}

            {seriesList.length > 0 && (
              <MediaRail title={t('personDetail.tvShows')} ariaLabel={t('personDetail.tvShows')} itemCount={seriesList.length}>
                {seriesList.map((series) => (
                  <div key={series.id} className="nv-person-work-card flex-shrink-0">
                    <MediaCard series={series} showBadges={false} />
                  </div>
                ))}
              </MediaRail>
            )}
          </div>
        )}

        {person.tmdb_id > 0 && (
          <section className="nv-person-external-links" aria-labelledby="person-external-title">
            <h2 id="person-external-title">外部链接</h2>
            <a href={`https://www.themoviedb.org/person/${person.tmdb_id}`} target="_blank" rel="noopener noreferrer">TMDb</a>
          </section>
        )}
      </div>
    </div>
  )
}

function PersonDetailSkeleton() {
  return (
    <div className="nv-person-detail-page animate-pulse">
      <div className="nv-person-profile-hero">
        <div className="h-8 w-16 rounded bg-[var(--nv-bg-surface-soft)]" />
        <div className="nv-person-profile-layout">
          <div className="skeleton aspect-square w-[120px] rounded-[var(--nv-radius-card)]" />
          <div className="w-full max-w-lg space-y-3">
            <div className="skeleton h-8 w-40" />
            <div className="skeleton h-4 w-28" />
            <div className="skeleton h-4 w-64" />
          </div>
        </div>
      </div>
      <div className="nv-person-detail-content"><WorksSkeleton /></div>
    </div>
  )
}

function WorksSkeleton() {
  return (
    <div className="space-y-8 animate-pulse">
      {[7, 5].map((count, sectionIndex) => (
        <section key={sectionIndex} className="space-y-3">
          <div className="skeleton h-5 w-24" />
          <div className="flex gap-3 overflow-hidden">
            {Array.from({ length: count }).map((_, index) => (
              <div key={index} className="w-[106px] flex-shrink-0">
                <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
                <div className="skeleton mt-2 h-3 w-3/4" />
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
