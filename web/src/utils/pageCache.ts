export interface CacheEntry<T> {
  data: T
  loadedAt: number
}

export interface InFlightEntry<T> {
  promise: Promise<T>
  superseded?: boolean
}

export const cacheState = { epoch: 0 }
export const globalCache = new Map<string, CacheEntry<unknown>>()
export const globalInFlight = new Map<string, InFlightEntry<unknown>>()
// 订阅者：某个 key 的数据被更新时通知同 key 的所有实例
type CacheEvent = 'changed' | 'revalidate'
type CacheListener = (event: CacheEvent) => void
const globalSubscribers = new Map<string, Set<CacheListener>>()

export function subscribe(key: string, cb: CacheListener) {
  let set = globalSubscribers.get(key)
  if (!set) {
    set = new Set()
    globalSubscribers.set(key, set)
  }
  set.add(cb)
  return () => {
    set!.delete(cb)
    if (set!.size === 0) globalSubscribers.delete(key)
  }
}

export function notify(key: string, event: CacheEvent = 'changed') {
  globalSubscribers.get(key)?.forEach((cb) => cb(event))
}

// ========== 导出工具 ==========

/** 手动使某个缓存失效，可用于跨页面场景（如修改后希望相关列表页下次进入重新加载） */
export function invalidatePageCache(key: string) {
  for (const current of globalCache.keys()) {
    if (current === key || current.startsWith(`${key}:user=`)) {
      globalCache.delete(current)
      notify(current)
    }
  }
}

/** 按前缀失效（如 invalidatePageCachePrefix('media:') 清掉所有媒体详情缓存） */
export function invalidatePageCachePrefix(prefix: string) {
  for (const k of Array.from(globalCache.keys())) {
    if (k.startsWith(prefix)) {
      globalCache.delete(k)
      notify(k)
    }
  }
}

/** 保留已显示的数据，立即刷新挂载的页面，或在下次挂载时刷新。 */
export function revalidatePageCachePrefix(prefix: string) {
  const keys = new Set([...globalCache.keys(), ...globalInFlight.keys(), ...globalSubscribers.keys()])
  for (const key of keys) {
    if (!key.startsWith(prefix)) continue
    const entry = globalCache.get(key)
    if (entry) entry.loadedAt = Number.NEGATIVE_INFINITY

    // 正在运行的请求可能读到了写入之前的数据。
    const inFlight = globalInFlight.get(key)
    if (inFlight) {
      inFlight.superseded = true
      globalInFlight.delete(key)
    }
    notify(key, 'revalidate')
  }
}

/** 清空全部缓存（如登出时调用） */
export function clearPageCache() {
  const keys = Array.from(new Set([...globalCache.keys(), ...globalSubscribers.keys()]))
  cacheState.epoch += 1
  globalInFlight.clear()
  globalCache.clear()
  keys.forEach((key) => notify(key))
}
