// Geometry, clamping and storage rules for the non-modal viewer assistant
// window. Everything here is pure so the same rules drive the UI, the unit
// tests and the Playwright checks.

export type AssistantVisibility = 'expanded' | 'minimized' | 'hidden'

export interface AssistantPoint {
  x: number
  y: number
}

export interface AssistantSize {
  width: number
  height: number
}

export interface AssistantViewport {
  width: number
  height: number
  /** env(safe-area-inset-bottom) in pixels; keeps the panel above notches. */
  safeBottom?: number
}

export interface AssistantInsets {
  top: number
  right: number
  bottom: number
  left: number
}

export interface AssistantWindowPreferences {
  visibility: AssistantVisibility
  position: AssistantPoint
  size: AssistantSize
  /** Where the minimized launcher sits; kept apart from the window position. */
  launcher: AssistantPoint
}

export const ASSISTANT_DEFAULT_WIDTH = 420
export const ASSISTANT_DEFAULT_HEIGHT = 620
export const ASSISTANT_MIN_WIDTH = 320
export const ASSISTANT_MAX_WIDTH = 720
export const ASSISTANT_MIN_HEIGHT = 300
export const ASSISTANT_EDGE = 12
export const ASSISTANT_MIN_VISIBLE = 96
export const ASSISTANT_LAUNCHER_WIDTH = 52
export const ASSISTANT_LAUNCHER_HEIGHT = 44

export interface AssistantLauncherSize {
  width: number
  height: number
}

const VISIBILITIES: AssistantVisibility[] = ['expanded', 'minimized', 'hidden']

export function isAssistantVisibility(value: unknown): value is AssistantVisibility {
  return typeof value === 'string' && (VISIBILITIES as string[]).includes(value)
}

function finite(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function clamp(value: number, min: number, max: number) {
  if (max < min) return min
  return Math.min(Math.max(value, min), max)
}

export function assistantPreferencesKey(userId: string) {
  return `nowen-assistant-window:v1:${userId}`
}

/**
 * The bottom inset leaves room for the mobile bottom navigation so the window
 * and the launcher never sit on top of it.
 */
export function assistantInsets(viewport: AssistantViewport): AssistantInsets {
  const compact = viewport.width < 768
  const safeBottom = Math.max(0, finite(viewport.safeBottom) ?? 0)
  return { top: ASSISTANT_EDGE, right: ASSISTANT_EDGE, left: ASSISTANT_EDGE, bottom: compact ? 82 + safeBottom : ASSISTANT_EDGE }
}

function sizeBudget(viewport: AssistantViewport, insets: AssistantInsets) {
  return {
    maxWidth: Math.max(120, viewport.width - insets.left - insets.right),
    maxHeight: Math.max(160, viewport.height - insets.top - insets.bottom),
  }
}

export function clampAssistantSize(size: AssistantSize, viewport: AssistantViewport): AssistantSize {
  const insets = assistantInsets(viewport)
  const budget = sizeBudget(viewport, insets)
  const maxWidth = Math.min(ASSISTANT_MAX_WIDTH, budget.maxWidth)
  const minWidth = Math.min(ASSISTANT_MIN_WIDTH, maxWidth)
  const maxHeight = budget.maxHeight
  const minHeight = Math.min(ASSISTANT_MIN_HEIGHT, maxHeight)
  const width = finite(size.width) ?? ASSISTANT_DEFAULT_WIDTH
  const height = finite(size.height) ?? ASSISTANT_DEFAULT_HEIGHT
  return {
    width: Math.round(clamp(width, minWidth, maxWidth)),
    height: Math.round(clamp(height, minHeight, maxHeight)),
  }
}

export function defaultAssistantSize(viewport: AssistantViewport): AssistantSize {
  return clampAssistantSize({ width: ASSISTANT_DEFAULT_WIDTH, height: ASSISTANT_DEFAULT_HEIGHT }, viewport)
}

/**
 * Keeps at least `minVisible` pixels of the window inside the viewport, so a
 * drag can never strand the title bar off screen.
 */
export function clampAssistantPosition(position: AssistantPoint, size: AssistantSize, viewport: AssistantViewport): AssistantPoint {
  const insets = assistantInsets(viewport)
  const keepX = Math.min(ASSISTANT_MIN_VISIBLE, size.width)
  const keepY = Math.min(ASSISTANT_MIN_VISIBLE, size.height)
  const minX = insets.left - size.width + keepX
  const maxX = viewport.width - insets.right - keepX
  const minY = insets.top
  const maxY = viewport.height - insets.bottom - keepY
  const x = finite(position.x) ?? minX
  const y = finite(position.y) ?? minY
  return { x: Math.round(clamp(x, minX, maxX)), y: Math.round(clamp(y, minY, maxY)) }
}

export function defaultAssistantPosition(size: AssistantSize, viewport: AssistantViewport): AssistantPoint {
  return containAssistantPosition(
    { x: viewport.width - assistantInsets(viewport).right - size.width, y: viewport.height - assistantInsets(viewport).bottom - size.height },
    size,
    viewport,
  )
}

/**
 * Unlike {@link clampAssistantPosition}, this pulls the whole panel inside the
 * viewport. Restoring, resizing or rotating the screen should never leave the
 * window hanging over an edge; an active drag is allowed to leave a handle.
 */
export function containAssistantPosition(position: AssistantPoint, size: AssistantSize, viewport: AssistantViewport): AssistantPoint {
  const insets = assistantInsets(viewport)
  const minX = insets.left
  const maxX = Math.max(minX, viewport.width - insets.right - size.width)
  const minY = insets.top
  const maxY = Math.max(minY, viewport.height - insets.bottom - size.height)
  const x = finite(position.x) ?? minX
  const y = finite(position.y) ?? minY
  return { x: Math.round(clamp(x, minX, maxX)), y: Math.round(clamp(y, minY, maxY)) }
}

export function moveAssistant(start: AssistantPoint, delta: AssistantPoint, size: AssistantSize, viewport: AssistantViewport): AssistantPoint {
  return clampAssistantPosition({ x: start.x + delta.x, y: start.y + delta.y }, size, viewport)
}

export function resizeAssistant(start: AssistantSize, delta: AssistantPoint, viewport: AssistantViewport): AssistantSize {
  return clampAssistantSize({ width: start.width + delta.x, height: start.height + delta.y }, viewport)
}

/**
 * The launcher is content sized, so callers pass its measured box; the defaults
 * only cover the first paint and the case where nothing has been measured yet.
 */
export function clampAssistantLauncher(position: AssistantPoint, viewport: AssistantViewport, size: AssistantLauncherSize = { width: ASSISTANT_LAUNCHER_WIDTH, height: ASSISTANT_LAUNCHER_HEIGHT }): AssistantPoint {
  const insets = assistantInsets(viewport)
  const width = Math.max(1, finite(size.width) ?? ASSISTANT_LAUNCHER_WIDTH)
  const height = Math.max(1, finite(size.height) ?? ASSISTANT_LAUNCHER_HEIGHT)
  const minX = insets.left
  const maxX = Math.max(minX, viewport.width - insets.right - width)
  const minY = insets.top
  const maxY = Math.max(minY, viewport.height - insets.bottom - height)
  const x = finite(position.x) ?? maxX
  const y = finite(position.y) ?? maxY
  return { x: Math.round(clamp(x, minX, maxX)), y: Math.round(clamp(y, minY, maxY)) }
}

export function defaultAssistantLauncherPosition(viewport: AssistantViewport, size?: AssistantLauncherSize): AssistantPoint {
  return clampAssistantLauncher({ x: Number.NaN, y: Number.NaN }, viewport, size)
}

export function defaultAssistantPreferences(viewport: AssistantViewport): AssistantWindowPreferences {
  const size = defaultAssistantSize(viewport)
  return {
    visibility: 'minimized',
    size,
    position: defaultAssistantPosition(size, viewport),
    launcher: defaultAssistantLauncherPosition(viewport),
  }
}

/** Never trusts stored JSON: any malformed field falls back to the default. */
export function normalizeAssistantPreferences(raw: unknown, viewport: AssistantViewport): AssistantWindowPreferences {
  const fallback = defaultAssistantPreferences(viewport)
  if (!raw || typeof raw !== 'object') return fallback
  const source = raw as Partial<Record<keyof AssistantWindowPreferences, unknown>>
  const size = clampAssistantSize(
    {
      width: finite((source.size as AssistantSize | undefined)?.width) ?? fallback.size.width,
      height: finite((source.size as AssistantSize | undefined)?.height) ?? fallback.size.height,
    },
    viewport,
  )
  return {
    visibility: isAssistantVisibility(source.visibility) ? source.visibility : fallback.visibility,
    size,
    position: containAssistantPosition(
      {
        x: finite((source.position as AssistantPoint | undefined)?.x) ?? fallback.position.x,
        y: finite((source.position as AssistantPoint | undefined)?.y) ?? fallback.position.y,
      },
      size,
      viewport,
    ),
    launcher: clampAssistantLauncher(
      {
        x: finite((source.launcher as AssistantPoint | undefined)?.x) ?? fallback.launcher.x,
        y: finite((source.launcher as AssistantPoint | undefined)?.y) ?? fallback.launcher.y,
      },
      viewport,
    ),
  }
}

interface PreferenceStorage {
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => void
}

/**
 * Storage is best effort: a private-mode browser, a quota error or a disabled
 * localStorage must never break the assistant.
 */
export function readAssistantPreferences(storage: PreferenceStorage | null | undefined, userId: string, viewport: AssistantViewport): AssistantWindowPreferences {
  if (!storage || !userId) return defaultAssistantPreferences(viewport)
  try {
    const raw = storage.getItem(assistantPreferencesKey(userId))
    if (!raw) return defaultAssistantPreferences(viewport)
    return normalizeAssistantPreferences(JSON.parse(raw), viewport)
  } catch {
    return defaultAssistantPreferences(viewport)
  }
}

export function writeAssistantPreferences(storage: PreferenceStorage | null | undefined, userId: string, preferences: AssistantWindowPreferences) {
  if (!storage || !userId) return
  try {
    storage.setItem(assistantPreferencesKey(userId), JSON.stringify(preferences))
  } catch {
    // Ignore: the window still works, it just will not be restored.
  }
}
