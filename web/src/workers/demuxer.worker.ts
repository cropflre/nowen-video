/**
 * 解封装 Worker：使用 mp4box.js 从 fMP4/MP4 容器中提取编码后的视频/音频帧
 *
 * 职责：
 *   1. 分段 Range 请求源文件
 *   2. 喂给 mp4box 解析
 *   3. 把 EncodedVideoChunk/EncodedAudioChunk 的原始数据通过 postMessage 发给主线程
 *
 * 主线程负责：
 *   - 调用 VideoDecoder / AudioDecoder 做硬件解码
 *   - 渲染 VideoFrame + 播放 AudioData
 *
 * 为什么不直接在主线程跑？
 *   - mp4box 解析是纯 CPU 工作，放 Worker 避免阻塞 UI（特别是 seek 时重新解析）
 */

// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-nocheck
/* eslint-disable */

import MP4Box from 'mp4box'

interface InitMessage {
  type: 'init'
  url: string
  token?: string
}

interface SeekMessage {
  type: 'seek'
  time: number // 秒
  epoch: number
}

interface FlowControlMessage {
  type: 'pause' | 'resume'
  epoch: number
}

interface StopMessage {
  type: 'stop'
}

type InMsg = InitMessage | SeekMessage | FlowControlMessage | StopMessage

interface TrackInfo {
  id: number
  type: 'video' | 'audio'
  codec: string
  timescale: number
  duration: number
  nb_samples: number
  width?: number
  height?: number
  sample_rate?: number
  channel_count?: number
  /** 编码器初始化数据（用于 VideoDecoder/AudioDecoder.configure.description） */
  description?: ArrayBuffer
}

let mp4boxFile: ReturnType<typeof MP4Box.createFile> | null = null
let sourceUrl = ''
let authToken = ''
let sourceSize = 0
let fetchOffset = 0
let abortCtrl: AbortController | null = null
let activeEpoch = 0
let paused = false
let pumpGeneration = 0
const CHUNK_SIZE = 1024 * 1024 * 2 // 2MB 每次拉取

function post(msg: any, transfer?: Transferable[]) {
  if (transfer && transfer.length) {
    (self as any).postMessage(msg, transfer)
  } else {
    (self as any).postMessage(msg)
  }
}

/**
 * 从 mp4box 的 track 信息中提取 codec description。
 * 视频使用 avcC/hvcC/vpcC/av1C，AAC 音频使用 esds 内的 DecoderSpecificInfo。
 */
function extractDescription(track: any): ArrayBuffer | undefined {
  const trak = mp4boxFile!.getTrackById(track.id)
  if (!trak) return undefined
  for (const entry of trak.mdia.minf.stbl.stsd.entries) {
    const box = entry.avcC || entry.hvcC || entry.vpcC || entry.av1C
    if (box) {
      const stream = new (MP4Box as any).DataStream(undefined, 0, (MP4Box as any).DataStream.BIG_ENDIAN)
      box.write(stream)
      // 去掉盒子头（8字节：size + type）
      return stream.buffer.slice(8)
    }

    // mp4a 的 WebCodecs 配置需要 AudioSpecificConfig，而不是完整的 esds box。
    const decoderConfig = entry.esds?.esd?.findDescriptor?.(0x04)
    const decoderSpecificInfo = decoderConfig?.findDescriptor?.(0x05)
    if (decoderSpecificInfo?.data) {
      return new Uint8Array(decoderSpecificInfo.data).slice().buffer
    }
  }
  return undefined
}

/**
 * 发起 Range 请求拉取一段数据，喂给 mp4box
 */
async function fetchRange(start: number, end: number): Promise<boolean> {
  if (abortCtrl) abortCtrl.abort()
  abortCtrl = new AbortController()

  const headers: Record<string, string> = {
    Range: `bytes=${start}-${end}`,
  }
  if (authToken) {
    headers.Authorization = `Bearer ${authToken}`
  }

  try {
    const resp = await fetch(sourceUrl, { headers, signal: abortCtrl.signal })
    if (!resp.ok && resp.status !== 206 && resp.status !== 200) {
      post({ type: 'error', message: `HTTP ${resp.status}` })
      return false
    }
    const requestedLength = end - start + 1
    const contentLength = Number(resp.headers.get('Content-Length') || 0)
    if (
      resp.status === 200
      && (start > 0 || contentLength === 0 || contentLength > requestedLength)
    ) {
      await resp.body?.cancel()
      post({ type: 'error', message: '源流不支持 Range 分段读取，无法使用 WebCodecs 播放' })
      return false
    }
    // 读取 Content-Range 获取总长度
    const cr = resp.headers.get('Content-Range')
    if (cr) {
      const m = cr.match(/\/(\d+)$/)
      if (m) sourceSize = parseInt(m[1], 10)
    } else if (resp.headers.get('Content-Length')) {
      sourceSize = parseInt(resp.headers.get('Content-Length')!, 10)
    }

    const buf = await resp.arrayBuffer()
    ;(buf as any).fileStart = start
    mp4boxFile!.appendBuffer(buf)
    fetchOffset = start + buf.byteLength
    return true
  } catch (e: any) {
    if (e.name === 'AbortError') return false
    post({ type: 'error', message: e.message || 'fetch failed' })
    return false
  }
}

/**
 * 持续拉取数据直到文件结束（或被打断）
 */
async function pumpLoop(generation: number) {
  while (!paused && generation === pumpGeneration && (sourceSize === 0 || fetchOffset < sourceSize)) {
    const end = sourceSize > 0
      ? Math.min(fetchOffset + CHUNK_SIZE - 1, sourceSize - 1)
      : fetchOffset + CHUNK_SIZE - 1
    const ok = await fetchRange(fetchOffset, end)
    if (!ok || paused || generation !== pumpGeneration) return
  }
  if (paused || generation !== pumpGeneration) return
  mp4boxFile!.flush()
  post({ type: 'eof', epoch: activeEpoch })
}

function startPump() {
  const generation = ++pumpGeneration
  void pumpLoop(generation)
}

/**
 * 初始化 mp4box 并绑定回调
 */
function initMp4box() {
  mp4boxFile = MP4Box.createFile()

  mp4boxFile.onError = (err: any) => {
    post({ type: 'error', message: String(err) })
  }

  mp4boxFile.onReady = (info: any) => {
    // 提取所有 track 信息
    const tracks: TrackInfo[] = []
    for (const t of info.tracks) {
      const type = t.type === 'video' ? 'video' : t.type === 'audio' ? 'audio' : null
      if (!type) continue
      const track: TrackInfo = {
        id: t.id,
        type,
        codec: t.codec,
        timescale: t.timescale,
        duration: t.duration,
        nb_samples: t.nb_samples,
      }
      if (type === 'video') {
        track.width = t.video?.width || t.track_width
        track.height = t.video?.height || t.track_height
      } else {
        track.sample_rate = t.audio?.sample_rate
        track.channel_count = t.audio?.channel_count
      }
      track.description = extractDescription(t)
      tracks.push(track)

      // 订阅该 track 的 samples
      mp4boxFile!.setExtractionOptions(t.id, null, { nbSamples: 100 })
    }

    post({
      type: 'ready',
      duration: info.duration / info.timescale,
      tracks,
    }, tracks.map(t => t.description).filter(Boolean) as ArrayBuffer[])

    // onSamples 回调
    mp4boxFile!.onSamples = (id: number, _user: any, samples: any[]) => {
      for (const s of samples) {
        const data = new Uint8Array(s.data).slice().buffer // 复制以便 transfer
        post({
          type: 'sample',
          epoch: activeEpoch,
          trackId: id,
          data,
          timestamp: (s.cts * 1e6) / s.timescale, // 微秒
          duration: (s.duration * 1e6) / s.timescale,
          isKey: s.is_sync,
        }, [data])
      }
    }

    mp4boxFile!.start()
  }
}

self.onmessage = (e: MessageEvent<InMsg>) => {
  const msg = e.data
  switch (msg.type) {
    case 'init': {
      sourceUrl = msg.url
      authToken = msg.token || ''
      fetchOffset = 0
      sourceSize = 0
      activeEpoch = 0
      paused = false
      initMp4box()
      startPump()
      break
    }
    case 'seek': {
      if (!mp4boxFile) return
      activeEpoch = msg.epoch
      paused = false
      pumpGeneration++
      if (abortCtrl) abortCtrl.abort()
      // mp4box 的 seek 返回 { offset, time } —— 需要从该 offset 重新拉取
      const res = mp4boxFile.seek(msg.time, true)
      if (res && typeof res.offset === 'number') {
        fetchOffset = res.offset
        startPump()
      }
      break
    }
    case 'pause': {
      if (msg.epoch !== activeEpoch || paused) return
      paused = true
      pumpGeneration++
      if (abortCtrl) abortCtrl.abort()
      break
    }
    case 'resume': {
      if (msg.epoch !== activeEpoch || !paused) return
      paused = false
      startPump()
      break
    }
    case 'stop': {
      paused = true
      pumpGeneration++
      if (abortCtrl) abortCtrl.abort()
      if (mp4boxFile) {
        try { mp4boxFile.stop() } catch { /* ignore */ }
      }
      mp4boxFile = null
      break
    }
  }
}

export {}
