import api from './client'
import type { PreprocessTask, PreprocessStatistics, SystemLoadInfo, PerformanceConfig } from '@/types'

// ==================== 视频预处理 ====================
export const preprocessApi = {
  // 提交单个媒体预处理
  submit: (mediaId: string, priority?: number) =>
    api.post<{ message: string; data: PreprocessTask }>('/admin/preprocess/submit', {
      media_id: mediaId,
      priority: priority || 0,
    }),

  // 批量提交预处理
  batchSubmit: (mediaIds: string[], priority?: number) =>
    api.post<{ message: string; data: { submitted: number; tasks: PreprocessTask[] } }>(
      '/admin/preprocess/batch',
      { media_ids: mediaIds, priority: priority || 0 }
    ),

  // 提交整个媒体库预处理
  submitLibrary: (libraryId: string, priority?: number) =>
    api.post<{ message: string; data: { submitted: number } }>(
      `/admin/preprocess/library/${libraryId}`,
      { priority: priority || 0 }
    ),

  // 获取任务列表
  listTasks: (page?: number, pageSize?: number, status?: string) =>
    api.get<{ data: { tasks: PreprocessTask[]; total: number; page: number; page_size: number } }>(
      '/admin/preprocess/tasks',
      { params: { page: page || 1, page_size: pageSize || 20, status: status || '' } }
    ),

  // 获取任务详情
  getTask: (taskId: string) =>
    api.get<{ data: PreprocessTask }>(`/admin/preprocess/tasks/${taskId}`),

  // 获取媒体的预处理状态
  getMediaStatus: (mediaId: string) =>
    api.get<{ data: PreprocessTask }>(`/preprocess/media/${mediaId}/status`),

  // 暂停任务
  pauseTask: (taskId: string) =>
    api.post(`/admin/preprocess/tasks/${taskId}/pause`),

  // 恢复任务
  resumeTask: (taskId: string) =>
    api.post(`/admin/preprocess/tasks/${taskId}/resume`),

  // 取消任务
  cancelTask: (taskId: string) =>
    api.post(`/admin/preprocess/tasks/${taskId}/cancel`),

  // 重试任务
  retryTask: (taskId: string) =>
    api.post(`/admin/preprocess/tasks/${taskId}/retry`),

  // 删除任务
  deleteTask: (taskId: string) =>
    api.delete(`/admin/preprocess/tasks/${taskId}`),

  // 批量删除任务
  batchDeleteTasks: (taskIds: string[]) =>
    api.post<{ message: string; data: { deleted: number } }>('/admin/preprocess/tasks/batch-delete', { task_ids: taskIds }),

  // 批量取消任务
  batchCancelTasks: (taskIds: string[]) =>
    api.post<{ message: string; data: { cancelled: number } }>('/admin/preprocess/tasks/batch-cancel', { task_ids: taskIds }),

  // 批量重试任务
  batchRetryTasks: (taskIds: string[]) =>
    api.post<{ message: string; data: { retried: number } }>('/admin/preprocess/tasks/batch-retry', { task_ids: taskIds }),

  // 获取统计信息
  getStatistics: () =>
    api.get<{ data: PreprocessStatistics }>('/admin/preprocess/statistics'),

  // 获取系统负载
  getSystemLoad: () =>
    api.get<{ data: SystemLoadInfo }>('/admin/preprocess/system-load'),

  // 清理预处理缓存
  cleanCache: (mediaId: string) =>
    api.delete(`/admin/preprocess/cache/${mediaId}`),

  // 获取性能配置
  getPerformanceConfig: () =>
    api.get<{ data: PerformanceConfig }>('/admin/preprocess/performance-config'),

  // 更新性能配置
  updatePerformanceConfig: (updates: Partial<Pick<PerformanceConfig, 'resource_limit' | 'max_transcode_jobs' | 'transcode_preset' | 'hw_accel'>>) =>
    api.put<{ message: string; data: PerformanceConfig }>('/admin/preprocess/performance-config', updates),
}
