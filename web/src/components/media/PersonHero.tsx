import { useEffect, useState } from 'react'
import { ArrowLeft, ExternalLink, Film, Tv, User } from 'lucide-react'
import type { Person } from '@/types'
import { streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import { Button, Tag, buttonClassName } from '@/components/design-system'

interface PersonHeroProps {
  person: Person
  personId: string
  movieCount: number
  seriesCount: number
  worksLoading: boolean
  onBack: () => void
}

export default function PersonHero({
  person,
  personId,
  movieCount,
  seriesCount,
  worksLoading,
  onBack,
}: PersonHeroProps) {
  const { t } = useTranslation()
  const [imageFailed, setImageFailed] = useState(false)
  const totalWorks = movieCount + seriesCount

  useEffect(() => {
    setImageFailed(false)
  }, [personId])

  return (
    <section className="relative overflow-hidden border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
      <div
        className="pointer-events-none absolute inset-0 opacity-70"
        style={{ background: 'var(--nv-hero-bottom-scrim)' }}
        aria-hidden="true"
      />

      <div className="relative mx-auto max-w-7xl px-4 pb-8 pt-6 sm:px-6 lg:px-8">
        <Button type="button" variant="secondary" size="sm" onClick={onBack} className="mb-6">
          <ArrowLeft size={15} aria-hidden="true" />
          {t('personDetail.goBack')}
        </Button>

        <div className="flex flex-col items-center gap-6 sm:flex-row sm:items-start">
          <div className="h-40 w-40 shrink-0 overflow-hidden rounded-[var(--nv-radius-hero)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] shadow-[var(--nv-shadow-elevated)] sm:h-48 sm:w-48">
            {!imageFailed ? (
              <img
                src={streamApi.getPersonProfileUrl(personId)}
                alt={person.name}
                className="h-full w-full object-cover"
                onError={() => setImageFailed(true)}
              />
            ) : (
              <div className="flex h-full w-full items-center justify-center bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-tertiary)]">
                <User size={58} strokeWidth={1.4} aria-hidden="true" />
              </div>
            )}
          </div>

          <div className="min-w-0 flex-1 text-center sm:pt-2 sm:text-left">
            <div className="flex flex-wrap items-center justify-center gap-2 sm:justify-start">
              <Tag tone="brand">人物</Tag>
              {!worksLoading && totalWorks > 0 && (
                <Tag>{t('personDetail.worksCount', { count: totalWorks })}</Tag>
              )}
            </div>

            <h1 className="mt-3 text-3xl font-bold leading-tight tracking-[-0.025em] text-[var(--nv-text-primary)] sm:text-4xl">
              {person.name}
            </h1>

            {person.orig_name && person.orig_name !== person.name && (
              <p className="mt-2 text-sm text-[var(--nv-text-secondary)] sm:text-base">{person.orig_name}</p>
            )}

            <div className="mt-5 flex flex-wrap items-center justify-center gap-2 sm:justify-start">
              {movieCount > 0 && (
                <Tag>
                  <Film size={12} aria-hidden="true" />
                  {t('personDetail.movieCount', { count: movieCount })}
                </Tag>
              )}
              {seriesCount > 0 && (
                <Tag>
                  <Tv size={12} aria-hidden="true" />
                  {t('personDetail.seriesCount', { count: seriesCount })}
                </Tag>
              )}
            </div>

            {person.tmdb_id > 0 && (
              <div className="mt-5 flex justify-center sm:justify-start">
                <a
                  href={`https://www.themoviedb.org/person/${person.tmdb_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={buttonClassName({ variant: 'secondary', size: 'sm' })}
                >
                  <ExternalLink size={13} aria-hidden="true" />
                  {t('personDetail.viewOnTMDb')}
                </a>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  )
}
