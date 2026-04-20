/**
 * usePageCache — 跨路由的页面级数据缓存 Hook
 *
 * 解决的问题：
 *   1. 从其他页面跳回同一页面时，重复显示 loading / 骨架屏 → 用户感知到"页面一直在加载"。
 *   2. React.StrictMode 开发环境下 effect 双跑导致请求被发两次。
 *   3. 多个并发触发（快速返回/WS 事件）导致竞态。
 *
 * 策略：
 *   - 使用模块级 Map 缓存每个 key 的上次数据与时间戳；
 *   - 组件 mount 时：
 *        • 有缓存且未过期 → 直接用缓存数据，不发请求，不显示 loading；
 *        • 有缓存但已过期 → 使用旧数据即时渲染，同时后台静默刷新（SWR）；
 *        • 无缓存        → 显示 loading，发请求。
 *   - 并发去重：同一 key 的飞行请求全局共享一个 Promise。
 *   - StrictMode 幂等：useRef 守护，第二次 mount 不重复触发。
 *   - 手动刷新：返回 refetch()，可显式触发（如 WS 事件），支持 silent 模式。
 *
 * 注意：缓存仅存在内存，刷新整个页面会丢失；这是有意的（避免脏数据）。
 */

import { useCallback, useEffect, useRef, useState } from 'react'

// ========== 类型定义 ==========

interface CacheEntry<T> {
  data: T
  loadedAt: number
}

interface InFlightEntry<T> {
  promise: Promise<T>
}

export interface UsePageCacheOptions {
  /** 缓存新鲜时间（毫秒），过期后后台静默刷新。默认 30s */
  ttl?: number
  /** 是否启用缓存（false 时退化为普通请求）。默认 true */
  enabled?: boolean
  /** 失败时是否保留旧缓存数据。默认 true（避免闪屏） */
  keepStaleOnError?: boolean
}

export interface UsePageCacheReturn<T> {
  data: T | undefined
  /** 仅在"无任何数据可显示"时为 true，有旧数据时永远为 false，避免闪屏 */
  loading: boolean
  /** 后台静默刷新中（数据已存在，只是在拉最新） */
  refreshing: boolean
  error: unknown
  /** 手动刷新。silent=true 不触发 loading 状态（默认 true） */
  refetch: (silent?: boolean) => Promise<T | undefined>
  /** 使当前 key 的缓存失效（下次挂载会重新请求） */
  invalidate: () => void
  /** 手动写入数据（配合乐观更新） */
  mutate: (next: T | ((prev: T | undefined) => T)) => void
}

// ========== 全局缓存存储 ==========

const globalCache = new Map<string, CacheEntry<unknown>>()
const globalInFlight = new Map<string, InFlightEntry<unknown>>()
// 订阅者：某个 key 的数据被更新时通知同 key 的所有实例
const globalSubscribers = new Map<string, Set<() => void>>()

function subscribe(key: string, cb: () => void) {
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

function notify(key: string) {
  globalSubscribers.get(key)?.forEach((cb) => cb())
}

// ========== 导出工具 ==========

/** 手动使某个缓存失效，可用于跨页面场景（如修改后希望相关列表页下次进入重新加载） */
export function invalidatePageCache(key: string) {
  globalCache.delete(key)
  notify(key)
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

/** 清空全部缓存（如登出时调用） */
export function clearPageCache() {
  const keys = Array.from(globalCache.keys())
  globalCache.clear()
  keys.forEach(notify)
}

// ========== 主 Hook ==========

export function usePageCache<T>(
  key: string | null | undefined,
  fetcher: () => Promise<T>,
  options: UsePageCacheOptions = {},
): UsePageCacheReturn<T> {
  const { ttl = 30_000, enabled = true, keepStaleOnError = true } = options

  const cachedEntry = key ? (globalCache.get(key) as CacheEntry<T> | undefined) : undefined

  const [data, setData] = useState<T | undefined>(() => cachedEntry?.data)
  const [loading, setLoading] = useState<boolean>(() => enabled && !!key && !cachedEntry)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<unknown>(null)

  // 保持 fetcher 最新引用，避免依赖变化导致重复触发
  const fetcherRef = useRef(fetcher)
  useEffect(() => { fetcherRef.current = fetcher }, [fetcher])

  // 幂等守护 + 跟踪当前 key
  const mountedRef = useRef(false)
  const currentKeyRef = useRef<string | null | undefined>(key)
  // 记录组件是否仍挂载（卸载后禁止 setState）
  const aliveRef = useRef(true)
  useEffect(() => {
    aliveRef.current = true
    return () => { aliveRef.current = false }
  }, [])

  // 执行 fetch（并发去重 + 缓存写入 + 通知订阅者）
  const doFetch = useCallback(async (k: string, silent: boolean): Promise<T | undefined> => {
    if (!silent && aliveRef.current) setLoading(true)
    else if (aliveRef.current) setRefreshing(true)

    // 并发去重
    let inFlight = globalInFlight.get(k) as InFlightEntry<T> | undefined
    if (!inFlight) {
      const promise = (async () => {
        try {
          const result = await fetcherRef.current()
          globalCache.set(k, { data: result, loadedAt: Date.now() })
          notify(k)
          return result
        } finally {
          globalInFlight.delete(k)
        }
      })()
      inFlight = { promise }
      globalInFlight.set(k, inFlight as InFlightEntry<unknown>)
    }

    try {
      const result = await inFlight.promise
      if (aliveRef.current && currentKeyRef.current === k) {
        setData(result)
        setError(null)
      }
      return result
    } catch (err) {
      if (aliveRef.current && currentKeyRef.current === k) {
        setError(err)
        if (!keepStaleOnError) setData(undefined)
      }
      return undefined
    } finally {
      if (aliveRef.current && currentKeyRef.current === k) {
        setLoading(false)
        setRefreshing(false)
      }
    }
  }, [keepStaleOnError])

  // 主加载逻辑（key 变化时重新评估）
  useEffect(() => {
    currentKeyRef.current = key
    if (!enabled || !key) {
      setLoading(false)
      return
    }

    const entry = globalCache.get(key) as CacheEntry<T> | undefined
    if (entry) {
      // 有缓存 → 立即渲染
      setData(entry.data)
      setLoading(false)
      // 过期 → 后台静默刷新
      if (Date.now() - entry.loadedAt > ttl) {
        doFetch(key, true)
      }
    } else {
      // 无缓存 → 首次加载（StrictMode 双挂载由 inFlight Map 去重）
      doFetch(key, false)
    }
    mountedRef.current = true
  }, [key, enabled, ttl, doFetch])

  // 订阅同 key 的缓存更新（其他组件 mutate/invalidate 后同步）
  useEffect(() => {
    if (!key) return
    return subscribe(key, () => {
      const entry = globalCache.get(key) as CacheEntry<T> | undefined
      if (aliveRef.current) {
        setData(entry?.data)
      }
    })
  }, [key])

  const refetch = useCallback(
    async (silent = true): Promise<T | undefined> => {
      if (!key) return undefined
      return doFetch(key, silent)
    },
    [key, doFetch],
  )

  const invalidate = useCallback(() => {
    if (!key) return
    globalCache.delete(key)
    notify(key)
  }, [key])

  const mutate = useCallback(
    (next: T | ((prev: T | undefined) => T)) => {
      if (!key) return
      const prev = (globalCache.get(key) as CacheEntry<T> | undefined)?.data
      const value = typeof next === 'function'
        ? (next as (p: T | undefined) => T)(prev)
        : next
      globalCache.set(key, { data: value, loadedAt: Date.now() })
      notify(key)
    },
    [key],
  )

  return { data, loading, refreshing, error, refetch, invalidate, mutate }
}
