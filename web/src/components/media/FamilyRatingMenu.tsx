import { useEffect, useLayoutEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { adminApi } from '@/api'
import { FAMILY_RATING_LEVELS } from '@/utils/familyRating'
import FamilyRatingEditor from './FamilyRatingEditor'

/** 读取已保存的家长分级：null=加载中，''=未确认，其余为档位。 */
export function useSavedFamilyRating(id: string, enabled: boolean) {
  const [level, setLevel] = useState<string | null>(null)
  useEffect(() => {
    if (!enabled) return
    setLevel(null)
    let cancelled = false
    void adminApi.getContentRating(id).then((response) => {
      if (!cancelled) setLevel(response.data.data.level || '')
    }).catch(() => { if (!cancelled) setLevel('') })
    return () => { cancelled = true }
  }, [id, enabled])
  return [level, setLevel] as const
}

export function familyRatingLabel(level: string | null) {
  if (level === null) return '家长分级'
  return (FAMILY_RATING_LEVELS as string[]).includes(level) ? `家长分级：${level}` : '家长分级：未确认'
}

/**
 * 家长分级弹层（仅管理员）：由「···」菜单里的“家长分级”条目打开，
 * 贴着「···」按钮弹出，内部沿用 FamilyRatingEditor 的分级建议与保存流程。
 */
export default function FamilyRatingPopover({ id, mpaa, tmdbRatingData, open, anchorRef, onClose, onConfirmedChange }: {
  id: string; mpaa?: string; tmdbRatingData?: string; open: boolean
  anchorRef: React.RefObject<HTMLElement | null>; onClose: () => void; onConfirmedChange: (level: string) => void
}) {
  return (
    <RatingPopover open={open} anchorRef={anchorRef} onClose={onClose}>
      <FamilyRatingEditor id={id} mpaa={mpaa} tmdbRatingData={tmdbRatingData} compact onConfirmedChange={onConfirmedChange} />
    </RatingPopover>
  )
}

/** 表单型弹层：固定宽度、不裁切、可点击外部 / Esc 关闭（不复用会在 Tab 时关闭的菜单容器）。 */
function RatingPopover({ open, anchorRef, onClose, children }: { open: boolean; anchorRef: React.RefObject<HTMLElement | null>; onClose: () => void; children: React.ReactNode }) {
  const [pos, setPos] = useState<{ top: number; left: number } | null>(null)
  const width = Math.min(352, window.innerWidth - 24)

  useLayoutEffect(() => {
    if (!open) { setPos(null); return }
    const update = () => {
      const rect = anchorRef.current?.getBoundingClientRect()
      if (!rect) return
      const left = Math.max(12, Math.min(rect.right - width, window.innerWidth - width - 12))
      setPos({ top: rect.bottom + 8, left })
    }
    update()
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') { onClose(); anchorRef.current?.focus() } }
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
      window.removeEventListener('keydown', onKey)
    }
  }, [open, anchorRef, onClose, width])

  if (!open || !pos) return null
  return createPortal(
    <>
      <div className="fixed inset-0 z-[var(--nv-z-dropdown)]" onClick={onClose} aria-hidden="true" />
      <div
        role="dialog"
        aria-label="家长确认分级"
        className="fixed z-[var(--nv-z-dropdown)] rounded-xl border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated,var(--nv-bg-surface,#fff))] shadow-xl"
        style={{ top: pos.top, left: pos.left, width }}
      >
        {children}
      </div>
    </>,
    document.body,
  )
}
