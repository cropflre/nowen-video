import { useState, useCallback, useMemo, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { streamApi } from '@/api'
import type { MediaPerson } from '@/types'
import { Tag } from '@/components/design-system'
import { User, Film } from 'lucide-react'
import { useTranslation } from '@/i18n'

interface CastGridProps {
  persons: MediaPerson[]
  initialCount?: number
}

function useRoleLabel() {
  const { t } = useTranslation()
  return (role: string) => {
    const map: Record<string, string> = {
      director: t('castGrid.roleDirector'),
      actor: t('castGrid.roleActor'),
      writer: t('castGrid.roleWriter'),
    }
    return map[role] || role
  }
}

const rolePriority: Record<string, number> = {
  director: 0,
  writer: 1,
  actor: 2,
}

export default function CastGrid({ persons }: CastGridProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const scrollRef = useRef<HTMLDivElement>(null)

  const dedupedPersons = useMemo(() => {
    const seen = new Set<string>()
    return persons.filter((mediaPerson) => {
      const key = `${mediaPerson.person_id}:${mediaPerson.role}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
  }, [persons])

  const sortedPersons = useMemo(() => {
    return [...dedupedPersons].sort((a, b) => {
      const firstPriority = rolePriority[a.role] ?? 99
      const secondPriority = rolePriority[b.role] ?? 99
      if (firstPriority !== secondPriority) return firstPriority - secondPriority
      return a.sort_order - b.sort_order
    })
  }, [dedupedPersons])

  const handleCardClick = useCallback((person: MediaPerson) => {
    if (person.person_id) navigate(`/person/${person.person_id}`)
  }, [navigate])

  if (dedupedPersons.length === 0) return null

  return (
    <section aria-labelledby="cast-grid-title">
      <div className="mb-4 flex items-center gap-2">
        <Film size={16} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
        <h2 id="cast-grid-title" className="text-base font-semibold text-[var(--nv-text-primary)]">
          {t('castGrid.title')}
        </h2>
        <span className="text-xs text-[var(--nv-text-tertiary)]">({dedupedPersons.length})</span>
      </div>

      <div
        ref={scrollRef}
        className="flex gap-3 overflow-x-auto pb-2"
        style={{
          scrollbarWidth: 'thin',
          scrollbarColor: 'var(--nv-border-strong) transparent',
        }}
        role="list"
        aria-label={t('castGrid.title')}
      >
        {sortedPersons.map((mediaPerson) => (
          <CastCard key={mediaPerson.id} mediaPerson={mediaPerson} onClick={handleCardClick} />
        ))}
      </div>
    </section>
  )
}

function CastCard({
  mediaPerson,
  onClick,
}: {
  mediaPerson: MediaPerson
  onClick: (person: MediaPerson) => void
}) {
  const { t } = useTranslation()
  const getRoleLabel = useRoleLabel()
  const [imgError, setImgError] = useState(false)
  const person = mediaPerson.person
  const profileSrc = person?.id ? streamApi.getPersonProfileUrl(person.id) : null
  const roleLabel = getRoleLabel(mediaPerson.role)

  return (
    <button
      type="button"
      onClick={() => onClick(mediaPerson)}
      className="group flex w-24 flex-shrink-0 flex-col items-center gap-2 rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] p-2 text-left transition-[background-color,border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)] hover:shadow-[var(--nv-shadow-card)] sm:w-28"
      role="listitem"
      aria-label={`${person?.name || t('castGrid.unknown')} · ${roleLabel}`}
    >
      <div className="relative aspect-square w-full overflow-hidden rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)]">
        {profileSrc && !imgError ? (
          <img
            src={profileSrc}
            alt={person?.name || ''}
            className="h-full w-full object-cover transition-transform duration-300 ease-out group-hover:scale-[1.025]"
            loading="lazy"
            onError={() => setImgError(true)}
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center bg-[var(--nv-bg-surface)] text-[var(--nv-text-tertiary)]">
            <User size={32} strokeWidth={1.5} aria-hidden="true" />
          </div>
        )}

        {mediaPerson.role && mediaPerson.role !== 'actor' && (
          <Tag
            tone="quality"
            className="absolute left-1.5 top-1.5 z-10 max-w-[calc(100%-12px)] text-[11px] shadow-sm"
          >
            {roleLabel}
          </Tag>
        )}
      </div>

      <div className="w-full min-w-0 text-center">
        <p className="truncate text-xs font-medium text-[var(--nv-text-primary)] transition-colors group-hover:text-[var(--nv-action-primary)]">
          {person?.name || t('castGrid.unknown')}
        </p>
        {mediaPerson.character && (
          <p
            className="mt-0.5 truncate text-[10px] text-[var(--nv-text-tertiary)]"
            title={t('castGrid.asRole', { character: mediaPerson.character })}
          >
            {t('castGrid.asRole', { character: mediaPerson.character })}
          </p>
        )}
        {!mediaPerson.character && mediaPerson.role !== 'actor' && (
          <p className="mt-0.5 truncate text-[10px] text-[var(--nv-text-tertiary)]">{roleLabel}</p>
        )}
      </div>
    </button>
  )
}
