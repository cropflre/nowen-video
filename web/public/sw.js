// Nowen Video Service Worker v6
// 仅缓存生产环境的静态资源；认证、API、HTML 导航、开发模块和非 GET 请求全部直连网络。

const CACHE_VERSION = 'v6'
const STATIC_CACHE = `nowen-static-${CACHE_VERSION}`
const IMAGE_CACHE = `nowen-images-${CACHE_VERSION}`

// 不缓存 HTML 应用壳。页面结构和导航必须始终来自当前部署版本，
// 避免已经下线的页面与菜单被离线回退重新启动。
const PRECACHE_ASSETS = [
  '/manifest.json',
]

const MAX_IMAGE_CACHE = 200

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(STATIC_CACHE).then((cache) => cache.addAll(PRECACHE_ASSETS)),
  )
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  const currentCaches = [STATIC_CACHE, IMAGE_CACHE]
  event.waitUntil((async () => {
    const keys = await caches.keys()
    await Promise.all(
      keys
        .filter((key) => key.startsWith('nowen-') && !currentCaches.includes(key))
        .map((key) => caches.delete(key)),
    )

    await self.clients.claim()

    // 当前打开的标签页可能仍在执行旧版 JS，无法监听新 Worker 的 controllerchange。
    // 激活时主动执行一次带版本参数的网络导航，强制退出旧页面与旧菜单。
    const windowClients = await self.clients.matchAll({ type: 'window' })
    await Promise.all(windowClients.map(async (client) => {
      try {
        const target = new URL(client.url)
        target.searchParams.set('__nowen_app_version', CACHE_VERSION)
        await client.navigate(target.href)
      } catch {
        // 标签页可能在刷新过程中关闭；不影响其他客户端更新。
      }
    }))
  })())
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

  // HTML 导航永远网络直连且禁止缓存。网络不可用时明确失败，
  // 不再回退可能包含旧导航或退役页面的应用壳。
  if (request.mode === 'navigate') {
    event.respondWith(fetch(request, { cache: 'no-store' }))
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

  // JS/CSS/字体等带内容哈希的静态资源：网络优先，离线时回退当前版本缓存。
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
        caches.open(IMAGE_CACHE).then((cache) => cache.keys()).then((keys) => keys.length),
      ]).then(([staticCount, imageCount]) => {
        event.ports[0]?.postMessage({
          static: staticCount,
          dynamic: 0,
          images: imageCount,
          total: staticCount + imageCount,
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
    // 仍然离线，等待下次同步。
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
