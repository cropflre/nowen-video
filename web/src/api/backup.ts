import api from './client'

// ==================== 智能通知系统 ====================
export const notificationApi = {
  // 获取通知配置
  getConfig: () =>
    api.get<{ data: import('@/types').NotificationConfig }>('/admin/notification/config'),

  // 更新通知配置
  updateConfig: (config: import('@/types').NotificationConfig) =>
    api.put<{ message: string; data: import('@/types').NotificationConfig }>('/admin/notification/config', config),

  // 测试通知
  test: (channel: 'email' | 'telegram' | 'webhook') =>
    api.post<{ message: string }>(`/admin/notification/test?channel=${channel}`),
}

// ==================== 批量元数据编辑 ====================
export const batchMetadataApi = {
  // 批量更新媒体元数据
  batchUpdateMedia: (data: import('@/types').BatchUpdateRequest) =>
    api.post<{ message: string; data: import('@/types').BatchUpdateResult }>('/admin/batch/metadata/media', data),

  // 批量更新剧集合集元数据
  batchUpdateSeries: (data: { series_ids: string[]; updates: Record<string, string> }) =>
    api.post<{ message: string; data: import('@/types').BatchUpdateResult }>('/admin/batch/metadata/series', data),
}

// ==================== 媒体库导入/导出 ====================
export const importExportApi = {
  // 测试连接
  testConnection: (source: import('@/types').ImportSource) =>
    api.post<{ message: string }>('/admin/import/test', source),

  // 获取外部服务器媒体库列表
  fetchLibraries: (source: import('@/types').ImportSource) =>
    api.post<{ data: import('@/types').EmbyLibrary[] }>('/admin/import/libraries', source),

  // 从外部服务器导入
  importFromExternal: (data: { source: import('@/types').ImportSource; library_id: string; target_library_id: string }) =>
    api.post<{ message: string; data: import('@/types').ImportResult }>('/admin/import/external', data),

  // 导出媒体库数据
  exportLibrary: (libraryId?: string) =>
    api.get<{ message: string; data: import('@/types').ExportData }>('/admin/export/library', { params: { library_id: libraryId } }),

  // 从导出数据导入
  importFromData: (data: { data: import('@/types').ExportData; target_library_id: string }) =>
    api.post<{ message: string; data: import('@/types').ImportResult }>('/admin/import/data', data),
}
