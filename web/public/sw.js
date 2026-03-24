// Nowen Video Service Worker
// 提供离线缓存和后台播放支持

const CACHE_NAME = 'nowen-video-v1'
const STATIC_ASSETS = [
  '/',
  '/manifest.json',
]

// 安装：缓存核心静态资源
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(STATIC_ASSETS)
    })
  )
  // 立即激活
  self.skipWaiting()
})

// 激活：清理旧缓存
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.filter((key) => key !== CACHE_NAME).map((key) => caches.delete(key))
      )
    })
  )
  self.clients.claim()
})

// 请求拦截：网络优先，失败回退缓存
self.addEventListener('fetch', (event) => {
  const { request } = event

  // API请求不缓存
  if (request.url.includes('/api/')) {
    return
  }

  // 流媒体请求不缓存
  if (request.url.includes('/stream/')) {
    return
  }

  // 静态资源：缓存优先
  if (request.url.match(/\.(js|css|png|jpg|svg|woff2?)$/)) {
    event.respondWith(
      caches.match(request).then((cached) => {
        if (cached) return cached
        return fetch(request).then((response) => {
          if (response.ok) {
            const clone = response.clone()
            caches.open(CACHE_NAME).then((cache) => cache.put(request, clone))
          }
          return response
        })
      })
    )
    return
  }

  // HTML导航请求：网络优先
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request).catch(() => caches.match('/'))
    )
    return
  }
})

// 支持后台音频播放（移动端）
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting()
  }
})
