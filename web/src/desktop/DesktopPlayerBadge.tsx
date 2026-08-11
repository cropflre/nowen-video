/**
 * DesktopPlayerBadge
 *
 * 桌面端播放内核提示。仅负责内核状态与切换入口，不影响播放决策。
 */

import { useState } from 'react'
import { Monitor, Sparkles, Zap } from 'lucide-react'
import { usePlayerEngine, useDesktop, MediaProfile, PlayOptions } from '@/desktop'

interface Props {
  profile?: MediaProfile
  streamUrl: string
  playOptions?: PlayOptions
  hidden?: boolean
  onSwitched?: (engine: 'mpv' | 'web') => void
}

export default function DesktopPlayerBadge({ profile, streamUrl, playOptions, hidden, onSwitched }: Props) {
  const { engine, reason, confidence, isDesktop, playInMpv, loading } = usePlayerEngine(profile)
  const { embedAvailable } = useDesktop()
  const [launching, setLaunching] = useState(false)
  const [opened, setOpened] = useState(false)

  if (hidden || !isDesktop || loading || embedAvailable) return null

  const handleLaunchMpv = async () => {
    if (!streamUrl) return
    setLaunching(true)
    try {
      const ok = await playInMpv(streamUrl, playOptions)
      if (ok) {
        setOpened(true)
        onSwitched?.('mpv')
      }
    } finally {
      setLaunching(false)
    }
  }

  if (engine === 'mpv') {
    const strict = confidence === 'strict'
    return (
      <div
        className="flex items-center gap-3 rounded-[var(--nv-player-radius-panel)] border bg-[var(--nv-player-surface-soft)] px-4 py-2.5 shadow-[var(--nv-shadow-card)] backdrop-blur-md"
        style={{ borderColor: strict ? 'var(--nv-player-warning)' : 'var(--nv-player-accent-border)' }}
      >
        <Sparkles className={strict ? 'h-5 w-5 shrink-0 text-[var(--nv-player-warning)]' : 'h-5 w-5 shrink-0 text-[var(--nv-player-accent)]'} aria-hidden="true" />
        <div className="min-w-0 flex-1">
          <div className="text-sm font-semibold text-[var(--nv-player-text-primary)]">
            {strict ? '需要原生内核播放' : '推荐使用原生内核'}
          </div>
          <div className="truncate text-xs text-[var(--nv-player-text-tertiary)]">{reason}</div>
        </div>
        <button
          type="button"
          onClick={handleLaunchMpv}
          disabled={launching || opened}
          className="flex shrink-0 items-center gap-1.5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] px-3 py-1.5 text-xs font-medium text-[var(--nv-player-accent)] transition-colors hover:bg-[var(--nv-player-accent-soft-hover)] disabled:opacity-50"
        >
          {opened ? <><Monitor className="h-3.5 w-3.5" aria-hidden="true" />已在 mpv 播放</> : launching ? '启动中...' : <><Zap className="h-3.5 w-3.5" aria-hidden="true" />用 mpv 打开</>}
        </button>
      </div>
    )
  }

  return (
    <div className="inline-flex items-center gap-1.5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-subtle)] px-2 py-1 text-xs text-[var(--nv-player-text-tertiary)]">
      <Monitor className="h-3 w-3" aria-hidden="true" />
      Web 内核播放
    </div>
  )
}
