import api from './client'

export interface STRMGlobalConfig {
  default_user_agent: string
  default_referer: string
  connect_timeout: number
  rewrite_hls: boolean
  remote_probe: boolean
  remote_probe_timeout: number
  domain_user_agents: Record<string, string>
  domain_referers: Record<string, string>
}

export interface MediaSTRMInfo {
  id: string
  title: string
  is_strm: boolean
  stream_url: string
  stream_ua: string
  stream_referer: string
  stream_cookie: string
  stream_headers: Record<string, string> | null
  stream_refresh_url: string
}

export interface UpdateMediaSTRMPayload {
  stream_url?: string
  user_agent?: string
  referer?: string
  cookie?: string
  headers?: Record<string, string>
  refresh_url?: string
  clear_headers?: boolean
}

/**
 * STRM（.strm 远程流）相关管理 API
 *
 * 覆盖两类端点：
 *  1. 全局配置：默认 UA/Referer、HLS 重写、远程 probe 开关、域名白名单
 *  2. 单条 media 覆写：应急情况下直接粘贴浏览器 F12 的 UA/Referer/Cookie 立即生效
 */
export const strmApi = {
  getConfig: () => api.get<{ data: STRMGlobalConfig }>('/admin/strm/config'),
  updateConfig: (cfg: Partial<STRMGlobalConfig>) =>
    api.put<{ message: string; data: STRMGlobalConfig }>('/admin/strm/config', cfg),

  getMediaSTRM: (mediaId: string) =>
    api.get<{ data: MediaSTRMInfo }>(`/admin/media/${mediaId}/strm`),
  updateMediaSTRM: (mediaId: string, payload: UpdateMediaSTRMPayload) =>
    api.put<{ message: string; data: Partial<MediaSTRMInfo> }>(
      `/admin/media/${mediaId}/strm`,
      payload,
    ),
}
