
/**
 * useDesktop / usePlayerEngine
 *
 * React Hook 封装 —— 在组件里一行代码判断播放内核。
 *
 * 使用示例：
 * ```tsx
 * const { engine, reason, playInMpv } = usePlayerEngine({
 *   container: 'mkv',
 *   video_codec: 'hevc',
 *   audio_codec: 'dts',
 * })
 *
 * if (engine === 'mpv') {
 *   // 提示用户："已切换至 mpv 原画内核 —— {reason}"
 *   <Button onClick={() => playInMpv(streamUrl)}>用原生播放器打开</Button>
 * } else {
 *   <VideoPlayer src={streamUrl} />
 * }
 * ```
 */

import { useEffect, useState } from 'react'
import { desktop, EngineDecision, MediaProfile, PlayOptions, PlatformInfo, MpvAvailability } from './bridge'

/** 桌面环境基础信息 */
export function useDesktop() {
  const [platform, setPlatform] = useState<PlatformInfo | null>(null)
  const [mpvAvailable, setMpvAvailable] = useState<boolean>(false)
  const [embedAvailable, setEmbedAvailable] = useState<boolean>(false)
  const [ready, setReady] = useState<boolean>(false)

  useEffect(() => {
    if (!desktop.isDesktop) {
      setReady(true)
      return
    }
    let canceled = false

    ;(async () => {
      const [p, mpv] = await Promise.all([
        desktop.platformInfo(),
        desktop.mpvAvailable(),
      ])
      if (canceled) return
      setPlatform(p)
      const m: MpvAvailability = mpv
      setMpvAvailable(m.available)
      setEmbedAvailable(m.embed_available)
      setReady(true)
    })()

    return () => {
      canceled = true
    }
  }, [])

  return {
    isDesktop: desktop.isDesktop,
    platform,
    mpvAvailable,
    embedAvailable,
    ready,
  }
}

/** 播放引擎决策 Hook */
export function usePlayerEngine(profile: MediaProfile | null | undefined) {
  const [decision, setDecision] = useState<EngineDecision | null>(null)
  const [loading, setLoading] = useState<boolean>(false)

  useEffect(() => {
    if (!desktop.isDesktop) {
      // 浏览器环境永远 web
      setDecision({
        engine: 'web',
        reason: '当前为浏览器环境',
        confidence: 'fallback',
      })
      return
    }

    if (!profile) {
      setDecision(null)
      return
    }

    let canceled = false
    setLoading(true)

    desktop
      .decideEngine(profile)
      .then((r) => {
        if (canceled) return
        setDecision(r)
      })
      .finally(() => {
        if (!canceled) setLoading(false)
      })

    return () => {
      canceled = true
    }
  }, [JSON.stringify(profile)])

  /** 用 mpv 播放指定 URL */
  const playInMpv = async (
    url: string,
    options?: PlayOptions,
    sessionId: string = 'main',
  ) => {
    return desktop.playWithMpv({ sessionId, url, options })
  }

  const stopMpv = async (sessionId: string = 'main') => {
    return desktop.stopMpv(sessionId)
  }

  return {
    engine: decision?.engine ?? 'web',
    reason: decision?.reason ?? '',
    confidence: decision?.confidence ?? 'fallback',
    decision,
    loading,
    isDesktop: desktop.isDesktop,
    playInMpv,
    stopMpv,
  }
}
