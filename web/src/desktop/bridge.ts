
/**
 * desktopBridge
 *
 * 统一的桌面端桥接层 —— 封装 Tauri 能力，向 Web 层提供一致 API。
 * 运行在浏览器中时自动降级（不报错，所有桌面方法返回 null/false）。
 *
 * 使用示例：
 * ```ts
 * import { desktop } from '@/desktop/bridge'
 *
 * if (desktop.isDesktop) {
 *   const decision = await desktop.decideEngine({ container: 'mkv', video_codec: 'hevc' })
 *   if (decision.engine === 'mpv') {
 *     await desktop.playWithMpv({ sessionId: 'main', url: streamUrl })
 *   }
 * }
 * ```
 */

export interface MediaProfile {
  container?: string
  video_codec?: string
  audio_codec?: string
  bit_depth?: number
  hdr?: string
  has_complex_subtitle?: boolean
  height?: number
  is_bluray?: boolean
}

export interface EngineDecision {
  engine: 'mpv' | 'web'
  reason: string
  confidence: 'strict' | 'recommend' | 'fallback'
}

export interface PlayOptions {
  title?: string
  start_time?: number
  subtitles?: string[]
  audio_lang?: string
  sub_lang?: string
  fullscreen?: boolean
  http_headers?: Record<string, string>
  user_agent?: string
}

export interface SidecarStatus {
  running: boolean
  pid?: number
  port: number
  mode: string
  uptime_secs: number
}

export interface DesktopSettings {
  server: {
    mode: 'embedded' | 'remote'
    remote_url: string
    sidecar_port: number
  }
  player: {
    engine: 'auto' | 'mpv' | 'web'
    mpv_path: string
    mpv_args: string[]
    hardware_accel: boolean
  }
  window: {
    width: number
    height: number
    remember_size: boolean
    minimize_to_tray: boolean
  }
}

export interface PlatformInfo {
  os: string        // windows | macos | linux | ...
  arch: string      // x86_64 | aarch64
  family: string
  is_desktop: boolean
}

export interface MpvAvailability {
  available: boolean
  embed_available: boolean
}

export interface UpdateInfo {
  available: boolean
  version: string
  current_version: string
  notes: string
  pub_date: string
}

export interface EmbedStartResult {
  wid: number
  session_id: string
}

/** libmpv 运行时视频信息（来自 set_embed_video_info 命令） */
export interface MpvVideoInfo {
  width: number
  height: number
  codec: string
  container: string
  duration: number
  position: number
  pixel_format: string
  primaries: string
  gamma: string
  /** SDR / HDR10 / HLG / DoVi */
  hdr: string
  paused: boolean
  volume: number
  mute: boolean
}

export type Anime4KLevel = 'off' | 'low' | 'medium' | 'high'

/** 是否运行在 Tauri 桌面环境 */
function detectDesktop(): boolean {
  if (typeof window === 'undefined') return false
  const w = window as any
  // Tauri 2.0 注入 window.__TAURI_INTERNALS__，旧版为 window.__TAURI__
  return Boolean(w.__TAURI_INTERNALS__ || w.__TAURI__)
}

const IS_DESKTOP = detectDesktop()

/**
 * 通过全局注入的 Tauri 对象直接拿到 invoke / listen，
 * 避免依赖 `@tauri-apps/api`（这是 Tauri 侧可选的 npm 包，
 * 纯浏览器构建时不存在，Vite 静态分析会直接报错）。
 *
 * Tauri v2 运行时会在 window 上挂 __TAURI_INTERNALS__.invoke，
 * 以及全局事件总线。若找不到则直接返回 null，表示不在桌面端。
 */
function getTauriInvoke(): ((cmd: string, args?: Record<string, unknown>) => Promise<unknown>) | null {
  if (!IS_DESKTOP) return null
  const w = window as any
  // Tauri v2
  if (w.__TAURI_INTERNALS__?.invoke) {
    return (cmd, args) => w.__TAURI_INTERNALS__.invoke(cmd, args || {})
  }
  // Tauri v1 兼容
  if (w.__TAURI__?.invoke) {
    return (cmd, args) => w.__TAURI__.invoke(cmd, args || {})
  }
  if (w.__TAURI__?.tauri?.invoke) {
    return (cmd, args) => w.__TAURI__.tauri.invoke(cmd, args || {})
  }
  return null
}

function getTauriEvent(): { listen: (event: string, cb: (e: { payload: unknown }) => void) => Promise<() => void> } | null {
  if (!IS_DESKTOP) return null
  const w = window as any
  // Tauri v2 —— event 模块挂在 __TAURI__.event（脚本注入）
  if (w.__TAURI__?.event?.listen) {
    return w.__TAURI__.event
  }
  // 部分版本会挂在 __TAURI_INTERNALS__.event
  if (w.__TAURI_INTERNALS__?.event?.listen) {
    return w.__TAURI_INTERNALS__.event
  }
  return null
}

/** 安全调用 invoke —— 桌面端才有效 */
async function invoke<T>(cmd: string, args?: Record<string, unknown>): Promise<T | null> {
  const fn = getTauriInvoke()
  if (!fn) return null
  try {
    return (await fn(cmd, args)) as T
  } catch (e) {
    console.warn(`[desktop] invoke ${cmd} 失败:`, e)
    return null
  }
}

/** 监听 Tauri 事件 —— 桌面端才有效 */
async function listen<T>(
  event: string,
  handler: (payload: T) => void,
): Promise<() => void> {
  const ev = getTauriEvent()
  if (!ev) return () => {}
  try {
    const un = await ev.listen(event, (e) => handler(e.payload as T))
    return un
  } catch (e) {
    console.warn(`[desktop] listen ${event} 失败:`, e)
    return () => {}
  }
}

export const desktop = {
  /** 是否运行在桌面端 */
  isDesktop: IS_DESKTOP,

  // ============ 播放器（外部进程） ============

  /** 决策使用哪个播放内核 */
  async decideEngine(profile: MediaProfile): Promise<EngineDecision | null> {
    return invoke<EngineDecision>('decide_engine', { profile })
  },

  /** mpv 是否可用（外部进程 + libmpv 嵌入） */
  async mpvAvailable(): Promise<MpvAvailability> {
    const r = await invoke<MpvAvailability>('mpv_available')
    return r ?? { available: false, embed_available: false }
  },

  /** 用 mpv 播放（独立窗口） */
  async playWithMpv(params: {
    sessionId: string
    url: string
    options?: PlayOptions
  }): Promise<boolean> {
    const r = await invoke<void>('play_with_mpv', {
      sessionId: params.sessionId,
      url: params.url,
      options: params.options,
    })
    return r !== null
  },

  /** 停止 mpv 播放 */
  async stopMpv(sessionId: string): Promise<void> {
    await invoke<void>('stop_mpv', { sessionId })
  },

  // ============ M4: libmpv 嵌入播放 ============

  /** 启动嵌入式 mpv（在 Tauri 子窗口内渲染） */
  async mpvEmbedStart(params: {
    sessionId: string
    url: string
    options?: PlayOptions
  }): Promise<EmbedStartResult | null> {
    return invoke<EmbedStartResult>('mpv_embed_start', {
      sessionId: params.sessionId,
      url: params.url,
      options: params.options,
    })
  },

  /** 同步嵌入窗口的位置/大小（前端布局变化时调用） */
  async mpvEmbedSync(params: {
    x: number
    y: number
    width: number
    height: number
    visible: boolean
  }): Promise<boolean> {
    const r = await invoke<void>('mpv_embed_sync', params)
    return r !== null
  },

  /** 发送 mpv 命令（loadfile, seek, cycle pause 等） */
  async mpvEmbedCommand(params: {
    sessionId: string
    command: string
    args?: string[]
  }): Promise<boolean> {
    const r = await invoke<void>('mpv_embed_command', params)
    return r !== null
  },

  /** 设置 mpv 属性（pause, volume, mute 等） */
  async mpvEmbedSetProperty(params: {
    sessionId: string
    name: string
    value: string
  }): Promise<boolean> {
    const r = await invoke<void>('mpv_embed_set_property', params)
    return r !== null
  },

  /** 销毁嵌入窗口 */
  async mpvEmbedDestroy(): Promise<void> {
    await invoke<void>('mpv_embed_destroy')
  },

  /** 应用 Anime4K 档位（off/low/medium/high） */
  async mpvEmbedSetAnime4K(params: {
    sessionId: string
    level: Anime4KLevel
  }): Promise<boolean> {
    const r = await invoke<void>('mpv_embed_set_anime4k', params)
    return r !== null
  },

  /** 查询嵌入 mpv 的当前视频信息（HDR/分辨率/进度/音量） */
  async mpvEmbedVideoInfo(sessionId: string): Promise<MpvVideoInfo | null> {
    return invoke<MpvVideoInfo>('mpv_embed_video_info', { sessionId })
  },

  // ============ M5: 自动更新 ============

  async checkUpdate(): Promise<UpdateInfo | null> {
    return invoke<UpdateInfo>('check_update')
  },

  async installUpdate(): Promise<boolean> {
    const r = await invoke<void>('install_update')
    return r !== null
  },

  // ============ sidecar ============

  /** 查询 Go sidecar 状态 */
  async sidecarStatus(): Promise<SidecarStatus | null> {
    return invoke<SidecarStatus>('sidecar_status')
  },

  /** 重启 sidecar */
  async sidecarRestart(): Promise<boolean> {
    const r = await invoke<void>('sidecar_restart')
    return r !== null
  },

  // ============ 设置 ============

  async getSettings(): Promise<DesktopSettings | null> {
    return invoke<DesktopSettings>('get_settings')
  },

  async saveSettings(s: DesktopSettings): Promise<boolean> {
    const r = await invoke<void>('save_settings', { newSettings: s })
    return r !== null
  },

  // ============ 系统 ============

  /** 平台信息 */
  async platformInfo(): Promise<PlatformInfo | null> {
    return invoke<PlatformInfo>('platform_info')
  },

  /** 用系统浏览器打开 URL */
  async openUrl(url: string): Promise<void> {
    await invoke<void>('open_url', { url })
  },

  /** 选择文件（返回路径） */
  async pickFile(): Promise<string | null> {
    return invoke<string | null>('pick_file')
  },

  /** 选择文件夹 */
  async pickFolder(): Promise<string | null> {
    return invoke<string | null>('pick_folder')
  },

  // ============ M7: 窗口 / 菜单 ============

  async windowMinimize(): Promise<void> {
    await invoke<void>('window_minimize')
  },

  async windowToggleFullscreen(): Promise<boolean> {
    const r = await invoke<boolean>('window_toggle_fullscreen')
    return Boolean(r)
  },

  async windowHideToTray(): Promise<void> {
    await invoke<void>('window_hide_to_tray')
  },

  // ============ M1 Hills 化：自绘标题栏 ============

  /** 切换最大化/还原，返回当前是否最大化 */
  async windowToggleMaximize(): Promise<boolean> {
    const r = await invoke<boolean>('window_toggle_maximize')
    return Boolean(r)
  },

  /** 查询当前是否最大化 */
  async windowIsMaximized(): Promise<boolean> {
    const r = await invoke<boolean>('window_is_maximized')
    return Boolean(r)
  },

  /** 关闭主窗口（走 CloseRequested，可被"最小化到托盘"拦截） */
  async windowClose(): Promise<void> {
    await invoke<void>('window_close')
  },

  /** 启用/禁用窗口 Mica/Acrylic 特效 */
  async windowSetEffect(enabled: boolean): Promise<void> {
    await invoke<void>('window_set_effect', { enabled })
  },

  // ============ 事件监听 ============

  /** 监听菜单点击事件 */
  async onMenuAction(handler: (actionId: string) => void): Promise<() => void> {
    return listen<string>('menu-action', handler)
  },

  /** 监听文件打开事件（文件关联触发） */
  async onOpenFiles(handler: (paths: string[]) => void): Promise<() => void> {
    return listen<string[]>('open-files', handler)
  },

  /** 监听 Deep Link 事件（nowen-video:// 协议） */
  async onDeepLink(handler: (url: string) => void): Promise<() => void> {
    return listen<string>('deep-link', handler)
  },

  /** 监听更新可用事件 */
  async onUpdateAvailable(handler: (info: UpdateInfo) => void): Promise<() => void> {
    return listen<UpdateInfo>('update-available', handler)
  },
}

export default desktop
