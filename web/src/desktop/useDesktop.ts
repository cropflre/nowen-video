import { useEffect, useState } from 'react'
import { desktop, PlatformInfo } from './bridge'

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
      const [platformInfo, available] = await Promise.all([
        desktop.platformInfo(),
        desktop.playerAvailable(),
      ])
      if (canceled) return

      setPlatform(platformInfo)
      setPlayerAvailable(available)
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
    ready,
  }
}
