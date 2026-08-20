/**
 * FluentAppProvider
 *
 * Fluent remains the implementation layer for complex admin/form controls,
 * while Nowen semantic tokens remain the visual source of truth.
 */
import { ReactNode, startTransition, useEffect, useRef, useState } from 'react'
import {
  FluentProvider,
  Theme,
  webDarkTheme,
  webLightTheme,
  BrandVariants,
  createDarkTheme,
  createLightTheme,
} from '@fluentui/react-components'

const nowenBrand: BrandVariants = {
  10: '#120B35',
  20: '#1E1254',
  30: '#2A1972',
  40: '#382291',
  50: '#462EB0',
  60: '#553AD0',
  70: '#6248EA',
  80: '#7057F5',
  90: '#755CFF',
  100: '#8A72FF',
  110: '#A18CFF',
  120: '#B8A7FF',
  130: '#CEC4FF',
  140: '#DED7FF',
  150: '#ECE8FF',
  160: '#F7F5FF',
}

const semanticFluentOverrides: Partial<Theme> = {
  colorNeutralBackground1: 'var(--nv-bg-elevated)',
  colorNeutralBackground2: 'var(--nv-bg-surface)',
  colorNeutralBackground3: 'var(--nv-bg-surface-soft)',
  colorNeutralBackground1Hover: 'var(--nv-bg-hover)',
  colorNeutralBackground1Pressed: 'var(--nv-bg-active)',
  colorNeutralBackground1Selected: 'var(--nv-bg-active)',
  colorNeutralStroke1: 'var(--nv-border-default)',
  colorNeutralStroke2: 'var(--nv-border-subtle)',
  colorNeutralForeground1: 'var(--nv-text-primary)',
  colorNeutralForeground2: 'var(--nv-text-secondary)',
  colorNeutralForeground3: 'var(--nv-text-tertiary)',
  colorBrandBackground: 'var(--nv-action-primary)',
  colorBrandBackgroundHover: 'var(--nv-action-primary-hover)',
  colorBrandBackgroundPressed: 'var(--nv-action-primary-active)',
  colorBrandForeground1: 'var(--nv-action-primary)',
  colorBrandForeground2: 'var(--nv-action-muted)',
  colorStrokeFocus2: 'var(--nv-focus-ring)',
  shadow8: 'var(--nv-shadow-card)',
  shadow16: 'var(--nv-shadow-card-hover)',
  shadow28: 'var(--nv-shadow-elevated)',
  fontFamilyBase: 'var(--nv-font-sans)',
}

const nowenDarkTheme: Theme = {
  ...createDarkTheme(nowenBrand),
  ...semanticFluentOverrides,
}

const nowenLightTheme: Theme = {
  ...createLightTheme(nowenBrand),
  ...semanticFluentOverrides,
}

// Explicit fallback imports keep the provider resilient if custom theme creation changes upstream.
void webDarkTheme
void webLightTheme

function readTheme(): 'dark' | 'light' {
  if (typeof document === 'undefined') return 'dark'
  return document.documentElement.getAttribute('data-theme') === 'light' ? 'light' : 'dark'
}

export function FluentAppProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<'dark' | 'light'>(() => readTheme())
  const modeRef = useRef(mode)
  const syncFrameRef = useRef<number | null>(null)

  useEffect(() => {
    const scheduleFluentThemeSync = () => {
      const nextMode = readTheme()
      if (nextMode === modeRef.current) return

      if (syncFrameRef.current !== null) {
        cancelAnimationFrame(syncFrameRef.current)
      }

      // CSS semantic variables repaint the visible app immediately. Fluent's
      // context update is intentionally moved out of that critical paint so a
      // provider-wide token refresh cannot make the click feel sticky.
      syncFrameRef.current = requestAnimationFrame(() => {
        syncFrameRef.current = requestAnimationFrame(() => {
          syncFrameRef.current = null
          modeRef.current = nextMode
          startTransition(() => setMode(nextMode))
        })
      })
    }

    const observer = new MutationObserver(scheduleFluentThemeSync)
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    })

    return () => {
      observer.disconnect()
      if (syncFrameRef.current !== null) cancelAnimationFrame(syncFrameRef.current)
    }
  }, [])

  return (
    <FluentProvider
      theme={mode === 'dark' ? nowenDarkTheme : nowenLightTheme}
      applyStylesToPortals
      style={{ background: 'transparent', minHeight: '100vh' }}
    >
      {children}
    </FluentProvider>
  )
}

export default FluentAppProvider
