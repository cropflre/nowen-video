import { create } from 'zustand'
import {
  clampAssistantLauncher,
  containAssistantPosition,
  clampAssistantSize,
  defaultAssistantLauncherPosition,
  defaultAssistantPosition,
  defaultAssistantSize,
  normalizeAssistantPreferences,
  readAssistantPreferences,
  writeAssistantPreferences,
  type AssistantPoint,
  type AssistantSize,
  type AssistantViewport,
  type AssistantVisibility,
  type AssistantLauncherSize,
  type AssistantWindowPreferences,
} from '@/utils/assistantWindow'

/** Reads env(safe-area-inset-bottom) so the panel never sits under a notch. */
let safeAreaCache: { key: string; value: number } | null = null
function safeAreaBottom() {
  if (typeof document === 'undefined' || !document.body) return 0
  const key = `${window.innerWidth}x${window.innerHeight}`
  if (safeAreaCache?.key === key) return safeAreaCache.value
  const probe = document.createElement('div')
  probe.style.cssText = 'position:fixed;visibility:hidden;pointer-events:none;height:0;padding-bottom:env(safe-area-inset-bottom,0px)'
  document.body.appendChild(probe)
  const value = Number.parseFloat(getComputedStyle(probe).paddingBottom) || 0
  probe.remove()
  const safe = Number.isFinite(value) ? value : 0
  safeAreaCache = { key, value: safe }
  return safe
}

export function assistantViewport(): AssistantViewport {
  if (typeof window === 'undefined') return { width: 1024, height: 768 }
  return { width: window.innerWidth, height: window.innerHeight, safeBottom: safeAreaBottom() }
}

function storage(): Storage | null {
  if (typeof window === 'undefined') return null
  try {
    return window.localStorage
  } catch {
    return null
  }
}

interface AssistantWindowState {
  userId: string
  preferences: AssistantWindowPreferences
  /** Reads this user's saved window state; safe to call more than once. */
  hydrate: (userId: string, viewport?: AssistantViewport) => void
  setVisibility: (visibility: AssistantVisibility) => void
  setPosition: (position: AssistantPoint) => void
  setSize: (size: AssistantSize) => void
  setLauncher: (position: AssistantPoint, size?: AssistantLauncherSize) => void
  resetWindow: () => void
  syncViewport: (viewport?: AssistantViewport) => void
}

function persist(userId: string, preferences: AssistantWindowPreferences) {
  writeAssistantPreferences(storage(), userId, preferences)
}

const initialViewport = assistantViewport()
const initialPreferences = normalizeAssistantPreferences(null, initialViewport)

export const useAssistantWindowStore = create<AssistantWindowState>((set, get) => ({
  userId: '',
  preferences: initialPreferences,

  hydrate: (userId, viewport = assistantViewport()) => {
    if (!userId) return
    const saved = readAssistantPreferences(storage(), userId, viewport)
    set({ userId, preferences: saved })
  },

  setVisibility: (visibility) => {
    const { userId, preferences } = get()
    const next = { ...preferences, visibility }
    set({ preferences: next })
    persist(userId, next)
  },

  setPosition: (position) => {
    const { userId, preferences } = get()
    const viewport = assistantViewport()
    // A finished drag settles fully inside the viewport so the header controls
    // never end up beyond the screen edge.
    const next = { ...preferences, position: containAssistantPosition(position, preferences.size, viewport) }
    set({ preferences: next })
    persist(userId, next)
  },

  setSize: (size) => {
    const { userId, preferences } = get()
    const viewport = assistantViewport()
    const clamped = clampAssistantSize(size, viewport)
    // Growing the panel must not push it past an edge either.
    const position = containAssistantPosition(preferences.position, clamped, viewport)
    const next = { ...preferences, size: clamped, position }
    set({ preferences: next })
    persist(userId, next)
  },

  setLauncher: (position, size) => {
    const { userId, preferences } = get()
    const next = { ...preferences, launcher: clampAssistantLauncher(position, assistantViewport(), size) }
    set({ preferences: next })
    persist(userId, next)
  },

  resetWindow: () => {
    const { userId, preferences } = get()
    const viewport = assistantViewport()
    const size = defaultAssistantSize(viewport)
    const next = {
      ...preferences,
      size,
      position: defaultAssistantPosition(size, viewport),
      launcher: defaultAssistantLauncherPosition(viewport),
    }
    set({ preferences: next })
    persist(userId, next)
  },

  syncViewport: (viewport = assistantViewport()) => {
    const { userId, preferences } = get()
    const size = clampAssistantSize(preferences.size, viewport)
    const next = {
      ...preferences,
      size,
      position: containAssistantPosition(preferences.position, size, viewport),
      launcher: clampAssistantLauncher(preferences.launcher, viewport),
    }
    if (next.size.width === preferences.size.width && next.size.height === preferences.size.height && next.position.x === preferences.position.x && next.position.y === preferences.position.y && next.launcher.x === preferences.launcher.x && next.launcher.y === preferences.launcher.y) return
    set({ preferences: next })
    persist(userId, next)
  },
}))
