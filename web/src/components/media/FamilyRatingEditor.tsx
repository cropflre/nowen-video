import { useEffect, useRef, useState } from 'react'
import { adminApi } from '@/api'
import { Button, Select } from '@/components/design-system'
import { useToast } from '@/components/Toast'

export default function FamilyRatingEditor({ id, series = false }: { id: string; series?: boolean }) {
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
  const save = async () => {
    if (pending.current || !loaded || level === 'choose') return
    pending.current = true
    setBusy(true)
    try { await adminApi.setFamilyRating(id, level, series); toast.success(series ? '当前所有集的分级已更新，新加入的集仍需确认' : '家庭分级已更新') }
    catch { toast.error('分级未能保存，请重试') }
    finally { pending.current = false; setBusy(false) }
  }
  return (
    <section className="space-y-3 rounded-lg border border-[var(--nv-border-default)] p-4" aria-label="家庭内容分级">
      <h2 className="text-base font-semibold">家长确认分级</h2>
      <p className="text-sm text-[var(--nv-text-tertiary)]">{series ? '确认后应用于当前所有集。请先检查内容；新增未确认的集会暂时让整部剧退出儿童空间。' : '儿童空间只展示家长已确认的内容，未确认的影片不会开放。'}</p>
      <div className="flex flex-wrap gap-3">
        <Select aria-label="家庭内容分级" value={level} disabled={busy || !loaded} onChange={(event) => setLevel(event.target.value)} className="!w-auto">
          <option value="choose" disabled>请选择分级</option><option value="">未确认 / 撤回确认</option><option value="G">G · 所有年龄</option><option value="PG">PG · 家长指导</option><option value="PG-13">PG-13 · 13 岁以上</option><option value="R">R · 限制级</option><option value="NC-17">NC-17 · 成人</option>
        </Select>
        <Button disabled={busy || !loaded || level === 'choose'} onClick={() => void save()} variant="secondary">{busy ? '保存中…' : series ? '应用于当前所有集' : '保存分级'}</Button>
      </div>
    </section>
  )
}
