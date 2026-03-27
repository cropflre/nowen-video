import api from './client'

// ==================== V3: AI 场景识别与内容理解 ====================
export const aiSceneApi = {
  // 生成视频章节
  generateChapters: (mediaId: string) =>
    api.post<{ data: import('@/types').AIAnalysisTask; message: string }>(`/media/${mediaId}/ai/chapters`),

  // 获取视频章节
  getChapters: (mediaId: string) =>
    api.get<{ data: import('@/types').VideoChapter[] }>(`/media/${mediaId}/chapters`),

  // 提取精彩片段
  extractHighlights: (mediaId: string) =>
    api.post<{ data: import('@/types').AIAnalysisTask; message: string }>(`/media/${mediaId}/ai/highlights`),

  // 获取精彩片段
  getHighlights: (mediaId: string) =>
    api.get<{ data: import('@/types').VideoHighlight[] }>(`/media/${mediaId}/highlights`),

  // 生成封面候选
  generateCoverCandidates: (mediaId: string) =>
    api.post<{ data: import('@/types').AIAnalysisTask; message: string }>(`/media/${mediaId}/ai/covers`),

  // 获取封面候选
  getCoverCandidates: (mediaId: string) =>
    api.get<{ data: import('@/types').CoverCandidate[] }>(`/media/${mediaId}/covers`),

  // 选择封面
  selectCover: (mediaId: string, candidateId: string) =>
    api.post<{ message: string }>(`/media/${mediaId}/covers/${candidateId}/select`),

  // 应用选中的封面
  applyCover: (mediaId: string) =>
    api.post<{ message: string }>(`/media/${mediaId}/covers/apply`),

  // 获取AI分析任务列表
  getAnalysisTasks: (mediaId: string) =>
    api.get<{ data: import('@/types').AIAnalysisTask[] }>(`/media/${mediaId}/ai/tasks`),

  // 获取单个分析任务
  getAnalysisTask: (taskId: string) =>
    api.get<{ data: import('@/types').AIAnalysisTask }>(`/ai/tasks/${taskId}`),
}

// ==================== V3: 家庭社交互动 ====================
export const familySocialApi = {
  // 创建家庭组
  createGroup: (name: string) =>
    api.post<{ data: import('@/types').FamilyGroup }>('/family/groups', { name }),

  // 获取家庭组列表
  listGroups: () =>
    api.get<{ data: import('@/types').FamilyGroup[] }>('/family/groups'),

  // 加入家庭组
  joinGroup: (inviteCode: string) =>
    api.post<{ data: import('@/types').FamilyGroup; message: string }>('/family/groups/join', { invite_code: inviteCode }),

  // 获取家庭组详情
  getGroup: (groupId: string) =>
    api.get<{ data: import('@/types').FamilyGroup }>(`/family/groups/${groupId}`),

  // 解散家庭组
  deleteGroup: (groupId: string) =>
    api.delete<{ message: string }>(`/family/groups/${groupId}`),

  // 离开家庭组
  leaveGroup: (groupId: string) =>
    api.post<{ message: string }>(`/family/groups/${groupId}/leave`),

  // 重新生成邀请码
  regenerateInviteCode: (groupId: string) =>
    api.post<{ invite_code: string }>(`/family/groups/${groupId}/invite-code`),

  // 分享媒体到家庭组
  shareMedia: (groupId: string, data: { media_id?: string; series_id?: string; message?: string }) =>
    api.post<{ data: import('@/types').MediaShare }>(`/family/groups/${groupId}/share`, data),

  // 获取家庭组分享列表
  listGroupShares: (groupId: string, page = 1, size = 20) =>
    api.get<{ data: import('@/types').MediaShare[]; total: number }>(`/family/groups/${groupId}/shares`, { params: { page, size } }),

  // 点赞
  likeMedia: (mediaId: string) =>
    api.post<{ message: string }>(`/media/${mediaId}/like`),

  // 取消点赞
  unlikeMedia: (mediaId: string) =>
    api.delete<{ message: string }>(`/media/${mediaId}/like`),

  // 获取点赞状态
  getLikeStatus: (mediaId: string) =>
    api.get<{ is_liked: boolean; count: number }>(`/media/${mediaId}/like`),

  // 推荐媒体
  recommendMedia: (data: { to_user_id: string; media_id?: string; series_id?: string; message?: string }) =>
    api.post<{ data: import('@/types').MediaRecommendation }>('/family/recommend', data),

  // 获取推荐列表
  listRecommendations: (page = 1, size = 20) =>
    api.get<{ data: import('@/types').MediaRecommendation[]; total: number }>('/family/recommendations', { params: { page, size } }),

  // 标记推荐已读
  markRecommendationRead: (recId: string) =>
    api.post<{ message: string }>(`/family/recommendations/${recId}/read`),

  // 获取未读推荐数
  getUnreadCount: () =>
    api.get<{ count: number }>('/family/recommendations/unread'),
}
