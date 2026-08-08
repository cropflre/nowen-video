import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import zhCN from './locales/zh-CN'
import enUS from './locales/en-US'
import jaJP from './locales/ja-JP'

// 支持的语言列表
export const SUPPORTED_LOCALES = [
  { code: 'zh-CN', name: '简体中文', flag: '🇨🇳' },
  { code: 'en-US', name: 'English', flag: '🇺🇸' },
  { code: 'ja-JP', name: '日本語', flag: '🇯🇵' },
] as const

export type LocaleCode = typeof SUPPORTED_LOCALES[number]['code']

const localeMessages: Record<LocaleCode, Record<string, string>> = {
  'zh-CN': zhCN,
  'en-US': enUS,
  'ja-JP': jaJP,
}

// i18n Store
interface I18nStore {
  locale: LocaleCode
  setLocale: (locale: LocaleCode) => void
}

// 检测浏览器语言
function detectBrowserLocale(): LocaleCode {
  const browserLang = navigator.language || (navigator as any).userLanguage || ''
  if (browserLang.startsWith('zh')) return 'zh-CN'
  if (browserLang.startsWith('ja')) return 'ja-JP'
  if (browserLang.startsWith('en')) return 'en-US'
  return 'zh-CN' // 默认中文
}

export const useI18nStore = create<I18nStore>()(
  persist(
    (set) => ({
      locale: detectBrowserLocale(),
      setLocale: (locale: LocaleCode) => {
        set({ locale })
        // 更新 HTML lang 属性
        document.documentElement.lang = locale
      },
    }),
    {
      name: 'nowen-i18n',
    }
  )
)

/**
 * 翻译函数
 * @param key 翻译键
 * @param params 插值参数，如 { count: 5 }
 * @returns 翻译后的文本
 */
export function t(key: string, params?: Record<string, string | number>): string {
  const locale = useI18nStore.getState().locale
  const messages = localeMessages[locale] || localeMessages['zh-CN']
  let text = messages[key] || localeMessages['zh-CN'][key] || key

  // 处理插值参数 {count} -> 5
  if (params) {
    Object.entries(params).forEach(([k, v]) => {
      text = text.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v))
    })
  }

  return text
}

/**
 * React Hook: 获取翻译函数（响应式）
 * 当语言切换时，使用此 hook 的组件会自动重新渲染
 */
export function useTranslation() {
  const locale = useI18nStore((s) => s.locale)
  const setLocale = useI18nStore((s) => s.setLocale)

  const translate = (key: string, params?: Record<string, string | number>): string => {
    const messages = localeMessages[locale] || localeMessages['zh-CN']
    let text = messages[key] || localeMessages['zh-CN'][key] || key

    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        text = text.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v))
      })
    }

    return text
  }

  return { t: translate, locale, setLocale }
}

// 初始化 i18n（App 启动时调用）
export function initI18n() {
  const state = useI18nStore.getState()
  document.documentElement.lang = state.locale
}
