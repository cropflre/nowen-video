import api from './client'
import type {
  SearchIntent,
  AIStatus,
  AIErrorLog,
  AICacheStats,
  AITestResult,
} from '@/types'

// ==================== AI 智能功能 ====================
export const aiApi = {
  // AI 智能搜索（解析自然语言查询）
  smartSearch: (q: string) =>
    api.get<{ data: SearchIntent }>('/ai/search', { params: { q } }),

  // 获取 AI 服务状态（管理员）
  getStatus: () =>
    api.get<{ data: AIStatus }>('/admin/ai/status'),

  // 更新 AI 配置（管理员）
  updateConfig: (updates: Partial<{
    enabled: boolean
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
  }>) =>
    api.put<{ message: string; data: AIStatus }>('/admin/ai/config', updates),

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
