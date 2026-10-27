import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, streamApi, seriesApi, userApi } from '@/api'
import type { Media, MediaPlayInfo } from '@/types'
import AdaptiveWebVideoPlayer, { type BrowserPlaybackMode } from '@/components/AdaptiveWebVideoPlayer'
import WebCodecsPlayerShell from '@/components/WebCodecsPlayerShell'
import STRMDiagnostics from '@/components/player/STRMDiagnostics'
import { useToast } from '@/components/Toast'
import { usePlayerStore } from '@/stores/player'
import { useAuthStore } from '@/stores/auth'
import { episodeLabel, resumePosition } from '@/utils/viewing'
import { Zap, Loader2, Cpu, Clapperboard } from 'lucide-react'
import { detectWebCodecs, canUseWebCodecs, type WebCodecsCapability } from '@/utils/webcodecs'
import { DesktopPlayer, useDesktop } from '@/desktop'
import { getMediaCapabilities, type BrowserMediaCapability } from '@/utils/media-capabilities'

function getBrowserCaps(): BrowserMediaCapability {
  return getMediaCapabilities()
}

function formatClipTime(seconds: number) {
  const value = Math.max(0, Math.floor(seconds))
  const minutes = Math.floor(value / 60)
  const secs = value % 60
  return `${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
}

function toAbsoluteDesktopUrl(url: string, serverBaseUrl: string | null): string {
  if (!url || /^https?:\/\//i.test(url) || !serverBaseUrl) return url
  const base = serverBaseUrl.replace(/\/+$/, '')
  return url.startsWith('/') ? `${base}${url}` : `${base}/${url}`
}

export default function PlayerPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const toast = useToast()
  const desktopRuntime = useDesktop()

  const query = new URLSearchParams(window.location.search)
  const requestedStart = Number(query.get('start'))
  const requestedEnd = Number(query.get('end'))
  const highlightMode = query.get('mode') === 'highlight'
    && Number.isFinite(requestedStart)
    && Number.isFinite(requestedEnd)
    && requestedStart >= 0
    && requestedEnd > requestedStart
  const explicitStart = !highlightMode && query.has('start') && Number.isFinite(requestedStart) && requestedStart >= 0 ? requestedStart : undefined
  const clipStart = highlightMode ? requestedStart : 0
  const clipEnd = highlightMode ? requestedEnd : 0

  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [accessRevoked, setAccessRevoked] = useState(false)
  const childMode = useAuthStore((state) => state.user?.child_mode ?? false)
  const [loadError, setLoadError] = useState(false)
  const [loadAttempt, setLoadAttempt] = useState(0)
  const isAdmin = useAuthStore((state) => state.user?.role === 'admin')
  const preferredSubtitle = useAuthStore((state) => state.user?.viewing_preferences?.subtitle_language || 'auto')
  const [loading, setLoading] = useState(true)
  const [nextEpisode, setNextEpisode] = useState<Media | null>(null)
  const [switchPosition, setSwitchPosition] = useState<number | undefined>(() => highlightMode ? clipStart : undefined)
  const [webcodecsFailed, setWebcodecsFailed] = useState(false)
  const [runtimeMode, setRuntimeMode] = useState<BrowserPlaybackMode | null>(null)
  const [webcodecsCap, setWebcodecsCap] = useState<WebCodecsCapability | null>(null)
  const currentTimeRef = useRef(0)
  const clipEndedRef = useRef(false)
  const hasHistoryOnMountRef = useRef<boolean>(Number(window.history.state?.idx) > 0)
  const { currentTime } = usePlayerStore()

  useEffect(() => { currentTimeRef.current = currentTime }, [currentTime])
  useEffect(() => { detectWebCodecs().then(setWebcodecsCap).catch(() => setWebcodecsCap(null)) }, [])

  // Windows Render Surface 位于透明 Tauri WebView 下方。只在 Desktop 普通播放路由临时
  // 清除浏览器根节点的全局 Aurora 背景，离开播放页后完整恢复，不影响 Web/其他页面。
  useEffect(() => {
    if (!desktopRuntime.isDesktop || highlightMode) return

    const nodes = [document.documentElement, document.body, document.getElementById('root')]
      .filter((node): node is HTMLElement => Boolean(node))
    const previous = nodes.map((node) => node.style.background)

    nodes.forEach((node) => {
      node.style.background = 'transparent'
    })

    return () => {
      nodes.forEach((node, index) => {
        node.style.background = previous[index]
      })
    }
  }, [desktopRuntime.isDesktop, highlightMode])

  useEffect(() => {
    if (!highlightMode || !clipEnd) return
    if (currentTime < clipEnd - 0.35) {
      clipEndedRef.current = false
      return
    }
    if (clipEndedRef.current) return
    clipEndedRef.current = true
    document.querySelectorAll('video').forEach((element) => element.pause())
    usePlayerStore.getState().setPlaying(false)
  }, [clipEnd, currentTime, highlightMode])

  useEffect(() => {
    if (!id) return
    let cancelled = false
    setLoading(true)
    setLoadError(false)
    setAccessRevoked(false)
    setMedia(null)
    setPlayInfo(null)
    setNextEpisode(null)
    setWebcodecsFailed(false)
    setRuntimeMode(null)
    clipEndedRef.current = false
    const timeout = window.setTimeout(() => {
      cancelled = true
      setLoadError(true)
      setLoading(false)
    }, 30_000)
    Promise.all([
      mediaApi.detail(id), streamApi.getPlayInfo(id),
      highlightMode || explicitStart !== undefined ? Promise.resolve(null) : userApi.getProgress(id),
    ]).then(([mediaRes, playInfoRes, progressRes]) => {
      if (cancelled) return
      const mediaData = mediaRes.data.data
      if (!mediaData || !playInfoRes.data.data) throw new Error('Missing playback information')
      setMedia(mediaData)
      setPlayInfo(playInfoRes.data.data)
      setSwitchPosition(highlightMode ? clipStart : explicitStart ?? resumePosition(progressRes?.data.data))
      if (mediaData.media_type === 'episode' && mediaData.series_id) {
        seriesApi.nextEpisode(mediaData.series_id, mediaData.season_num, mediaData.episode_num)
          .then((res) => { if (!cancelled) setNextEpisode(res.data.data || null) })
          .catch(() => {})
      }
    }).catch(() => {
      if (!cancelled) setLoadError(true)
    }).finally(() => {
      window.clearTimeout(timeout)
      if (!cancelled) setLoading(false)
    })
    return () => { cancelled = true; window.clearTimeout(timeout) }
  }, [clipStart, explicitStart, highlightMode, id, loadAttempt])

  useEffect(() => {
    if (!childMode || !id || loading || loadError) return
    let disposed = false
    const check = () => {
      void mediaApi.detail(id).catch((error: { response?: { status?: number } }) => {
        if (disposed || ![403, 404].includes(error.response?.status || 0)) return
        document.querySelectorAll('video').forEach((video) => video.pause())
        usePlayerStore.getState().setPlaying(false)
        setAccessRevoked(true)
        setLoadError(true)
      })
    }
    const timer = window.setInterval(check, 15_000)
    window.addEventListener('focus', check)
    return () => { disposed = true; window.clearInterval(timer); window.removeEventListener('focus', check) }
  }, [childMode, id, loading, loadError])

  const handleBack = useCallback(() => {
    if (hasHistoryOnMountRef.current) navigate(-1)
    else if (media?.media_type === 'episode' && media.series_id) navigate(`/series/${media.series_id}`, { replace: true })
    else navigate(id ? `/media/${id}` : '/browse', { replace: true })
  }, [id, media, navigate])

  const handleNext = useCallback(() => {
    if (nextEpisode) navigate(`/play/${nextEpisode.id}`, { replace: true })
  }, [nextEpisode, navigate])

  const handlePreprocessReady = useCallback(() => {
    if (!id) return
    streamApi.getPlayInfo(id).then((res) => {
      const newPlayInfo = res.data.data
      if (newPlayInfo.is_preprocessed && newPlayInfo.preprocessed_url) {
        setSwitchPosition(currentTimeRef.current)
        setRuntimeMode(null)
        setPlayInfo(newPlayInfo)
      }
    }).catch(() => {})
  }, [id])

  const handleWebCodecsFallback = useCallback(() => {
    setSwitchPosition(currentTimeRef.current)
    setRuntimeMode(null)
    toast.info('正在尝试另一种播放方式，请稍候')
    setWebcodecsFailed(true)
  }, [toast])

  const handleRuntimeModeChange = useCallback((mode: BrowserPlaybackMode) => setRuntimeMode(mode), [])

  const handlePlaybackTransition = useCallback(() => {
    toast.info('正在尝试另一种播放方式，请稍候')
  }, [toast])

  if (loadError) {
    return (
      <div className="flex h-dvh flex-col items-center justify-center gap-4 bg-[var(--nv-player-canvas)] px-6 text-center text-[var(--nv-player-text-primary)]" role="alert">
        <h1 className="text-lg font-semibold">{accessRevoked ? '这部影片暂未向儿童账号开放' : '暂时无法准备这部影片'}</h1>
        <p className="text-sm text-[var(--nv-player-text-secondary)]">{accessRevoked ? '请返回选择其他影片，或请家长确认内容分级。' : '影片或观看记录暂时无法读取，请重试。'}</p>
        <div className="flex gap-3">
          <button type="button" className="nv-button" data-variant="primary" onClick={() => { setLoadError(false); setLoading(true); setLoadAttempt((value) => value + 1) }}>重试</button>
          <button type="button" className="nv-button" data-variant="secondary" onClick={handleBack}>返回</button>
        </div>
      </div>
    )
  }

  if (loading || (desktopRuntime.isDesktop && !desktopRuntime.ready) || !media || media.id !== id || !playInfo || !id) {
    return (
      <div className="group/player flex h-screen items-center justify-center bg-[var(--nv-player-canvas)]">
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={32} className="animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" />
          <p className="text-sm text-[var(--nv-player-text-tertiary)]">正在准备播放…</p>
        </div>
      </div>
    )
  }

  if (!highlightMode && desktopRuntime.isDesktop && !desktopRuntime.playerAvailable) {
    return (
      <div className="flex h-screen items-center justify-center bg-[var(--nv-player-canvas)] p-6">
        <div className="max-w-lg rounded-[var(--nv-radius-lg)] border border-[var(--nv-player-danger-border)] bg-[var(--nv-player-danger-soft)] p-6 text-center">
          <h2 className="text-lg font-semibold text-[var(--nv-player-text-primary)]">播放器暂时不可用</h2>
          <p className="mt-2 text-sm leading-6 text-[var(--nv-player-text-secondary)]">请重新启动客户端后再试。如果仍然无法播放，请联系管理员检查播放器安装。</p>
          <button type="button" onClick={handleBack} className="mt-5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-4 py-2 text-sm text-[var(--nv-player-text-primary)] hover:bg-[var(--nv-player-surface-hover)]">返回</button>
        </div>
      </div>
    )
  }

  const isPreprocessed = playInfo.is_preprocessed && playInfo.preprocessed_url
  const videoCodecLower = (playInfo.video_codec || '').toLowerCase()
  const isHEVCSource = videoCodecLower.includes('hevc') || videoCodecLower.includes('h265') || videoCodecLower === 'h265'
  const browserCaps = getBrowserCaps()
  const browserSupportsHEVC = browserCaps.video.hevc.main !== 'unsupported'
  const canDirectHEVC = isHEVCSource && browserSupportsHEVC && !isPreprocessed && playInfo.can_direct_play

  // Desktop 2.0：普通播放固定走原生 Player Core。
  // 精彩片段仍暂时复用 Web 播放器的精确 end-time 合约，后续单独迁入原生片段控制器。
  const useDesktopPlayer = !highlightMode && desktopRuntime.isDesktop

  const nativeCanPlay = playInfo.can_direct_play || canDirectHEVC
  const isRandomAccessMp4 = playInfo.file_ext === '.mp4' || playInfo.file_ext === '.m4v'
  const canUseWC =
    !highlightMode && (preferredSubtitle === 'auto' || preferredSubtitle === 'off') && !webcodecsFailed && !isPreprocessed && !playInfo.is_strm && !nativeCanPlay && !!webcodecsCap &&
    canUseWebCodecs(playInfo.video_codec, playInfo.audio_codec, webcodecsCap) &&
    isRandomAccessMp4

  const mode: 'direct' | 'hls' | 'remux' | 'webcodecs' = isPreprocessed
    ? 'hls'
    : canDirectHEVC
      ? 'direct'
      : playInfo.can_direct_play
        ? 'direct'
        : canUseWC
          ? 'webcodecs'
          : playInfo.can_remux
            ? 'remux'
            : 'hls'

  const requiresSessionTranscode = mode === 'hls' && !isPreprocessed && streamApi.requiresPlaybackSession(id)
  const browserSrc = isPreprocessed
    ? streamApi.withTokenUrl(playInfo.preprocessed_url!)
    : mode === 'direct'
      ? streamApi.getDirectUrl(id)
      : mode === 'remux'
        ? streamApi.getRemuxUrl(id)
        : mode === 'webcodecs'
          ? streamApi.getDirectUrl(id)
          : requiresSessionTranscode
            ? ''
            : streamApi.getMasterUrl(id)

  const desktopSrc = toAbsoluteDesktopUrl(streamApi.getDirectUrl(id), desktopRuntime.serverBaseUrl)

  const effectiveBrowserMode = runtimeMode || (mode === 'webcodecs' ? null : mode)
  const browserPlaybackResetKey = `${id}:${isPreprocessed ? playInfo.preprocessed_url : 'planned'}:${webcodecsFailed ? 'wc-fallback' : 'initial'}:${highlightMode ? `${clipStart}-${clipEnd}` : 'full'}`
  const playerTitle = media.media_type === 'episode'
    ? `${media.series?.title || media.title} · ${episodeLabel(media.season_num, media.episode_num)}${media.episode_title ? ` - ${media.episode_title}` : ''}`
    : media.title
  const nextTitle = nextEpisode
    ? `${episodeLabel(nextEpisode.season_num, nextEpisode.episode_num)}${nextEpisode.episode_title ? ` ${nextEpisode.episode_title}` : ''}`
    : undefined

  const statusContent = useDesktopPlayer
    ? { icon: <Cpu size={12} />, label: '原生播放', tone: 'accent' as const }
    : highlightMode
      ? { icon: <Clapperboard size={12} />, label: `精彩片段 ${formatClipTime(clipStart)}–${formatClipTime(clipEnd)}`, tone: 'accent' as const }
      : mode === 'webcodecs'
        ? { icon: <Cpu size={12} />, label: 'WebCodecs 硬解播放', tone: 'accent' as const }
        : isPreprocessed
          ? { icon: <Zap size={12} />, label: '秒开播放', tone: 'success' as const }
          : effectiveBrowserMode === 'direct' && canDirectHEVC
            ? { icon: <Zap size={12} />, label: 'HEVC 直接播放', tone: 'accent' as const }
            : effectiveBrowserMode === 'remux'
              ? { icon: <Cpu size={12} />, label: 'Remux 兼容播放', tone: 'accent' as const }
              : effectiveBrowserMode === 'smart_remux'
                ? { icon: <Cpu size={12} />, label: 'Smart Remux（音频转码）', tone: 'success' as const }
                : effectiveBrowserMode === 'hls'
                  ? { icon: <Cpu size={12} />, label: 'HLS 转码播放', tone: 'warning' as const }
                  : playInfo.preprocess_status === 'running'
                    ? { icon: <Loader2 size={12} className="animate-spin" />, label: '正在预处理中...', tone: 'accent' as const }
                    : playInfo.preprocess_status === 'pending' || playInfo.preprocess_status === 'queued'
                      ? { icon: <Loader2 size={12} />, label: '等待预处理', tone: 'warning' as const }
                      : null

  const statusColor = statusContent?.tone === 'success'
    ? 'var(--nv-player-success)'
    : statusContent?.tone === 'warning'
      ? 'var(--nv-player-warning)'
      : 'var(--nv-player-accent)'

  return (
    <div className={`group/player relative h-screen w-screen ${useDesktopPlayer ? 'bg-transparent' : 'bg-[var(--nv-player-canvas)]'}`}>
      <div className="nv-player-runtime-status absolute right-4 top-4 z-50 flex flex-col items-end gap-2 transition-opacity duration-200">
        {isAdmin && !highlightMode && (
          <details className="rounded-lg bg-[var(--nv-player-surface-soft)] p-2 text-sm text-[var(--nv-player-text-secondary)]">
            <summary className="cursor-pointer min-h-11 flex items-center">播放信息</summary>
            {playInfo.is_strm && <STRMDiagnostics mediaId={id} compact />}
            {statusContent && <p className="p-2">{statusContent.label}</p>}
          </details>
        )}
        {highlightMode && statusContent && (
          <div className="flex items-center gap-2 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-3 py-1.5 text-xs shadow-[var(--nv-shadow-card)] backdrop-blur-md" style={{ color: statusColor }}>
            {statusContent.icon}
            <span>{statusContent.label}</span>
          </div>
        )}
        {highlightMode && (
          <button
            type="button"
            onClick={() => navigate(`/media/${id}`)}
            className="rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-3 py-1.5 text-xs text-[var(--nv-player-text-secondary)] backdrop-blur-md transition hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]"
          >
            退出片段模式
          </button>
        )}
      </div>

      {useDesktopPlayer ? (
        <DesktopPlayer
          streamUrl={desktopSrc}
          sessionId={`media-${id}`}
          title={playerTitle}
          playOptions={{ title: playerTitle, start_time: switchPosition }}
          onBack={handleBack}
          className="h-full w-full"
        />
      ) : mode === 'webcodecs' ? (
        <WebCodecsPlayerShell
          src={browserSrc}
          mediaId={id}
          title={playerTitle}
          startPosition={switchPosition}
          knownDuration={playInfo.duration}
          onBack={handleBack}
          onNext={nextEpisode ? handleNext : undefined}
          nextTitle={nextTitle}
          onFallback={handleWebCodecsFallback}
        />
      ) : (
        <AdaptiveWebVideoPlayer
          mediaId={id}
          initialPlan={streamApi.getCachedPlaybackPlan(id)}
          initialMode={mode as BrowserPlaybackMode}
          initialSrc={browserSrc}
          initialRequiresSession={requiresSessionTranscode}
          resetKey={browserPlaybackResetKey}
          supportsHEVC={browserSupportsHEVC}
          title={playerTitle}
          isStrm={playInfo.is_strm}
          knownDuration={playInfo.duration}
          startPosition={switchPosition}
          spriteVttUrl={playInfo.sprite_vtt_url ? streamApi.withTokenUrl(playInfo.sprite_vtt_url) : undefined}
          onPreprocessReady={handlePreprocessReady}
          onModeChange={handleRuntimeModeChange}
          onTransition={handlePlaybackTransition}
          onBack={handleBack}
          onNext={highlightMode ? undefined : nextEpisode ? handleNext : undefined}
          nextTitle={highlightMode ? undefined : nextTitle}
        />
      )}
    </div>
  )
}
