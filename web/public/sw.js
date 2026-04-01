// Nowen Video Service Worker v2
// 提供增强的离线缓存、后台播放和离线访问支持

const CACHE_VERSION = 'v3'
const STATIC_CACHE = `nowen-static-${CACHE_VERSION}`
const DYNAMIC_CACHE = `nowen-dynamic-${CACHE_VERSION}`
const IMAGE_CACHE = `nowen-images-${CACHE_VERSION}`

// 核心静态资源（安装时预缓存）
const PRECACHE_ASSETS = [
  '/',
  '/manifest.json',
]

// 最大缓存条目数
const MAX_DYNAMIC_CACHE = 50
const MAX_IMAGE_CACHE = 200

// 安装：预缓存核心静态资源
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(STATIC_CACHE).then((cache) => {
      return cache.addAll(PRECACHE_ASSETS)
    })
  )
  self.skipWaiting()
})

// 激活：清理旧版本缓存
self.addEventListener('activate', (event) => {
  const currentCaches = [STATIC_CACHE, DYNAMIC_CACHE, IMAGE_CACHE]
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys
          .filter((key) => !currentCaches.includes(key))
          .map((key) => caches.delete(key))
      )
    })
  )
  self.clients.claim()
})

// 限制缓存大小
async function trimCache(cacheName, maxItems) {
  const cache = await caches.open(cacheName)
  const keys = await cache.keys()
  if (keys.length > maxItems) {
    await cache.delete(keys[0])
    return trimCache(cacheName, maxItems)
  }
}

// 请求拦截策略
self.addEventListener('fetch', (event) => {
  const { request } = event
  const url = new URL(request.url)

  // 仅处理 http/https 协议，跳过 chrome-extension:// 等不支持缓存的协议
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return
  }

  // API 请求不缓存
  if (url.pathname.startsWith('/api/')) {
    return
  }

  // 流媒体请求不缓存
  if (url.pathname.includes('/stream/')) {
    return
  }

  // 图片资源：缓存优先 + 后台更新
  if (request.destination === 'image' || url.pathname.match(/\.(png|jpg|jpeg|webp|gif|svg|ico)$/i)) {
    event.respondWith(
      caches.match(request).then((cached) => {
        const fetchPromise = fetch(request).then((response) => {
          if (response.ok) {
            const clone = response.clone()
            caches.open(IMAGE_CACHE).then((cache) => {
              cache.put(request, clone)
              trimCache(IMAGE_CACHE, MAX_IMAGE_CACHE)
            })
          }
          return response
        }).catch(() => cached)

        return cached || fetchPromise
      })
    )
    return
  }

  // JS/CSS/字体等静态资源：网络优先，离线回退缓存
  // Vite 构建的 JS/CSS 文件名带 content hash，使用网络优先确保始终加载最新版本
  if (request.url.match(/\.(js|css|woff2?)$/)) {
    event.respondWith(
      fetch(request)
        .then((response) => {
          if (response.ok) {
            const clone = response.clone()
            caches.open(STATIC_CACHE).then((cache) => cache.put(request, clone))
          }
          return response
        })
        .catch(() => {
          // 网络失败时回退到缓存（离线支持）
          return caches.match(request)
        })
    )
    return
  }

  // HTML 导航请求：网络优先，离线回退到缓存的首页
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          // 缓存成功的导航响应
          if (response.ok) {
            const clone = response.clone()
            caches.open(DYNAMIC_CACHE).then((cache) => cache.put(request, clone))
          }
          return response
        })
        .catch(() => {
          return caches.match(request).then((cached) => {
            return cached || caches.match('/')
          })
        })
    )
    return
  }

  // 其他请求：网络优先，缓存回退
  event.respondWith(
    fetch(request)
      .then((response) => {
        if (response.ok && request.method === 'GET') {
          const clone = response.clone()
          caches.open(DYNAMIC_CACHE).then((cache) => {
            cache.put(request, clone)
            trimCache(DYNAMIC_CACHE, MAX_DYNAMIC_CACHE)
          })
        }
        return response
      })
      .catch(() => caches.match(request))
  )
})

// 消息处理
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting()
  }

  // 清除所有缓存
  if (event.data && event.data.type === 'CLEAR_CACHE') {
    caches.keys().then((keys) => {
      Promise.all(keys.map((key) => caches.delete(key))).then(() => {
        event.ports[0]?.postMessage({ success: true })
      })
    })
  }

  // 获取缓存统计
  if (event.data && event.data.type === 'CACHE_STATS') {
    Promise.all([
      caches.open(STATIC_CACHE).then((c) => c.keys()).then((k) => k.length),
      caches.open(DYNAMIC_CACHE).then((c) => c.keys()).then((k) => k.length),
      caches.open(IMAGE_CACHE).then((c) => c.keys()).then((k) => k.length),
    ]).then(([staticCount, dynamicCount, imageCount]) => {
      event.ports[0]?.postMessage({
        static: staticCount,
        dynamic: dynamicCount,
        images: imageCount,
        total: staticCount + dynamicCount + imageCount,
      })
    })
  }
})

// 后台同步（当网络恢复时）
self.addEventListener('sync', (event) => {
  if (event.tag === 'sync-progress') {
    // 同步离线期间的播放进度
    event.waitUntil(syncPlaybackProgress())
  }
})

async function syncPlaybackProgress() {
  // 从 IndexedDB 读取离线期间保存的播放进度并同步到服务器
  // 此功能需要前端配合将进度写入 IndexedDB
  try {
    const response = await fetch('/api/users/me', { method: 'GET' })
    if (response.ok) {
      // 网络已恢复，可以同步数据
      console.log('[SW] 网络已恢复，可同步离线数据')
    }
  } catch (e) {
    // 仍然离线，等待下次同步
  }
}

// 推送通知处理
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

    event.waitUntil(
      self.registration.showNotification(data.title || 'Nowen Video', options)
    )
  } catch (e) {
    // 非 JSON 格式的推送
    event.waitUntil(
      self.registration.showNotification('Nowen Video', {
        body: event.data.text(),
        icon: '/assets/icon-192.png',
      })
    )
  }
})

// 点击通知
self.addEventListener('notificationclick', (event) => {
  event.notification.close()

  const url = event.notification.data?.url || '/'
  event.waitUntil(
    self.clients.matchAll({ type: 'window' }).then((clients) => {
      // 如果已有窗口打开，聚焦并导航
      for (const client of clients) {
        if ('focus' in client) {
          client.focus()
          client.navigate(url)
          return
        }
      }
      // 否则打开新窗口
      return self.clients.openWindow(url)
    })
  )
})
