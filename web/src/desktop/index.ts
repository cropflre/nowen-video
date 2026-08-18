/**
 * Nowen Video Desktop 2.0 平台入口。
 *
 * Desktop 只暴露平台能力、原生 Player Core、更新与服务器连接能力；
 * 不再向产品层暴露 Web/mpv 引擎决策器或技术型播放器切换提示。
 */
export { desktop } from './bridge'
export type {
  PlayOptions,
  SidecarStatus,
  DesktopSettings,
  PlatformInfo,
  MpvAvailability,
  UpdateInfo,
  EmbedStartResult,
  MpvVideoInfo,
} from './bridge'

export { useDesktop } from './useDesktop'
export { default as MpvEmbedPlayer, mpvControl } from './MpvEmbedPlayer'
export type { MpvEmbedHandle } from './MpvEmbedPlayer'
export { default as Anime4KPanel } from './Anime4KPanel'
export type { Anime4KLevel } from './Anime4KPanel'
export { default as UpdateBanner } from './UpdateBanner'
export { default as DesktopEventBinder } from './DesktopEventBinder'
export { default as DesktopServerPicker } from './DesktopServerPicker'
