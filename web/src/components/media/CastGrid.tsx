import { useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import { useNavigate } from 'react-router-dom'
import { streamApi } from '@/api'
import type { MediaPerson } from '@/types'
import { Button, EmptyState, Tag } from '@/components/design-system'
import { PersonCard } from '@/ui'
import { ChevronRight, ChevronUp, Users } from 'lucide-react'
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

function getCollapsedCount(width: number) {
  if (width >= 1120) return 10
  if (width >= 920) return 8
  if (width >= 720) return 7
  if (width >= 540) return 5
  return 4
}

export default function CastGrid({ persons, initialCount }: CastGridProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const getRoleLabel = useRoleLabel()
  const sectionRef = useRef<HTMLElement | null>(null)
  const [expanded, setExpanded] = useState(false)
  const [collapsedCount, setCollapsedCount] = useState(initialCount || 10)

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

  useEffect(() => {
    const node = sectionRef.current
    if (!node || initialCount || dedupedPersons.length === 0) return

    const updateCount = (width: number) => {
      const nextCount = getCollapsedCount(width)
      setCollapsedCount((current) => current === nextCount ? current : nextCount)
    }

    updateCount(node.getBoundingClientRect().width)
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width
      if (typeof width === 'number') updateCount(width)
    })
    observer.observe(node)
    return () => observer.disconnect()
  }, [dedupedPersons.length, initialCount])

  useEffect(() => {
    setExpanded(false)
  }, [persons])

  const visiblePersons = expanded ? sortedPersons : sortedPersons.slice(0, collapsedCount)
  const hasMore = sortedPersons.length > collapsedCount

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
    <section ref={sectionRef} className="nv-cast-grid-section" aria-labelledby="cast-grid-title">
      <div className="nv-cast-grid-header mb-3 flex items-center justify-between gap-3">
        <div className="flex items-baseline gap-2">
          <h2 id="cast-grid-title" className="nv-section-title">{t('castGrid.title')}</h2>
          <span className="text-[11px] text-[var(--nv-text-tertiary)]">{dedupedPersons.length}</span>
        </div>

        {hasMore && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="nv-detail-inline-more"
            aria-expanded={expanded}
            onClick={() => setExpanded((value) => !value)}
          >
            {expanded ? '收起' : '查看更多'}
            {expanded ? <ChevronUp size={14} aria-hidden="true" /> : <ChevronRight size={14} aria-hidden="true" />}
          </Button>
        )}
      </div>

      <div
        className={`nv-cast-grid-list ${expanded ? 'is-expanded' : 'is-collapsed'}`}
        style={{ '--nv-cast-preview-count': collapsedCount } as CSSProperties}
        role="list"
        aria-label={t('castGrid.title')}
      >
        {visiblePersons.map((mediaPerson) => {
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
              badge={mediaPerson.role && mediaPerson.role !== 'actor' ? <Tag tone="quality">{roleLabel}</Tag> : undefined}
              onClick={() => handleCardClick(mediaPerson)}
              ariaLabel={`${person?.name || t('castGrid.unknown')} · ${roleLabel}`}
            />
          )
        })}
      </div>
    </section>
  )
}
