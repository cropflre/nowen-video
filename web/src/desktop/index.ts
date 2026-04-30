
/**
 * 桌面端桥接入口
 *
 * 使用方式：
 *   1. 在 App 根组件挂载 <DesktopEventBinder />（一次即可）
 *   2. 在 PlayerPage 挂 <DesktopPlayerBadge /> 提示可切 mpv
 *   3. 想要 libmpv 嵌入体验时，改用 <MpvEmbedPlayer />
 *   4. 自动更新：在布局挂 <UpdateBanner />
 *
 * 设计原则：
 * 1. Web 端零侵入 —— 未运行在 Tauri 时所有方法安全降级
 * 2. 类型完备 —— 所有接口都有 TypeScript 定义
 * 3. 按需调用 —— 桌面能力动态 import，浏览器不会打包 Tauri SDK
 */

export { desktop } from './bridge'
export type {
  MediaProfile,
  EngineDecision,
  PlayOptions,
  SidecarStatus,
  DesktopSettings,
  PlatformInfo,
  MpvAvailability,
  UpdateInfo,
  EmbedStartResult,
} from './bridge'

export { useDesktop, usePlayerEngine } from './useDesktop'
export { default as DesktopPlayerBadge } from './DesktopPlayerBadge'
export { default as MpvEmbedPlayer, mpvControl } from './MpvEmbedPlayer'
export { default as UpdateBanner } from './UpdateBanner'
export { default as DesktopEventBinder } from './DesktopEventBinder'
