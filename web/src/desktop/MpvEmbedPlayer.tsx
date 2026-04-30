
/**
 * MpvEmbedPlayer —— 在 Tauri 桌面端内嵌渲染 mpv
 *
 * 原理：
 * - 组件挂载时调用 Rust 创建无边框 mpv 子窗口
 * - 通过 ResizeObserver 同步占位 div 的位置/大小给子窗口
 * - 组件销毁时回收子窗口
 *
 * 注意：
 * - 仅在 Tauri 桌面端生效；浏览器环境返回 null
 * - 要求编译时启用 embed-mpv feature，否则仍可用（但退化为独立窗口）
 *
 * 用法：
 * ```tsx
 * <MpvEmbedPlayer
 *   streamUrl={url}
 *   title="电影标题"
 *   onReady={() => console.log('mpv ready')}
 * />
 * ```
 */

import { useEffect, useRef, useState } from 'react'
import { desktop, PlayOptions } from './bridge'

interface Props {
  /** 播放 URL */
  streamUrl: string
  /** 会话 ID，多实例时区分 */
  sessionId?: string
  /** 播放选项 */
  playOptions?: PlayOptions
  /** 自动销毁时机：组件卸载 / 手动 */
  autoDestroy?: boolean
  /** 容器 className（不影响功能） */
  className?: string
  onReady?: () => void
  onError?: (e: string) => void
}

export default function MpvEmbedPlayer({
  streamUrl,
  sessionId = 'main-player',
  playOptions,
  autoDestroy = true,
  className = '',
  onReady,
  onError,
}: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // 启动嵌入播放
  useEffect(() => {
    if (!desktop.isDesktop || !streamUrl) return

    let canceled = false
    ;(async () => {
      try {
        const r = await desktop.mpvEmbedStart({
          sessionId,
          url: streamUrl,
          options: playOptions,
        })
        if (canceled) return
        if (r) {
          setReady(true)
          onReady?.()
        } else {
          const msg = '无法启动嵌入式 mpv（可能未启用 embed-mpv feature）'
          setError(msg)
          onError?.(msg)
        }
      } catch (e: any) {
        if (canceled) return
        const msg = e?.message || String(e)
        setError(msg)
        onError?.(msg)
      }
    })()

    return () => {
      canceled = true
      if (autoDestroy) {
        desktop.mpvEmbedDestroy().catch(() => {})
      }
    }
  }, [streamUrl, sessionId])

  // 同步占位元素的位置/大小给子窗口
  useEffect(() => {
    if (!ready || !ref.current || !desktop.isDesktop) return

    const el = ref.current
    let rafId = 0

    const sync = () => {
      const rect = el.getBoundingClientRect()
      const dpr = window.devicePixelRatio || 1
      desktop
        .mpvEmbedSync({
          x: Math.round(rect.left * dpr),
          y: Math.round(rect.top * dpr),
          width: Math.max(1, Math.round(rect.width * dpr)),
          height: Math.max(1, Math.round(rect.height * dpr)),
          visible: rect.width > 0 && rect.height > 0,
        })
        .catch(() => {})
    }

    const scheduleSync = () => {
      cancelAnimationFrame(rafId)
      rafId = requestAnimationFrame(sync)
    }

    const ro = new ResizeObserver(scheduleSync)
    ro.observe(el)

    window.addEventListener('resize', scheduleSync)
    window.addEventListener('scroll', scheduleSync, true)

    // 首次立即同步
    sync()

    return () => {
      cancelAnimationFrame(rafId)
      ro.disconnect()
      window.removeEventListener('resize', scheduleSync)
      window.removeEventListener('scroll', scheduleSync, true)
      // 隐藏子窗口
      desktop.mpvEmbedSync({ x: 0, y: 0, width: 1, height: 1, visible: false }).catch(() => {})
    }
  }, [ready])

  if (!desktop.isDesktop) {
    return (
      <div className={`flex items-center justify-center bg-black text-white/60 ${className}`}>
        <span className="text-sm">嵌入式 mpv 仅在桌面端可用</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className={`flex items-center justify-center bg-black text-red-300 ${className}`}>
        <span className="text-sm">mpv 启动失败: {error}</span>
      </div>
    )
  }

  return (
    <div
      ref={ref}
      className={`relative bg-black ${className}`}
      data-mpv-embed-placeholder
    >
      {!ready && (
        <div className="absolute inset-0 flex items-center justify-center text-white/60 text-sm">
          正在加载 mpv 内核...
        </div>
      )}
    </div>
  )
}

/** 供外部控制播放的工具方法 */
export const mpvControl = {
  togglePause(sessionId = 'main-player') {
    return desktop.mpvEmbedCommand({ sessionId, command: 'cycle', args: ['pause'] })
  },
  seek(sessionId: string, seconds: number, absolute = false) {
    return desktop.mpvEmbedCommand({
      sessionId,
      command: 'seek',
      args: [String(seconds), absolute ? 'absolute' : 'relative'],
    })
  },
  setVolume(sessionId: string, volume: number) {
    return desktop.mpvEmbedSetProperty({ sessionId, name: 'volume', value: String(volume) })
  },
  setMute(sessionId: string, mute: boolean) {
    return desktop.mpvEmbedSetProperty({
      sessionId,
      name: 'mute',
      value: mute ? 'yes' : 'no',
    })
  },
  setSubtitle(sessionId: string, sid: number | 'no') {
    return desktop.mpvEmbedSetProperty({ sessionId, name: 'sid', value: String(sid) })
  },
  setAudioTrack(sessionId: string, aid: number | 'no') {
    return desktop.mpvEmbedSetProperty({ sessionId, name: 'aid', value: String(aid) })
  },
}
