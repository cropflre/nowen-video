import { invoke as tauriInvoke } from '@tauri-apps/api/core'
import { desktop as platformDesktop } from '../platform/desktop/bridge'

// Desktop 2.0 的正式平台实现位于 platform/desktop。
// 本兼容入口只额外挂接 Media Compute Node V2 的无窗口稀疏抽帧能力，
// 不恢复旧的 Web/mpv 双内核、外部 mpv 或播放器策略器。
export * from '../platform/desktop/bridge'

export interface HighlightCaptureFrameRequest {
  url: string
  time: number
  headers?: Record<string, string>
  max_width?: number
}

export interface HighlightCaptureFrameResult {
  data_base64: string
  mime: string
  byte_size: number
  max_width: number
}

export interface MpvAvailability {
  available: boolean
  embed_available: boolean
}

async function highlightCaptureFrame(
  request: HighlightCaptureFrameRequest,
): Promise<HighlightCaptureFrameResult | null> {
  if (!platformDesktop.isDesktop) return null
  try {
    return await tauriInvoke<HighlightCaptureFrameResult>('highlight_capture_frame', { request })
  } catch (error) {
    console.warn('[desktop] invoke highlight_capture_frame 失败:', error)
    return null
  }
}

export const desktop = {
  ...platformDesktop,

  highlightCaptureFrame,

  // 仅兼容现有 Media Compute Agent 的能力探测命名。
  // Desktop 2.0 正常播放仍只使用 playerAvailable / Player Core。
  async mpvAvailable(): Promise<MpvAvailability> {
    const available = await platformDesktop.playerAvailable()
    return { available, embed_available: available }
  },
}

export default desktop
