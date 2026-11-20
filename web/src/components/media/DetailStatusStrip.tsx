import { useEffect, useState } from 'react'
import { adminApi } from '@/api'
import { Button } from '@/components/design-system'
import { useTranslation } from '@/i18n'
import type { Media, WatchHistory } from '@/types'
import {
  FAMILY_RATING_AGE_KEY,
  FAMILY_RATING_LEVELS,
  type FamilyRatingLevel,
} from '@/utils/familyRating'
import { formatTime, getDisplayDurationSeconds } from '@/utils/format'
import FamilyRatingEditor from './FamilyRatingEditor'

interface DetailStatusStripProps {
  media: Media
  watchProgress: WatchHistory | null
  isAdmin: boolean
}

/**
 * 详情页状态条：播放进度与家长确认状态放一起。
 *
 * - 播放进度对所有人显示（有观看记录时）。
 * - 家长确认只对管理员显示：未确认时展开完整编辑器；
 *   已确认时折成一行状态 chip，点“修改分级”再展开编辑器；
 *   非管理员不显示分级内容。
 * 编辑器本身（分级建议流程）一个字没动，只由这里决定展开还是收起。
 */
export default function DetailStatusStrip({ media, watchProgress, isAdmin }: DetailStatusStripProps) {
  const { t } = useTranslation()
  // null = 尚未读到已保存的分级，此时默认展开编辑器（与改动前行为一致）。
  const [savedLevel, setSavedLevel] = useState<string | null>(null)
  const [ratingLoaded, setRatingLoaded] = useState(false)
  const [expanded, setExpanded] = useState(false)

  useEffect(() => {
    if (!isAdmin) return
    setSavedLevel(null)
    setRatingLoaded(false)
    setExpanded(false)
    let cancelled = false
    void adminApi.getContentRating(media.id).then((response) => {
      if (cancelled) return
      const saved = response.data.data.level || ''
      setSavedLevel(saved)
      setExpanded(!saved)
      setRatingLoaded(true)
    }).catch(() => {
      if (cancelled) return
      setSavedLevel('')
      setExpanded(true)
      setRatingLoaded(true)
    })
    return () => { cancelled = true }
  }, [media.id, isAdmin])

  const displaySeconds = getDisplayDurationSeconds(media.duration, media.runtime)
  const totalSeconds = watchProgress?.duration || displaySeconds
  const hasProgress = !!watchProgress && totalSeconds > 0
  const percent = hasProgress
    ? Math.min(100, Math.max(0, Math.round((watchProgress.position / totalSeconds) * 100)))
    : 0
  const knownLevel = savedLevel && (FAMILY_RATING_LEVELS as string[]).includes(savedLevel)
    ? (savedLevel as Exclude<FamilyRatingLevel, ''>)
    : null
  const showChip = isAdmin && ratingLoaded && knownLevel !== null && !expanded
  const showEditor = isAdmin && (!ratingLoaded || expanded)
  const showStrip = hasProgress || showChip

  if (!showStrip && !showEditor) return null

  return (
    <div className="mx-auto w-full max-w-[var(--nv-content-max)] px-[var(--nv-page-gutter)] pt-5">
      {showStrip && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-[var(--nv-border-default)] px-4 py-3">
          {hasProgress && (
            <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-3 gap-y-2" aria-label={`已观看 ${percent}%`}>
              <span className="shrink-0 text-sm font-medium text-[var(--nv-text-primary)]">
                {t('mediaDetail.watchedPercent', { percent })}
              </span>
              <div className="h-1 min-w-24 flex-1 overflow-hidden rounded-full bg-[var(--nv-fill-hover)]" role="progressbar" aria-valuenow={percent} aria-valuemin={0} aria-valuemax={100}>
                <span className="block h-full bg-[var(--nv-action-primary)]" style={{ width: `${percent}%` }} />
              </div>
              <span className="shrink-0 text-xs text-[var(--nv-text-tertiary)]">
                {t('mediaDetail.lastWatchedAt', { position: formatTime(watchProgress.position), duration: formatTime(totalSeconds) })}
              </span>
            </div>
          )}
          {showChip && (
            <div className="flex flex-wrap items-center gap-2" data-testid="family-rating-status-chip">
              <span className="rounded-full bg-[var(--nv-fill-hover)] px-2 py-0.5 text-xs text-[var(--nv-text-secondary)]">
                {t('family.ratingConfirmed', { label: `${knownLevel} · ${t(FAMILY_RATING_AGE_KEY[knownLevel])}` })}
              </span>
              <Button type="button" variant="ghost" size="sm" onClick={() => setExpanded(true)}>
                {t('mediaDetail.editRating')}
              </Button>
            </div>
          )}
        </div>
      )}
      {showEditor && (
        <div className={showStrip ? 'mt-3' : undefined}>
          <FamilyRatingEditor id={media.id} mpaa={media.mpaa} tmdbRatingData={media.tmdb_rating_data} />
        </div>
      )}
    </div>
  )
}
