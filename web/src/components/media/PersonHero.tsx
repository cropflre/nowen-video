import { ArrowLeft, ExternalLink, Film, Tv, User } from 'lucide-react'
import type { Person } from '@/types'
import { streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import { Button, Tag, buttonClassName } from '@/components/design-system'
import { HeroContent, MediaArtwork } from '@/ui'

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
  const totalWorks = movieCount + seriesCount

  return (
    <section className="relative overflow-hidden border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-canvas)]">
      <div
        className="pointer-events-none absolute inset-0 opacity-70"
        style={{ background: 'var(--nv-hero-bottom-scrim)' }}
        aria-hidden="true"
      />
      <div
        className="pointer-events-none absolute inset-0 opacity-75"
        style={{ background: 'radial-gradient(circle at 18% 24%, var(--nv-ambient-purple-soft), transparent 30rem)' }}
        aria-hidden="true"
      />

      <div className="relative mx-auto max-w-[var(--nv-content-max)] px-[var(--nv-page-gutter)] pb-8 pt-6">
        <Button type="button" variant="secondary" size="sm" onClick={onBack} className="mb-6">
          <ArrowLeft size={15} aria-hidden="true" />
          {t('personDetail.goBack')}
        </Button>

        <div className="grid items-center gap-6 sm:grid-cols-[12rem_minmax(0,1fr)] lg:gap-8">
          <MediaArtwork
            src={streamApi.getPersonProfileUrl(personId)}
            alt={person.name}
            ratio="square"
            loading="eager"
            className="mx-auto w-40 rounded-[var(--nv-radius-hero)] shadow-[var(--nv-shadow-elevated)] sm:mx-0 sm:w-48"
            fallback={(
              <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
                <User size={58} strokeWidth={1.4} aria-hidden="true" />
              </div>
            )}
          />

          <HeroContent
            compact
            className="text-center sm:text-left"
            eyebrow={(
              <div className="flex flex-wrap items-center justify-center gap-2 sm:justify-start">
                <Tag tone="brand">人物</Tag>
                {!worksLoading && totalWorks > 0 && (
                  <Tag>{t('personDetail.worksCount', { count: totalWorks })}</Tag>
                )}
              </div>
            )}
            title={person.name}
            subtitle={person.orig_name && person.orig_name !== person.name ? person.orig_name : undefined}
            badges={(
              <div className="flex flex-wrap items-center justify-center gap-2 sm:justify-start">
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
            )}
            actions={person.tmdb_id > 0 ? (
              <div className="flex w-full justify-center sm:justify-start">
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
            ) : undefined}
          />
        </div>
      </div>
    </section>
  )
}
