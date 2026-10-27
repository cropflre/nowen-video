import { useEffect, useRef, useState } from 'react'
import { useAuthStore } from '@/stores/auth'
import { userApi } from '@/api'
import type { ViewingPreferences as Preferences } from '@/types'
import { Section, Select } from '@/components/design-system'
import { useToast } from '@/components/Toast'

export function ViewingPreferenceBridge() {
  const userId = useAuthStore((state) => state.user?.id)
  const largeText = useAuthStore((state) => state.user?.viewing_preferences?.large_text ?? false)
  useEffect(() => {
    document.documentElement.dataset.largeText = largeText ? 'true' : 'false'
  }, [largeText])
  useEffect(() => {
    if (!userId) return
    let cancelled = false
    const initialUser = useAuthStore.getState().user
    void userApi.profile().then(({ data }) => {
      const store = useAuthStore.getState()
      if (!cancelled && data.data?.id === userId && store.user === initialUser) store.updateUser(data.data)
    }).catch(() => {})
    return () => { cancelled = true }
  }, [userId])
  return null
}

export default function ViewingPreferences() {
  const user = useAuthStore((state) => state.user)
  const toast = useToast()
  const [saving, setSaving] = useState(false)
  const savingRef = useRef(false)
  const preferences: Preferences = {
    large_text: user?.viewing_preferences?.large_text ?? false,
    subtitle_language: user?.viewing_preferences?.subtitle_language || 'auto',
    auto_play_next: user?.viewing_preferences?.auto_play_next ?? true,
  }
  const save = async (patch: Partial<Preferences>) => {
    if (!user || savingRef.current) return
    const previous = useAuthStore.getState().user
    if (previous?.id !== user.id) return
    const optimistic = { ...previous, viewing_preferences: { ...preferences, ...patch } }
    savingRef.current = true
    setSaving(true)
    useAuthStore.getState().updateUser(optimistic)
    try {
      const response = await userApi.updateProfile({ viewing_preferences: { ...preferences, ...patch } })
      if (response.data.data?.id !== user.id) throw new Error('Missing updated profile')
      if (useAuthStore.getState().user?.id === user.id) {
        useAuthStore.getState().updateUser(response.data.data)
        toast.success('观看偏好已保存')
      }
    } catch {
      if (useAuthStore.getState().user === optimistic) useAuthStore.getState().updateUser(previous)
      toast.error('偏好未能保存，请重试')
    }
    finally { savingRef.current = false; setSaving(false) }
  }
  return (
    <Section title="观看偏好" description="跟随账号保存，下次登录也会记住。字幕和连播偏好用于网页播放器。">
      <div className="nv-viewing-preferences space-y-4" aria-busy={saving}>
        <label className="flex min-h-12 cursor-pointer items-center justify-between gap-4">
          <span><strong className="block">大字模式</strong><span className="block text-sm text-[var(--nv-text-tertiary)]">文字更大，按钮更容易点中</span></span>
          <input type="checkbox" role="switch" aria-label="大字模式" checked={preferences.large_text} disabled={saving} onChange={(event) => void save({ large_text: event.target.checked })} className="h-6 w-6 shrink-0" />
        </label>
        <label className="flex min-h-12 cursor-pointer items-center justify-between gap-4">
          <span><strong className="block">自动播放下一集</strong><span className="block text-sm text-[var(--nv-text-tertiary)]">关闭后，看完一集就停下</span></span>
          <input type="checkbox" role="switch" aria-label="自动播放下一集" checked={preferences.auto_play_next} disabled={saving} onChange={(event) => void save({ auto_play_next: event.target.checked })} className="h-6 w-6 shrink-0" />
        </label>
        <label className="flex flex-wrap items-center justify-between gap-3">
          <span><strong className="block">默认字幕</strong><span className="block text-sm text-[var(--nv-text-tertiary)]">优先选择指定语言；没有匹配字幕时保持关闭</span></span>
          <Select aria-label="默认字幕" className="!w-auto min-w-32" value={preferences.subtitle_language} disabled={saving} onChange={(event) => void save({ subtitle_language: event.target.value as Preferences['subtitle_language'] })}>
            <option value="auto">自动选择</option><option value="zh">中文</option><option value="en">英语</option><option value="ja">日语</option><option value="off">关闭字幕</option>
          </Select>
        </label>
      </div>
    </Section>
  )
}
