import api from './client'
import type {
  Tag,
  CreateTagRequest,
  BatchAddTagsRequest,
  ShareLink,
  CreateShareLinkRequest,
  MatchRule,
  CreateMatchRuleRequest,
  TestMatchRuleRequest,
  BatchMoveRequest,
  BatchMoveResult,
  ListResponse,
} from '@/types'

// ==================== P1: 批量移动媒体 ====================
export const batchMoveApi = {
  batchMove: (data: BatchMoveRequest) =>
    api.post<{ data: BatchMoveResult }>('/admin/media/batch-move', data),
}

// ==================== P2: 标签管理 ====================
export const tagApi = {
  list: (category?: string) =>
    api.get<ListResponse<Tag>>('/tags', { params: { category } }),

  create: (data: CreateTagRequest) =>
    api.post<{ data: Tag }>('/tags', data),

  update: (id: string, data: Partial<CreateTagRequest>) =>
    api.put<{ data: Tag }>(`/tags/${id}`, data),

  delete: (id: string) =>
    api.delete(`/tags/${id}`),

  listCategories: () =>
    api.get<ListResponse<string>>('/tags/categories'),

  addToMedia: (mediaID: string, tagID: string) =>
    api.post('/tags/media', { media_id: mediaID, tag_id: tagID }),

  removeFromMedia: (mediaID: string, tagID: string) =>
    api.delete(`/tags/media/${mediaID}/${tagID}`),

  getMediaTags: (mediaID: string) =>
    api.get<ListResponse<Tag>>(`/tags/media/${mediaID}`),

  batchAdd: (data: BatchAddTagsRequest) =>
    api.post<{ data: { added: number } }>('/tags/media/batch', data),
}

// ==================== P2: 分享链接 ====================
export const shareApi = {
  create: (data: CreateShareLinkRequest) =>
    api.post<{ data: ShareLink }>('/shares', data),

  list: (page = 1, size = 20) =>
    api.get<{ data: ShareLink[]; total: number; page: number; size: number }>('/shares', { params: { page, size } }),

  delete: (id: string) =>
    api.delete(`/shares/${id}`),

  toggle: (id: string) =>
    api.post<{ data: ShareLink }>(`/shares/${id}/toggle`),

  // 公开接口（无需认证）
  getByCode: (code: string, password?: string) =>
    api.get<{ data: { share: ShareLink; media?: unknown; series?: unknown } }>(`/share/${code}`, { params: { password } }),
}

// ==================== P3: 自定义匹配规则 ====================
export const matchRuleApi = {
  list: (libraryID?: string) =>
    api.get<ListResponse<MatchRule>>('/admin/match-rules', { params: { library_id: libraryID } }),

  create: (data: CreateMatchRuleRequest) =>
    api.post<{ data: MatchRule }>('/admin/match-rules', data),

  update: (id: string, data: Partial<MatchRule>) =>
    api.put<{ data: MatchRule }>(`/admin/match-rules/${id}`, data),

  delete: (id: string) =>
    api.delete(`/admin/match-rules/${id}`),

  test: (data: TestMatchRuleRequest) =>
    api.post<{ data: { matched: boolean } }>('/admin/match-rules/test', data),
}
