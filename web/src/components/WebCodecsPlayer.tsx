/**
 * WebCodecsPlayer —— 基于 WebCodecs API 的客户端硬解码播放器
 *
 * 架构：
 *   Worker (demuxer) → 主线程 VideoDecoder / AudioDecoder
 *                    ↓
 *                    Canvas 渲染 + AudioContext 播放
 *
 * 职责：
 *   - 向父组件暴露类似 <video> 的事件（loadedmetadata/timeupdate/ended/error）
 *   - 处理播放/暂停/seek
 *   - AV 同步：以 AudioContext.currentTime 为主时钟
 *
 * 暴露 ref 方法：play/pause/seek/getCurrentTime/getDuration
 */

import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { useAuthStore } from '@/stores/auth'

export interface WebCodecsPlayerHandle {
  play: () => Promise<void>
  pause: () => void
  seek: (time: number) => void
  getCurrentTime: () => number
  getDuration: () => number
  setVolume: (v: number) => void
  setMuted: (m: boolean) => void
  setPlaybackRate: (r: number) => void
}

export interface WebCodecsPlayerProps {
  src: string
  startPosition?: number
  onLoadedMetadata?: (info: { duration: number; width: number; height: number }) => void
  onTimeUpdate?: (time: number) => void
  onDurationChange?: (d: number) => void
  onPlay?: () => void
  onPause?: () => void
  onEnded?: () => void
  onError?: (msg: string) => void
  onReady?: () => void
  className?: string
}

interface TrackMeta {
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
  description?: ArrayBuffer
}

const WebCodecsPlayer = forwardRef<WebCodecsPlayerHandle, WebCodecsPlayerProps>(
  function WebCodecsPlayer(props, ref) {
    const { src, startPosition = 0 } = props
    const canvasRef = useRef<HTMLCanvasElement>(null)

    // Worker
    const workerRef = useRef<Worker | null>(null)
    // 解码器
    const videoDecoderRef = useRef<VideoDecoder | null>(null)
    const audioDecoderRef = useRef<AudioDecoder | null>(null)
    // 音频
    const audioCtxRef = useRef<AudioContext | null>(null)
    const audioGainRef = useRef<GainNode | null>(null)
    const audioStartTimeRef = useRef(0) // AudioContext 时基下的首帧播放时刻
    const audioScheduledEndRef = useRef(0) // 已排队音频的末尾时间（AudioContext 时基）
    // 视频帧队列（按时间戳排序）
    const videoQueueRef = useRef<VideoFrame[]>([])
    // 播放状态
    const playingRef = useRef(false)
    const currentTimeRef = useRef(0) // 秒
    const durationRef = useRef(0)
    const rafRef = useRef(0)
    const volumeRef = useRef(1)
    const mutedRef = useRef(false)
    const rateRef = useRef(1)
    // 轨道元数据
    const tracksRef = useRef<{ video?: TrackMeta; audio?: TrackMeta }>({})
    // seek 时钟重置标记
    const seekBaseRef = useRef(0) // 本次 seek 起点（秒），用于计算 video 显示时间
    // 已喂给解码器的视频样本数（用于背压）
    const pendingVideoRef = useRef(0)

    const [error, setError] = useState<string | null>(null)

    // 稳定回调：存到 ref，避免每次重新挂载
    const cbRef = useRef(props)
    cbRef.current = props

    useImperativeHandle(ref, () => ({
      play: async () => {
        if (playingRef.current) return
        const ctx = audioCtxRef.current
        if (ctx && ctx.state === 'suspended') {
          await ctx.resume()
        }
        playingRef.current = true
        // 重新对齐音频时钟
        if (ctx) {
          audioStartTimeRef.current = ctx.currentTime - currentTimeRef.current
          audioScheduledEndRef.current = ctx.currentTime
        }
        startRenderLoop()
        cbRef.current.onPlay?.()
      },
      pause: () => {
        if (!playingRef.current) return
        playingRef.current = false
        audioCtxRef.current?.suspend().catch(() => {})
        if (rafRef.current) cancelAnimationFrame(rafRef.current)
        cbRef.current.onPause?.()
      },
      seek: (time: number) => {
        doSeek(time)
      },
      getCurrentTime: () => currentTimeRef.current,
      getDuration: () => durationRef.current,
      setVolume: (v: number) => {
        volumeRef.current = v
        if (audioGainRef.current) {
          audioGainRef.current.gain.value = mutedRef.current ? 0 : v
        }
      },
      setMuted: (m: boolean) => {
        mutedRef.current = m
        if (audioGainRef.current) {
          audioGainRef.current.gain.value = m ? 0 : volumeRef.current
        }
      },
      setPlaybackRate: (r: number) => {
        rateRef.current = r
        // WebCodecs 变速较复杂，这里简化：音频丢弃，视频按加速呈现（仅支持 1x 完整音频）
        // 非 1x 时 mute 音频避免音调异常
        if (audioGainRef.current) {
          if (r !== 1) {
            audioGainRef.current.gain.value = 0
          } else {
            audioGainRef.current.gain.value = mutedRef.current ? 0 : volumeRef.current
          }
        }
      },
    }))

    // ===== 初始化 =====
    useEffect(() => {
      if (!src) return

      let cancelled = false

      // 1. 初始化 AudioContext
      const audioCtx = new AudioContext({ latencyHint: 'playback' })
      const gain = audioCtx.createGain()
      gain.gain.value = volumeRef.current
      gain.connect(audioCtx.destination)
      audioCtxRef.current = audioCtx
      audioGainRef.current = gain

      // 2. 启动 Worker
      const worker = new Worker(new URL('../workers/demuxer.worker.ts', import.meta.url), {
        type: 'module',
      })
      workerRef.current = worker

      worker.onmessage = async (e: MessageEvent) => {
        if (cancelled) return
        const msg = e.data
        switch (msg.type) {
          case 'ready':
            await handleReady(msg)
            break
          case 'sample':
            handleSample(msg)
            break
          case 'error':
            setError(msg.message)
            cbRef.current.onError?.(msg.message)
            break
          case 'eof':
            // 解封装完成（样本已全部送达解码器），等播放耗尽后触发 ended
            break
        }
      }

      const token = useAuthStore.getState().token || ''
      // 去掉 URL 里可能已有的 token 参数（用 header 注入更规范），但保留其他参数
      let cleanUrl = src
      try {
        const u = new URL(src, window.location.origin)
        u.searchParams.delete('token')
        cleanUrl = u.toString()
      } catch { /* ignore */ }

      worker.postMessage({ type: 'init', url: cleanUrl, token })

      return () => {
        cancelled = true
        if (rafRef.current) cancelAnimationFrame(rafRef.current)
        try { worker.postMessage({ type: 'stop' }) } catch { /* ignore */ }
        worker.terminate()
        videoDecoderRef.current?.close()
        audioDecoderRef.current?.close()
        videoDecoderRef.current = null
        audioDecoderRef.current = null
        for (const f of videoQueueRef.current) f.close()
        videoQueueRef.current = []
        audioCtx.close().catch(() => {})
        audioCtxRef.current = null
        audioGainRef.current = null
        playingRef.current = false
      }
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [src])

    async function handleReady(msg: { duration: number; tracks: TrackMeta[] }) {
      durationRef.current = msg.duration
      cbRef.current.onDurationChange?.(msg.duration)

      const vtrack = msg.tracks.find(t => t.type === 'video')
      const atrack = msg.tracks.find(t => t.type === 'audio')
      tracksRef.current = { video: vtrack, audio: atrack }

      // 视频解码器
      if (vtrack) {
        const decoder = new VideoDecoder({
          output: (frame) => onVideoFrame(frame),
          error: (e) => {
            console.error('[WebCodecs] VideoDecoder error:', e)
            setError(`视频解码错误: ${e.message}`)
            cbRef.current.onError?.(e.message)
          },
        })
        const config: VideoDecoderConfig = {
          codec: vtrack.codec,
          codedWidth: vtrack.width,
          codedHeight: vtrack.height,
        }
        if (vtrack.description) {
          config.description = vtrack.description
        }
        try {
          const { supported } = await VideoDecoder.isConfigSupported(config)
          if (!supported) {
            const msg2 = `视频编码 ${vtrack.codec} 不被浏览器支持`
            setError(msg2)
            cbRef.current.onError?.(msg2)
            return
          }
          decoder.configure(config)
          videoDecoderRef.current = decoder
        } catch (e: any) {
          setError(`视频解码器配置失败: ${e.message || e}`)
          cbRef.current.onError?.(e.message || String(e))
          return
        }
      }

      // 音频解码器
      if (atrack) {
        const decoder = new AudioDecoder({
          output: (data) => onAudioData(data),
          error: (e) => {
            console.error('[WebCodecs] AudioDecoder error:', e)
            // 音频失败不中断播放（可能是不支持的 codec），静音继续
          },
        })
        const config: AudioDecoderConfig = {
          codec: atrack.codec,
          sampleRate: atrack.sample_rate || 48000,
          numberOfChannels: atrack.channel_count || 2,
        }
        if (atrack.description) {
          config.description = atrack.description
        }
        try {
          const { supported } = await AudioDecoder.isConfigSupported(config)
          if (supported) {
            decoder.configure(config)
            audioDecoderRef.current = decoder
          }
        } catch (e) {
          console.warn('[WebCodecs] 音频解码器配置失败，将静音播放:', e)
        }
      }

      // 通知父组件就绪
      cbRef.current.onLoadedMetadata?.({
        duration: msg.duration,
        width: vtrack?.width || 0,
        height: vtrack?.height || 0,
      })
      cbRef.current.onReady?.()

      // 应用起始位置
      if (startPosition > 0) {
        doSeek(startPosition)
      } else {
        // 自动播放
        playingRef.current = true
        audioStartTimeRef.current = audioCtxRef.current!.currentTime
        audioScheduledEndRef.current = audioCtxRef.current!.currentTime
        cbRef.current.onPlay?.()
        startRenderLoop()
      }
    }

    function handleSample(msg: {
      trackId: number
      data: ArrayBuffer
      timestamp: number
      duration: number
      isKey: boolean
    }) {
      const { video, audio } = tracksRef.current
      if (video && msg.trackId === video.id && videoDecoderRef.current) {
        // 背压：视频队列太长时丢弃（正常播放下不应发生）
        if (pendingVideoRef.current > 60) return
        const chunk = new EncodedVideoChunk({
          type: msg.isKey ? 'key' : 'delta',
          timestamp: msg.timestamp,
          duration: msg.duration,
          data: msg.data,
        })
        try {
          videoDecoderRef.current.decode(chunk)
          pendingVideoRef.current++
        } catch (e) {
          console.warn('[WebCodecs] 视频 decode 失败:', e)
        }
      } else if (audio && msg.trackId === audio.id && audioDecoderRef.current) {
        const chunk = new EncodedAudioChunk({
          type: msg.isKey ? 'key' : 'delta',
          timestamp: msg.timestamp,
          duration: msg.duration,
          data: msg.data,
        })
        try {
          audioDecoderRef.current.decode(chunk)
        } catch (e) {
          console.warn('[WebCodecs] 音频 decode 失败:', e)
        }
      }
    }

    function onVideoFrame(frame: VideoFrame) {
      pendingVideoRef.current--
      // 队列丢弃过期帧（比当前时间早太多）
      const nowSec = currentTimeRef.current
      const frameSec = (frame.timestamp || 0) / 1e6
      if (frameSec < nowSec - 1) {
        frame.close()
        return
      }
      videoQueueRef.current.push(frame)
      // 按 timestamp 排序
      videoQueueRef.current.sort((a, b) => (a.timestamp || 0) - (b.timestamp || 0))
    }

    function onAudioData(data: AudioData) {
      const ctx = audioCtxRef.current
      if (!ctx) { data.close(); return }
      const numChannels = data.numberOfChannels
      const frameCount = data.numberOfFrames
      const sampleRate = data.sampleRate
      // 拷贝到 AudioBuffer
      const buffer = ctx.createBuffer(numChannels, frameCount, sampleRate)
      for (let ch = 0; ch < numChannels; ch++) {
        const arr = new Float32Array(frameCount)
        try {
          data.copyTo(arr, { planeIndex: ch, format: 'f32-planar' })
        } catch {
          // 非 planar，降级
          try {
            data.copyTo(arr, { planeIndex: 0, format: 'f32' })
          } catch { /* ignore */ }
        }
        buffer.copyToChannel(arr, ch)
      }
      data.close()

      const audioTime = (data.timestamp || 0) / 1e6 // 秒（媒体时间轴）
      // 目标 AudioContext 时刻 = audioStartTime + mediaTime
      const playAt = Math.max(
        ctx.currentTime,
        audioStartTimeRef.current + audioTime,
      )
      const src = ctx.createBufferSource()
      src.buffer = buffer
      src.connect(audioGainRef.current!)
      try {
        src.start(playAt)
      } catch { /* ignore */ }
      audioScheduledEndRef.current = Math.max(
        audioScheduledEndRef.current,
        playAt + buffer.duration,
      )
    }

    /** 主渲染循环：按 AudioContext 时钟显示视频帧 */
    function startRenderLoop() {
      const canvas = canvasRef.current
      if (!canvas) return
      const ctx2d = canvas.getContext('2d')
      if (!ctx2d) return

      const render = () => {
        if (!playingRef.current) return

        // 当前媒体时间 = AudioContext.currentTime - audioStartTime
        const actx = audioCtxRef.current
        let mediaTime = currentTimeRef.current
        if (actx) {
          mediaTime = actx.currentTime - audioStartTimeRef.current
          // 无音频时回退到基于 RAF 的增量（这里简化为 audio 时钟也推进）
        }
        if (mediaTime < 0) mediaTime = 0
        if (durationRef.current > 0 && mediaTime >= durationRef.current) {
          mediaTime = durationRef.current
          playingRef.current = false
          cbRef.current.onEnded?.()
        }
        currentTimeRef.current = mediaTime
        cbRef.current.onTimeUpdate?.(mediaTime)

        // 从队列取出应当显示的帧（timestamp <= mediaTime + 少量容差）
        const queue = videoQueueRef.current
        let picked: VideoFrame | null = null
        while (queue.length > 0) {
          const head = queue[0]
          const hs = (head.timestamp || 0) / 1e6
          if (hs <= mediaTime + 0.02) {
            if (picked) picked.close()
            picked = head
            queue.shift()
          } else {
            break
          }
        }
        if (picked) {
          if (canvas.width !== picked.displayWidth) canvas.width = picked.displayWidth
          if (canvas.height !== picked.displayHeight) canvas.height = picked.displayHeight
          try {
            ctx2d.drawImage(picked as any, 0, 0)
          } catch { /* ignore */ }
          picked.close()
        }

        if (playingRef.current) {
          rafRef.current = requestAnimationFrame(render)
        }
      }
      rafRef.current = requestAnimationFrame(render)
    }

    /** 执行 seek */
    function doSeek(time: number) {
      const dur = durationRef.current
      const target = Math.max(0, Math.min(dur || Number.MAX_VALUE, time))

      // 清空视频帧队列
      for (const f of videoQueueRef.current) f.close()
      videoQueueRef.current = []
      pendingVideoRef.current = 0

      // flush 解码器
      videoDecoderRef.current?.reset()
      audioDecoderRef.current?.reset()

      // 重新 configure（reset 之后 configure 才能再用）
      try {
        const vt = tracksRef.current.video
        if (vt && videoDecoderRef.current) {
          const cfg: VideoDecoderConfig = {
            codec: vt.codec,
            codedWidth: vt.width,
            codedHeight: vt.height,
          }
          if (vt.description) cfg.description = vt.description
          videoDecoderRef.current.configure(cfg)
        }
        const at = tracksRef.current.audio
        if (at && audioDecoderRef.current) {
          const cfg: AudioDecoderConfig = {
            codec: at.codec,
            sampleRate: at.sample_rate || 48000,
            numberOfChannels: at.channel_count || 2,
          }
          if (at.description) cfg.description = at.description
          audioDecoderRef.current.configure(cfg)
        }
      } catch (e) {
        console.warn('[WebCodecs] seek reconfigure 失败:', e)
      }

      // 重置时钟
      currentTimeRef.current = target
      seekBaseRef.current = target
      const actx = audioCtxRef.current
      if (actx) {
        audioStartTimeRef.current = actx.currentTime - target
        audioScheduledEndRef.current = actx.currentTime
      }

      // 通知 Worker 重新从新位置拉取
      workerRef.current?.postMessage({ type: 'seek', time: target })

      cbRef.current.onTimeUpdate?.(target)

      if (playingRef.current) {
        startRenderLoop()
      }
    }

    if (error) {
      return (
        <div className="flex h-full w-full items-center justify-center bg-black text-red-400 text-sm">
          WebCodecs 播放失败：{error}
        </div>
      )
    }

    return (
      <canvas
        ref={canvasRef}
        className={props.className}
        style={{ width: '100%', height: '100%', objectFit: 'contain', background: '#000' }}
      />
    )
  },
)

export default WebCodecsPlayer
