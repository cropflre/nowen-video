import { create } from 'zustand'
import { persist } from 'zustand/middleware'

/**
 * Theme store semantic boundary.
 *
 * The application now has one formal visual system with light/dark modes.
 * Historical theme IDs and custom-theme records are kept in the persisted
 * store shape so existing installations can upgrade without localStorage
 * migration failures, but runtime CSS variables are no longer injected here.
 * Design-system tokens in design-system.css are the single source of truth.
 */
export interface ThemeConfig {
  id: string
  name: string
  author: string
  description: string
  /** Deprecated compatibility payload. Runtime theme application ignores it. */
  vars: Record<string, string>
}

export const builtinThemes: ThemeConfig[] = [
  {
    id: 'neon-dark',
    name: '深色',
    author: 'nowen',
    description: 'Nowen Video 默认深色外观',
    vars: {},
  },
  {
    id: 'pure-light',
    name: '浅色',
    author: 'nowen',
    description: 'Nowen Video 浅色外观',
    vars: {},
  },
]

const LIGHT_THEME_IDS = new Set(['pure-light', 'light'])
const THEME_CHROME_COLOR = {
  dark: '#070b17',
  light: '#f4f6fb',
} as const

let transitionCleanupFrame: number | null = null

function resolveMode(themeId: string): 'dark' | 'light' {
  return LIGHT_THEME_IDS.has(themeId) ? 'light' : 'dark'
}

function canonicalThemeId(themeId: string): string {
  return resolveMode(themeId) === 'light' ? 'pure-light' : 'neon-dark'
}

function syncBrowserThemeChrome(mode: 'dark' | 'light') {
  const themeColor = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]')
  themeColor?.setAttribute('content', THEME_CHROME_COLOR[mode])

  // Standalone/PWA browser chrome should visually follow the application mode.
  // This does not affect the cinematic player surface itself.
  const appleStatusBar = document.querySelector<HTMLMetaElement>('meta[name="apple-mobile-web-app-status-bar-style"]')
  appleStatusBar?.setAttribute('content', mode === 'light' ? 'default' : 'black-translucent')
  document.documentElement.style.colorScheme = mode
}

/**
 * Theme token swaps can touch hundreds of painted elements on media-heavy pages.
 * Temporarily suppress per-component transitions so the browser performs one
 * visual theme repaint instead of animating every background/border/shadow.
 * The class is removed after the new theme has painted twice.
 */
function runWithoutThemeTransitions(update: () => void) {
  const root = document.documentElement
  root.classList.add('no-theme-transition')
  update()

  if (transitionCleanupFrame !== null) {
    cancelAnimationFrame(transitionCleanupFrame)
  }

  transitionCleanupFrame = requestAnimationFrame(() => {
    transitionCleanupFrame = requestAnimationFrame(() => {
      root.classList.remove('no-theme-transition')
      transitionCleanupFrame = null
    })
  })
}

interface ThemeStore {
  currentThemeId: string
  theme: 'dark' | 'light'
  customThemes: ThemeConfig[]
  setTheme: (id: string) => void
  toggleTheme: () => void
  addCustomTheme: (theme: ThemeConfig) => void
  removeCustomTheme: (id: string) => void
  getAllThemes: () => ThemeConfig[]
  getCurrentTheme: () => ThemeConfig | undefined
}

export const useThemeStore = create<ThemeStore>()(
  persist(
    (set, get) => ({
      currentThemeId: 'neon-dark',
      theme: 'dark' as const,
      customThemes: [],

      setTheme: (id: string) => {
        const mode = resolveMode(id)
        const nextId = canonicalThemeId(id)
        set({ currentThemeId: nextId, theme: mode })
        applyTheme(nextId)
      },

      toggleTheme: () => {
        const nextMode = get().theme === 'dark' ? 'light' : 'dark'
        const nextId = nextMode === 'light' ? 'pure-light' : 'neon-dark'
        set({ currentThemeId: nextId, theme: nextMode })
        applyTheme(nextId)
      },

      // Compatibility APIs remain so old persisted state/import code cannot
      // crash an upgrade. Custom color variable payloads are intentionally not
      // applied: cyan remains the functional brand color across all modes.
      addCustomTheme: (theme: ThemeConfig) => {
        set((state) => ({
          customThemes: [...state.customThemes.filter((item) => item.id !== theme.id), theme],
        }))
      },

      removeCustomTheme: (id: string) => {
        set((state) => ({
          customThemes: state.customThemes.filter((item) => item.id !== id),
          currentThemeId: state.currentThemeId === id ? 'neon-dark' : state.currentThemeId,
          theme: state.currentThemeId === id ? 'dark' : state.theme,
        }))
        if (get().currentThemeId === 'neon-dark') applyTheme('neon-dark')
      },

      getAllThemes: () => [...builtinThemes, ...get().customThemes],

      getCurrentTheme: () => {
        const id = canonicalThemeId(get().currentThemeId)
        return builtinThemes.find((theme) => theme.id === id)
      },
    }),
    {
      name: 'nowen-theme',
      version: 2,
      migrate: (persistedState: unknown) => {
        const state = (persistedState && typeof persistedState === 'object'
          ? persistedState
          : {}) as Partial<ThemeStore>
        const currentThemeId = canonicalThemeId(state.currentThemeId || 'neon-dark')
        return {
          ...state,
          currentThemeId,
          theme: resolveMode(currentThemeId),
          customThemes: Array.isArray(state.customThemes) ? state.customThemes : [],
        } as ThemeStore
      },
    },
  ),
)

/** Apply only the formal light/dark mode. CSS owns all application color tokens. */
export function applyTheme(themeId: string) {
  if (typeof document === 'undefined') return
  const mode = resolveMode(themeId)
  const root = document.documentElement

  if (root.getAttribute('data-theme') === mode) {
    syncBrowserThemeChrome(mode)
    return
  }

  runWithoutThemeTransitions(() => {
    root.setAttribute('data-theme', mode)
    syncBrowserThemeChrome(mode)
  })
}

/** Initialize the formal mode before React renders to avoid a theme flash. */
export function initTheme() {
  const state = useThemeStore.getState()
  const canonicalId = canonicalThemeId(state.currentThemeId)
  const mode = resolveMode(canonicalId)

  if (state.currentThemeId !== canonicalId || state.theme !== mode) {
    useThemeStore.setState({ currentThemeId: canonicalId, theme: mode })
  }

  // Startup runs before the app is painted, so there is no need to allocate
  // two animation frames just to suppress transitions that cannot be visible.
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', mode)
    syncBrowserThemeChrome(mode)
  }
}
