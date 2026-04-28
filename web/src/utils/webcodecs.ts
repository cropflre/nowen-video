/**
 * WebCodecs 能力检测与编码工具
 *
 * 路线 A：使用浏览器原生 WebCodecs API 进行硬件解码
 * - Chrome 94+ / Edge 94+ / Safari 16.4+ / Firefox 130+ 支持
 * - 相比 WASM 软解：性能接近原生、功耗低、不依赖大体积库
 * - 相比服务端转码：服务端 0 开销、支持离线回放（未来）
 */

export interface WebCodecsCapability {
  /** 浏览器是否实现 WebCodecs API */
  supported: boolean
  /** 是否支持 H.264 硬件解码 */
  h264: boolean
  /** 是否支持 HEVC/H.265 硬件解码 */
  hevc: boolean
  /** 是否支持 VP9 硬件解码 */
  vp9: boolean
  /** 是否支持 AV1 硬件解码 */
  av1: boolean
  /** AAC 音频解码 */
  aac: boolean
  /** Opus 音频解码 */
  opus: boolean
}

/** 缓存能力检测结果 */
let cachedCapability: WebCodecsCapability | null = null

/**
 * 检测 WebCodecs 能力（异步，因为 isConfigSupported 是 Promise）
 * 结果会被缓存，同一会话内只检测一次
 */
export async function detectWebCodecs(): Promise<WebCodecsCapability> {
  if (cachedCapability) return cachedCapability

  const cap: WebCodecsCapability = {
    supported: false,
    h264: false,
    hevc: false,
    vp9: false,
    av1: false,
    aac: false,
    opus: false,
  }

  // 基础 API 检测
  if (typeof VideoDecoder === 'undefined' || typeof AudioDecoder === 'undefined') {
    cachedCapability = cap
    return cap
  }
  cap.supported = true

  // 逐个编码探测（使用典型 profile）
  try {
    const [h264, hevc, vp9, av1] = await Promise.all([
      VideoDecoder.isConfigSupported({ codec: 'avc1.640028' }).then(r => !!r.supported).catch(() => false),
      VideoDecoder.isConfigSupported({ codec: 'hev1.1.6.L93.B0' }).then(r => !!r.supported).catch(() => false),
      VideoDecoder.isConfigSupported({ codec: 'vp09.00.10.08' }).then(r => !!r.supported).catch(() => false),
      VideoDecoder.isConfigSupported({ codec: 'av01.0.05M.08' }).then(r => !!r.supported).catch(() => false),
    ])
    cap.h264 = h264
    cap.hevc = hevc
    cap.vp9 = vp9
    cap.av1 = av1
  } catch {
    /* ignore */
  }

  try {
    const [aac, opus] = await Promise.all([
      AudioDecoder.isConfigSupported({
        codec: 'mp4a.40.2',
        sampleRate: 48000,
        numberOfChannels: 2,
      }).then(r => !!r.supported).catch(() => false),
      AudioDecoder.isConfigSupported({
        codec: 'opus',
        sampleRate: 48000,
        numberOfChannels: 2,
      }).then(r => !!r.supported).catch(() => false),
    ])
    cap.aac = aac
    cap.opus = opus
  } catch {
    /* ignore */
  }

  cachedCapability = cap
  return cap
}

/**
 * 根据媒体的编码信息，判断是否可以用 WebCodecs 硬解
 * @param videoCodec 后端返回的视频编码名（如 h264/hevc/vp9/av1）
 * @param audioCodec 后端返回的音频编码名（如 aac/opus）
 * @param cap        能力检测结果
 */
export function canUseWebCodecs(
  videoCodec: string,
  audioCodec: string,
  cap: WebCodecsCapability,
): boolean {
  if (!cap.supported) return false

  const v = (videoCodec || '').toLowerCase()
  const a = (audioCodec || '').toLowerCase()

  // 视频：必须明确支持
  const videoOk =
    (v.includes('h264') || v.includes('avc')) ? cap.h264 :
    (v.includes('hevc') || v.includes('h265')) ? cap.hevc :
    v.includes('vp9') ? cap.vp9 :
    v.includes('av1') ? cap.av1 :
    false
  if (!videoOk) return false

  // 音频：空（单轨或未探测）一律放行；明确编码则严格要求
  if (!a) return true
  if (a.includes('aac') || a.includes('mp4a')) return cap.aac
  if (a.includes('opus')) return cap.opus
  // 其它音频（mp3/flac/ac3/eac3）WebCodecs 暂不原生支持，走其他路径
  return false
}

/**
 * 把 FFmpeg/ffprobe 风格的 codec 名转成 WebCodecs 的 codec 字符串
 * （实际 codec 字符串需要从 avcC/hvcC 盒子里解析，这里只作为兜底）
 */
export function codecNameToWebCodecs(codec: string): string {
  const c = (codec || '').toLowerCase()
  if (c.includes('h264') || c.includes('avc')) return 'avc1.640028'
  if (c.includes('hevc') || c.includes('h265')) return 'hev1.1.6.L93.B0'
  if (c.includes('vp9')) return 'vp09.00.10.08'
  if (c.includes('av1')) return 'av01.0.05M.08'
  if (c.includes('aac') || c.includes('mp4a')) return 'mp4a.40.2'
  if (c.includes('opus')) return 'opus'
  return c
}

/**
 * 重置能力检测缓存（测试用）
 */
export function resetWebCodecsCache(): void {
  cachedCapability = null
}
