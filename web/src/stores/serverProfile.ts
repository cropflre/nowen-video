import { create } from 'zustand'
import { serverApi } from '@/api/server'
import type { ServerCapability, ServerProfileManifest } from '@/api/server'

const unavailable = (): ServerCapability => ({
  available: false,
  enabled: false,
  configurable: false,
  requires_restart: false,
})

function legacyManifest(features: Record<string, unknown> = {}, profileHint?: string): ServerProfileManifest {
  const profile = profileHint === 'lite' || features.profile === 'lite' ? 'lite' : 'full'
  const enabled = (name: string) => Boolean(features[name])
  const capability = (name: string, available = true): ServerCapability => ({
    available,
    enabled: available && enabled(name),
    configurable: false,
    requires_restart: false,
    mode: available ? 'legacy' : 'unknown',
  })

  return {
    schema_version: 0,
    profile,
    capabilities: {
      library: { available: true, enabled: true, configurable: false, requires_restart: false, mode: 'core' },
      metadata: { available: true, enabled: true, configurable: false, requires_restart: false, mode: 'core' },
      playback: { available: true, enabled: true, configurable: false, requires_restart: false, mode: 'core' },
      transcode: { available: true, enabled: true, configurable: false, requires_restart: false, mode: 'core' },
      subtitles: { available: true, enabled: true, configurable: false, requires_restart: false, mode: 'core' },
      ai: capability('ai_enabled'),
      webdav: capability('webdav'),
      alist: capability('alist'),
      s3: capability('s3'),
      preprocess: capability('preprocess', profile === 'full'),
      emby_compat: capability('emby_compat', profile === 'full'),
      cast: capability('cast', profile === 'full'),
      music: capability('music', profile === 'full'),
      photos: capability('photos', profile === 'full'),
      federation: capability('federation', profile === 'full'),
      plugins: capability('plugins', profile === 'full'),
    },
  }
}

interface ServerProfileState {
  manifest: ServerProfileManifest | null
  loading: boolean
  loaded: boolean
  error: string | null
  load: (force?: boolean) => Promise<void>
  capability: (name: string) => ServerCapability
}

let inflight: Promise<void> | null = null

export const useServerProfileStore = create<ServerProfileState>((set, get) => ({
  manifest: null,
  loading: false,
  loaded: false,
  error: null,

  load: async (force = false) => {
    if (!force && get().loaded) return
    if (!force && inflight) return inflight

    inflight = (async () => {
      set({ loading: true, error: null })
      try {
        const res = await serverApi.capabilities()
        set({ manifest: res.data.data, loaded: true, loading: false })
      } catch {
        try {
          // Full servers and older deployments may only expose /health.
          const res = await serverApi.health()
          const health = res.data.data || res.data
          const manifest = health.capabilities
            ? {
                schema_version: health.schema_version || 1,
                profile: health.profile === 'lite' ? 'lite' : 'full',
                capabilities: health.capabilities,
              } satisfies ServerProfileManifest
            : legacyManifest(health.features || {}, health.profile)
          set({ manifest, loaded: true, loading: false })
        } catch (error) {
          set({
            manifest: null,
            loaded: true,
            loading: false,
            error: error instanceof Error ? error.message : '无法读取服务端能力',
          })
        }
      } finally {
        inflight = null
      }
    })()

    return inflight
  },

  capability: (name: string) => get().manifest?.capabilities[name] || unavailable(),
}))
