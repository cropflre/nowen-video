import {
  MEDIA_COMPUTE_CAPABILITY_CHAPTER_DETECT_V1,
  MEDIA_COMPUTE_JOB_CHAPTER_DETECT_V1,
  mediaAnalysisApi,
  type MediaComputeChapterCandidate,
  type MediaComputeChapterDetectInput,
  type MediaComputeChapterDetectResult,
  type MediaComputeTaskClaim,
} from '@/api/mediaAnalysis'
import { desktop, type HighlightCaptureFrameResult } from './bridge'

export { MEDIA_COMPUTE_CAPABILITY_CHAPTER_DETECT_V1, MEDIA_COMPUTE_JOB_CHAPTER_DETECT_V1 }

const CHAPTER_FRAME_SOFT_LIMIT = 220 * 1024
const CHAPTER_SIGNATURE_WIDTH = 32
const CHAPTER_SIGNATURE_HEIGHT = 18

function isChapterDetectInput(value: unknown): value is MediaComputeChapterDetectInput {
  if (!value || typeof value !== 'object') return false
  const input = value as Partial<MediaComputeChapterDetectInput>
  return typeof input.media_id === 'string'
    && typeof input.fingerprint === 'string'
    && typeof input.duration === 'number'
    && typeof input.stream_url === 'string'
    && Array.isArray(input.sample_times)
    && typeof input.probe_gap_seconds === 'number'
    && typeof input.min_chapter_seconds === 'number'
    && typeof input.max_chapters === 'number'
    && typeof input.capture_width === 'number'
    && typeof input.engine_version === 'number'
}

function resolveChapterInput(claim: MediaComputeTaskClaim): MediaComputeChapterDetectInput {
  if (claim.required_capability !== MEDIA_COMPUTE_CAPABILITY_CHAPTER_DETECT_V1) {
    throw new Error(`桌面媒体计算节点缺少章节检测能力：${claim.required_capability || 'unknown'}`)
  }
  if (!isChapterDetectInput(claim.input)) {
    throw new Error('服务器返回的 chapter_detect_v1 input 格式无效')
  }
  if (!claim.input.fingerprint || claim.input.duration <= 0 || claim.input.sample_times.length === 0) {
    throw new Error('服务器没有返回可用的章节检测采样计划')
  }
  return claim.input
}

function resolveStreamUrl(streamUrl: string) {
  const rawApiBase = String((window as any).__NOWEN_API_BASE__ || '/api')
  const apiBase = new URL(rawApiBase, window.location.origin)
  const target = new URL(streamUrl, apiBase)
  if (target.origin !== apiBase.origin) {
    throw new Error('章节检测媒体流跨越当前服务器源，已拒绝发送登录凭证')
  }
  return target.toString()
}

async function captureChapterFrame(url: string, token: string, time: number, requestedWidth: number) {
  const requested = Math.max(160, Math.min(320, Math.round(requestedWidth || 240)))
  const widths = Array.from(new Set([requested, 240, 200, 160])).filter((width) => width <= requested)
  let last: HighlightCaptureFrameResult | null = null
  for (const maxWidth of widths) {
    const result = await desktop.highlightCaptureFrame({
      url,
      time,
      max_width: maxWidth,
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!result?.data_base64 || result.mime !== 'image/webp') {
      throw new Error('桌面 libmpv 未生成有效章节检测 WebP 帧')
    }
    last = result
    if (result.byte_size <= CHAPTER_FRAME_SOFT_LIMIT) return result
  }
  throw new Error(`桌面章节检测帧超过大小限制：${last?.byte_size || 0} 字节`)
}

function bytesFromBase64(value: string) {
  const binary = window.atob(value)
  const bytes = new Uint8Array(binary.length)
  for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index)
  return bytes
}

async function frameSignature(frame: HighlightCaptureFrameResult) {
  const bytes = bytesFromBase64(frame.data_base64)
  const buffer = new ArrayBuffer(bytes.byteLength)
  new Uint8Array(buffer).set(bytes)
  const bitmap = await createImageBitmap(new Blob([buffer], { type: frame.mime }))
  try {
    const canvas = document.createElement('canvas')
    canvas.width = CHAPTER_SIGNATURE_WIDTH
    canvas.height = CHAPTER_SIGNATURE_HEIGHT
    const context = canvas.getContext('2d', { willReadFrequently: true })
    if (!context) throw new Error('桌面章节检测无法创建签名 Canvas')
    context.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
    const pixels = context.getImageData(0, 0, canvas.width, canvas.height).data
    const signature = new Float32Array(canvas.width * canvas.height)
    for (let index = 0, pixel = 0; index < pixels.length; index += 4, pixel += 1) {
      signature[pixel] = 0.2126 * pixels[index] + 0.7152 * pixels[index + 1] + 0.0722 * pixels[index + 2]
    }
    return signature
  } finally {
    bitmap.close()
  }
}

function signatureDifference(before: Float32Array, after: Float32Array) {
  if (before.length === 0 || before.length !== after.length) return 0
  let total = 0
  for (let index = 0; index < before.length; index += 1) total += Math.abs(before[index] - after[index])
  return Math.max(0, Math.min(1, total / before.length / 255))
}

export async function processDesktopChapterClaim(claim: MediaComputeTaskClaim, token: string) {
  const input = resolveChapterInput(claim)
  const streamUrl = resolveStreamUrl(input.stream_url)
  const gap = Math.max(1, Math.min(8, input.probe_gap_seconds || 3))
  const candidates: MediaComputeChapterCandidate[] = []

  for (let index = 0; index < input.sample_times.length; index += 1) {
    const center = input.sample_times[index]
    if (!Number.isFinite(center) || center <= 0 || center >= input.duration) {
      throw new Error('服务器返回了无效章节检测时间点')
    }
    const beforeTime = Math.max(0, center - gap)
    const afterTime = Math.min(input.duration, center + gap)
    const beforeFrame = await captureChapterFrame(streamUrl, token, beforeTime, input.capture_width)
    const afterFrame = await captureChapterFrame(streamUrl, token, afterTime, input.capture_width)
    const [beforeSignature, afterSignature] = await Promise.all([
      frameSignature(beforeFrame),
      frameSignature(afterFrame),
    ])
    candidates.push({ time: center, score: signatureDifference(beforeSignature, afterSignature) })

    if ((index + 1) % 4 === 0 || index === input.sample_times.length - 1) {
      await mediaAnalysisApi.updateWorkerProgress(claim.task_id, {
        claim_token: claim.claim_token,
        stage: 'client_chapter_probe',
        progress: 8 + 84 * (index + 1) / input.sample_times.length,
      })
    }
  }

  if (candidates.length !== input.sample_times.length) {
    throw new Error('桌面章节检测没有完成全部采样点')
  }
  const result: MediaComputeChapterDetectResult = { fingerprint: input.fingerprint, candidates }
  await mediaAnalysisApi.completeComputeTask(claim.task_id, {
    claim_token: claim.claim_token,
    job_type: MEDIA_COMPUTE_JOB_CHAPTER_DETECT_V1,
    result,
  })
}
