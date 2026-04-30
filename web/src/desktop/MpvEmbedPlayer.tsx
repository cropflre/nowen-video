
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
import { desktop, PlayOptions, MpvVideoInfo } from './bridge'
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

  // 实时视频信息（2s 轮询 libmpv）
  const [videoInfo, setVideoInfo] = useState<MpvVideoInfo | null>(null)
  const [seeking, setSeeking] = useState(false)
  const [seekPreview, setSeekPreview] = useState<number | null>(null)

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

  // ========== Anime4K 切换（直接走 Rust 侧命令，避免前端拼路径） ==========
  const applyAnime4K = useCallback(
    async (level: Anime4KLevel) => {
      const ok = await desktop.mpvEmbedSetAnime4K({ sessionId, level })
      if (ok) {
        setAnime4kLevel(level)
      } else {
        console.warn('[mpv] 切换 Anime4K 档位失败:', level)
      }
    },
    [sessionId],
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

  // ========== 视频信息轮询（进度条 + HDR 徽章） ==========
  useEffect(() => {
    if (!ready || !desktop.isDesktop) return
    let canceled = false

    const tick = async () => {
      if (canceled) return
      try {
        const info = await desktop.mpvEmbedVideoInfo(sessionId)
        if (info && !canceled) {
          setVideoInfo(info)
          // 同步 paused/muted/volume 状态（外部快捷键等改了属性也能同步回 UI）
          if (info.paused !== paused) setPaused(info.paused)
          if (info.mute !== muted) setMuted(info.mute)
        }
      } catch {
        /* ignore */
      }
    }
    tick()
    const id = window.setInterval(tick, 1000)
    return () => {
      canceled = true
      window.clearInterval(id)
    }
    // 仅依赖 ready + sessionId，paused/muted 仅用作入参比较
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, sessionId])

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

  // 绝对位置 seek（给进度条拖拽用）
  const seekAbsolute = useCallback(
    async (seconds: number) => {
      await cmd('seek', [String(seconds), 'absolute'])
      resetHideTimer()
    },
    [cmd, resetHideTimer],
  )

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

          {/* 视频信息徽章（HDR / 分辨率） */}
          {videoInfo && videoInfo.width > 0 && (
            <>
              {videoInfo.hdr !== 'SDR' && videoInfo.hdr !== '' && (
                <div
                  className="text-xs px-2 py-1 rounded-md bg-gradient-to-r from-amber-500/30 to-orange-500/30 backdrop-blur border border-amber-400/40 text-amber-100 font-semibold tracking-wider"
                  title={`色域 ${videoInfo.primaries} · Gamma ${videoInfo.gamma}`}
                >
                  {videoInfo.hdr}
                </div>
              )}
              <div
                className="text-white/70 text-xs px-2 py-1 rounded-md bg-white/10 backdrop-blur font-mono"
                title={videoInfo.codec}
              >
                {videoInfo.height >= 2160
                  ? '4K'
                  : videoInfo.height >= 1440
                    ? '2K'
                    : videoInfo.height >= 1080
                      ? '1080p'
                      : `${videoInfo.height}p`}
              </div>
            </>
          )}
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

          {/* 进度条 */}
          {videoInfo && videoInfo.duration > 0 && (
            <div className="mb-3 flex items-center gap-3 text-white/80 text-xs font-mono select-none">
              <span className="w-14 text-right tabular-nums">
                {formatTime(seeking && seekPreview !== null ? seekPreview : videoInfo.position)}
              </span>
              <input
                type="range"
                min={0}
                max={Math.max(1, videoInfo.duration)}
                step={0.1}
                value={seeking && seekPreview !== null ? seekPreview : videoInfo.position}
                onChange={(e) => {
                  setSeeking(true)
                  setSeekPreview(Number(e.target.value))
                }}
                onMouseUp={async (e) => {
                  const v = Number((e.target as HTMLInputElement).value)
                  await seekAbsolute(v)
                  setSeeking(false)
                  setSeekPreview(null)
                }}
                onTouchEnd={async (e) => {
                  const v = Number((e.target as HTMLInputElement).value)
                  await seekAbsolute(v)
                  setSeeking(false)
                  setSeekPreview(null)
                }}
                className="flex-1 h-1.5 accent-violet-400 cursor-pointer"
                style={{
                  background: `linear-gradient(to right,
                    rgba(167, 139, 250, 0.9) 0%,
                    rgba(167, 139, 250, 0.9) ${((seeking && seekPreview !== null ? seekPreview : videoInfo.position) / videoInfo.duration) * 100}%,
                    rgba(255,255,255,0.15) ${((seeking && seekPreview !== null ? seekPreview : videoInfo.position) / videoInfo.duration) * 100}%,
                    rgba(255,255,255,0.15) 100%)`,
                  borderRadius: 999,
                  appearance: 'none',
                }}
              />
              <span className="w-14 tabular-nums text-white/60">
                {formatTime(videoInfo.duration)}
              </span>
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

/** 格式化秒数 → mm:ss / hh:mm:ss */
function formatTime(sec: number): string {
  if (!Number.isFinite(sec) || sec < 0) return '00:00'
  const s = Math.floor(sec % 60)
  const m = Math.floor((sec / 60) % 60)
  const h = Math.floor(sec / 3600)
  const pad = (n: number) => String(n).padStart(2, '0')
  return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`
}

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
