import { create } from 'zustand'

/**
 * 全局海报/封面"版本戳"。
 *
 * 用途：当刮削完成（WS: scrape_completed）或用户手动替换元数据时，
 * 调用 bumpPosterVersion() 使版本号 +1。所有使用 streamApi.getPosterUrl(id, version)
 * 的组件会自动拿到新 URL（?v=xxx），浏览器视作新请求，强制加载最新海报。
 *
 * 设计要点：
 * 1. 用 Date.now() 作为初始值，保证页面刷新后也会与之前的 URL 区分；
 * 2. bump 时也用 Date.now() 而不是 +1，避免并发多次调用被合并为一次自增；
 * 3. 组件中通过 useMediaRefreshStore(s => s.posterVersion) 订阅，获得细粒度更新。
 */
interface MediaRefreshState {
  posterVersion: number
  bumpPosterVersion: () => void
}

export const useMediaRefreshStore = create<MediaRefreshState>((set) => ({
  posterVersion: Date.now(),
  bumpPosterVersion: () => set({ posterVersion: Date.now() }),
}))

/** 便捷 hook：仅订阅版本号（避免在 selector 中解构整个 state） */
export const usePosterVersion = () => useMediaRefreshStore((s) => s.posterVersion)

/** 非组件环境（如 WS 回调）下直接 bump */
export const bumpPosterVersion = () => useMediaRefreshStore.getState().bumpPosterVersion()
