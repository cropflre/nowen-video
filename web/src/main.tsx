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
import './styles/neo-aurora-components.css'
import './styles/player-cinema.css'
import './styles/player-overlay-panels.css'
import './styles/player-design-system.css'
import './styles/player-subtitles.css'
import './styles/player-navi.css'
import './styles/neo-aurora-responsive.css'
import './styles/neo-aurora-light.css'
import './styles/modern-cinema.css'
import './styles/modern-cinema-reference.css'
import './styles/modern-cinema-light-fix.css'
import './styles/media-detail-cinema.css'
import './styles/media-detail-shell-fix.css'
import './styles/admin-menu-layer-fix.css'

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
    console.warn('[PWA] Failed to cleanup development service worker state:', error)
  }
}

async function registerProductionServiceWorker() {
  try {
    const registration = await navigator.serviceWorker.register('/sw.js', { scope: '/' })

    const activateUpdate = (worker: ServiceWorker | null) => {
      if (!worker || worker.state !== 'installed' || !navigator.serviceWorker.controller) return
      worker.postMessage({ type: 'SKIP_WAITING' })
    }

    registration.addEventListener('updatefound', () => {
      const installing = registration.installing
      installing?.addEventListener('statechange', () => activateUpdate(installing))
    })

    if (registration.waiting && navigator.serviceWorker.controller) {
      registration.waiting.postMessage({ type: 'SKIP_WAITING' })
    }

    navigator.serviceWorker.addEventListener('controllerchange', () => {
      if (sessionStorage.getItem(SW_UPDATE_RELOAD_KEY) === '1') return
      sessionStorage.setItem(SW_UPDATE_RELOAD_KEY, '1')
      window.location.reload()
    })

    window.addEventListener('load', () => {
      sessionStorage.removeItem(SW_UPDATE_RELOAD_KEY)
      void registration.update()
    }, { once: true })
  } catch (error) {
    console.warn('[PWA] Failed to register service worker:', error)
  }
}

if ('serviceWorker' in navigator) {
  if (import.meta.env.DEV) void cleanupDevelopmentServiceWorker()
  else void registerProductionServiceWorker()
}

installSubtitleTrackActivationGuard()

initTheme()
initI18n()

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <LazyMotion features={domAnimation} strict>
      <FluentAppProvider>
        <App />
      </FluentAppProvider>
    </LazyMotion>
  </React.StrictMode>,
)
