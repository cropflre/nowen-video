import { create } from 'zustand'
import { persist } from 'zustand/middleware'

type Theme = 'dark' | 'light'

interface ThemeState {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
}

/**
 * 应用到 DOM 的副作用
 * 在 <html> 上设置 data-theme 属性，并更新 color-scheme
 */
function applyTheme(theme: Theme) {
  const root = document.documentElement
  root.setAttribute('data-theme', theme)
  root.style.colorScheme = theme

  // 更新 <meta name="theme-color">
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) {
    meta.setAttribute('content', theme === 'dark' ? '#060a13' : '#f5f7fa')
  }
}

/**
 * 检测系统偏好
 */
function getSystemPreference(): Theme {
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
  }
  return 'dark'
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      theme: 'dark', // 默认暗色

      setTheme: (theme) => {
        applyTheme(theme)
        set({ theme })
      },

      toggleTheme: () => {
        const next = get().theme === 'dark' ? 'light' : 'dark'
        applyTheme(next)
        set({ theme: next })
      },
    }),
    {
      name: 'nowen-theme', // localStorage key
      partialize: (state) => ({ theme: state.theme }),
      onRehydrateStorage: () => {
        return (state) => {
          // 如果 localStorage 没有存储过主题，则跟随系统偏好
          const stored = localStorage.getItem('nowen-theme')
          if (!stored) {
            const sys = getSystemPreference()
            state?.setTheme(sys)
          } else {
            // 恢复已存储的主题
            applyTheme(state?.theme ?? 'dark')
          }
        }
      },
    }
  )
)

/**
 * 初始化主题（在应用启动时调用一次）
 * 确保 DOM 状态与 store 一致
 */
export function initTheme() {
  const { theme } = useThemeStore.getState()
  applyTheme(theme)

  // 监听系统偏好变更
  if (typeof window !== 'undefined' && window.matchMedia) {
    const mql = window.matchMedia('(prefers-color-scheme: light)')
    mql.addEventListener('change', (e) => {
      // 只有当用户没有手动选择过主题时才跟随系统
      const stored = localStorage.getItem('nowen-theme')
      if (!stored) {
        useThemeStore.getState().setTheme(e.matches ? 'light' : 'dark')
      }
    })
  }
}
