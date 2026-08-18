import { useEffect, useState } from 'react'
import { desktop, PlatformInfo, MpvAvailability } from './bridge'

/**
 * Desktop 2.0 环境 Hook。
 *
 * 桌面端不再暴露“Web / mpv 引擎选择”概念：Desktop 只有一个原生 Player Core，
 * 浏览器平台的 WebCodecs/HLS/Remux 由 Web Player 自己负责。
 */
export function useDesktop() {
  const [platform, setPlatform] = useState<PlatformInfo | null>(null)
  const [playerAvailable, setPlayerAvailable] = useState(false)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    if (!desktop.isDesktop) {
      setReady(true)
      return
    }

    let canceled = false
    ;(async () => {
      const [platformInfo, player] = await Promise.all([
        desktop.platformInfo(),
        desktop.mpvAvailable(),
      ])
      if (canceled) return

      const availability: MpvAvailability = player
      setPlatform(platformInfo)
      setPlayerAvailable(availability.embed_available)
      setReady(true)
    })()

    return () => {
      canceled = true
    }
  }, [])

  return {
    isDesktop: desktop.isDesktop,
    platform,
    playerAvailable,
    // 兼容过渡：旧调用方在完全迁移前仍可读取 embedAvailable。
    embedAvailable: playerAvailable,
    ready,
  }
}
