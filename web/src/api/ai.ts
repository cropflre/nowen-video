import api from './client'
import { useServerProfileStore } from '@/stores/serverProfile'
import type {
  SearchIntent,
  AIStatus,
  AIErrorLog,
  AICacheStats,
  AITestResult,
} from '@/types'

let cachedAIStatus: AIStatus | null = null

type AIStatusReadOptions = {
  allowCachedOnError?: boolean
}

function rememberAIStatus(status: AIStatus | null | undefined) {
  if (status) cachedAIStatus = status
}

async function refreshServerProfile() {
  await useServerProfileStore.getState().load(true)
}

// ==================== AI 智能功能 ====================
export const aiApi = {
  // AI 智能搜索（解析自然语言查询）
  smartSearch: (q: string) =>
    api.get<{ data: SearchIntent }>('/ai/search', { params: { q } }),

  // 获取 AI 服务状态（管理员）。首次读取失败必须显式失败；一旦成功读取过，
  // 后续瞬时网络抖动允许调用方继续使用最后一次已确认状态，避免把“读取失败”
  // 错误展示成“未配置/未启用”。管理页的入口预检会关闭该回退以区分真实失败。
  getStatus: async (options: AIStatusReadOptions = {}) => {
    try {
      const response = await api.get<{ data: AIStatus }>('/admin/ai/status')
      rememberAIStatus(response.data.data)
      return response
    } catch (error) {
      if (options.allowCachedOnError !== false && cachedAIStatus) {
        return { data: { data: cachedAIStatus } }
      }
      throw error
    }
  },

  // 更新 AI 配置（管理员）
  updateConfig: async (updates: Partial<{
    enabled: boolean
    auto_pilot: boolean
    block_local_ai: boolean
    provider: string
    api_base: string
    api_key: string
    model: string
    timeout: number
    enable_smart_search: boolean
    enable_recommend_reason: boolean
    enable_metadata_enhance: boolean
    monthly_budget: number
    cache_ttl_hours: number
    max_concurrent: number
    request_interval_ms: number
    // 各 provider 配置档案（仅传需要更新的 provider；api_key 留空字符串则保留原值）
    profiles: Record<string, { api_base?: string; api_key?: string; model?: string; model_chain?: string[] }>
  }>) => {
    const response = await api.put<{ message: string; data: AIStatus }>('/admin/ai/config', updates)
    rememberAIStatus(response.data.data)
    await refreshServerProfile()
    return response
  },

  // 一键启用全自动托管模式（管理员）
  // 提供 provider / api_key 即可，会自动填好 api_base / model，并打开所有 AI 子开关
  enableAutoPilot: async (params?: { provider?: string; api_key?: string }) => {
    const response = await api.post<{ message: string; data: AIStatus }>('/admin/ai/auto-pilot', params || {})
    rememberAIStatus(response.data.data)
    await refreshServerProfile()
    return response
  },

  // 测试 AI 连接（管理员）
  testConnection: () =>
    api.post<{ data: AITestResult }>('/admin/ai/test'),

  // 清空 AI 缓存（管理员）
  clearCache: () =>
    api.delete<{ message: string; data: { cleared: number } }>('/admin/ai/cache'),

  // 获取缓存统计（管理员）
  getCacheStats: () =>
    api.get<{ data: AICacheStats }>('/admin/ai/cache'),

  // 获取错误日志（管理员）
  getErrorLogs: () =>
    api.get<{ data: AIErrorLog[] }>('/admin/ai/errors'),

  // 测试智能搜索（管理员）
  testSmartSearch: (query: string) =>
    api.post<{ data: AITestResult }>('/admin/ai/test/search', { query }),

  // 测试推荐理由（管理员）
  testRecommendReason: (title: string, genres: string) =>
    api.post<{ data: AITestResult }>('/admin/ai/test/recommend', { title, genres }),

  // ==================== V7：AI 智能调度器 / 用量监控 / 故障转移 ====================

  // 列出所有 AI 提供商的开箱即用预设
  listProviderPresets: () =>
    api.get<{ data: import('@/types').AIProviderPreset[] }>('/admin/ai/presets'),

  // 一键配置通义千问（仅需提供 api_key）
  quickConfigQwen: async (apiKey: string) => {
    const response = await api.post<{
      message: string
      data: { status: import('@/types').AIStatus; test: import('@/types').AITestResult }
    }>('/admin/ai/quick-config/qwen', { api_key: apiKey })
    rememberAIStatus(response.data.data.status)
    await refreshServerProfile()
    return response
  },

  // 获取 AIRouter 当前状态（首选/生效 provider、月用量、切换链等）
  getRouterSnapshot: () =>
    api.get<{ data: import('@/types').AIRouterSnapshot }>('/admin/ai/router'),

  // 强制切换到指定 provider
  forceSwitchProvider: (provider: string) =>
    api.post<{ message: string; data: import('@/types').AIRouterSnapshot }>('/admin/ai/router/switch', { provider }),

  // 恢复主 provider
  restoreProvider: () =>
    api.post<{ message: string; data: import('@/types').AIRouterSnapshot }>('/admin/ai/router/restore'),

  // 切换审计日志
  listFailoverLogs: (limit = 100) =>
    api.get<{ data: import('@/types').AIFailoverLog[] }>('/admin/ai/router/logs', { params: { limit } }),

  // 用量曲线（按天聚合）
  getUsageBuckets: (range: 'day' | 'week' | 'month' | 'year' = 'month', provider?: string) =>
    api.get<{ data: import('@/types').AIUsageReport }>('/admin/ai/usage', { params: { range, provider } }),
}

// ==================== AI 助手 ====================
export const aiAssistantApi = {
  // 对话
  chat: (data: {
    session_id?: string
    message: string
    media_ids?: string[]
    library_id?: string
  }) =>
    api.post<{ data: import('@/types').ChatResponse }>('/admin/assistant/chat', data),

  // 执行操作
  executeAction: (data: { session_id: string; action_id: string }) =>
    api.post<{ data: import('@/types').ExecuteResponse }>('/admin/assistant/execute', data),

  // 撤销操作
  undoOperation: (opId: string) =>
    api.post<{ data: import('@/types').ExecuteResponse }>(`/admin/assistant/undo/${opId}`),

  // 获取会话
  getSession: (sessionId: string) =>
    api.get<{ data: import('@/types').ChatSession }>(`/admin/assistant/session/${sessionId}`),

  // 删除会话
  deleteSession: (sessionId: string) =>
    api.delete(`/admin/assistant/session/${sessionId}`),

  // 获取操作历史
  getOperationHistory: (limit?: number) =>
    api.get<{ data: import('@/types').AssistantOperation[] }>('/admin/assistant/history', { params: { limit } }),

  // 误分类分析
  analyzeMisclassification: () =>
    api.get<{ data: import('@/types').MisclassificationReport }>('/admin/assistant/misclassification'),

  // 批量重分类
  reclassifyFiles: (data: import('@/types').ReclassifyRequest) =>
    api.post<{ data: import('@/types').ReclassifyResult }>('/admin/assistant/reclassify', data),
}