import axios, { AxiosError, AxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth'

/**
 * 计算 API 基础地址。
 *
 * 运行环境判断：
 * - Tauri 桌面端（window.__TAURI_INTERNALS__ 注入）：protocol 为 `tauri:` / `http://tauri.localhost`，
 *   必须显式指向本地 sidecar，否则所有请求会被解析为 `tauri://localhost/api/...` 导致失败。
 * - 浏览器：继续使用相对路径 `/api`，由 Vite dev-proxy 或反代处理。
 *
 * sidecar 端口：
 * - 默认 `8080`（与 Rust 端 `default_sidecar_port` 一致）。
 * - 允许通过 `localStorage.nowen_sidecar_port` 覆盖，便于诊断或多实例场景。
 */
function resolveApiBaseURL(): string {
  if (typeof window === 'undefined') return '/api'
  const w = window as any
  const isTauri = Boolean(w.__TAURI_INTERNALS__ || w.__TAURI__)
  if (!isTauri) return '/api'

  let port = 8080
  try {
    const override = window.localStorage.getItem('nowen_sidecar_port')
    if (override && /^\d+$/.test(override)) port = parseInt(override, 10)
  } catch {
    /* localStorage 不可用时忽略 */
  }
  return `http://127.0.0.1:${port}/api`
}

const API_BASE = resolveApiBaseURL()

// 暴露为全局属性便于播放器等模块拼接绝对 URL（StreamURL 需要 origin）
if (typeof window !== 'undefined') {
  ;(window as any).__NOWEN_API_BASE__ = API_BASE
}

const api = axios.create({
  baseURL: API_BASE,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 请求拦截器：自动添加Token
api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// ========== 401 处理：静默刷新 + 并发抑制 ==========
// 不触发自动登出/刷新的白名单路径
const AUTH_SAFE_PATHS = ['/auth/login', '/auth/register', '/auth/status', '/auth/refresh']

// 正在进行中的刷新请求（去重，避免并发多次调用 /auth/refresh）
let refreshPromise: Promise<string | null> | null = null

// 是否已经在执行 logout（避免并发 401 重复 logout + 重复跳转）
let loggingOut = false

/** 调用后端刷新 token；返回新 token 或 null（刷新失败） */
async function refreshAccessToken(): Promise<string | null> {
  if (refreshPromise) return refreshPromise

  refreshPromise = (async () => {
    try {
      // 直接用底层 axios，避开拦截器，防止递归
      const oldToken = useAuthStore.getState().token
      if (!oldToken) return null
      const resp = await axios.post<{ token: string; user: unknown; expires_at: number }>(
        `${API_BASE}/auth/refresh`,
        {},
        { headers: { Authorization: `Bearer ${oldToken}` }, timeout: 10000 },
      )
      const { token, user } = resp.data as { token: string; user: any }
      if (token && user) {
        useAuthStore.getState().setAuth(token, user)
        return token
      }
      return null
    } catch (e) {
      return null
    } finally {
      // 下一轮 401 允许再尝试
      setTimeout(() => { refreshPromise = null }, 0)
    }
  })()

  return refreshPromise
}

/** 真正执行登出（防抖，避免并发 401 重复跳转） */
function doLogout(reason: string) {
  if (loggingOut) return
  loggingOut = true
  // eslint-disable-next-line no-console
  console.warn('[auth] forced logout:', reason)
  try {
    useAuthStore.getState().logout()
  } catch { /* ignore */ }
  // 已经在登录页就不跳转，避免白屏
  if (!window.location.pathname.startsWith('/login')) {
    window.location.href = '/login'
  }
  // 留一小段时间让页面跳转完成
  setTimeout(() => { loggingOut = false }, 3000)
}

// 响应拦截器：401 自动刷新 token，刷新失败再登出
api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const status = error.response?.status
    const url = error.config?.url || ''
    const isSafe = AUTH_SAFE_PATHS.some((p) => url.includes(p))

    if (status !== 401 || isSafe) {
      return Promise.reject(error)
    }

    // 打印服务端返回的错误详情，便于定位"突然退出登录"根因
    const serverErr = (error.response?.data as any)?.error || ''
    // eslint-disable-next-line no-console
    console.warn(`[auth] 401 on ${url}: ${serverErr}`)

    // 已经重试过一次就不再重试，避免无限循环
    const cfg = error.config as AxiosRequestConfig & { _retry?: boolean }
    if (!cfg || cfg._retry) {
      doLogout(serverErr || '令牌无效')
      return Promise.reject(error)
    }

    // 尝试刷新 token
    const newToken = await refreshAccessToken()
    if (!newToken) {
      doLogout(serverErr || '令牌刷新失败')
      return Promise.reject(error)
    }

    // 刷新成功，重放原请求
    cfg._retry = true
    cfg.headers = { ...(cfg.headers || {}), Authorization: `Bearer ${newToken}` }
    return api.request(cfg)
  },
)

export default api
