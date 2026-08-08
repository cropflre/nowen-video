/**
 * FluentAppProvider
 *
 * Fluent remains the implementation layer for complex admin/form controls,
 * while Nowen semantic tokens remain the visual source of truth.
 */
import { ReactNode, useEffect, useState } from 'react'
import {
  FluentProvider,
  Theme,
  webDarkTheme,
  webLightTheme,
  BrandVariants,
  createDarkTheme,
  createLightTheme,
} from '@fluentui/react-components'

const nowenCyanBrand: BrandVariants = {
  10: '#001014',
  20: '#00242B',
  30: '#003A44',
  40: '#00525F',
  50: '#006A7A',
  60: '#008497',
  70: '#009FB4',
  80: '#00BAD2',
  90: '#06B6D4',
  100: '#22D3EE',
  110: '#67E8F9',
  120: '#A5F3FC',
  130: '#CFFAFE',
  140: '#DDFBFF',
  150: '#E8FCFF',
  160: '#F2FDFF',
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
  colorBrandForeground2: 'var(--nv-action-primary)',
  colorStrokeFocus2: 'var(--nv-focus-ring)',
  shadow8: 'var(--nv-shadow-card)',
  shadow16: 'var(--nv-shadow-card-hover)',
  shadow28: 'var(--nv-shadow-elevated)',
  fontFamilyBase: 'var(--nv-font-sans)',
}

const nowenDarkTheme: Theme = {
  ...createDarkTheme(nowenCyanBrand),
  ...semanticFluentOverrides,
}

const nowenLightTheme: Theme = {
  ...createLightTheme(nowenCyanBrand),
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

  useEffect(() => {
    const observer = new MutationObserver(() => {
      setMode(readTheme())
    })
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme'],
    })
    return () => observer.disconnect()
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
