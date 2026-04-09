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
