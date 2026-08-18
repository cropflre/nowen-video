import axios, { AxiosError, AxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth'
import { desktop } from '@/platform/desktop/bridge'

/**
 * 计算 API 基础地址。
 *
 * Desktop 2.0 的环境判断统一通过 platform/desktop，业务代码不再读取
 * window.__TAURI__ / window.__TAURI_INTERNALS__。
 *
 * 当前阶段仍保留固定 sidecar 端口 21114；动态端口和运行时令牌会在
 * Sidecar Runtime 阶段切换为启动握手协议。
 */
function resolveApiBaseURL(): string {
  if (typeof window === 'undefined' || !desktop.isDesktop) return '/api'

  try {
    const custom = window.localStorage.getItem('nowen_server_url')
    if (custom && /^https?:\/\//i.test(custom)) {
      return custom.replace(/\/+$/, '') + '/api'
    }
  } catch {
    /* localStorage 不可用时忽略 */
  }

  const runtimeWindow = window as Window & { __NOWEN_SERVER_URL__?: string; __NOWEN_API_BASE__?: string }
  if (
    typeof runtimeWindow.__NOWEN_SERVER_URL__ === 'string' &&
    /^https?:\/\//i.test(runtimeWindow.__NOWEN_SERVER_URL__)
  ) {
    return runtimeWindow.__NOWEN_SERVER_URL__.replace(/\/+$/, '') + '/api'
  }

  let port = 21114
  try {
    const override = window.localStorage.getItem('nowen_sidecar_port')
    if (override && /^\d+$/.test(override)) port = parseInt(override, 10)
  } catch {
    /* localStorage 不可用时忽略 */
  }

  return `http://127.0.0.1:${port}/api`
}

const API_BASE = resolveApiBaseURL()

if (typeof window !== 'undefined') {
  ;(window as Window & { __NOWEN_API_BASE__?: string }).__NOWEN_API_BASE__ = API_BASE
}

const api = axios.create({
  baseURL: API_BASE,
  timeout: 60000,
  headers: {
    'Content-Type': 'application/json',
  },
})

type AuthRequestConfig = AxiosRequestConfig & {
  _retry?: boolean
  /** 发出请求时使用的 Token，用于识别跨登录会话的迟到 401。 */
  _authToken?: string | null
}

api.interceptors.request.use((config) => {
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
      const resp = await axios.post<{ token: string; user: unknown; expires_at: number }>(
        `${API_BASE}/auth/refresh`,
        {},
        { headers: { Authorization: `Bearer ${expectedToken}` }, timeout: 10000 },
      )
      const { token, user } = resp.data as { token: string; user: any }
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

    const cfg = error.config as AuthRequestConfig | undefined
    const failedToken = cfg?._authToken ?? null
    const currentToken = useAuthStore.getState().token
    const serverErr = (error.response?.data as any)?.error || ''
    console.warn(`[auth] 401 on ${url}: ${serverErr}`)

    if (cfg && currentToken && failedToken !== currentToken) {
      if (cfg._retry) return Promise.reject(error)
      cfg._retry = true
      cfg._authToken = currentToken
      cfg.headers = { ...(cfg.headers || {}), Authorization: `Bearer ${currentToken}` }
      return api.request(cfg)
    }

    if (!cfg || cfg._retry) {
      doLogout(serverErr || '令牌无效', failedToken)
      return Promise.reject(error)
    }

    if (!currentToken) {
      doLogout(serverErr || '缺少登录凭证', failedToken)
      return Promise.reject(error)
    }

    const newToken = await refreshAccessToken(currentToken)
    if (!newToken) {
      doLogout(serverErr || '令牌刷新失败', currentToken)
      return Promise.reject(error)
    }

    cfg._retry = true
    cfg._authToken = newToken
    cfg.headers = { ...(cfg.headers || {}), Authorization: `Bearer ${newToken}` }
    return api.request(cfg)
  },
)

export default api
