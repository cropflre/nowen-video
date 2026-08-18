/**
 * Nowen Video Desktop 2.0 平台入口。
 *
 * Desktop 只暴露平台能力、原生 Player Core、更新与服务器连接能力；
 * mpv 作为底层实现只存在 Rust Player Core 内部。
 */
export { desktop } from './bridge'
export type {
  PlayOptions,
  SidecarStatus,
  DesktopSettings,
  PlatformInfo,
  UpdateInfo,
  PlayerStartResult,
  PlayerVideoInfo,
  PlayerTrack,
  PlayerChapter,
  PlayerMediaInfo,
  PlayerStateEvent,
} from './bridge'

export { useDesktop } from './useDesktop'
export { default as DesktopPlayer, desktopPlayerControl } from './DesktopPlayer'
export type { DesktopPlayerHandle } from './DesktopPlayer'
export { default as UpdateBanner } from './UpdateBanner'
export { default as DesktopEventBinder } from './DesktopEventBinder'
export { default as DesktopServerPicker } from './DesktopServerPicker'
