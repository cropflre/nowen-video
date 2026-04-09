import api from './client'
import type { SubtitlePreprocessTask, SubtitlePreprocessStatistics } from '@/types'

// ==================== 字幕预处理 ====================
export const subtitlePreprocessApi = {
  // 提交单个媒体字幕预处理
  submit: (mediaId: string, targetLangs?: string[], forceRegenerate?: boolean) =>
    api.post<{ message: string; data: SubtitlePreprocessTask }>('/admin/subtitle-preprocess/submit', {
      media_id: mediaId,
      target_langs: targetLangs || [],
      force_regenerate: forceRegenerate || false,
    }),

  // 批量提交字幕预处理
  batchSubmit: (mediaIds: string[], targetLangs?: string[], forceRegenerate?: boolean) =>
    api.post<{ message: string; data: { submitted: number; tasks: SubtitlePreprocessTask[] } }>('/admin/subtitle-preprocess/batch', {
      media_ids: mediaIds,
      target_langs: targetLangs || [],
      force_regenerate: forceRegenerate || false,
    }),

  // 提交整个媒体库字幕预处理
  submitLibrary: (libraryId: string, targetLangs?: string[], forceRegenerate?: boolean) =>
    api.post<{ message: string; data: { submitted: number } }>(`/admin/subtitle-preprocess/library/${libraryId}`, {
      target_langs: targetLangs || [],
      force_regenerate: forceRegenerate || false,
    }),

  // 获取任务列表
  listTasks: (params?: { page?: number; page_size?: number; status?: string }) =>
    api.get<{ data: { tasks: SubtitlePreprocessTask[]; total: number; page: number; page_size: number } }>('/admin/subtitle-preprocess/tasks', { params }),

  // 获取任务详情
  getTask: (taskId: string) =>
    api.get<{ data: SubtitlePreprocessTask }>(`/admin/subtitle-preprocess/tasks/${taskId}`),

  // 取消任务
  cancelTask: (taskId: string) =>
    api.post(`/admin/subtitle-preprocess/tasks/${taskId}/cancel`),

  // 重试任务
  retryTask: (taskId: string) =>
    api.post(`/admin/subtitle-preprocess/tasks/${taskId}/retry`),

  // 删除任务
  deleteTask: (taskId: string) =>
    api.delete(`/admin/subtitle-preprocess/tasks/${taskId}`),

  // 批量删除任务
  batchDeleteTasks: (taskIds: string[]) =>
    api.post<{ message: string; data: { deleted: number } }>('/admin/subtitle-preprocess/tasks/batch-delete', { task_ids: taskIds }),

  // 批量取消任务
  batchCancelTasks: (taskIds: string[]) =>
    api.post<{ message: string; data: { cancelled: number } }>('/admin/subtitle-preprocess/tasks/batch-cancel', { task_ids: taskIds }),

  // 批量重试任务
  batchRetryTasks: (taskIds: string[]) =>
    api.post<{ message: string; data: { retried: number } }>('/admin/subtitle-preprocess/tasks/batch-retry', { task_ids: taskIds }),

  // 获取统计信息
  getStatistics: () =>
    api.get<{ data: SubtitlePreprocessStatistics }>('/admin/subtitle-preprocess/statistics'),

  // 获取媒体字幕预处理状态（用户可查询）
  getMediaStatus: (mediaId: string) =>
    api.get<{ data: SubtitlePreprocessTask }>(`/subtitle-preprocess/media/${mediaId}/status`),
}
