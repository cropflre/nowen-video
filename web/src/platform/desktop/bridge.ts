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
  }
  player: {
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
  os: string
  arch: string
  family: string
  is_desktop: boolean
}

export interface UpdateInfo {
  available: boolean
  version: string
  current_version: string
  notes: string
  pub_date: string
}

export interface PlayerStartResult {
  session_id: string
}

export interface PlayerVideoInfo {
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

const IS_DESKTOP = typeof window !== 'undefined' && isTauri()
let cachedEmbeddedServerBase: string | null = null

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

function localStorageServerBase(): string | null {
  if (typeof window === 'undefined') return null
  try {
    const value = window.localStorage.getItem('nowen_server_url')?.trim().replace(/\/+$/, '')
    return value && /^https?:\/\//i.test(value) ? value : null
  } catch {
    return null
  }
}

async function resolveServerBase(): Promise<string | null> {
  const configured = localStorageServerBase()
  if (configured) return configured
  if (!IS_DESKTOP) return null
  if (cachedEmbeddedServerBase) return cachedEmbeddedServerBase

  const settings = await invoke<DesktopSettings>('get_settings')
  if (settings?.server.mode === 'remote') {
    const remote = settings.server.remote_url.trim().replace(/\/+$/, '')
    return /^https?:\/\//i.test(remote) ? remote : null
  }

  for (let attempt = 0; attempt < 30; attempt += 1) {
    const status = await invoke<SidecarStatus>('sidecar_status')
    if (status?.running && status.port > 0) {
      cachedEmbeddedServerBase = `http://127.0.0.1:${status.port}`
      return cachedEmbeddedServerBase
    }
    await new Promise((resolve) => window.setTimeout(resolve, 100))
  }

  return null
}

/** Desktop 2.0 唯一桌面平台适配层。 */
export const desktop = {
  isDesktop: IS_DESKTOP,

  async serverBaseUrl(): Promise<string | null> {
    return resolveServerBase()
  },

  async playerAvailable(): Promise<boolean> {
    return Boolean(await invoke<boolean>('player_available'))
  },

  async playerStart(params: {
    sessionId: string
    url: string
    options?: PlayOptions
  }): Promise<PlayerStartResult | null> {
    return invoke<PlayerStartResult>('player_start', {
      sessionId: params.sessionId,
      url: params.url,
      options: params.options,
    })
  },

  async playerStop(sessionId: string): Promise<void> {
    await invoke<void>('player_stop', { sessionId })
  },

  async playerSyncSurface(params: {
    x: number
    y: number
    width: number
    height: number
    visible: boolean
  }): Promise<boolean> {
    const result = await invoke<void>('player_sync_surface', params)
    return result !== null
  },

  async playerCommand(params: {
    sessionId: string
    command: string
    args?: string[]
  }): Promise<boolean> {
    const result = await invoke<void>('player_command', params)
    return result !== null
  },

  async playerSetProperty(params: {
    sessionId: string
    name: string
    value: string
  }): Promise<boolean> {
    const result = await invoke<void>('player_set_property', params)
    return result !== null
  },

  async playerDestroy(): Promise<void> {
    await invoke<void>('player_destroy')
  },

  async playerVideoInfo(sessionId: string): Promise<PlayerVideoInfo | null> {
    return invoke<PlayerVideoInfo>('player_video_info', { sessionId })
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
    cachedEmbeddedServerBase = null
    const result = await invoke<void>('sidecar_restart')
    return result !== null
  },

  async getSettings(): Promise<DesktopSettings | null> {
    return invoke<DesktopSettings>('get_settings')
  },

  async saveSettings(settings: DesktopSettings): Promise<boolean> {
    cachedEmbeddedServerBase = null
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
