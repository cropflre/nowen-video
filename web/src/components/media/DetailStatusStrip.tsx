import { useTranslation } from '@/i18n'
import type { Media, WatchHistory } from '@/types'
import { formatTime, getDisplayDurationSeconds } from '@/utils/format'

interface DetailStatusStripProps {
  media: Media
  watchProgress: WatchHistory | null
  isAdmin?: boolean
}

/**
 * 详情页播放进度条（有观看记录时显示）。
 * 家长分级已移到顶部操作栏（FamilyRatingMenu），不再占用正文空间。
 */
export default function DetailStatusStrip({ media, watchProgress }: DetailStatusStripProps) {
  const { t } = useTranslation()
  const displaySeconds = getDisplayDurationSeconds(media.duration, media.runtime)
  const totalSeconds = watchProgress?.duration || displaySeconds
  const hasProgress = !!watchProgress && totalSeconds > 0
  const percent = hasProgress
    ? Math.min(100, Math.max(0, Math.round((watchProgress.position / totalSeconds) * 100)))
    : 0

  if (!hasProgress) return null

  return (
    <div className="mx-auto w-full max-w-[var(--nv-content-max)] px-[var(--nv-page-gutter)] pt-5">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg border border-[var(--nv-border-default)] px-4 py-3">
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
      </div>
    </div>
  )
}
