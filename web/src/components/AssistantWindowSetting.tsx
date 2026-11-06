import { MessageCircle, RotateCcw } from 'lucide-react'
import { Button, Section } from '@/components/design-system'
import { useAssistantWindowStore } from '@/stores/assistantWindow'

const stateLabel = { expanded: '已展开', minimized: '已最小化', hidden: '已隐藏' } as const
const stateHint = {
  expanded: '悬浮窗正显示在页面上，可以拖动标题栏移动。',
  minimized: '已最小化为右下角的问问助手按钮，点一下即可展开。',
  hidden: '页面上的悬浮窗和入口都已隐藏，聊天记录不会保留到下次刷新。',
} as const

/**
 * Discoverable restore path: a hidden assistant must never be a dead end.
 */
export default function AssistantWindowSetting() {
  const visibility = useAssistantWindowStore(state => state.preferences.visibility)
  const setVisibility = useAssistantWindowStore(state => state.setVisibility)
  const resetWindow = useAssistantWindowStore(state => state.resetWindow)

  return (
    <Section title="问问助手" description="悬浮窗会记住位置、大小和显示状态，只保存在这台设备上。">
      <div className="flex flex-wrap items-center justify-between gap-3 border-y border-[var(--nv-border-subtle)] py-3">
        <div className="min-w-0">
          <p className="text-sm font-medium text-[var(--nv-text-primary)]">悬浮窗状态 · {stateLabel[visibility]}</p>
          <p className="mt-0.5 text-[11px] leading-5 text-[var(--nv-text-tertiary)]">{stateHint[visibility]}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {visibility === 'hidden'
            ? <Button type="button" variant="primary" onClick={() => setVisibility('expanded')}><MessageCircle size={15} aria-hidden="true" />重新开启问问助手</Button>
            : <Button type="button" onClick={() => setVisibility('hidden')}>隐藏问问助手</Button>}
          <Button type="button" variant="ghost" onClick={resetWindow} disabled={visibility === 'hidden'}><RotateCcw size={15} aria-hidden="true" />重置位置和大小</Button>
        </div>
      </div>
    </Section>
  )
}
