import { invoke as tauriInvoke, isTauri } from '@tauri-apps/api/core'
import { listen as tauriListen } from '@tauri-apps/api/event'

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
    hardware_accel: boolean
    engine?: 'auto' | 'mpv' | 'web'
    mpv_path?: string
    mpv_args?: string[]
  }
  window: {
    width: number
    height: number
    remember_size: boolean
    minimize_to_tray: boolean
  }
}

export interface PlatformInfo {
  os: string
  arch: string
  family: string
  is_desktop: boolean
}

interface PlayerAvailability {
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
  hdr: string
  paused: boolean
  volume: number
  mute: boolean
}

export type Anime4KLevel = 'off' | 'low' | 'medium' | 'high'

const IS_DESKTOP = typeof window !== 'undefined' && isTauri()

async function invoke<T>(cmd: string, args?: Record<string, unknown>): Promise<T | null> {
  if (!IS_DESKTOP) return null
  try {
    return await tauriInvoke<T>(cmd, args)
  } catch (error) {
    console.warn(`[desktop] invoke ${cmd} 失败:`, error)
    return null
  }
}

async function listen<T>(event: string, handler: (payload: T) => void): Promise<() => void> {
  if (!IS_DESKTOP) return () => {}
  try {
    return await tauriListen<T>(event, (message) => handler(message.payload))
  } catch (error) {
    console.warn(`[desktop] listen ${event} 失败:`, error)
    return () => {}
  }
}

/**
 * Desktop 2.0 平台适配层。
 *
 * React 业务只能通过这里访问桌面能力。Web/mpv 引擎选择、外部 mpv 进程等
 * Desktop 1.x 实验接口已从产品层删除。
 */
export const desktop = {
  isDesktop: IS_DESKTOP,

  async playerAvailable(): Promise<boolean> {
    const result = await invoke<PlayerAvailability>('mpv_available')
    return Boolean(result?.embed_available)
  },

  async playerStart(params: {
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

  async playerSyncSurface(params: {
    x: number
    y: number
    width: number
    height: number
    visible: boolean
  }): Promise<boolean> {
    const result = await invoke<void>('mpv_embed_sync', params)
    return result !== null
  },

  async playerCommand(params: {
    sessionId: string
    command: string
    args?: string[]
  }): Promise<boolean> {
    const result = await invoke<void>('mpv_embed_command', params)
    return result !== null
  },

  async playerSetProperty(params: {
    sessionId: string
    name: string
    value: string
  }): Promise<boolean> {
    const result = await invoke<void>('mpv_embed_set_property', params)
    return result !== null
  },

  async playerDestroy(): Promise<void> {
    await invoke<void>('mpv_embed_destroy')
  },

  async playerSetAnime4K(params: {
    sessionId: string
    level: Anime4KLevel
  }): Promise<boolean> {
    const result = await invoke<void>('mpv_embed_set_anime4k', params)
    return result !== null
  },

  async playerVideoInfo(sessionId: string): Promise<MpvVideoInfo | null> {
    return invoke<MpvVideoInfo>('mpv_embed_video_info', { sessionId })
  },

  async checkUpdate(): Promise<UpdateInfo | null> {
    return invoke<UpdateInfo>('check_update')
  },

  async installUpdate(): Promise<boolean> {
    const result = await invoke<void>('install_update')
    return result !== null
  },

  async sidecarStatus(): Promise<SidecarStatus | null> {
    return invoke<SidecarStatus>('sidecar_status')
  },

  async sidecarRestart(): Promise<boolean> {
    const result = await invoke<void>('sidecar_restart')
    return result !== null
  },

  async getSettings(): Promise<DesktopSettings | null> {
    return invoke<DesktopSettings>('get_settings')
  },

  async saveSettings(settings: DesktopSettings): Promise<boolean> {
    const result = await invoke<void>('save_settings', { newSettings: settings })
    return result !== null
  },

  async platformInfo(): Promise<PlatformInfo | null> {
    return invoke<PlatformInfo>('platform_info')
  },

  async openUrl(url: string): Promise<void> {
    await invoke<void>('open_url', { url })
  },

  async pickFile(): Promise<string | null> {
    return invoke<string | null>('pick_file')
  },

  async pickFolder(): Promise<string | null> {
    return invoke<string | null>('pick_folder')
  },

  async windowMinimize(): Promise<void> {
    await invoke<void>('window_minimize')
  },

  async windowToggleFullscreen(): Promise<boolean> {
    return Boolean(await invoke<boolean>('window_toggle_fullscreen'))
  },

  async windowHideToTray(): Promise<void> {
    await invoke<void>('window_hide_to_tray')
  },

  async windowToggleMaximize(): Promise<boolean> {
    return Boolean(await invoke<boolean>('window_toggle_maximize'))
  },

  async windowIsMaximized(): Promise<boolean> {
    return Boolean(await invoke<boolean>('window_is_maximized'))
  },

  async windowClose(): Promise<void> {
    await invoke<void>('window_close')
  },

  async windowPipEnter(): Promise<void> {
    await invoke<void>('window_pip_enter')
  },

  async windowPipExit(): Promise<void> {
    await invoke<void>('window_pip_exit')
  },

  async windowPipIsActive(): Promise<boolean> {
    return Boolean(await invoke<boolean>('window_pip_is_active'))
  },

  async windowSetAlwaysOnTop(enabled: boolean): Promise<void> {
    await invoke<void>('window_set_always_on_top', { enabled })
  },

  async windowSetEffect(enabled: boolean): Promise<void> {
    await invoke<void>('window_set_effect', { enabled })
  },

  async onMenuAction(handler: (actionId: string) => void): Promise<() => void> {
    return listen<string>('menu-action', handler)
  },

  async onOpenFiles(handler: (paths: string[]) => void): Promise<() => void> {
    return listen<string[]>('open-files', handler)
  },

  async onDeepLink(handler: (url: string) => void): Promise<() => void> {
    return listen<string>('deep-link', handler)
  },

  async onUpdateAvailable(handler: (info: UpdateInfo) => void): Promise<() => void> {
    return listen<UpdateInfo>('update-available', handler)
  },
}

export default desktop
