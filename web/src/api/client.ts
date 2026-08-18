import axios, { AxiosError, AxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth'
import { desktop } from '@/platform/desktop/bridge'

let currentApiBaseURL = '/api'

function publishApiBaseURL(baseURL: string) {
  currentApiBaseURL = baseURL
  if (typeof window !== 'undefined') {
    ;(window as Window & { __NOWEN_API_BASE__?: string }).__NOWEN_API_BASE__ = baseURL
  }
}

/**
 * 返回当前平台真实 API 地址。
 *
 * Web 始终使用同源 /api；Desktop 从 Tauri Runtime 获取远程服务器配置或
 * 当前动态 Sidecar 端口，不再假设 localhost:21114。
 */
export async function resolveApiBaseURL(): Promise<string> {
  if (typeof window === 'undefined' || !desktop.isDesktop) {
    publishApiBaseURL('/api')
    return '/api'
  }

  const serverBase = await desktop.serverBaseUrl()
  if (!serverBase) {
    throw new Error('Desktop Media Core 尚未就绪')
  }

  const baseURL = `${serverBase.replace(/\/+$/, '')}/api`
  publishApiBaseURL(baseURL)
  return baseURL
}

/**
 * 同步读取最近一次已解析的 API 地址。
 * 用于 pagehide / keepalive 等无法等待异步 Tauri IPC 的生命周期场景。
 */
export function getResolvedApiBaseURL(): string {
  return currentApiBaseURL
}

const api = axios.create({
  baseURL: '/api',
  timeout: 60000,
  headers: {
    'Content-Type': 'application/json',
  },
})

type AuthRequestConfig = AxiosRequestConfig & {
  _retry?: boolean
  _authToken?: string | null
}

api.interceptors.request.use(async (config) => {
  config.baseURL = await resolveApiBaseURL()

  const token = useAuthStore.getState().token
  const authConfig = config as AuthRequestConfig
  authConfig._authToken = token
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  } else if (config.headers) {
    delete config.headers.Authorization
  }
  return config
})

const AUTH_SAFE_PATHS = ['/auth/login', '/auth/register', '/auth/status', '/auth/refresh']

let refreshState: { token: string; promise: Promise<string | null> } | null = null
let loggingOut = false

async function refreshAccessToken(expectedToken: string): Promise<string | null> {
  const currentToken = useAuthStore.getState().token
  if (currentToken !== expectedToken) return currentToken
  if (refreshState?.token === expectedToken) return refreshState.promise

  const promise = (async () => {
    try {
      const baseURL = await resolveApiBaseURL()
      const response = await axios.post<{ token: string; user: unknown; expires_at: number }>(
        `${baseURL}/auth/refresh`,
        {},
        { headers: { Authorization: `Bearer ${expectedToken}` }, timeout: 10000 },
      )
      const { token, user } = response.data as { token: string; user: any }
      if (!token || !user) return null

      const tokenBeforeApply = useAuthStore.getState().token
      if (tokenBeforeApply !== expectedToken) return tokenBeforeApply

      useAuthStore.getState().setAuth(token, user)
      return token
    } catch {
      return null
    } finally {
      window.setTimeout(() => {
        if (refreshState?.token === expectedToken) refreshState = null
      }, 0)
    }
  })()

  refreshState = { token: expectedToken, promise }
  return promise
}

function doLogout(reason: string, expectedToken: string | null) {
  const currentToken = useAuthStore.getState().token
  if (currentToken !== expectedToken) {
    console.warn('[auth] ignored stale 401 from an older session:', reason)
    return
  }
  if (loggingOut) return
  loggingOut = true
  console.warn('[auth] forced logout:', reason)
  try {
    useAuthStore.getState().logout()
  } catch {
    /* ignore */
  }
  if (!window.location.pathname.startsWith('/login')) {
    window.location.replace('/login')
  }
  window.setTimeout(() => {
    loggingOut = false
  }, 3000)
}

api.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const status = error.response?.status
    const url = error.config?.url || ''
    const isSafe = AUTH_SAFE_PATHS.some((path) => url.includes(path))

    if (status !== 401 || isSafe) {
      return Promise.reject(error)
    }

    const config = error.config as AuthRequestConfig | undefined
    const failedToken = config?._authToken ?? null
    const currentToken = useAuthStore.getState().token
    const serverError = (error.response?.data as any)?.error || ''
    console.warn(`[auth] 401 on ${url}: ${serverError}`)

    if (config && currentToken && failedToken !== currentToken) {
      if (config._retry) return Promise.reject(error)
      config._retry = true
      config._authToken = currentToken
      config.headers = { ...(config.headers || {}), Authorization: `Bearer ${currentToken}` }
      return api.request(config)
    }

    if (!config || config._retry) {
      doLogout(serverError || '令牌无效', failedToken)
      return Promise.reject(error)
    }

    if (!currentToken) {
      doLogout(serverError || '缺少登录凭证', failedToken)
      return Promise.reject(error)
    }

    const newToken = await refreshAccessToken(currentToken)
    if (!newToken) {
      doLogout(serverError || '令牌刷新失败', currentToken)
      return Promise.reject(error)
    }

    config._retry = true
    config._authToken = newToken
    config.headers = { ...(config.headers || {}), Authorization: `Bearer ${newToken}` }
    return api.request(config)
  },
)

export default api
