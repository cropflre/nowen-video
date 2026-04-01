import api from './client'

// ==================== 刮削数据管理 ====================
export const scrapeApi = {
  // 创建单个刮削任务
  createTask: (data: { url: string; source?: string; media_type?: string; title?: string }) =>
    api.post<{ data: import('@/types').ScrapeTask; message: string }>('/admin/scrape/tasks', data),

  // 批量创建刮削任务
  batchCreateTasks: (data: { urls: string[]; source?: string; media_type?: string }) =>
    api.post<{ message: string; created: number; skipped: number; errors: string[] }>('/admin/scrape/tasks/batch', data),

  // 列表查询
  listTasks: (params: { page?: number; size?: number; status?: string; source?: string }) =>
    api.get<import('@/types').PaginatedResponse<import('@/types').ScrapeTask>>('/admin/scrape/tasks', { params }),

  // 获取任务详情
  getTask: (id: string) =>
    api.get<{ data: import('@/types').ScrapeTask }>(`/admin/scrape/tasks/${id}`),

  // 更新任务
  updateTask: (id: string, data: Record<string, unknown>) =>
    api.put<{ data: import('@/types').ScrapeTask; message: string }>(`/admin/scrape/tasks/${id}`, data),

  // 删除任务
  deleteTask: (id: string) =>
    api.delete(`/admin/scrape/tasks/${id}`),

  // 开始刮削
  startScrape: (id: string) =>
    api.post(`/admin/scrape/tasks/${id}/scrape`),

  // 翻译任务
  translateTask: (id: string, data: { target_lang: string; fields?: string[] }) =>
    api.post(`/admin/scrape/tasks/${id}/translate`, data),

  // 批量刮削
  batchStartScrape: (taskIds: string[]) =>
    api.post<{ message: string; started: number; errors: string[] }>('/admin/scrape/batch/scrape', { task_ids: taskIds }),

  // 批量翻译
  batchTranslate: (data: { task_ids: string[]; target_lang: string; fields?: string[] }) =>
    api.post<{ message: string; started: number; errors: string[] }>('/admin/scrape/batch/translate', data),

  // 批量删除
  batchDeleteTasks: (taskIds: string[]) =>
    api.post<{ message: string; deleted: number }>('/admin/scrape/batch/delete', { task_ids: taskIds }),

  // 导出
  exportTasks: (taskIds: string[]) =>
    api.post<{ data: Record<string, unknown>[] }>('/admin/scrape/export', { task_ids: taskIds }),

  // 统计信息
  getStatistics: () =>
    api.get<{ data: import('@/types').ScrapeStatistics }>('/admin/scrape/statistics'),

  // 操作历史
  getHistory: (params: { task_id?: string; limit?: number }) =>
    api.get<{ data: import('@/types').ScrapeHistory[] }>('/admin/scrape/history', { params }),
}

// ==================== 影视文件管理 ====================
export const fileManagerApi = {
  // 文件列表
  listFiles: (params: {
    page?: number; size?: number; library_id?: string;
    media_type?: string; keyword?: string; sort_by?: string;
    sort_order?: string; scraped?: string
  }) =>
    api.get<import('@/types').PaginatedResponse<import('@/types').Media>>('/admin/files', { params }),

  // 文件详情
  getFileDetail: (id: string) =>
    api.get<{ data: import('@/types').Media }>(`/admin/files/${id}`),

  // 导入单个文件
  importFile: (data: import('@/types').FileImportRequest) =>
    api.post<{ data: import('@/types').Media; message: string }>('/admin/files/import', data),

  // 批量导入
  batchImportFiles: (files: import('@/types').FileImportRequest[]) =>
    api.post<{ message: string; data: import('@/types').BatchImportResult }>('/admin/files/import/batch', { files }),

  // 扫描目录
  scanDirectory: (path: string) =>
    api.get<{ data: import('@/types').ScannedFile[]; total: number }>('/admin/files/scan', { params: { path } }),

  // 更新文件信息
  updateFile: (id: string, data: Record<string, unknown>) =>
    api.put<{ data: import('@/types').Media; message: string }>(`/admin/files/${id}`, data),

  // 删除文件记录
  deleteFile: (id: string) =>
    api.delete(`/admin/files/${id}`),

  // 批量删除
  batchDeleteFiles: (mediaIds: string[]) =>
    api.post<{ message: string; deleted: number; errors: string[] }>('/admin/files/batch/delete', { media_ids: mediaIds }),

  // 单个文件刮削
  scrapeFile: (id: string, source?: string) =>
    api.post(`/admin/files/${id}/scrape`, { source }),

  // 批量刮削
  batchScrapeFiles: (mediaIds: string[], source?: string) =>
    api.post<{ message: string; started: number; errors: string[] }>('/admin/files/batch/scrape', { media_ids: mediaIds, source }),

  // 预览重命名
  previewRename: (mediaIds: string[], template?: string) =>
    api.post<{ data: import('@/types').RenamePreview[] }>('/admin/files/rename/preview', { media_ids: mediaIds, template }),

  // 执行重命名
  executeRename: (mediaIds: string[], template?: string) =>
    api.post<{ message: string; renamed: number; errors: string[] }>('/admin/files/rename/execute', { media_ids: mediaIds, template }),

  // AI智能重命名（支持多语言翻译）
  aiGenerateRenames: (mediaIds: string[], targetLang?: string) =>
    api.post<{ data: import('@/types').RenamePreview[] }>('/admin/files/rename/ai', { media_ids: mediaIds, target_lang: targetLang }),

  // 获取重命名模板
  getRenameTemplates: () =>
    api.get<{ data: import('@/types').RenameTemplate[] }>('/admin/files/rename/templates'),

  // 统计信息
  getStats: () =>
    api.get<{ data: import('@/types').FileManagerStats }>('/admin/files/stats'),

  // 操作日志
  getOperationLogs: (limit?: number) =>
    api.get<{ data: import('@/types').FileOperationLog[] }>('/admin/files/logs', { params: { limit } }),

  // 获取文件夹树形结构
  getFolderTree: (libraryId?: string) =>
    api.get<{ data: import('@/types').FolderNode[] }>('/admin/files/folders', { params: { library_id: libraryId } }),

  // 按文件夹路径查询文件
  listFilesByFolder: (params: {
    path: string; page?: number; size?: number; library_id?: string;
    media_type?: string; keyword?: string; sort_by?: string;
    sort_order?: string; scraped?: string
  }) =>
    api.get<{
      data: import('@/types').Media[]; total: number; page: number; size: number;
      sub_folders: string[]; folder_path: string
    }>('/admin/files/by-folder', { params }),

  // 创建文件夹
  createFolder: (parentPath: string, folderName: string) =>
    api.post<{ message: string }>('/admin/files/folders/create', { parent_path: parentPath, folder_name: folderName }),

  // 重命名文件夹
  renameFolder: (folderPath: string, newName: string) =>
    api.post<{ message: string }>('/admin/files/folders/rename', { folder_path: folderPath, new_name: newName }),

  // 删除文件夹
  deleteFolder: (folderPath: string, force?: boolean) =>
    api.post<{ message: string }>('/admin/files/folders/delete', { folder_path: folderPath, force }),
}
