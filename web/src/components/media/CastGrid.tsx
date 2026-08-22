import { useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { streamApi } from '@/api'
import type { MediaPerson } from '@/types'
import { EmptyState } from '@/components/design-system'
import { PersonCard } from '@/ui'
import { Users } from 'lucide-react'
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
  const getRoleLabel = useRoleLabel()

  const sortedPersons = useMemo(() => {
    const seen = new Set<string>()
    return persons
      .filter((mediaPerson) => {
        const key = `${mediaPerson.person_id}:${mediaPerson.role}`
        if (seen.has(key)) return false
        seen.add(key)
        return true
      })
      .sort((a, b) => {
        const firstPriority = rolePriority[a.role] ?? 99
        const secondPriority = rolePriority[b.role] ?? 99
        if (firstPriority !== secondPriority) return firstPriority - secondPriority
        return a.sort_order - b.sort_order
      })
  }, [persons])

  const handleCardClick = useCallback((person: MediaPerson) => {
    if (person.person_id) navigate(`/person/${person.person_id}`)
  }, [navigate])

  if (sortedPersons.length === 0) {
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
    <section className="nv-cast-grid-section" aria-labelledby="cast-grid-title">
      <div className="nv-cast-grid-header">
        <h2 id="cast-grid-title" className="nv-section-title inline-flex items-center gap-2">
          <Users size={17} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
          {t('castGrid.title')}
        </h2>
        <span>{sortedPersons.length}</span>
      </div>

      <div className="nv-cast-rail" role="list" aria-label={t('castGrid.title')}>
        {sortedPersons.map((mediaPerson) => {
          const person = mediaPerson.person
          const roleLabel = getRoleLabel(mediaPerson.role)
          const subtitle = mediaPerson.character
            ? t('castGrid.asRole', { character: mediaPerson.character })
            : roleLabel

          return (
            <PersonCard
              key={mediaPerson.id}
              name={person?.name || t('castGrid.unknown')}
              subtitle={subtitle}
              imageSrc={person?.id ? streamApi.getPersonProfileUrl(person.id) : null}
              onClick={() => handleCardClick(mediaPerson)}
              ariaLabel={`${person?.name || t('castGrid.unknown')} · ${roleLabel}`}
            />
          )
        })}
      </div>
    </section>
  )
}
