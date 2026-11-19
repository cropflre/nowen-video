import { useEffect, useRef, useState } from 'react'
import { adminApi } from '@/api'
import { Button, Select } from '@/components/design-system'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import {
  FAMILY_RATING_AGE_KEY,
  FAMILY_RATING_DESC_KEY,
  FAMILY_RATING_LEVELS,
  type FamilyRatingLevel,
} from '@/utils/familyRating'

/**
 * 家长确认分级。展示层用中文年龄档（全年龄 / 7+ / 13+ / 16+ / 成人），
 * 提交给后端的值仍是原枚举 G / PG / PG-13 / R / NC-17（'' 表示未确认），
 * 映射关系见 utils/familyRating.ts，后端枚举不改。
 */
export default function FamilyRatingEditor({ id, series = false }: { id: string; series?: boolean }) {
  const { t } = useTranslation()
  const [level, setLevel] = useState(series ? 'choose' : '')
  const [busy, setBusy] = useState(false)
  const [loaded, setLoaded] = useState(series)
  const pending = useRef(false)
  const toast = useToast()
  useEffect(() => {
    setLevel(series ? 'choose' : '')
    setLoaded(series)
    if (series) return
    let cancelled = false
    void adminApi.getContentRating(id).then((response) => {
      if (!cancelled) { setLevel(response.data.data.level || ''); setLoaded(true) }
    }).catch(() => { if (!cancelled) toast.error('暂时无法读取家庭分级，请重新打开页面后重试') })
    return () => { cancelled = true }
  }, [id, series, toast])
  const save = async (override?: string) => {
    const next = override ?? level
    if (pending.current || !loaded || next === 'choose') return
    pending.current = true
    setBusy(true)
    try {
      await adminApi.setFamilyRating(id, next, series)
      setLevel(next)
      toast.success(series ? '当前所有集的分级已更新，新加入的集仍需确认' : '家庭分级已更新')
    } catch { toast.error('分级未能保存，请重试') }
    finally { pending.current = false; setBusy(false) }
  }
  // 一键确认：未确认时直接按“全年龄”保存，详情页点开即点确认，两步完成。
  const quickConfirm = () => { void save('G') }
  const isUnconfirmed = level === '' || level === 'choose'
  const knownLevel = (FAMILY_RATING_LEVELS as string[]).includes(level) ? (level as Exclude<FamilyRatingLevel, ''>) : null
  return (
    <section
      data-testid="family-rating-card"
      className="space-y-3 rounded-lg border p-4"
      style={isUnconfirmed && loaded ? { borderColor: 'var(--nv-status-warning)' } : { borderColor: 'var(--nv-border-default)' }}
      aria-label="家庭内容分级"
    >
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-base font-semibold">家长确认分级</h2>
        {loaded && isUnconfirmed && (
          <span className="rounded-full px-2 py-0.5 text-xs" style={{ color: 'var(--nv-status-warning)', background: 'var(--nv-fill-hover)' }}>{t('family.ratingUnconfirmed')}</span>
        )}
        {loaded && knownLevel && (
          <span className="rounded-full bg-[var(--nv-fill-hover)] px-2 py-0.5 text-xs text-[var(--nv-text-secondary)]">{t('family.ratingConfirmed', { label: `${knownLevel} · ${t(FAMILY_RATING_AGE_KEY[knownLevel])}` })}</span>
        )}
      </div>
      <p className="text-sm text-[var(--nv-text-tertiary)]">{series ? '确认后应用于当前所有集。请先检查内容；新增未确认的集会暂时让整部剧退出儿童空间。' : '儿童空间只展示家长已确认的内容，未确认的影片不会开放。'}</p>
      {knownLevel && <p className="text-sm text-[var(--nv-text-secondary)]">{t(FAMILY_RATING_DESC_KEY[knownLevel])}</p>}
      <div className="flex flex-wrap gap-3">
        <Select aria-label="家庭内容分级" value={level} disabled={busy || !loaded} onChange={(event) => setLevel(event.target.value)} className="!w-auto">
          <option value="choose" disabled>请选择分级</option><option value="">未确认 / 撤回确认</option>
          {FAMILY_RATING_LEVELS.map((option) => (
            <option key={option} value={option}>{option} · {t(FAMILY_RATING_AGE_KEY[option])}</option>
          ))}
        </Select>
        <Button disabled={busy || !loaded || level === 'choose'} onClick={() => void save()} variant="secondary">{busy ? '保存中…' : series ? '应用于当前所有集' : '保存分级'}</Button>
        {loaded && isUnconfirmed && (
          <Button disabled={busy} onClick={quickConfirm} variant="primary" data-testid="family-rating-quick-confirm">{t('family.ratingQuickConfirm')}</Button>
        )}
      </div>
    </section>
  )
}
