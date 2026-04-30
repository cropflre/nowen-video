/**
 * FluentAppProvider
 *
 * 为整个应用注入 Fluent v9 设计系统。
 * - 以「深空霓虹」为基础构建自定义 brandVariants
 * - 让 Fluent 组件与项目既有的 `--neon-blue` / `--bg-base` 等令牌无缝对齐
 * - 只有包裹在 Provider 内的 Fluent 组件才能吃到主题
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

/**
 * 项目霓虹色（neon-blue #00F0FF）派生的 Fluent BrandVariants
 * 10 → 最浅，160 → 最深。标准配方见 Fluent v9 文档：
 * https://react.fluentui.dev/?path=/docs/theme-colors--docs
 */
const neonBrand: BrandVariants = {
  10: '#001014',
  20: '#00242B',
  30: '#003A44',
  40: '#00525F',
  50: '#006A7A',
  60: '#008497',
  70: '#009FB4',
  80: '#00BAD2',
  90: '#00D6F0',   // 贴近 --neon-blue
  100: '#2AE6FF',
  110: '#55EDFF',
  120: '#7AF0FF',
  130: '#9CF3FF',
  140: '#BAF6FF',
  150: '#D4F9FF',
  160: '#EBFCFF',
}

const nowenDarkTheme: Theme = {
  ...createDarkTheme(neonBrand),
  // 让 Fluent 面板背景透出底层 Mica
  colorNeutralBackground1: 'rgba(11, 17, 32, 0.55)',
  colorNeutralBackground2: 'rgba(11, 17, 32, 0.65)',
  colorNeutralBackground3: 'rgba(11, 17, 32, 0.75)',
  colorNeutralBackground1Hover: 'rgba(0, 240, 255, 0.05)',
  colorNeutralBackground1Pressed: 'rgba(0, 240, 255, 0.08)',
  colorNeutralBackground1Selected: 'rgba(0, 240, 255, 0.10)',
  colorNeutralStroke1: 'rgba(0, 240, 255, 0.12)',
  colorNeutralStroke2: 'rgba(0, 240, 255, 0.08)',
  colorNeutralForeground1: '#ffffff',
  colorNeutralForeground2: '#9fb3c8',
  colorNeutralForeground3: '#627d98',
  colorBrandBackground: '#00BAD2',
  colorBrandBackgroundHover: '#00D6F0',
  colorBrandForeground1: '#00E6FF',
  colorBrandForeground2: '#7AF0FF',
  // 阴影与霓虹发光统一
  shadow8: '0 4px 12px rgba(0, 240, 255, 0.10)',
  shadow16: '0 8px 24px rgba(0, 240, 255, 0.12)',
  shadow28: '0 12px 36px rgba(0, 240, 255, 0.15)',
  // 字体族：桌面端优先使用 Segoe UI Variable，其次思源黑体 CN
  fontFamilyBase:
    '"Segoe UI Variable", "Segoe UI", "Source Han Sans CN", "PingFang SC", "Microsoft YaHei", sans-serif',
}

const nowenLightTheme: Theme = {
  ...createLightTheme(neonBrand),
  fontFamilyBase: nowenDarkTheme.fontFamilyBase,
}

// 兼容后备：万一自定义主题出问题就回落官方
void webDarkTheme
void webLightTheme

/** 根据 <html data-theme="..."> 决定当前主题 */
function readTheme(): 'dark' | 'light' {
  if (typeof document === 'undefined') return 'dark'
  const attr = document.documentElement.getAttribute('data-theme')
  return attr === 'light' ? 'light' : 'dark'
}

export function FluentAppProvider({ children }: { children: ReactNode }) {
  const [mode, setMode] = useState<'dark' | 'light'>(() => readTheme())

  useEffect(() => {
    // 监听主题切换（stores/theme 会切换 data-theme）
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
