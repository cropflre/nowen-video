// Nowen Video Service Worker v4
// 仅缓存生产环境的静态资源；认证、API、开发模块和非 GET 请求全部直连网络。

const CACHE_VERSION = 'v4'
const STATIC_CACHE = `nowen-static-${CACHE_VERSION}`
const DYNAMIC_CACHE = `nowen-dynamic-${CACHE_VERSION}`
const IMAGE_CACHE = `nowen-images-${CACHE_VERSION}`

const PRECACHE_ASSETS = [
  '/',
  '/manifest.json',
]

const MAX_DYNAMIC_CACHE = 50
const MAX_IMAGE_CACHE = 200

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(STATIC_CACHE).then((cache) => cache.addAll(PRECACHE_ASSETS)),
  )
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  const currentCaches = [STATIC_CACHE, DYNAMIC_CACHE, IMAGE_CACHE]
  event.waitUntil(
    caches.keys().then((keys) => Promise.all(
      keys
        .filter((key) => key.startsWith('nowen-') && !currentCaches.includes(key))
        .map((key) => caches.delete(key)),
    )),
  )
  self.clients.claim()
})

async function trimCache(cacheName, maxItems) {
  const cache = await caches.open(cacheName)
  const keys = await cache.keys()
  while (keys.length > maxItems) {
    const oldest = keys.shift()
    if (oldest) await cache.delete(oldest)
  }
}

function isDevelopmentRequest(url) {
  return (
    url.pathname.startsWith('/src/') ||
    url.pathname.startsWith('/@vite/') ||
    url.pathname.startsWith('/@react-refresh') ||
    url.pathname.startsWith('/node_modules/.vite/') ||
    url.pathname.includes('__vite_ping')
  )
}

function isAuthenticationRoute(url) {
  return url.pathname === '/login' || url.pathname === '/force-change-password'
}

function isBackendRequest(request, url) {
  const accept = request.headers.get('accept') || ''
  return (
    url.pathname === '/api' ||
    url.pathname.startsWith('/api/') ||
    url.pathname.startsWith('/emby/') ||
    url.pathname.includes('/stream/') ||
    request.destination === '' ||
    accept.includes('application/json')
  )
}

self.addEventListener('fetch', (event) => {
  const { request } = event
  const url = new URL(request.url)

  // Service Worker 不应参与写请求、跨域请求、认证页、API 或 Vite 开发模块。
  // 这些请求一旦被旧缓存接管，最容易造成登录成功后仍回到登录页的循环。
  if (
    request.method !== 'GET' ||
    url.protocol !== 'http:' && url.protocol !== 'https:' ||
    url.origin !== self.location.origin ||
    isAuthenticationRoute(url) ||
    isDevelopmentRequest(url) ||
    isBackendRequest(request, url)
  ) {
    return
  }

  // 图片：缓存优先，同时后台更新。
  if (request.destination === 'image' || url.pathname.match(/\.(png|jpg|jpeg|webp|gif|svg|ico)$/i)) {
    event.respondWith(
      caches.match(request).then((cached) => {
        const network = fetch(request)
          .then((response) => {
            if (response.ok && response.type === 'basic') {
              const clone = response.clone()
              void caches.open(IMAGE_CACHE).then(async (cache) => {
                await cache.put(request, clone)
                await trimCache(IMAGE_CACHE, MAX_IMAGE_CACHE)
              })
            }
            return response
          })
          .catch(() => cached)
        return cached || network
      }),
    )
    return
  }

  // JS/CSS/字体等带内容哈希的静态资源：网络优先，离线时回退缓存。
  if (
    ['script', 'style', 'font', 'manifest'].includes(request.destination) ||
    url.pathname.match(/\.(js|css|woff2?|json)$/i)
  ) {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok && response.type === 'basic') {
            const clone = response.clone()
            void caches.open(STATIC_CACHE).then((cache) => cache.put(request, clone))
          }
          return response
        })
        .catch(() => caches.match(request)),
    )
    return
  }

  // 非认证 HTML 导航：网络优先；离线时仅回退应用壳。
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request, { cache: 'no-store' })
        .then((response) => {
          if (response.ok && response.type === 'basic') {
            const clone = response.clone()
            void caches.open(DYNAMIC_CACHE).then(async (cache) => {
              await cache.put('/', clone)
              await trimCache(DYNAMIC_CACHE, MAX_DYNAMIC_CACHE)
            })
          }
          return response
        })
        .catch(async () => {
          return (await caches.match('/')) || Response.error()
        }),
    )
  }
})

self.addEventListener('message', (event) => {
  if (event.data?.type === 'SKIP_WAITING') {
    self.skipWaiting()
  }

  if (event.data?.type === 'CLEAR_CACHE') {
    event.waitUntil(
      caches.keys().then((keys) => Promise.all(
        keys.filter((key) => key.startsWith('nowen-')).map((key) => caches.delete(key)),
      )).then(() => {
        event.ports[0]?.postMessage({ success: true })
      }),
    )
  }

  if (event.data?.type === 'CACHE_STATS') {
    event.waitUntil(
      Promise.all([
        caches.open(STATIC_CACHE).then((cache) => cache.keys()).then((keys) => keys.length),
        caches.open(DYNAMIC_CACHE).then((cache) => cache.keys()).then((keys) => keys.length),
        caches.open(IMAGE_CACHE).then((cache) => cache.keys()).then((keys) => keys.length),
      ]).then(([staticCount, dynamicCount, imageCount]) => {
        event.ports[0]?.postMessage({
          static: staticCount,
          dynamic: dynamicCount,
          images: imageCount,
          total: staticCount + dynamicCount + imageCount,
        })
      }),
    )
  }
})

self.addEventListener('sync', (event) => {
  if (event.tag === 'sync-progress') {
    event.waitUntil(syncPlaybackProgress())
  }
})

async function syncPlaybackProgress() {
  try {
    const response = await fetch('/api/users/me', { method: 'GET', cache: 'no-store' })
    if (response.ok) {
      console.log('[SW] 网络已恢复，可同步离线数据')
    }
  } catch {
    // 仍然离线，等待下次同步
  }
}

self.addEventListener('push', (event) => {
  if (!event.data) return

  try {
    const data = event.data.json()
    const options = {
      body: data.body || '您有新的通知',
      icon: '/assets/icon-192.png',
      badge: '/assets/icon-192.png',
      tag: data.tag || 'nowen-notification',
      data: data.url ? { url: data.url } : undefined,
      actions: data.actions || [],
      vibrate: [200, 100, 200],
    }
    event.waitUntil(self.registration.showNotification(data.title || 'Nowen Video', options))
  } catch {
    event.waitUntil(
      self.registration.showNotification('Nowen Video', {
        body: event.data.text(),
        icon: '/assets/icon-192.png',
      }),
    )
  }
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  const target = event.notification.data?.url || '/'
  event.waitUntil(
    self.clients.matchAll({ type: 'window' }).then((clients) => {
      for (const client of clients) {
        if ('focus' in client) {
          client.focus()
          client.navigate(target)
          return
        }
      }
      return self.clients.openWindow(target)
    }),
  )
})
