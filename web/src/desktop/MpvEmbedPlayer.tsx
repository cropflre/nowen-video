
/**
 * MpvEmbedPlayer —— Hills 风格的嵌入式 libmpv 播放器
 *
 * 原理：
 * - 组件挂载时调用 Rust 创建无边框 mpv 子窗口（原生 HWND），
 *   libmpv 直接把解码后画面渲染到这个窗口，性能 ≈ 独立 mpv。
 * - 通过 ResizeObserver 同步占位 div 的位置/大小给子窗口。
 * - 组件销毁时回收子窗口。
 * - 控制条是 Web 层叠加（Fluent UI 风格），命令通过 IPC 发送到 libmpv。
 *
 * 仅在 Tauri 桌面端生效；浏览器环境显示占位提示。
 */

import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { Pause, Play, Volume2, VolumeX, Maximize, Subtitles, Sparkles, Loader2 } from 'lucide-react'
import { desktop, PlayOptions } from './bridge'
import Anime4KPanel, { Anime4KLevel } from './Anime4KPanel'

interface Props {
  /** 播放 URL */
  streamUrl: string
  /** 会话 ID，多实例时区分 */
  sessionId?: string
  /** 标题（显示在控制条顶部） */
  title?: string
  /** 播放选项 */
  playOptions?: PlayOptions
  /** 初始音量 0-100 */
  initialVolume?: number
  /** 自动销毁时机：组件卸载 / 手动 */
  autoDestroy?: boolean
  /** 容器 className（不影响功能） */
  className?: string
  onReady?: () => void
  onError?: (e: string) => void
  /** 点击返回 */
  onBack?: () => void
}

/** 外部可通过 ref 直接调用 */
export interface MpvEmbedHandle {
  play(): Promise<void>
  pause(): Promise<void>
  togglePause(): Promise<void>
  seek(seconds: number, absolute?: boolean): Promise<void>
  setVolume(v: number): Promise<void>
  setMute(m: boolean): Promise<void>
  setSubtitle(sid: number | 'no'): Promise<void>
  setAudioTrack(aid: number | 'no'): Promise<void>
  setAnime4K(level: Anime4KLevel): Promise<void>
  loadFile(url: string): Promise<void>
}

function MpvEmbedPlayerInner(
  {
    streamUrl,
    sessionId = 'main-player',
    title,
    playOptions,
    initialVolume = 80,
    autoDestroy = true,
    className = '',
    onReady,
    onError,
    onBack,
  }: Props,
  ref: React.Ref<MpvEmbedHandle>,
) {
  const wrapperRef = useRef<HTMLDivElement>(null)
  const placeholderRef = useRef<HTMLDivElement>(null)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // UI 状态
  const [paused, setPaused] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(initialVolume)
  const [anime4kLevel, setAnime4kLevel] = useState<Anime4KLevel>('off')
  const [controlsVisible, setControlsVisible] = useState(true)
  const [showAnime4KPanel, setShowAnime4KPanel] = useState(false)
  const hideTimer = useRef<number | null>(null)

  // ========== 基础命令封装 ==========
  const cmd = useCallback(
    (command: string, args?: string[]) =>
      desktop.mpvEmbedCommand({ sessionId, command, args }),
    [sessionId],
  )
  const setProp = useCallback(
    (name: string, value: string) =>
      desktop.mpvEmbedSetProperty({ sessionId, name, value }),
    [sessionId],
  )

  // ========== 对外 API（ref 暴露） ==========
  useImperativeHandle(
    ref,
    (): MpvEmbedHandle => ({
      async play() {
        await setProp('pause', 'no')
        setPaused(false)
      },
      async pause() {
        await setProp('pause', 'yes')
        setPaused(true)
      },
      async togglePause() {
        await cmd('cycle', ['pause'])
        setPaused((p) => !p)
      },
      async seek(seconds, absolute = false) {
        await cmd('seek', [String(seconds), absolute ? 'absolute' : 'relative'])
      },
      async setVolume(v) {
        await setProp('volume', String(v))
        setVolume(v)
      },
      async setMute(m) {
        await setProp('mute', m ? 'yes' : 'no')
        setMuted(m)
      },
      async setSubtitle(sid) {
        await setProp('sid', String(sid))
      },
      async setAudioTrack(aid) {
        await setProp('aid', String(aid))
      },
      async setAnime4K(level) {
        await applyAnime4K(level)
      },
      async loadFile(url) {
        await cmd('loadfile', [url, 'replace'])
      },
    }),
    [cmd, setProp],
  )

  // ========== Anime4K 切换 ==========
  const applyAnime4K = useCallback(
    async (level: Anime4KLevel) => {
      // 三档预设（对应 resources/shaders 里的官方 shader 组合）
      const presets: Record<Anime4KLevel, string[]> = {
        off: [],
        // A 组：面向 Anime4K_Restore + Upscale，最省性能（适合 1080p->2K）
        low: [
          'Anime4K_Clamp_Highlights.glsl',
          'Anime4K_Restore_CNN_M.glsl',
          'Anime4K_Upscale_CNN_x2_M.glsl',
        ],
        // B 组：A + 额外 deblur/denoise（中等开销，适合 720p->1080p 老番）
        medium: [
          'Anime4K_Clamp_Highlights.glsl',
          'Anime4K_Restore_CNN_VL.glsl',
          'Anime4K_Upscale_CNN_x2_VL.glsl',
        ],
        // C 组：最强质量 VL + AutoDownscale（高 GPU 占用，适合 RTX 3060 以上）
        high: [
          'Anime4K_Clamp_Highlights.glsl',
          'Anime4K_Restore_CNN_UL.glsl',
          'Anime4K_Upscale_CNN_x2_UL.glsl',
          'Anime4K_AutoDownscalePre_x2.glsl',
          'Anime4K_AutoDownscalePre_x4.glsl',
          'Anime4K_Upscale_CNN_x2_M.glsl',
        ],
      }

      const list = presets[level]
      if (list.length === 0) {
        // 清空 shader 链
        await setProp('glsl-shaders', '')
      } else {
        // mpv 的 glsl-shaders 是 path 列表，分隔符 Windows 用 ; 其它 :
        // resources/shaders 会在运行时被 Tauri bundle 到 exe 同级
        const sep = ';' // Windows，若跨平台需探测
        const joined = list.map((f) => `resources/shaders/${f}`).join(sep)
        await setProp('glsl-shaders', joined)
      }
      setAnime4kLevel(level)
    },
    [setProp],
  )

  // ========== 启动嵌入播放 ==========
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
          // 应用初始音量
          await setProp('volume', String(initialVolume))
          setReady(true)
          onReady?.()
        } else {
          const msg = '无法启动嵌入式 mpv（请确认已启用 embed-mpv 且 libmpv-2.dll 存在）'
          setError(msg)
          onError?.(msg)
        }
      } catch (e: unknown) {
        if (canceled) return
        const msg = (e as Error)?.message || String(e)
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
    // 仅在关键 props 变化时重新启动
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [streamUrl, sessionId])

  // ========== 同步占位元素的位置/大小给子窗口 ==========
  useEffect(() => {
    if (!ready || !placeholderRef.current || !desktop.isDesktop) return

    const el = placeholderRef.current
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

  // ========== 控制条自动隐藏 ==========
  const resetHideTimer = useCallback(() => {
    setControlsVisible(true)
    if (hideTimer.current) clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => {
      setControlsVisible(false)
      setShowAnime4KPanel(false)
    }, 3500)
  }, [])

  useEffect(() => {
    if (!ready) return
    resetHideTimer()
    return () => {
      if (hideTimer.current) clearTimeout(hideTimer.current)
    }
  }, [ready, resetHideTimer])

  // 鼠标移动唤起控制条
  const handleMouseMove = useCallback(() => resetHideTimer(), [resetHideTimer])

  // ========== 交互回调 ==========
  const togglePause = useCallback(async () => {
    await cmd('cycle', ['pause'])
    setPaused((p) => !p)
    resetHideTimer()
  }, [cmd, resetHideTimer])

  const toggleMute = useCallback(async () => {
    const next = !muted
    await setProp('mute', next ? 'yes' : 'no')
    setMuted(next)
    resetHideTimer()
  }, [muted, setProp, resetHideTimer])

  const onVolumeChange = useCallback(
    async (v: number) => {
      setVolume(v)
      await setProp('volume', String(v))
      if (v > 0 && muted) {
        await setProp('mute', 'no')
        setMuted(false)
      }
      resetHideTimer()
    },
    [setProp, muted, resetHideTimer],
  )

  const toggleFullscreen = useCallback(async () => {
    await desktop.windowToggleFullscreen()
    resetHideTimer()
  }, [resetHideTimer])

  // ========== 渲染降级 ==========
  if (!desktop.isDesktop) {
    return (
      <div className={`flex items-center justify-center bg-black text-white/60 ${className}`}>
        <span className="text-sm">嵌入式 mpv 仅在桌面端可用</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className={`flex flex-col items-center justify-center gap-3 bg-black text-red-300 ${className}`}>
        <span className="text-sm">mpv 启动失败</span>
        <code className="text-xs text-red-400/80 px-3 py-1 bg-red-950/40 rounded">{error}</code>
        {onBack && (
          <button
            onClick={onBack}
            className="mt-2 px-3 py-1.5 text-xs rounded bg-white/10 hover:bg-white/20 text-white/80"
          >
            返回
          </button>
        )}
      </div>
    )
  }

  return (
    <div
      ref={wrapperRef}
      className={`relative bg-black overflow-hidden ${className}`}
      onMouseMove={handleMouseMove}
      onDoubleClick={toggleFullscreen}
    >
      {/* 视频占位区：libmpv 会把无边框子窗口贴到这个元素上方 */}
      <div
        ref={placeholderRef}
        className="absolute inset-0"
        data-mpv-embed-placeholder
      />

      {/* 加载指示 */}
      {!ready && (
        <div className="absolute inset-0 flex flex-col items-center justify-center text-white/60 gap-3 pointer-events-none">
          <Loader2 className="w-10 h-10 animate-spin" />
          <span className="text-sm">正在加载 libmpv 内核...</span>
        </div>
      )}

      {/* 顶部标题栏 + 返回 */}
      {ready && (
        <div
          className={`absolute top-0 left-0 right-0 p-4 flex items-center gap-3 transition-opacity duration-300
            ${controlsVisible ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
          style={{
            background: 'linear-gradient(to bottom, rgba(0,0,0,0.65), transparent)',
          }}
        >
          {onBack && (
            <button
              onClick={onBack}
              className="h-9 w-9 rounded-full bg-black/50 hover:bg-black/70 text-white flex items-center justify-center backdrop-blur-md"
              title="返回"
            >
              ←
            </button>
          )}
          {title && (
            <div className="flex-1 text-white font-medium text-sm drop-shadow truncate">{title}</div>
          )}
          <div className="text-white/60 text-xs flex items-center gap-1.5 px-2 py-1 rounded-md bg-white/10 backdrop-blur">
            <Sparkles className="w-3.5 h-3.5 text-violet-300" />
            libmpv · gpu-next
          </div>
        </div>
      )}

      {/* 底部控制条 */}
      {ready && (
        <div
          className={`absolute bottom-0 left-0 right-0 p-4 transition-opacity duration-300
            ${controlsVisible ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
          style={{
            background: 'linear-gradient(to top, rgba(0,0,0,0.7), transparent)',
          }}
        >
          {/* Anime4K 面板（悬浮在按钮上方） */}
          {showAnime4KPanel && (
            <div className="mb-3 flex justify-end">
              <Anime4KPanel value={anime4kLevel} onChange={applyAnime4K} />
            </div>
          )}

          <div className="flex items-center gap-3">
            {/* 播放/暂停 */}
            <button
              onClick={togglePause}
              className="h-10 w-10 rounded-full bg-white/10 hover:bg-white/20 text-white flex items-center justify-center transition"
              title={paused ? '播放' : '暂停'}
            >
              {paused ? <Play className="w-5 h-5" /> : <Pause className="w-5 h-5" />}
            </button>

            {/* 静音切换 */}
            <button
              onClick={toggleMute}
              className="h-9 w-9 rounded-full bg-white/10 hover:bg-white/20 text-white flex items-center justify-center transition"
              title={muted ? '取消静音' : '静音'}
            >
              {muted ? <VolumeX className="w-4 h-4" /> : <Volume2 className="w-4 h-4" />}
            </button>

            {/* 音量滑块 */}
            <input
              type="range"
              min={0}
              max={100}
              value={muted ? 0 : volume}
              onChange={(e) => onVolumeChange(Number(e.target.value))}
              className="w-28 accent-violet-400"
            />

            {/* 字幕切换（演示：0/1/no 三档循环） */}
            <button
              onClick={async () => {
                await cmd('cycle', ['sid'])
                resetHideTimer()
              }}
              className="h-9 w-9 rounded-full bg-white/10 hover:bg-white/20 text-white flex items-center justify-center transition"
              title="切换字幕"
            >
              <Subtitles className="w-4 h-4" />
            </button>

            <div className="flex-1" />

            {/* Anime4K 开关 */}
            <button
              onClick={() => setShowAnime4KPanel((v) => !v)}
              className={`px-3 h-9 rounded-full text-white flex items-center gap-1.5 text-xs font-medium transition
                ${anime4kLevel !== 'off'
                  ? 'bg-gradient-to-r from-violet-500 to-fuchsia-500'
                  : 'bg-white/10 hover:bg-white/20'}`}
              title="Anime4K 超分"
            >
              <Sparkles className="w-4 h-4" />
              {anime4kLevel === 'off' ? 'Anime4K' : `Anime4K · ${anime4kLevel.toUpperCase()}`}
            </button>

            {/* 全屏 */}
            <button
              onClick={toggleFullscreen}
              className="h-9 w-9 rounded-full bg-white/10 hover:bg-white/20 text-white flex items-center justify-center transition"
              title="全屏"
            >
              <Maximize className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

const MpvEmbedPlayer = forwardRef<MpvEmbedHandle, Props>(MpvEmbedPlayerInner)
MpvEmbedPlayer.displayName = 'MpvEmbedPlayer'

export default MpvEmbedPlayer

/** 便捷的无 ref 控制（通过 sessionId） */
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
  setVolume(sessionId: string, v: number) {
    return desktop.mpvEmbedSetProperty({ sessionId, name: 'volume', value: String(v) })
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
