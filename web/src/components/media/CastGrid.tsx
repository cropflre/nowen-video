import { useState, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { streamApi } from '@/api'
import type { MediaPerson } from '@/types'
import { EmptyState, Tag } from '@/components/design-system'
import { User, Users } from 'lucide-react'
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

const rolePriority: Record<string, number> = { director: 0, writer: 1, actor: 2 }

export default function CastGrid({ persons }: CastGridProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const dedupedPersons = useMemo(() => {
    const seen = new Set<string>()
    return persons.filter((mediaPerson) => {
      const key = `${mediaPerson.person_id}:${mediaPerson.role}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
  }, [persons])

  const sortedPersons = useMemo(() => [...dedupedPersons].sort((a, b) => {
    const firstPriority = rolePriority[a.role] ?? 99
    const secondPriority = rolePriority[b.role] ?? 99
    if (firstPriority !== secondPriority) return firstPriority - secondPriority
    return a.sort_order - b.sort_order
  }), [dedupedPersons])

  const handleCardClick = useCallback((person: MediaPerson) => {
    if (person.person_id) navigate(`/person/${person.person_id}`)
  }, [navigate])

  if (dedupedPersons.length === 0) {
    return (
      <EmptyState
        className="nv-detail-tab-empty-state"
        icon={<Users size={23} aria-hidden="true" />}
        title="暂无演职人员信息"
        description="当前媒体还没有可展示的导演、演员或编剧信息。"
      />
    )
  }

  return (
    <section aria-labelledby="cast-grid-title">
      <div className="mb-3 flex items-baseline gap-2">
        <h2 id="cast-grid-title" className="nv-section-title">{t('castGrid.title')}</h2>
        <span className="text-[11px] text-[var(--nv-text-tertiary)]">{dedupedPersons.length}</span>
      </div>

      <div
        className="flex flex-wrap items-start gap-x-3 gap-y-5 pb-2"
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

function CastCard({ mediaPerson, onClick }: { mediaPerson: MediaPerson; onClick: (person: MediaPerson) => void }) {
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
      className="group w-[84px] flex-shrink-0 text-left sm:w-[96px]"
      role="listitem"
      aria-label={`${person?.name || t('castGrid.unknown')} · ${roleLabel}`}
    >
      <div className="relative aspect-[4/5] w-full overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-poster)] shadow-[0_5px_16px_rgba(0,0,0,.12)] transition-[transform,box-shadow,border-color] duration-200 group-hover:-translate-y-[3px] group-hover:border-[var(--nv-border-default)] group-hover:shadow-[var(--nv-shadow-card-hover)]">
        {profileSrc && !imgError ? (
          <img src={profileSrc} alt={person?.name || ''} className="h-full w-full object-cover transition-[filter] duration-200 group-hover:brightness-[.88]" loading="lazy" onError={() => setImgError(true)} />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-[var(--nv-text-tertiary)]">
            <User size={28} strokeWidth={1.4} aria-hidden="true" />
          </div>
        )}
        {mediaPerson.role && mediaPerson.role !== 'actor' && (
          <Tag tone="quality" className="absolute left-1.5 top-1.5 max-w-[calc(100%-12px)] truncate">{roleLabel}</Tag>
        )}
      </div>

      <p className="mt-1.5 truncate text-xs font-medium text-[var(--nv-text-primary)]">{person?.name || t('castGrid.unknown')}</p>
      <p className="mt-0.5 truncate text-[10px] text-[var(--nv-text-tertiary)]" title={mediaPerson.character || roleLabel}>
        {mediaPerson.character ? t('castGrid.asRole', { character: mediaPerson.character }) : roleLabel}
      </p>
    </button>
  )
}
