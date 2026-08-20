import { useEffect, useMemo, useState } from 'react'
import { Film, Tv, User } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { personApi } from '@/api'
import type { Media, Person, Series } from '@/types'
import { useTranslation } from '@/i18n'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import MediaCard from '@/components/MediaCard'
import PersonHero from '@/components/media/PersonHero'
import { Button, EmptyState, PageContainer, Section, Tag } from '@/components/design-system'
import { MediaGrid as SharedMediaGrid } from '@/ui'

export default function PersonDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()

  const [person, setPerson] = useState<Person | null>(null)
  const [mediaList, setMediaList] = useState<Media[]>([])
  const [seriesList, setSeriesList] = useState<Series[]>([])
  const [loading, setLoading] = useState(true)
  const [worksLoading, setWorksLoading] = useState(true)

  const moviePagination = usePagination({ initialSize: 18, syncToUrl: true, pageKey: 'mp', sizeKey: 'mps' })
  const seriesPagination = usePagination({ initialSize: 18, syncToUrl: true, pageKey: 'sp', sizeKey: 'sps' })

  useEffect(() => {
    moviePagination.setPage(1)
    seriesPagination.setPage(1)
    // Pagination setters are stable for this route lifecycle; reset only when the person changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  const pagedMovies = useMemo(() => {
    const start = (moviePagination.page - 1) * moviePagination.size
    return mediaList.slice(start, start + moviePagination.size)
  }, [mediaList, moviePagination.page, moviePagination.size])

  const pagedSeries = useMemo(() => {
    const start = (seriesPagination.page - 1) * seriesPagination.size
    return seriesList.slice(start, start + seriesPagination.size)
  }, [seriesList, seriesPagination.page, seriesPagination.size])

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
    <div className="-mx-4 -mt-6 sm:-mx-6 lg:-mx-8">
      <PersonHero
        person={person}
        personId={id}
        movieCount={mediaList.length}
        seriesCount={seriesList.length}
        worksLoading={worksLoading}
        onBack={() => navigate(-1)}
      />

      <PageContainer width="wide" className="py-8">
        {worksLoading ? (
          <WorksSkeleton />
        ) : totalWorks === 0 ? (
          <EmptyState
            icon={<Film size={26} aria-hidden="true" />}
            title={t('personDetail.noWorks')}
            description={person.orig_name && person.orig_name !== person.name ? person.orig_name : undefined}
          />
        ) : (
          <div className="space-y-10">
            {mediaList.length > 0 && (
              <Section
                title={
                  <span className="inline-flex items-center gap-2">
                    <Film size={18} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                    {t('personDetail.movies')}
                  </span>
                }
                action={<Tag>{mediaList.length}</Tag>}
              >
                <SharedMediaGrid>
                  {pagedMovies.map((media) => <MediaCard key={media.id} media={media} />)}
                </SharedMediaGrid>
                <Pagination
                  page={moviePagination.page}
                  totalPages={moviePagination.totalPages(mediaList.length)}
                  total={mediaList.length}
                  pageSize={moviePagination.size}
                  pageSizeOptions={[12, 18, 24, 48]}
                  onPageChange={moviePagination.setPage}
                  onPageSizeChange={moviePagination.setSize}
                />
              </Section>
            )}

            {seriesList.length > 0 && (
              <Section
                title={
                  <span className="inline-flex items-center gap-2">
                    <Tv size={18} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                    {t('personDetail.tvShows')}
                  </span>
                }
                action={<Tag>{seriesList.length}</Tag>}
              >
                <SharedMediaGrid>
                  {pagedSeries.map((series) => <MediaCard key={series.id} series={series} />)}
                </SharedMediaGrid>
                <Pagination
                  page={seriesPagination.page}
                  totalPages={seriesPagination.totalPages(seriesList.length)}
                  total={seriesList.length}
                  pageSize={seriesPagination.size}
                  pageSizeOptions={[12, 18, 24, 48]}
                  onPageChange={seriesPagination.setPage}
                  onPageSizeChange={seriesPagination.setSize}
                />
              </Section>
            )}
          </div>
        )}
      </PageContainer>
    </div>
  )
}

function PersonDetailSkeleton() {
  return (
    <div className="-mx-4 -mt-6 animate-pulse sm:-mx-6 lg:-mx-8">
      <div className="border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
        <div className="mx-auto flex max-w-[var(--nv-content-max)] flex-col items-center gap-6 px-[var(--nv-page-gutter)] py-8 sm:flex-row sm:items-start">
          <div className="h-40 w-40 shrink-0 rounded-[var(--nv-radius-hero)] bg-[var(--nv-bg-surface-soft)] sm:h-48 sm:w-48" />
          <div className="w-full max-w-xl space-y-3 sm:pt-4">
            <div className="h-5 w-20 rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-surface-soft)]" />
            <div className="h-9 w-2/3 rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]" />
            <div className="h-5 w-1/3 rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-surface-soft)]" />
          </div>
        </div>
      </div>
      <PageContainer width="wide" className="py-8">
        <WorksSkeleton />
      </PageContainer>
    </div>
  )
}

function WorksSkeleton() {
  return (
    <div className="space-y-8 animate-pulse">
      {[6, 3].map((count, sectionIndex) => (
        <section key={sectionIndex} className="space-y-4">
          <div className="flex items-center justify-between gap-3">
            <div className="h-6 w-28 rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-surface-soft)]" />
            <div className="h-6 w-9 rounded-full bg-[var(--nv-bg-surface-soft)]" />
          </div>
          <SharedMediaGrid>
            {Array.from({ length: count }).map((_, index) => (
              <div key={index}>
                <div className="skeleton aspect-[2/3] rounded-[var(--nv-radius-card)]" />
                <div className="skeleton mt-2 h-3 w-3/4" />
                <div className="skeleton mt-1.5 h-2.5 w-1/2" />
              </div>
            ))}
          </SharedMediaGrid>
        </section>
      ))}
    </div>
  )
}
