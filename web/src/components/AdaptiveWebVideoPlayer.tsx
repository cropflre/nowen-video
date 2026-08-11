import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AlertTriangle, Loader2, ShieldCheck } from 'lucide-react'
import VideoPlayer from './VideoPlayer'
import SessionVideoPlayer from './SessionVideoPlayer'
import { streamApi, type PlaybackMethod, type PlaybackPlan } from '@/api/stream'
import { usePlayerStore } from '@/stores/player'
import { getMediaCapabilities, analyzeMediaError } from '@/utils/media-capabilities'

export type BrowserPlaybackMode = 'direct' | 'remux' | 'smart_remux' | 'hls'

interface PlaybackSnapshot {
  position: number
  volume: number
  muted: boolean
  playbackRate: number
  paused: boolean
}

export interface PlaybackTransition {
  from: BrowserPlaybackMode
  to: BrowserPlaybackMode
  reason: string
}

interface AdaptiveWebVideoPlayerProps {
  mediaId: string
  initialPlan?: PlaybackPlan
  initialMode: BrowserPlaybackMode
  initialSrc: string
  initialRequiresSession: boolean
  resetKey: string
  supportsHEVC: boolean
  title?: string
  startPosition?: number
  onBack?: () => void
  onNext?: () => void
  nextTitle?: string
  isStrm?: boolean
  knownDuration?: number
  onPreprocessReady?: () => void
  spriteVttUrl?: string
  onModeChange?: (mode: BrowserPlaybackMode, plan?: PlaybackPlan) => void
  onTransition?: (transition: PlaybackTransition) => void
}

const PLAYBACK_MODE_RANK: Record<BrowserPlaybackMode, number> = {
  direct: 0,
  remux: 1,
  smart_remux: 2,
  hls: 3,
}

const MAX_NETWORK_RETRIES = 1
const NETWORK_RETRY_DELAY_MS = 500

function modeForMethod(method?: PlaybackMethod): BrowserPlaybackMode | null {
  switch (method) {
    case 'direct': return 'direct'
    case 'remux': return 'remux'
    case 'smart_remux': return 'smart_remux'
    case 'startup_stream':
    case 'transcode': return 'hls'
    default: return null
  }
}

function mediaErrorReason(error: MediaError | null): string {
  if (!error) return '浏览器未提供媒体错误详情'
  switch (error.code) {
    case error.MEDIA_ERR_ABORTED: return '媒体请求被浏览器中止'
    case error.MEDIA_ERR_NETWORK: return '媒体网络读取失败'
    case error.MEDIA_ERR_DECODE: return '浏览器解码失败'
    case error.MEDIA_ERR_SRC_NOT_SUPPORTED: return '浏览器不支持当前媒体源或编码'
    default: return error.message || `媒体错误 ${error.code}`
  }
}

function errorMessage(cause: unknown): string {
  if (cause instanceof Error && cause.message) return cause.message
  return '服务端无法生成下一种兼容播放方案'
}

function planHasUsableSource(plan: PlaybackPlan): boolean {
  if (plan.method === 'transcode' && plan.session_required) return Boolean(plan.session_template)
  return Boolean(plan.url)
}

function labelForMode(mode: BrowserPlaybackMode): string {
  if (mode === 'direct') return '直接播放'
  if (mode === 'remux') return 'Remux 兼容播放'
  if (mode === 'smart_remux') return 'Smart Remux（音频转码）'
  return 'HLS 转码播放'
}

export default function AdaptiveWebVideoPlayer({
  mediaId,
  initialPlan,
  initialMode,
  initialSrc,
  initialRequiresSession,
  resetKey,
  title,
  startPosition = 0,
  onBack,
  onNext,
  nextTitle,
  isStrm = false,
  knownDuration,
  onPreprocessReady,
  spriteVttUrl,
  onModeChange,
  onTransition,
}: AdaptiveWebVideoPlayerProps) {
  const rootRef = useRef<HTMLDivElement>(null)
  const operationRef = useRef(0)
  const transitionInFlightRef = useRef(false)
  const failedModesRef = useRef<Set<BrowserPlaybackMode>>(new Set())
  const networkRetryCountRef = useRef<Map<BrowserPlaybackMode, number>>(new Map())
  const snapshotRef = useRef<PlaybackSnapshot | null>(null)
  const resetConfigRef = useRef({ initialPlan, startPosition })
  resetConfigRef.current = { initialPlan, startPosition }

  const [activePlan, setActivePlan] = useState<PlaybackPlan | undefined>(initialPlan)
  const [resumePosition, setResumePosition] = useState(Math.max(0, startPosition))
  const [transitioning, setTransitioning] = useState(false)
  const [terminalError, setTerminalError] = useState<string | null>(null)
  const [lastTransition, setLastTransition] = useState<PlaybackTransition | null>(null)

  useEffect(() => {
    const resetConfig = resetConfigRef.current
    operationRef.current += 1
    transitionInFlightRef.current = false
    failedModesRef.current.clear()
    networkRetryCountRef.current.clear()
    snapshotRef.current = null
    setActivePlan(resetConfig.initialPlan)
    setResumePosition(Math.max(0, resetConfig.startPosition))
    setTransitioning(false)
    setTerminalError(null)
    setLastTransition(null)
  }, [mediaId, resetKey])

  const activeMode = useMemo(() => modeForMethod(activePlan?.method) || initialMode, [activePlan?.method, initialMode])

  const requiresSession = useMemo(() => {
    if (!activePlan) return initialRequiresSession
    return activePlan.method === 'transcode' && activePlan.session_required
  }, [activePlan, initialRequiresSession])

  const activeSrc = useMemo(() => {
    if (!activePlan || requiresSession) return initialSrc
    return activePlan.url ? streamApi.withTokenUrl(activePlan.url) : initialSrc
  }, [activePlan, initialSrc, requiresSession])

  useEffect(() => { onModeChange?.(activeMode, activePlan) }, [activeMode, activePlan, onModeChange])

  const requestFallback = useCallback(async (video: HTMLVideoElement) => {
    const from = activeMode
    if (from === 'hls' || transitionInFlightRef.current || failedModesRef.current.has(from)) return

    transitionInFlightRef.current = true
    failedModesRef.current.add(from)
    const operation = ++operationRef.current
    const store = usePlayerStore.getState()
    const storePosition = Number.isFinite(store.currentTime) ? Math.max(0, store.currentTime) : 0
    const elementPosition = Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0
    const position = Math.max(storePosition, elementPosition)
    const reason = mediaErrorReason(video.error)
    const caps = getMediaCapabilities()
    const analysis = analyzeMediaError(video.error, caps)

    snapshotRef.current = {
      position,
      volume: video.volume,
      muted: video.muted,
      playbackRate: video.playbackRate || 1,
      paused: video.paused && position > 0 && !store.isPlaying,
    }
    setResumePosition(position)
    setTransitioning(true)
    setTerminalError(null)

    if (analysis.errorType === 'aborted') {
      failedModesRef.current.delete(from)
      transitionInFlightRef.current = false
      setTransitioning(false)
      return
    }

    if (analysis.suggestedFallback === 'retry') {
      const retries = networkRetryCountRef.current.get(from) ?? 0
      if (retries < MAX_NETWORK_RETRIES) {
        networkRetryCountRef.current.set(from, retries + 1)
        failedModesRef.current.delete(from)
        transitionInFlightRef.current = false
        setTransitioning(false)
        window.setTimeout(() => {
          if (operationRef.current === operation) {
            video.load()
            void video.play().catch(() => undefined)
          }
        }, NETWORK_RETRY_DELAY_MS)
        return
      }
      failedModesRef.current.delete(from)
      transitionInFlightRef.current = false
      setTransitioning(false)
      setTerminalError(`${reason}；网络重试失败，请检查网络或媒体存储连接后重试`)
      return
    }

    try {
      const nextDirect = false
      const nextRemux = from === 'direct'
      const nextForceTranscode = from !== 'direct'

      const response = await streamApi.getPlaybackPlan(mediaId, {
        supportsDirect: nextDirect,
        supportsRemux: nextRemux,
        forceTranscode: nextForceTranscode,
      })
      if (operationRef.current !== operation) return

      const nextPlan = response.data.data
      const to = modeForMethod(nextPlan.method)
      if (!to || PLAYBACK_MODE_RANK[to] <= PLAYBACK_MODE_RANK[from]) throw new Error(`服务端返回了无效的降级方案：${nextPlan.method}`)
      if (failedModesRef.current.has(to)) throw new Error(`兼容播放方案 ${nextPlan.method} 已经失败，已阻止循环重试`)
      if (!planHasUsableSource(nextPlan)) throw new Error(`兼容播放方案 ${nextPlan.method} 没有可用地址或会话模板`)

      const transition = { from, to, reason }
      setActivePlan(nextPlan)
      setLastTransition(transition)
      onTransition?.(transition)
    } catch (cause) {
      if (operationRef.current === operation) setTerminalError(`${reason}；${errorMessage(cause)}`)
    } finally {
      if (operationRef.current === operation) {
        setTransitioning(false)
        transitionInFlightRef.current = false
      }
    }
  }, [activeMode, mediaId, onTransition])

  useEffect(() => {
    const root = rootRef.current
    if (!root) return
    const handleMediaError = (event: Event) => {
      if (!(event.target instanceof HTMLVideoElement) || activeMode === 'hls') return
      void requestFallback(event.target)
    }
    root.addEventListener('error', handleMediaError, true)
    return () => root.removeEventListener('error', handleMediaError, true)
  }, [activeMode, requestFallback])

  useEffect(() => {
    const root = rootRef.current
    if (!root) return
    const restorePlaybackState = (event: Event) => {
      if (!(event.target instanceof HTMLVideoElement)) return
      networkRetryCountRef.current.delete(activeMode)
      const snapshot = snapshotRef.current
      if (!snapshot) return
      const video = event.target
      window.setTimeout(() => {
        if (!root.contains(video)) return
        video.volume = snapshot.volume
        video.muted = snapshot.muted
        video.playbackRate = snapshot.playbackRate
        if (snapshot.paused) video.pause()
        else void video.play().catch(() => undefined)
      }, 0)
    }
    root.addEventListener('loadedmetadata', restorePlaybackState, true)
    return () => root.removeEventListener('loadedmetadata', restorePlaybackState, true)
  }, [activeMode, activeSrc, requiresSession])

  useEffect(() => () => {
    operationRef.current += 1
    transitionInFlightRef.current = false
  }, [])

  if (terminalError) {
    return (
      <div className="group/player flex h-full w-full flex-col items-center justify-center gap-3 bg-[var(--nv-player-canvas)] px-6 text-center text-[var(--nv-player-text-primary)]">
        <AlertTriangle className="text-[var(--nv-player-danger)]" size={34} aria-hidden="true" />
        <p className="text-base font-medium">所有兼容播放方式都失败了</p>
        <p className="max-w-2xl text-sm text-[var(--nv-player-text-tertiary)]">{terminalError}</p>
        {onBack && <button type="button" onClick={onBack} className="mt-2 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-4 py-2 text-sm transition-colors hover:bg-[var(--nv-player-surface-hover)]">返回</button>}
      </div>
    )
  }

  return (
    <div ref={rootRef} className="group/player relative h-full w-full bg-[var(--nv-player-canvas)]">
      {requiresSession ? (
        <SessionVideoPlayer
          key={`${mediaId}:${activePlan?.method || activeMode}:${activePlan?.url || 'session'}`}
          mediaId={mediaId}
          title={title}
          startPosition={resumePosition}
          knownDuration={knownDuration}
          onBack={onBack}
          onNext={onNext}
          nextTitle={nextTitle}
          onPreprocessReady={onPreprocessReady}
          spriteVttUrl={spriteVttUrl}
        />
      ) : (
        <VideoPlayer
          key={`${mediaId}:${activeMode}:${activeSrc}`}
          src={activeSrc}
          mode={activeMode}
          mediaId={mediaId}
          title={title}
          startPosition={resumePosition}
          onBack={onBack}
          onNext={onNext}
          nextTitle={nextTitle}
          isStrm={isStrm}
          knownDuration={knownDuration}
          onPreprocessReady={onPreprocessReady}
          spriteVttUrl={spriteVttUrl}
        />
      )}

      {transitioning && (
        <div className="absolute inset-0 z-[70] flex flex-col items-center justify-center gap-3 bg-[color-mix(in_srgb,var(--nv-player-canvas)_80%,transparent)] text-[var(--nv-player-text-primary)] backdrop-blur-sm">
          <Loader2 className="animate-spin text-[var(--nv-player-accent)]" size={32} aria-hidden="true" />
          <p className="text-sm text-[var(--nv-player-text-secondary)]">当前方式播放失败，正在切换兼容方案...</p>
        </div>
      )}

      {!transitioning && lastTransition && (
        <div className="pointer-events-none absolute bottom-20 right-4 z-[65] flex items-center gap-2 rounded-[var(--nv-player-radius-control)] border px-3 py-2 text-xs backdrop-blur-md" style={{ borderColor: 'color-mix(in srgb, var(--nv-player-success) 25%, transparent)', background: 'color-mix(in srgb, var(--nv-player-canvas) 70%, transparent)', color: 'var(--nv-player-success)' }}>
          <ShieldCheck size={14} aria-hidden="true" />
          <span>已自动切换为{labelForMode(lastTransition.to)}</span>
        </div>
      )}
    </div>
  )
}
