/**
 * 从捕获到的错误对象中提取可展示给用户的真实错误消息。
 *
 * 优先级：
 *   1) axios 响应体中的 message / error / msg 字段（后端标准错误格式）
 *   2) 响应状态文本（statusText）
 *   3) 网络错误码（ECONNABORTED / ERR_NETWORK 等，给出网络类提示）
 *   4) 原生 Error.message
 *   5) 返回空字符串（调用方自行决定 fallback 文案）
 */
export function extractErrMsg(err: unknown): string {
  if (!err) return ''

  // axios error 结构：{ response: { data: { message } }, code, message, ... }
  const anyErr = err as any

  // 1) 后端返回的业务错误 message
  const data = anyErr?.response?.data
  if (data) {
    if (typeof data === 'string' && data.trim()) return data.trim()
    const msg = data.message || data.error || data.msg
    if (typeof msg === 'string' && msg.trim()) return msg.trim()
  }

  // 2) HTTP 层状态
  const status = anyErr?.response?.status
  const statusText = anyErr?.response?.statusText
  if (status) {
    if (status === 401) return '未授权（401），请重新登录'
    if (status === 403) return '无权限（403）'
    if (status === 404) return '资源不存在（404）'
    if (status >= 500) return `服务端错误（${status}${statusText ? ' ' + statusText : ''}）`
  }

  // 3) 网络层错误（常见：TMDb 国内直连超时就走这里）
  const code = anyErr?.code
  if (code === 'ECONNABORTED') return '请求超时，请检查网络或配置 TMDb 代理'
  if (code === 'ERR_NETWORK') return '网络不通，请检查服务端网络或代理配置'

  // 4) 兜底：原生 error message
  if (typeof anyErr?.message === 'string' && anyErr.message.trim()) {
    return anyErr.message.trim()
  }

  return ''
}

/**
 * 生成带有真实错误内容的 toast 文案。
 *
 * @param err         捕获到的错误
 * @param fallback    提取不到具体内容时使用的兜底文案（如 '刮削失败'）
 * @returns           最终展示文案，格式为 `${fallback}: ${msg}` 或 `${fallback}`
 */
export function formatErrMsg(err: unknown, fallback: string): string {
  const msg = extractErrMsg(err)
  if (!msg) return fallback
  // 避免重复拼接
  if (msg === fallback) return fallback
  return `${fallback}: ${msg}`
}
