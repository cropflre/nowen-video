import axios, { AxiosError, AxiosRequestConfig } from 'axios'
import { useAuthStore } from '@/stores/auth'

/**
 * 计算 API 基础地址。
 *
 * 运行环境判断：
 * - Tauri 桌面端（window.__TAURI_INTERNALS__ 注入）：
 *     1) 优先使用 localStorage.nowen_server_url（用户配置的"服务器地址"）
 *     2) 其次读取 window.__NOWEN_SERVER_URL__（Rust 首次启动时注入，可选）
 *     3) 兜底使用内嵌 sidecar 端口 `http://127.0.0.1:<port>/api`
 *        —— sidecar 默认端口 21114（与本机独立运行的 Go server 的 8080 互不冲突）
 * - 浏览器：继续使用相对路径 `/api`，由 Vite dev-proxy 或反代处理。
 *
 * sidecar 端口：
 * - 默认 `21114`（与 Rust 端 `default_sidecar_port` 保持一致）。
 * - 允许通过 `localStorage.nowen_sidecar_port` 覆盖，便于诊断或多实例场景。
 */
function resolveApiBaseURL(): string {
  if (typeof window === 'undefined') return '/api'
  const w = window as any
  const isTauri = Boolean(w.__TAURI_INTERNALS__ || w.__TAURI__)
  if (!isTauri) return '/api'

  // 1) 用户显式配置的"远程服务器"地址
  try {
    const custom = window.localStorage.getItem('nowen_server_url')
    if (custom && /^https?:\/\//i.test(custom)) {
      return custom.replace(/\/+$/, '') + '/api'
    }
  } catch {
    /* localStorage 不可用时忽略 */
  }

  // 2) 启动阶段由 Rust 注入的服务器地址
  if (typeof w.__NOWEN_SERVER_URL__ === 'string' && /^https?:\/\//i.test(w.__NOWEN_SERVER_URL__)) {
    return String(w.__NOWEN_SERVER_URL__).replace(/\/+$/, '') + '/api'
  }

  // 3) 内嵌 sidecar 端口
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

// 暴露为全局属性便于播放器等模块拼接绝对 URL（StreamURL 需要 origin）
if (typeof window !== 'undefined') {
  ;(window as any).__NOWEN_API_BASE__ = API_BASE
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

// 请求拦截器：自动添加 Token，并记录本次请求所属的登录会话。
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

// ========== 401 处理：静默刷新 + 并发抑制 + 会话竞态保护 ==========
// 不触发自动登出/刷新的白名单路径
const AUTH_SAFE_PATHS = ['/auth/login', '/auth/register', '/auth/status', '/auth/refresh']

// 刷新请求必须按 Token 隔离，防止旧会话的刷新结果覆盖刚登录的新会话。
let refreshState: { token: string; promise: Promise<string | null> } | null = null

// 是否已经在执行 logout（避免同一会话并发 401 重复跳转）
let loggingOut = false

/** 调用后端刷新 token；仅允许刷新 expectedToken 对应的当前会话。 */
async function refreshAccessToken(expectedToken: string): Promise<string | null> {
  const currentToken = useAuthStore.getState().token
  if (currentToken !== expectedToken) return currentToken
  if (refreshState?.token === expectedToken) return refreshState.promise

  const promise = (async () => {
    try {
      // 直接用底层 axios，避开拦截器，防止递归
      const resp = await axios.post<{ token: string; user: unknown; expires_at: number }>(
        `${API_BASE}/auth/refresh`,
        {},
        { headers: { Authorization: `Bearer ${expectedToken}` }, timeout: 10000 },
      )
      const { token, user } = resp.data as { token: string; user: any }
      if (!token || !user) return null

      // 刷新期间如果用户已经重新登录或切换会话，禁止旧刷新覆盖新 Token。
      const tokenBeforeApply = useAuthStore.getState().token
      if (tokenBeforeApply !== expectedToken) return tokenBeforeApply

      useAuthStore.getState().setAuth(token, user)
      return token
    } catch {
      return null
    } finally {
      // 仅清理自己所属的刷新任务，不影响新会话可能已经启动的刷新。
      window.setTimeout(() => {
        if (refreshState?.token === expectedToken) refreshState = null
      }, 0)
    }
  })()

  refreshState = { token: expectedToken, promise }
  return promise
}

/**
 * 真正执行登出。
 * expectedToken 必须仍是当前 Token；迟到的旧请求无权清除更新后的登录态。
 */
function doLogout(reason: string, expectedToken: string | null) {
  const currentToken = useAuthStore.getState().token
  if (currentToken !== expectedToken) {
    // eslint-disable-next-line no-console
    console.warn('[auth] ignored stale 401 from an older session:', reason)
    return
  }
  if (loggingOut) return
  loggingOut = true
  // eslint-disable-next-line no-console
  console.warn('[auth] forced logout:', reason)
  try {
    useAuthStore.getState().logout()
  } catch { /* ignore */ }
  // 已经在登录页就不跳转，避免登录页自身形成整页刷新循环
  if (!window.location.pathname.startsWith('/login')) {
    window.location.replace('/login')
  }
  window.setTimeout(() => { loggingOut = false }, 3000)
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

    const cfg = error.config as AuthRequestConfig | undefined
    const failedToken = cfg?._authToken ?? null
    const currentToken = useAuthStore.getState().token
    const serverErr = (error.response?.data as any)?.error || ''
    // eslint-disable-next-line no-console
    console.warn(`[auth] 401 on ${url}: ${serverErr}`)

    // 请求发出后发生过登录、刷新或账号切换：直接使用当前新 Token 重放一次。
    // 旧请求的 401 绝不能刷新或清除新会话。
    if (cfg && currentToken && failedToken !== currentToken) {
      if (cfg._retry) return Promise.reject(error)
      cfg._retry = true
      cfg._authToken = currentToken
      cfg.headers = { ...(cfg.headers || {}), Authorization: `Bearer ${currentToken}` }
      return api.request(cfg)
    }

    // 已重试仍失败，只允许清除产生该请求的同一会话。
    if (!cfg || cfg._retry) {
      doLogout(serverErr || '令牌无效', failedToken)
      return Promise.reject(error)
    }

    if (!currentToken) {
      doLogout(serverErr || '缺少登录凭证', failedToken)
      return Promise.reject(error)
    }

    // 尝试刷新当前会话 Token
    const newToken = await refreshAccessToken(currentToken)
    if (!newToken) {
      doLogout(serverErr || '令牌刷新失败', currentToken)
      return Promise.reject(error)
    }

    // 刷新成功，重放原请求
    cfg._retry = true
    cfg._authToken = newToken
    cfg.headers = { ...(cfg.headers || {}), Authorization: `Bearer ${newToken}` }
    return api.request(cfg)
  },
)

export default api
