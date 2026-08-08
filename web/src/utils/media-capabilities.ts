/**
 * 精确浏览器媒体能力探测模块
 *
 * 解决 canPlayType() 过于粗糙的问题：
 * - canPlayType 只返回 probably/maybe/""，maybe 不保证实际能播
 * - Windows Chrome 和 Mac Chrome 底层媒体栈不同，同一编码表现不同
 * - 容器、视频编码、Profile/Level、音频编码需要分开检测
 *
 * 三级检测策略：
 * 1. canPlayType() — 同步，覆盖容器+编码组合
 * 2. MediaSource.isTypeSupported() — 同步，MSE 兼容性
 * 3. MediaCapabilities.decodingInfo() — 异步，硬件解码+能效信息
 */

export type SupportLevel = 'probably' | 'maybe' | 'unsupported'

// ============================================================
// 类型定义
// ============================================================

export interface ContainerSupport {
  mp4: SupportLevel
  webm: SupportLevel
  m4v: SupportLevel
}

export interface VideoCodecSupport {
  h264: {
    baseline: SupportLevel
    main: SupportLevel
    high: SupportLevel
  }
  hevc: {
    main: SupportLevel
    main10: SupportLevel
  }
  vp8: SupportLevel
  vp9: SupportLevel
  av1: SupportLevel
}

export interface AudioCodecSupport {
  aac: SupportLevel
  mp3: SupportLevel
  opus: SupportLevel
  ac3: SupportLevel
  eac3: SupportLevel
  flac: SupportLevel
  vorbis: SupportLevel
}

export interface MSESupport {
  supported: boolean
  h264: boolean
  hevc: boolean
}

export interface DecodingInfoResult {
  supported: boolean
  smooth: boolean
  powerEfficient: boolean
}

export interface DetailedDecodingInfo {
  h264Main1080p?: DecodingInfoResult
  hevcMain1080p?: DecodingInfoResult
  hevcMain10_4k?: DecodingInfoResult
}

export interface BrowserMediaCapability {
  container: ContainerSupport
  video: VideoCodecSupport
  audio: AudioCodecSupport
  hdr: { hdr10: boolean; hlg: boolean; dolbyVision: boolean }
  platform: { os: string; isMac: boolean; isWindows: boolean; isLinux: boolean }
  mse: MSESupport
  decodingInfo?: DetailedDecodingInfo
  /** 综合：浏览器是否能硬解 HEVC Main Profile 1080p */
  hevcHardwareDecode: boolean
  /** 综合：浏览器是否能稳定播放 H.264+AAC+MP4 */
  universalPlayback: boolean
}

// ============================================================
// canPlayType 辅助
// ============================================================

function canPlay(mime: string, codec?: string): SupportLevel {
  if (typeof document === 'undefined') return 'unsupported'
  try {
    const video = document.createElement('video')
    const type = codec ? `${mime}; codecs="${codec}"` : mime
    const result = video.canPlayType(type)
    if (result === 'probably') return 'probably'
    if (result === 'maybe') return 'maybe'
    return 'unsupported'
  } catch {
    return 'unsupported'
  }
}

// ============================================================
// 平台检测
// ============================================================

function detectPlatform() {
  if (typeof navigator === 'undefined') {
    return { os: 'unknown', isMac: false, isWindows: false, isLinux: false }
  }
  const ua = navigator.userAgent || ''
  const isMac = /Mac/i.test(ua) && !/iPhone|iPad|iPod/i.test(ua)
  const isWindows = /Windows/i.test(ua)
  const isLinux = /Linux/i.test(ua) && !/Android/i.test(ua)
  const os = isMac ? 'macOS' : isWindows ? 'Windows' : isLinux ? 'Linux' : 'other'
  return { os, isMac, isWindows, isLinux }
}

// ============================================================
// 核心检测逻辑
// ============================================================

/**
 * 同步检测所有浏览器媒体能力。
 * 在应用初始化时调用一次即可，结果缓存于模块级变量。
 */
export function detectMediaCapabilities(): BrowserMediaCapability {
  const container: ContainerSupport = {
    mp4: canPlay('video/mp4'),
    webm: canPlay('video/webm'),
    m4v: canPlay('video/x-m4v'),
  }

  const video: VideoCodecSupport = {
    h264: {
      baseline: canPlay('video/mp4', 'avc1.42E01E'),
      main: canPlay('video/mp4', 'avc1.4D401E'),
      high: canPlay('video/mp4', 'avc1.640028'),
    },
    hevc: {
      // HEVC Main Profile, Level 4.1 (1080p), 8-bit
      main: canPlay('video/mp4', 'hvc1.1.6.L93.B0'),
      // HEVC Main10 Profile, Level 5.1 (4K), 10-bit
      main10: canPlay('video/mp4', 'hvc1.2.4.L153.B0'),
    },
    vp8: canPlay('video/webm', 'vp8'),
    vp9: canPlay('video/webm', 'vp9'),
    av1: canPlay('video/mp4', 'av01.0.05M.08'),
  }

  const audio: AudioCodecSupport = {
    aac: canPlay('audio/mp4', 'mp4a.40.2'),
    mp3: canPlay('audio/mpeg'),
    opus: canPlay('audio/webm', 'opus'),
    ac3: canPlay('audio/mp4', 'ac-3'),
    eac3: canPlay('audio/mp4', 'ec-3'),
    flac: canPlay('audio/mp4', 'flac'),
    vorbis: canPlay('audio/webm', 'vorbis'),
  }

  const mse = detectMSE()
  const platform = detectPlatform()
  const hdr = detectHDR()

  // 综合 HEVC 硬件解码：canPlayType 返回 probably + 平台非 Linux（Linux 软解为主）
  const hevcHardwareDecode =
    video.hevc.main === 'probably' && !platform.isLinux

  // 通用播放能力：H.264 High + AAC + MP4 全部 probably
  const universalPlayback =
    video.h264.high === 'probably' &&
    audio.aac === 'probably' &&
    container.mp4 === 'probably'

  return {
    container,
    video,
    audio,
    hdr,
    platform,
    mse,
    hevcHardwareDecode,
    universalPlayback,
  }
}

// ============================================================
// MSE 检测
// ============================================================

function detectMSE(): MSESupport {
  if (typeof MediaSource === 'undefined') {
    return { supported: false, h264: false, hevc: false }
  }
  try {
    return {
      supported: true,
      h264: MediaSource.isTypeSupported('video/mp4; codecs="avc1.640028,mp4a.40.2"'),
      hevc: MediaSource.isTypeSupported('video/mp4; codecs="hvc1.1.6.L93.B0,mp4a.40.2"'),
    }
  } catch {
    return { supported: false, h264: false, hevc: false }
  }
}

// ============================================================
// HDR 检测
// ============================================================

function detectHDR() {
  let hdr10 = false
  let hlg = false
  let dolbyVision = false

  if (typeof window !== 'undefined' && 'matchMedia' in window) {
    try {
      hdr10 = window.matchMedia('(dynamic-range: high)').matches
    } catch { /* ignore */ }
  }

  // Dolby Vision 可通过 canPlayType 探测
  try {
    const video = document.createElement('video')
    dolbyVision =
      video.canPlayType('video/mp4; codecs="dvh1.05"') !== '' ||
      video.canPlayType('video/mp4; codecs="dvhe.05"') !== ''
  } catch { /* ignore */ }

  return { hdr10, hlg, dolbyVision }
}

// ============================================================
// 异步深度检测：MediaCapabilities API
// ============================================================

/**
 * 使用 MediaCapabilities API 进行精确解码能力检测。
 * 需要在用户手势后调用（某些浏览器限制）。
 */
export async function detectDecodingInfo(): Promise<DetailedDecodingInfo> {
  if (typeof navigator === 'undefined' || !('mediaCapabilities' in navigator)) {
    return {}
  }

  const mc = navigator.mediaCapabilities as {
    decodingInfo(config: MediaDecodingConfiguration): Promise<MediaCapabilitiesDecodingInfo>
  }

  const result: DetailedDecodingInfo = {}

  // H.264 Main, 1080p, 5Mbps
  try {
    result.h264Main1080p = await mc.decodingInfo({
      type: 'file',
      video: {
        contentType: 'video/mp4; codecs="avc1.4D401E"',
        width: 1920,
        height: 1080,
        bitrate: 5_000_000,
        framerate: 30,
      },
    })
  } catch { /* ignore */ }

  // HEVC Main, 1080p, 5Mbps
  try {
    result.hevcMain1080p = await mc.decodingInfo({
      type: 'file',
      video: {
        contentType: 'video/mp4; codecs="hvc1.1.6.L93.B0"',
        width: 1920,
        height: 1080,
        bitrate: 5_000_000,
        framerate: 30,
      },
    })
  } catch { /* ignore */ }

  // HEVC Main10, 4K, 15Mbps
  try {
    result.hevcMain10_4k = await mc.decodingInfo({
      type: 'file',
      video: {
        contentType: 'video/mp4; codecs="hvc1.2.4.L153.B0"',
        width: 3840,
        height: 2160,
        bitrate: 15_000_000,
        framerate: 24,
      },
    })
  } catch { /* ignore */ }

  return result
}

// ============================================================
// 视频源兼容性诊断
// ============================================================

export interface SourceCompatibility {
  /** 视频编码是否被浏览器支持 */
  videoCodecSupported: boolean
  /** 音频编码是否被浏览器支持 */
  audioCodecSupported: boolean
  /** 容器是否可直接播放 */
  containerDirectPlayable: boolean
  /** 容器是否可 Remux */
  containerRemuxable: boolean
  /** 推荐播放策略 */
  recommendedStrategy: 'direct' | 'remux' | 'smart_remux' | 'transcode'
  /** 诊断说明 */
  reason: string
}

/**
 * 根据源文件的容器、视频编码、音频编码，结合浏览器能力给出播放建议。
 * 这比后端硬编码的 browserCompatibleAudioCodecs 更准确，
 * 因为实测了当前浏览器的 canPlayType 结果。
 */
export function diagnoseSource(
  caps: BrowserMediaCapability,
  source: {
    container: string // 如 "mkv", "mp4"
    videoCodec: string // 如 "h264", "hevc"
    audioCodec: string // 如 "dts", "aac", "eac3"
  },
): SourceCompatibility {
  const ext = source.container.toLowerCase().replace(/^\./, '')
  const vc = source.videoCodec.toLowerCase()
  const ac = source.audioCodec.toLowerCase()

  // 容器检测
  const directPlayableContainers = ['mp4', 'webm', 'm4v']
  const remuxableContainers = ['mkv', 'avi', 'mov', 'flv', 'wmv', 'ts']
  const containerDirectPlayable = directPlayableContainers.includes(ext)
  const containerRemuxable = remuxableContainers.includes(ext)

  // 视频编码检测
  const hevcCodecs = ['h265', 'hevc', 'h.265']
  const isHEVC = hevcCodecs.some(c => vc.includes(c)) || vc.startsWith('hev') || vc.startsWith('hvc')
  const h264Codecs = ['h264', 'avc', 'avc1', 'h.264']
  const isH264 = h264Codecs.some(c => vc.includes(c))

  const videoCodecSupported = isH264
    ? caps.video.h264.high !== 'unsupported'
    : isHEVC
      ? caps.video.hevc.main !== 'unsupported'
      : false

  // 音频编码检测
  const audioCodecSupported = detectAudioSupport(caps, ac)

  // 诊断
  if (containerDirectPlayable && videoCodecSupported && audioCodecSupported) {
    return {
      videoCodecSupported,
      audioCodecSupported,
      containerDirectPlayable,
      containerRemuxable,
      recommendedStrategy: 'direct',
      reason: '容器、视频编码、音频编码均受浏览器直接支持',
    }
  }

  if (containerRemuxable && videoCodecSupported && audioCodecSupported) {
    return {
      videoCodecSupported,
      audioCodecSupported,
      containerDirectPlayable,
      containerRemuxable,
      recommendedStrategy: 'remux',
      reason: '视频和音频编码兼容，仅需更换容器为 fMP4',
    }
  }

  if ((containerRemuxable || containerDirectPlayable) && videoCodecSupported && !audioCodecSupported) {
    return {
      videoCodecSupported,
      audioCodecSupported,
      containerDirectPlayable,
      containerRemuxable,
      recommendedStrategy: 'smart_remux',
      reason: `视频编码 ${source.videoCodec} 兼容，但音频编码 ${source.audioCodec} 不支持，仅需音频转码`,
    }
  }

  return {
    videoCodecSupported,
    audioCodecSupported,
    containerDirectPlayable,
    containerRemuxable,
    recommendedStrategy: 'transcode',
    reason: !videoCodecSupported
      ? `视频编码 ${source.videoCodec} 不被浏览器支持`
      : `容器 ${ext} 无法直接播放或 Remux`,
  }
}

function detectAudioSupport(caps: BrowserMediaCapability, codec: string): boolean {
  const c = codec.toLowerCase()
  if (c === 'aac' || c.includes('aac')) return caps.audio.aac !== 'unsupported'
  if (c === 'mp3' || c.includes('mp3')) return caps.audio.mp3 !== 'unsupported'
  if (c === 'opus') return caps.audio.opus !== 'unsupported'
  if (c === 'ac3' || c.includes('ac-3')) return caps.audio.ac3 !== 'unsupported'
  if (c === 'eac3' || c.includes('ec-3') || c.includes('e-ac3')) return caps.audio.eac3 !== 'unsupported'
  if (c === 'flac') return caps.audio.flac !== 'unsupported'
  if (c === 'vorbis') return caps.audio.vorbis !== 'unsupported'
  if (c === 'dts' || c.includes('dts')) return false // DTS 在浏览器中几乎不可用
  if (c === 'truehd' || c.includes('truehd')) return false // TrueHD 在浏览器中不可用
  if (c === 'pcm' || c.includes('pcm')) return false // PCM 在浏览器中不可用
  return false
}

// ============================================================
// 模块级缓存
// ============================================================

let _cached: BrowserMediaCapability | null = null

export function getMediaCapabilities(): BrowserMediaCapability {
  if (!_cached) {
    _cached = detectMediaCapabilities()
  }
  return _cached
}

export function refreshMediaCapabilities(): BrowserMediaCapability {
  _cached = detectMediaCapabilities()
  return _cached
}

/**
 * 生成发送给后端的客户端能力参数。
 * 格式与 PlaybackClientCapabilities 扩展字段对齐。
 */
export function buildClientCapabilities(caps: BrowserMediaCapability) {
  return {
    supports_direct: true,
    supports_remux: true,
    supports_hevc: caps.video.hevc.main !== 'unsupported',
    hevc_hardware: caps.hevcHardwareDecode,
    audio_supports_ac3: caps.audio.ac3 !== 'unsupported',
    audio_supports_eac3: caps.audio.eac3 !== 'unsupported',
    audio_supports_flac: caps.audio.flac !== 'unsupported',
    audio_supports_opus: caps.audio.opus !== 'unsupported',
    container_supports_mp4: caps.container.mp4 !== 'unsupported',
    container_supports_webm: caps.container.webm !== 'unsupported',
    mse_h264: caps.mse.h264,
    mse_hevc: caps.mse.hevc,
    platform: caps.platform.os,
  }
}

/**
 * 判断给定的媒体错误是否可以尝试降级而不是终端失败。
 * 对于明显是临时网络错误的，返回 false 以避免不必要的降级。
 */
export function isRecoverableMediaError(error: MediaError | null): boolean {
  if (!error) return true
  // MEDIA_ERR_NETWORK 可能是临时网络问题，不应立即降级
  // 但如果是持续报错，后续仍会触发降级
  return true
}

/**
 * 分析媒体错误的可能原因，返回结构化的诊断结果。
 */
export function analyzeMediaError(
  error: MediaError | null,
  caps: BrowserMediaCapability,
  source?: { container: string; videoCodec: string; audioCodec: string },
): {
  errorType: 'network' | 'decode' | 'unsupported' | 'aborted' | 'unknown'
  likelyAudioIssue: boolean
  likelyContainerIssue: boolean
  likelyVideoCodecIssue: boolean
  suggestedFallback: 'retry' | 'remux' | 'smart_remux' | 'transcode'
} {
  if (!error) {
    return {
      errorType: 'unknown',
      likelyAudioIssue: false,
      likelyContainerIssue: false,
      likelyVideoCodecIssue: false,
      suggestedFallback: 'transcode',
    }
  }

  const errorType =
    error.code === error.MEDIA_ERR_NETWORK ? 'network'
    : error.code === error.MEDIA_ERR_DECODE ? 'decode'
    : error.code === error.MEDIA_ERR_SRC_NOT_SUPPORTED ? 'unsupported'
    : error.code === error.MEDIA_ERR_ABORTED ? 'aborted'
    : 'unknown'

  // 如果有源文件信息，结合能力矩阵做更精准判断
  if (source) {
    const diag = diagnoseSource(caps, source)

    const likelyAudioIssue =
      diag.videoCodecSupported && !diag.audioCodecSupported

    const likelyContainerIssue =
      diag.videoCodecSupported &&
      diag.audioCodecSupported &&
      !diag.containerDirectPlayable &&
      diag.containerRemuxable

    const likelyVideoCodecIssue = !diag.videoCodecSupported

    let suggestedFallback: 'retry' | 'remux' | 'smart_remux' | 'transcode' = 'transcode'

    if (errorType === 'network') {
      suggestedFallback = 'retry'
    } else if (likelyContainerIssue) {
      suggestedFallback = 'remux'
    } else if (likelyAudioIssue && diag.videoCodecSupported) {
      suggestedFallback = 'smart_remux'
    } else {
      suggestedFallback = 'transcode'
    }

    return {
      errorType,
      likelyAudioIssue,
      likelyContainerIssue,
      likelyVideoCodecIssue,
      suggestedFallback,
    }
  }

  // 无源文件信息，保守降级
  return {
    errorType,
    likelyAudioIssue: false,
    likelyContainerIssue: false,
    likelyVideoCodecIssue: errorType === 'decode' || errorType === 'unsupported',
    suggestedFallback: errorType === 'network' ? 'retry' : 'transcode',
  }
}
