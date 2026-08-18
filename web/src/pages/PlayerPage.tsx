import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, streamApi, seriesApi } from '@/api'
import type { Media, MediaPlayInfo } from '@/types'
import AdaptiveWebVideoPlayer, { type BrowserPlaybackMode, type PlaybackTransition } from '@/components/AdaptiveWebVideoPlayer'
import WebCodecsPlayerShell from '@/components/WebCodecsPlayerShell'
import STRMDiagnostics from '@/components/player/STRMDiagnostics'
import { useToast } from '@/components/Toast'
import { usePlayerStore } from '@/stores/player'
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
  const clipStart = highlightMode ? requestedStart : 0
  const clipEnd = highlightMode ? requestedEnd : 0

  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [nextEpisode, setNextEpisode] = useState<Media | null>(null)
  const [switchPosition, setSwitchPosition] = useState<number | undefined>(() => highlightMode ? clipStart : undefined)
  const [webcodecsFailed, setWebcodecsFailed] = useState(false)
  const [runtimeMode, setRuntimeMode] = useState<BrowserPlaybackMode | null>(null)
  const [webcodecsCap, setWebcodecsCap] = useState<WebCodecsCapability | null>(null)
  const currentTimeRef = useRef(0)
  const clipEndedRef = useRef(false)
  const hasHistoryOnMountRef = useRef<boolean>(window.history.length > 1)
  const { currentTime } = usePlayerStore()

  useEffect(() => { currentTimeRef.current = currentTime }, [currentTime])
  useEffect(() => { detectWebCodecs().then(setWebcodecsCap).catch(() => setWebcodecsCap(null)) }, [])

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
    setLoading(true)
    setNextEpisode(null)
    setSwitchPosition(highlightMode ? clipStart : undefined)
    setWebcodecsFailed(false)
    setRuntimeMode(null)
    clipEndedRef.current = false
    Promise.all([mediaApi.detail(id), streamApi.getPlayInfo(id)])
      .then(([mediaRes, playInfoRes]) => {
        const mediaData = mediaRes.data.data
        setMedia(mediaData)
        setPlayInfo(playInfoRes.data.data)
        if (mediaData.media_type === 'episode' && mediaData.series_id) {
          seriesApi.nextEpisode(mediaData.series_id, mediaData.season_num, mediaData.episode_num)
            .then((res) => { if (res.data.data) setNextEpisode(res.data.data) })
            .catch(() => {})
        }
      })
      .catch(() => {
        toast.error('加载播放信息失败')
        navigate('/')
      })
      .finally(() => setLoading(false))
  }, [clipStart, highlightMode, id, navigate, toast])

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
    toast.info('WebCodecs 播放遇到问题，已切换到兼容模式')
    setWebcodecsFailed(true)
  }, [toast])

  const handleRuntimeModeChange = useCallback((mode: BrowserPlaybackMode) => setRuntimeMode(mode), [])

  const handlePlaybackTransition = useCallback((transition: PlaybackTransition) => {
    const target = transition.to === 'remux' ? 'Remux 兼容播放'
      : transition.to === 'smart_remux' ? 'Smart Remux（音频转码）'
      : 'HLS 转码播放'
    toast.info(`当前播放方式不兼容，已自动切换到${target}`)
  }, [toast])

  if (loading || (desktopRuntime.isDesktop && !desktopRuntime.ready) || !media || !playInfo || !id) {
    return (
      <div className="group/player flex h-screen items-center justify-center bg-[var(--nv-player-canvas)]">
        <div className="flex flex-col items-center gap-3">
          <Loader2 size={32} className="animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" />
          <p className="text-sm text-[var(--nv-player-text-tertiary)]">正在加载播放信息...</p>
        </div>
      </div>
    )
  }

  if (!highlightMode && desktopRuntime.isDesktop && !desktopRuntime.playerAvailable) {
    return (
      <div className="flex h-screen items-center justify-center bg-[var(--nv-player-canvas)] p-6">
        <div className="max-w-lg rounded-[var(--nv-radius-lg)] border border-[var(--nv-player-danger-border)] bg-[var(--nv-player-danger-soft)] p-6 text-center">
          <h2 className="text-lg font-semibold text-[var(--nv-player-text-primary)]">原生播放器不可用</h2>
          <p className="mt-2 text-sm leading-6 text-[var(--nv-player-text-secondary)]">Desktop 2.0 只使用原生 Player Core，不会回退到浏览器播放器。请检查 libmpv 运行资源后重新启动客户端。</p>
          <button type="button" onClick={() => navigate(-1)} className="mt-5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-4 py-2 text-sm text-[var(--nv-player-text-primary)] hover:bg-[var(--nv-player-surface-hover)]">返回</button>
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
    !highlightMode && !webcodecsFailed && !isPreprocessed && !playInfo.is_strm && !nativeCanPlay && !!webcodecsCap &&
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
    ? `${media.series?.title || media.title} S${String(media.season_num).padStart(2, '0')}E${String(media.episode_num).padStart(2, '0')}${media.episode_title ? ` - ${media.episode_title}` : ''}`
    : media.title
  const nextTitle = nextEpisode
    ? `S${String(nextEpisode.season_num).padStart(2, '0')}E${String(nextEpisode.episode_num).padStart(2, '0')}${nextEpisode.episode_title ? ` ${nextEpisode.episode_title}` : ''}`
    : undefined

  const handleBack = () => {
    if (highlightMode) {
      navigate(`/media/${id}`)
      return
    }
    if (hasHistoryOnMountRef.current) {
      navigate(-1)
      return
    }
    if (media.media_type === 'episode' && media.series_id) navigate(`/series/${media.series_id}`, { replace: true })
    else navigate(`/media/${id}`, { replace: true })
  }

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
    <div className="group/player relative h-screen w-screen bg-[var(--nv-player-canvas)]">
      <div className="nv-player-runtime-status absolute right-4 top-4 z-50 flex flex-col items-end gap-2 transition-opacity duration-200">
        {!highlightMode && playInfo.is_strm && <STRMDiagnostics mediaId={id} compact />}
        {statusContent && (
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
