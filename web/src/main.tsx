import React from 'react'
import ReactDOM from 'react-dom/client'
import { LazyMotion, domAnimation } from 'framer-motion'
import App from './App'
import FluentAppProvider from './components/FluentAppProvider'
import { initTheme } from './stores/theme'
import { initI18n } from './i18n'
import { installSubtitleTrackActivationGuard } from './utils/subtitleTrackActivation'
import './index.css'
import './styles/fluent.css'
import './styles/design-system.css'
import './styles/player-cinema.css'
import './styles/player-speed-menu.css'
import './styles/player-overlay-panels.css'
import './styles/player-design-system.css'

const SW_DEV_RELOAD_KEY = 'nowen-sw-dev-cleanup-reload'
const SW_UPDATE_RELOAD_KEY = 'nowen-sw-production-update-reload'

/**
 * Service Worker 只属于生产 PWA。
 *
 * Vite 开发环境如果曾被生产 SW 接管，会导致 /src/*.tsx、登录路由和热更新请求
 * 持续经过旧 Worker，进而出现旧鉴权代码反复加载、登录后又回到登录页等问题。
 */
async function cleanupDevelopmentServiceWorker() {
  try {
    const registrations = await navigator.serviceWorker.getRegistrations()
    await Promise.all(
      registrations
        .filter((registration) => new URL(registration.scope).origin === window.location.origin)
        .map((registration) => registration.unregister()),
    )

    if ('caches' in window) {
      const keys = await window.caches.keys()
      await Promise.all(
        keys.filter((key) => key.startsWith('nowen-')).map((key) => window.caches.delete(key)),
      )
    }

    // 注销不会立刻解除当前页面的 controller，需要只刷新一次才能彻底脱离旧 SW。
    if (navigator.serviceWorker.controller && sessionStorage.getItem(SW_DEV_RELOAD_KEY) !== '1') {
      sessionStorage.setItem(SW_DEV_RELOAD_KEY, '1')
      window.location.reload()
      return
    }
    sessionStorage.removeItem(SW_DEV_RELOAD_KEY)
  } catch (error) {
    // eslint-disable-next-line no-console
    console.warn('[PWA] 清理开发环境 Service Worker 失败:', error)
  }
}

function registerProductionServiceWorker() {
  // 新 Worker 接管页面后必须刷新一次，否则当前标签仍会继续执行旧 JS 壳，
  // 已删除的菜单和页面会一直保留到用户手动刷新。
  window.addEventListener('pageshow', () => {
    sessionStorage.removeItem(SW_UPDATE_RELOAD_KEY)
  }, { once: true })

  navigator.serviceWorker.addEventListener('controllerchange', () => {
    if (sessionStorage.getItem(SW_UPDATE_RELOAD_KEY) === '1') return
    sessionStorage.setItem(SW_UPDATE_RELOAD_KEY, '1')
    window.location.reload()
  })

  window.addEventListener('load', async () => {
    try {
      const registration = await navigator.serviceWorker.register('/sw.js', {
        updateViaCache: 'none',
      })

      registration.addEventListener('updatefound', () => {
        const worker = registration.installing
        worker?.addEventListener('statechange', () => {
          if (worker.state === 'installed' && navigator.serviceWorker.controller) {
            worker.postMessage({ type: 'SKIP_WAITING' })
          }
        })
      })

      // 每次启动主动检查 SW 更新，避免浏览器长期使用旧鉴权或旧导航逻辑。
      await registration.update()
      registration.waiting?.postMessage({ type: 'SKIP_WAITING' })
    } catch (error) {
      // eslint-disable-next-line no-console
      console.warn('[PWA] Service Worker 注册失败:', error)
    }
  })
}

if ('serviceWorker' in navigator) {
  if (import.meta.env.PROD) {
    registerProductionServiceWorker()
  } else {
    void cleanupDevelopmentServiceWorker()
  }
}

// 动态 blob: WebVTT <track> 默认处于 disabled。先安装兼容层，确保在线字幕和
// 字幕菜单选择都能触发浏览器实际加载并进入 showing 状态。
installSubtitleTrackActivationGuard()

// 在渲染前初始化主题和国际化，避免闪烁
initTheme()
initI18n()

// 标记桌面端运行时（供 CSS 切换透明/Mica 样式）
if (typeof window !== 'undefined') {
  const w = window as unknown as { __TAURI_INTERNALS__?: unknown; __TAURI__?: unknown }
  if (w.__TAURI_INTERNALS__ || w.__TAURI__) {
    document.documentElement.setAttribute('data-runtime', 'tauri')
  }
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <FluentAppProvider>
      <LazyMotion features={domAnimation} strict>
        <App />
      </LazyMotion>
    </FluentAppProvider>
  </React.StrictMode>,
)