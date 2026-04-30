
/**
 * DesktopPlayerBadge
 *
 * 在播放页显示的"播放内核徽章"：
 * - 运行在浏览器：不显示
 * - 运行在桌面端 + 推荐 mpv：显示提示 + "用原画内核打开"按钮
 * - 运行在桌面端 + web 播放足够：显示小标识
 *
 * 这个组件为可选接入，不影响现有 VideoPlayer。
 */

import { useState } from 'react'
import { Monitor, Sparkles, Zap } from 'lucide-react'
import { usePlayerEngine, MediaProfile, PlayOptions } from '@/desktop'

interface Props {
  /** 媒体特征（可选，缺省时按文件名推断） */
  profile?: MediaProfile
  /** 播放 URL */
  streamUrl: string
  /** mpv 播放参数 */
  playOptions?: PlayOptions
  /** 强制隐藏 */
  hidden?: boolean
  /** 切换成功回调 */
  onSwitched?: (engine: 'mpv' | 'web') => void
}

export default function DesktopPlayerBadge({
  profile,
  streamUrl,
  playOptions,
  hidden,
  onSwitched,
}: Props) {
  const { engine, reason, confidence, isDesktop, playInMpv, loading } =
    usePlayerEngine(profile)
  const [launching, setLaunching] = useState(false)
  const [opened, setOpened] = useState(false)

  if (hidden || !isDesktop) return null
  if (loading) return null

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

  // mpv 推荐或强制场景
  if (engine === 'mpv') {
    const color =
      confidence === 'strict'
        ? 'from-red-500/20 to-orange-500/20 border-orange-400/40 text-orange-200'
        : 'from-blue-500/20 to-purple-500/20 border-blue-400/40 text-blue-200'

    return (
      <div
        className={`flex items-center gap-3 px-4 py-2.5 rounded-xl bg-gradient-to-r ${color} border backdrop-blur-md shadow-lg`}
      >
        <Sparkles className="w-5 h-5 shrink-0" />
        <div className="flex-1 min-w-0">
          <div className="text-sm font-semibold">
            {confidence === 'strict' ? '需要原生内核播放' : '推荐使用原生内核'}
          </div>
          <div className="text-xs opacity-80 truncate">{reason}</div>
        </div>
        <button
          onClick={handleLaunchMpv}
          disabled={launching || opened}
          className="shrink-0 px-3 py-1.5 rounded-lg bg-white/20 hover:bg-white/30 disabled:opacity-50 transition text-xs font-medium flex items-center gap-1.5"
        >
          {opened ? (
            <>
              <Monitor className="w-3.5 h-3.5" />
              已在 mpv 播放
            </>
          ) : launching ? (
            '启动中...'
          ) : (
            <>
              <Zap className="w-3.5 h-3.5" />
              用 mpv 打开
            </>
          )}
        </button>
      </div>
    )
  }

  // web 播放足够的场景 —— 只在 confidence=fallback 时显示小徽章
  return (
    <div className="inline-flex items-center gap-1.5 px-2 py-1 rounded-md bg-white/5 border border-white/10 text-xs text-white/60">
      <Monitor className="w-3 h-3" />
      Web 内核播放
    </div>
  )
}
