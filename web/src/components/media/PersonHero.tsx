import { ArrowLeft, ExternalLink, Film, Tv, User } from 'lucide-react'
import type { Person } from '@/types'
import { streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import { Button } from '@/components/design-system'
import { MediaArtwork } from '@/ui'

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
    <section className="nv-person-profile-hero">
      <Button type="button" variant="ghost" size="sm" onClick={onBack} className="nv-person-back-button">
        <ArrowLeft size={15} aria-hidden="true" />
        {t('personDetail.goBack')}
      </Button>

      <div className="nv-person-profile-layout">
        <MediaArtwork
          src={streamApi.getPersonProfileUrl(personId)}
          alt={person.name}
          ratio="square"
          loading="eager"
          className="nv-person-profile-artwork"
          fallback={(
            <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
              <User size={42} strokeWidth={1.35} aria-hidden="true" />
            </div>
          )}
        />

        <div className="nv-person-profile-copy">
          <div className="nv-person-profile-title-row">
            <div className="min-w-0">
              <h1>{person.name}</h1>
              {person.orig_name && person.orig_name !== person.name && <p className="nv-person-original-name">{person.orig_name}</p>}
            </div>
            {person.tmdb_id > 0 && (
              <a
                href={`https://www.themoviedb.org/person/${person.tmdb_id}`}
                target="_blank"
                rel="noopener noreferrer"
                className="nv-person-inline-link"
              >
                TMDb
                <ExternalLink size={12} aria-hidden="true" />
              </a>
            )}
          </div>

          {!worksLoading && totalWorks > 0 && (
            <div className="nv-person-work-summary" aria-label="作品统计">
              {movieCount > 0 && <span><Film size={13} aria-hidden="true" />{movieCount} 部电影</span>}
              {seriesCount > 0 && <span><Tv size={13} aria-hidden="true" />{seriesCount} 部剧集</span>}
            </div>
          )}

          {/* Person currently exposes name/original name/TMDb only. Do not fabricate biography fields. */}
        </div>
      </div>
    </section>
  )
}
