import React from 'react'
import ReactDOM from 'react-dom/client'
import { LazyMotion, domAnimation } from 'framer-motion'
import App from './App'
import FluentAppProvider from './components/FluentAppProvider'
import { initTheme } from './stores/theme'
import { initI18n } from './i18n'
import { installSubtitleTrackActivationGuard } from './utils/subtitleTrackActivation'
import './styles/global.css'
import './styles/fluent.css'
import './styles/design-system.css'
import './styles/admin-design-system.css'
import './styles/neo-aurora.css'
import './styles/player-cinema.css'
import './styles/player-overlay-panels.css'
import './styles/player-design-system.css'
import './styles/player-subtitles.css'
import './styles/player-navi.css'

const SW_DEV_RELOAD_KEY = 'nowen-sw-dev-cleanup-reload'
const SW_UPDATE_RELOAD_KEY = 'nowen-sw-production-update-reload'

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
      await Promise.all(keys.filter((key) => key.startsWith('nowen-')).map((key) => window.caches.delete(key)))
    }

    if (navigator.serviceWorker.controller && sessionStorage.getItem(SW_DEV_RELOAD_KEY) !== '1') {
      sessionStorage.setItem(SW_DEV_RELOAD_KEY, '1')
      window.location.reload()
      return
    }
    sessionStorage.removeItem(SW_DEV_RELOAD_KEY)
  } catch (error) {
    console.warn('[PWA] 清理开发环境 Service Worker 失败:', error)
  }
}

function registerProductionServiceWorker() {
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
      const registration = await navigator.serviceWorker.register('/sw.js', { updateViaCache: 'none' })
      registration.addEventListener('updatefound', () => {
        const worker = registration.installing
        worker?.addEventListener('statechange', () => {
          if (worker.state === 'installed' && navigator.serviceWorker.controller) worker.postMessage({ type: 'SKIP_WAITING' })
        })
      })
      await registration.update()
      registration.waiting?.postMessage({ type: 'SKIP_WAITING' })
    } catch (error) {
      console.warn('[PWA] Service Worker 注册失败:', error)
    }
  })
}

if ('serviceWorker' in navigator) {
  if (import.meta.env.PROD) registerProductionServiceWorker()
  else void cleanupDevelopmentServiceWorker()
}

installSubtitleTrackActivationGuard()
initTheme()
initI18n()

if (typeof window !== 'undefined') {
  const w = window as unknown as { __TAURI_INTERNALS__?: unknown; __TAURI__?: unknown }
  if (w.__TAURI_INTERNALS__ || w.__TAURI__) document.documentElement.setAttribute('data-runtime', 'tauri')
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
