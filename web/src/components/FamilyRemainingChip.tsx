import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { Clock } from 'lucide-react'
import { userApi, type FamilyViewingStatus } from '@/api/user'
import { useTranslation } from '@/i18n'
import { isLowRemaining, remainingMinutesFromSeconds } from '@/utils/familyRating'

/**
 * 首页儿童模式顶部的常驻剩余时间 chip。
 * 数据来自 familyControls（与 ChildViewingSummary 同源），30 秒刷新一次；
 * 点 chip 可以去“我的”页看完整进度。读不到时不占位，避免首页闪烁。
 */
export default function FamilyRemainingChip() {
  const { t } = useTranslation()
  const [status, setStatus] = useState<FamilyViewingStatus | null>(null)
  const [failed, setFailed] = useState(false)
  const request = useRef(0)
  const load = useCallback(() => {
    const generation = ++request.current
    void userApi.familyControls().then(({ data }) => {
      if (generation === request.current) { setStatus(data.data); setFailed(false) }
    }).catch(() => { if (generation === request.current) setFailed(true) })
  }, [])
  useEffect(() => {
    load()
    const focus = () => { if (document.visibilityState === 'visible') load() }
    const timer = window.setInterval(load, 30_000)
    window.addEventListener('focus', focus)
    return () => { request.current++; window.clearInterval(timer); window.removeEventListener('focus', focus) }
  }, [load])

  if (failed || !status || !status.child_mode) return null
  const limitSeconds = status.daily_limit_seconds
  if (limitSeconds <= 0) {
    return (
      <Link to="/my" data-testid="family-remaining-chip" className="inline-flex min-h-11 items-center gap-2 rounded-full border border-[var(--nv-border-subtle)] bg-[var(--nv-fill-hover)] px-3 py-1.5 text-sm text-[var(--nv-text-secondary)]">
        <Clock size={15} aria-hidden="true" />
        <span>{t('family.noDailyLimit')}</span>
      </Link>
    )
  }
  const minutes = remainingMinutesFromSeconds(status.remaining_seconds) ?? 0
  const usedPercent = Math.min(100, Math.round((status.used_seconds / limitSeconds) * 100))
  const low = isLowRemaining(status.remaining_seconds)
  return (
    <Link
      to="/my"
      data-testid="family-remaining-chip"
      aria-label={t('family.remainingToday', { minutes })}
      className="inline-flex min-h-11 items-center gap-2 rounded-full border px-3 py-1.5 text-sm"
      style={low
        ? { borderColor: 'var(--nv-status-warning)', color: 'var(--nv-status-warning)', background: 'var(--nv-fill-hover)' }
        : { borderColor: 'var(--nv-border-subtle)', color: 'var(--nv-text-secondary)', background: 'var(--nv-fill-hover)' }}
    >
      <Clock size={15} aria-hidden="true" />
      <span>{t('family.remainingToday', { minutes })}</span>
      <span
        role="progressbar"
        aria-label={t('family.todayProgress')}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={usedPercent}
        className="inline-block h-1.5 w-16 overflow-hidden rounded-full bg-[var(--nv-border-subtle)]"
      >
        <span className="block h-full rounded-full bg-[var(--nv-action-primary)]" style={{ width: `${usedPercent}%` }} />
      </span>
      {low && <span>{t('family.remainingLow')}</span>}
    </Link>
  )
}
