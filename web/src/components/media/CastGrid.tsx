import { useState, useCallback, useMemo, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { streamApi } from '@/api'
import type { MediaPerson } from '@/types'
import { User, Film } from 'lucide-react'
import { useTranslation } from '@/i18n'

interface CastGridProps {
  persons: MediaPerson[]
  /** 初始展示数量，超出后折叠 */
  initialCount?: number
}

/** 获取角色类型的国际化标签 */
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

  // 去重：相同 person_id + role 只保留第一条（兜底，后端合并时已去重）
  const dedupedPersons = useMemo(() => {
    const seen = new Set<string>()
    return persons.filter((mp) => {
      const key = `${mp.person_id}:${mp.role}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
  }, [persons])

  // 按角色排序：导演 > 编剧 > 演员，同角色按 sort_order 排序
  const sortedPersons = useMemo(() => {
    return [...dedupedPersons].sort((a, b) => {
      const pa = rolePriority[a.role] ?? 99
      const pb = rolePriority[b.role] ?? 99
      if (pa !== pb) return pa - pb
      return a.sort_order - b.sort_order
    })
  }, [dedupedPersons])

  // 点击演员头像 → 跳转到独立的演员详情页
  const handleCardClick = useCallback((person: MediaPerson) => {
    if (person.person_id) {
      navigate(`/person/${person.person_id}`)
    }
  }, [navigate])

  if (dedupedPersons.length === 0) return null

  return (
    <section>
      {/* 标题 */}
      <h3
        className="mb-4 flex items-center gap-2 font-display text-base font-semibold tracking-wide"
        style={{ color: 'var(--text-primary)' }}
      >
        <Film size={16} className="text-neon/60" />
        {t('castGrid.title')}
        <span className="text-xs font-normal" style={{ color: 'var(--text-muted)' }}>
          ({dedupedPersons.length})
        </span>
      </h3>

      {/* 横向滚动布局 */}
      <div
        ref={scrollRef}
        className="flex gap-3 overflow-x-auto pb-2"
        style={{
          scrollbarWidth: 'thin',
          scrollbarColor: 'var(--border-strong) transparent',
        }}
      >
        {sortedPersons.map((mp) => (
          <CastCard key={mp.id} mediaPerson={mp} onClick={handleCardClick} />
        ))}
      </div>
    </section>
  )
}

/** 单个演员卡片 */
function CastCard({
  mediaPerson,
  onClick,
}: {
  mediaPerson: MediaPerson
  onClick: (mp: MediaPerson) => void
}) {
  const { t } = useTranslation()
  const getRoleLabel = useRoleLabel()
  const [imgError, setImgError] = useState(false)
  const person = mediaPerson.person
  // 优先使用本地 API 代理头像（解决国内无法直连 TMDb 的问题）
  const profileSrc = person?.person_id ? streamApi.getPersonProfileUrl(person.person_id) : null

  return (
    <button
      onClick={() => onClick(mediaPerson)}
      className="group flex w-24 flex-shrink-0 flex-col items-center gap-2 rounded-xl p-2 transition-all duration-300 hover:scale-[1.03] sm:w-28"
      style={{
        background: 'var(--bg-card)',
        border: '1px solid var(--border-default)',
      }}
    >
      {/* 头像 */}
      <div
        className="relative aspect-square w-full overflow-hidden rounded-lg"
        style={{ background: 'var(--bg-surface)' }}
      >
        {profileSrc && !imgError ? (
          <img
            src={profileSrc}
            alt={person?.name || ''}
            className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-110"
            loading="lazy"
            onError={() => setImgError(true)}
          />
        ) : (
          <div
            className="flex h-full w-full items-center justify-center"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue-4), var(--neon-purple-4, var(--neon-blue-8)))',
              color: 'var(--text-muted)',
            }}
          >
            <User size={32} strokeWidth={1.5} />
          </div>
        )}

        {/* 角色类型标签 */}
        {mediaPerson.role && mediaPerson.role !== 'actor' && (
          <div
            className="absolute left-1 top-1 rounded px-1.5 py-0.5 text-[9px] font-bold uppercase"
            style={{
              background: 'rgba(0, 0, 0, 0.7)',
              backdropFilter: 'blur(4px)',
              color: mediaPerson.role === 'director' ? '#FBBF24' : '#93C5FD',
            }}
          >
            {getRoleLabel(mediaPerson.role)}
          </div>
        )}
      </div>

      {/* 姓名 */}
      <div className="w-full text-center">
        <p
          className="truncate text-xs font-medium transition-colors group-hover:text-neon"
          style={{ color: 'var(--text-primary)' }}
        >
          {person?.name || t('castGrid.unknown')}
        </p>
        {/* 饰演角色 */}
        {mediaPerson.character && (
          <p
            className="mt-0.5 truncate text-[10px]"
            style={{ color: 'var(--text-muted)' }}
            title={t('castGrid.asRole', { character: mediaPerson.character })}
          >
            {t('castGrid.asRole', { character: mediaPerson.character })}
          </p>
        )}
        {/* 导演/编剧没有 character 时显示角色类型 */}
        {!mediaPerson.character && mediaPerson.role !== 'actor' && (
          <p
            className="mt-0.5 truncate text-[10px]"
            style={{ color: 'var(--text-muted)' }}
          >
            {getRoleLabel(mediaPerson.role)}
          </p>
        )}
      </div>
    </button>
  )
}

