import { useEffect } from 'react'
import {
  MEDIA_COMPUTE_CAPABILITY_HIGHLIGHT_V1,
  MEDIA_COMPUTE_JOB_HIGHLIGHT_V1,
  MEDIA_COMPUTE_PROTOCOL_VERSION,
  mediaAnalysisApi,
  type MediaAnalysisWorkerHeartbeat,
  type MediaAnalysisWorkerResultItem,
  type MediaComputeHighlightInput,
  type MediaComputeTaskClaim,
} from '@/api/mediaAnalysis'
import { useAuthStore } from '@/stores/auth'
import { desktop, type HighlightCaptureFrameResult, type PlatformInfo } from './bridge'

const WORKER_ID_KEY = 'nowen-desktop-highlight-worker-id'
const IDLE_POLL_MS = 4_000
const INELIGIBLE_POLL_MS = 8_000
const DESKTOP_MIN_SEPARATION_SECONDS = 45
const DESKTOP_THUMBNAIL_SOFT_LIMIT = 300 * 1024
const DESKTOP_CAPTURE_WIDTHS = [640, 480, 360, 320]

type DesktopSample = {
  time: number
  rawScore: number
  normalizedScore: number
  thumbnail: HighlightCaptureFrameResult
}

function sleep(ms: number) {
  return new Promise<void>((resolve) => window.setTimeout(resolve, ms))
}

function persistentWorkerId() {
  try {
    const existing = window.localStorage.getItem(WORKER_ID_KEY)
    if (existing) return existing
    const created = typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : `desktop-${Date.now()}-${Math.random().toString(36).slice(2)}`
    window.localStorage.setItem(WORKER_ID_KEY, created)
    return created
  } catch {
    return `desktop-${Date.now()}-${Math.random().toString(36).slice(2)}`
  }
}

function workerName(platform: PlatformInfo | null) {
  if (!platform) return 'Nowen Video Desktop'
  return `Nowen Video Desktop · ${platform.os} ${platform.arch}`
}

function workerVersion() {
  const appVersion = String(import.meta.env.VITE_APP_VERSION || '').trim()
  return `desktop-v${MEDIA_COMPUTE_PROTOCOL_VERSION}/${appVersion || 'dev'}`
}

function heartbeat(workerId: string, platform: PlatformInfo | null, available: boolean): MediaAnalysisWorkerHeartbeat {
  return {
    worker_id: workerId,
    kind: 'desktop',
    name: workerName(platform),
    version: workerVersion(),
    capabilities: available ? [MEDIA_COMPUTE_CAPABILITY_HIGHLIGHT_V1] : [],
    network: 'desktop',
    charging: false,
    battery_percent: 100,
  }
}

function resolveStreamUrl(streamUrl: string) {
  const rawApiBase = String((window as any).__NOWEN_API_BASE__ || '/api')
  const apiBase = new URL(rawApiBase, window.location.origin)
  const target = new URL(streamUrl, apiBase)
  if (target.origin !== apiBase.origin) {
    throw new Error('服务器返回的媒体计算流跨越了当前服务器源，已拒绝发送登录凭证')
  }
  return target.toString()
}

function isHighlightInput(value: unknown): value is MediaComputeHighlightInput {
  if (!value || typeof value !== 'object') return false
  const input = value as Partial<MediaComputeHighlightInput>
  return typeof input.media_id === 'string'
    && typeof input.fingerprint === 'string'
    && typeof input.duration === 'number'
    && typeof input.stream_url === 'string'
    && Array.isArray(input.sample_times)
    && typeof input.max_highlights === 'number'
    && typeof input.engine_version === 'number'
}

function resolveHighlightInput(claim: MediaComputeTaskClaim): MediaComputeHighlightInput {
  const protocolVersion = claim.protocol_version ?? 1
  const jobType = claim.job_type || MEDIA_COMPUTE_JOB_HIGHLIGHT_V1
  if (protocolVersion >= MEDIA_COMPUTE_PROTOCOL_VERSION && jobType !== MEDIA_COMPUTE_JOB_HIGHLIGHT_V1) {
    throw new Error(`桌面媒体计算节点暂不支持任务类型：${jobType}`)
  }
  if (claim.required_capability && claim.required_capability !== MEDIA_COMPUTE_CAPABILITY_HIGHLIGHT_V1) {
    throw new Error(`桌面媒体计算节点缺少任务能力：${claim.required_capability}`)
  }
  if (claim.input !== undefined) {
    if (!isHighlightInput(claim.input)) {
      throw new Error('服务器返回的 highlight_v1 input 格式无效')
    }
    return claim.input
  }
  return {
    media_id: claim.media_id || '',
    fingerprint: claim.fingerprint || '',
    duration: claim.duration || 0,
    stream_url: claim.stream_url || '',
    sample_times: claim.sample_times || [],
    max_highlights: claim.max_highlights || 8,
    engine_version: claim.engine_version || 3,
  }
}

function bytesFromBase64(value: string) {
  const binary = window.atob(value)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }
  return bytes
}

async function visualInformationScore(thumbnail: HighlightCaptureFrameResult) {
  const bytes = bytesFromBase64(thumbnail.data_base64)
  const buffer = new ArrayBuffer(bytes.byteLength)
  new Uint8Array(buffer).set(bytes)
  const bitmap = await createImageBitmap(new Blob([buffer], { type: thumbnail.mime }))
  try {
    const width = Math.max(1, Math.min(96, bitmap.width))
    const height = Math.max(1, Math.round(bitmap.height * width / Math.max(1, bitmap.width)))
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const context = canvas.getContext('2d', { willReadFrequently: true })
    if (!context) throw new Error('桌面客户端无法创建画面评分 Canvas')
    context.drawImage(bitmap, 0, 0, width, height)
    const pixels = context.getImageData(0, 0, width, height).data
    const luminance = new Float64Array(width * height)
    let sum = 0
    for (let index = 0, pixel = 0; index < pixels.length; index += 4, pixel += 1) {
      const value = 0.2126 * pixels[index] + 0.7152 * pixels[index + 1] + 0.0722 * pixels[index + 2]
      luminance[pixel] = value
      sum += value
    }
    const mean = sum / Math.max(1, luminance.length)
    let variance = 0
    let edge = 0
    let edges = 0
    for (let y = 0; y < height; y += 1) {
      for (let x = 0; x < width; x += 1) {
        const index = y * width + x
        const delta = luminance[index] - mean
        variance += delta * delta
        if (x > 0) {
          edge += Math.abs(luminance[index] - luminance[index - 1])
          edges += 1
        }
        if (y > 0) {
          edge += Math.abs(luminance[index] - luminance[index - width])
          edges += 1
        }
      }
    }
    const standardDeviation = Math.sqrt(variance / Math.max(1, luminance.length))
    const edgeMean = edges > 0 ? edge / edges : 0
    const exposureBalance = 1 - Math.min(1, Math.abs(mean - 128) / 128)
    return standardDeviation * 0.52 + edgeMean * 0.38 + exposureBalance * 10
  } finally {
    bitmap.close()
  }
}

async function captureThumbnail(url: string, token: string, time: number) {
  let last: HighlightCaptureFrameResult | null = null
  for (const maxWidth of DESKTOP_CAPTURE_WIDTHS) {
    const result = await desktop.highlightCaptureFrame({
      url,
      time,
      max_width: maxWidth,
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!result?.data_base64 || result.mime !== 'image/webp') {
      throw new Error('桌面 libmpv 未生成有效 WebP 缩略图')
    }
    last = result
    if (result.byte_size <= DESKTOP_THUMBNAIL_SOFT_LIMIT) return result
  }
  if (!last || last.byte_size > DESKTOP_THUMBNAIL_SOFT_LIMIT) {
    throw new Error(`桌面精彩片段缩略图仍然过大：${last?.byte_size || 0} 字节`)
  }
  return last
}

function normalizeScores(samples: DesktopSample[]) {
  const raw = samples.map((item) => item.rawScore)
  const min = Math.min(...raw)
  const max = Math.max(...raw)
  const span = max - min
  for (const item of samples) {
    const normalized = span < 0.001 ? 0.5 : (item.rawScore - min) / span
    item.normalizedScore = 5.5 + Math.max(0, Math.min(1, normalized)) * 4.5
  }
}

function selectHighlights(samples: DesktopSample[], requestedLimit: number) {
  const limit = Math.max(1, Math.min(8, requestedLimit || 8))
  const selected: DesktopSample[] = []
  for (const candidate of [...samples].sort((a, b) => b.normalizedScore - a.normalizedScore)) {
    if (selected.every((item) => Math.abs(item.time - candidate.time) >= DESKTOP_MIN_SEPARATION_SECONDS)) {
      selected.push(candidate)
    }
    if (selected.length >= limit) break
  }
  return selected.sort((a, b) => a.time - b.time)
}

async function processHighlightClaim(claim: MediaComputeTaskClaim, token: string) {
  const input = resolveHighlightInput(claim)
  if (!Number.isFinite(input.duration) || input.duration <= 0 || input.sample_times.length === 0) {
    throw new Error('服务器没有返回可用的桌面精彩片段采样计划')
  }
  const streamUrl = resolveStreamUrl(input.stream_url)
  const samples: DesktopSample[] = []

  for (let index = 0; index < input.sample_times.length; index += 1) {
    const time = input.sample_times[index]
    if (!Number.isFinite(time) || time < 0 || time > input.duration + 1) continue
    const thumbnail = await captureThumbnail(streamUrl, token, time)
    const rawScore = await visualInformationScore(thumbnail)
    samples.push({ time, rawScore, normalizedScore: 5.5, thumbnail })
    await mediaAnalysisApi.updateWorkerProgress(claim.task_id, {
      claim_token: claim.claim_token,
      stage: 'client_sampling',
      progress: 8 + 72 * (index + 1) / input.sample_times.length,
    })
  }

  if (samples.length === 0) {
    throw new Error('桌面客户端无法从当前媒体提取有效采样帧')
  }

  normalizeScores(samples)
  const selected = selectHighlights(samples, input.max_highlights)
  if (selected.length === 0) {
    throw new Error('桌面客户端没有生成有效精彩片段候选')
  }

  await mediaAnalysisApi.updateWorkerProgress(claim.task_id, {
    claim_token: claim.claim_token,
    stage: 'client_analysis',
    progress: 92,
  })

  const highlights: MediaAnalysisWorkerResultItem[] = selected.map((sample) => {
    const start = Math.max(0, sample.time - 10)
    const end = Math.min(input.duration, start + 30)
    if (end <= start) throw new Error('桌面客户端生成了无效精彩片段时间范围')
    return {
      start_time: start,
      end_time: end,
      score: Math.max(0, Math.min(10, sample.normalizedScore)),
      analysis_method: 'desktop_mpv_sparse_v1',
      thumbnail_base64: sample.thumbnail.data_base64,
      thumbnail_mime: sample.thumbnail.mime,
    }
  })

  await mediaAnalysisApi.completeComputeTask(claim.task_id, {
    claim_token: claim.claim_token,
    job_type: MEDIA_COMPUTE_JOB_HIGHLIGHT_V1,
    result: {
      fingerprint: input.fingerprint,
      highlights,
    },
  })
}

async function processClaim(claim: MediaComputeTaskClaim, token: string) {
  const jobType = claim.job_type || MEDIA_COMPUTE_JOB_HIGHLIGHT_V1
  switch (jobType) {
    case MEDIA_COMPUTE_JOB_HIGHLIGHT_V1:
      await processHighlightClaim(claim, token)
      return
    default:
      throw new Error(`桌面媒体计算节点尚未注册执行器：${jobType}`)
  }
}

function errorMessage(error: unknown) {
  if (error instanceof Error && error.message) return error.message
  const responseMessage = (error as any)?.response?.data?.error
  return typeof responseMessage === 'string' && responseMessage ? responseMessage : '桌面客户端媒体计算失败'
}

/**
 * Desktop Media Compute Node V2：
 * - 只声明真实可执行的 capabilities；
 * - Claim 后先按 job_type 分派执行器，再解析对应 input；
 * - 当前第一个 adapter 是 highlight_v1，后续任务不需要重造心跳/Claim/租约协议；
 * - 播放页期间暂停领取，避免后台计算与观影争抢 libmpv 解码资源。
 */
export default function DesktopHighlightComputeAgent() {
  useEffect(() => {
    if (!desktop.isDesktop) return
    let cancelled = false
    const workerId = persistentWorkerId()

    void (async () => {
      let platform: PlatformInfo | null = null
      let embedAvailable = false
      let lastCapabilityCheck = 0

      while (!cancelled) {
        const auth = useAuthStore.getState()
        if (!auth.isAuthenticated || auth.user?.role !== 'admin' || auth.user?.must_change_pwd || !auth.token) {
          await sleep(INELIGIBLE_POLL_MS)
          continue
        }

        const now = Date.now()
        if (!platform) platform = await desktop.platformInfo()
        if (now - lastCapabilityCheck > 60_000) {
          const availability = await desktop.mpvAvailable()
          embedAvailable = availability.embed_available
          lastCapabilityCheck = now
          if (!embedAvailable) {
            await mediaAnalysisApi.heartbeatWorker(heartbeat(workerId, platform, false)).catch(() => {})
          }
        }
        if (!embedAvailable) {
          await sleep(INELIGIBLE_POLL_MS)
          continue
        }

        if (window.location.pathname.startsWith('/play/')) {
          await mediaAnalysisApi.heartbeatWorker(heartbeat(workerId, platform, false)).catch(() => {})
          await sleep(INELIGIBLE_POLL_MS)
          continue
        }

        let claim: MediaComputeTaskClaim | undefined
        try {
          const response = await mediaAnalysisApi.claimWorkerTask(heartbeat(workerId, platform, true))
          claim = response.status === 204 ? undefined : response.data?.data
        } catch {
          await sleep(IDLE_POLL_MS)
          continue
        }
        if (!claim) {
          await sleep(IDLE_POLL_MS)
          continue
        }

        try {
          const currentToken = useAuthStore.getState().token
          if (!currentToken) throw new Error('桌面客户端登录状态已失效')
          await processClaim(claim, currentToken)
        } catch (error) {
          const message = errorMessage(error).slice(0, 500)
          await mediaAnalysisApi.failWorkerTask(claim.task_id, {
            claim_token: claim.claim_token,
            error: message,
          }).catch(() => {})
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [])

  return null
}
