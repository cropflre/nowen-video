import React from 'react'
import ReactDOM from 'react-dom/client'
import { LazyMotion, domAnimation } from 'framer-motion'
import App from './App'
import FluentAppProvider from './components/FluentAppProvider'
import { initTheme } from './stores/theme'
import { initI18n } from './i18n'
import { installSubtitleTrackActivationGuard } from './utils/subtitleTrackActivation'
import { installMediaDetailHeroEnhancer } from './utils/mediaDetailHeroEnhancer'
import './styles/global.css'
import './styles/fluent.css'
import './styles/design-system.css'
import './styles/admin-design-system.css'
import './styles/legacy-ui-foundation.css'
import './styles/player.css'
import './styles/legacy-ui-pages.css'
import './styles/ui.css'

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
installMediaDetailHeroEnhancer()

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
