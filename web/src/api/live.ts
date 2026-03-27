import api from './client'

// ==================== V3: 实时直播 ====================
export const liveApi = {
  // 获取直播源列表
  listSources: (category?: string, page = 1, size = 50) =>
    api.get<{ data: import('@/types').LiveSource[]; total: number }>('/live/sources', { params: { category, page, size } }),

  // 获取直播源详情
  getSource: (id: string) =>
    api.get<{ data: import('@/types').LiveSource }>(`/live/sources/${id}`),

  // 获取直播分类
  getCategories: () =>
    api.get<{ data: string[] }>('/live/categories'),

  // 添加直播源（管理员）
  addSource: (data: { name: string; url: string; type?: string; category?: string; logo?: string; quality?: string }) =>
    api.post<{ data: import('@/types').LiveSource }>('/admin/live/sources', data),

  // 管理员获取直播源列表（包含禁用的）
  listSourcesAdmin: (params?: { category?: string; keyword?: string; page?: number; size?: number }) =>
    api.get<{ data: import('@/types').LiveSource[]; total: number }>('/admin/live/sources', { params }),

  // 切换直播源启用/禁用（管理员）
  toggleSourceActive: (id: string) =>
    api.post<{ data: import('@/types').LiveSource }>(`/admin/live/sources/${id}/toggle`),

  // 更新直播源（管理员）
  updateSource: (id: string, data: Partial<import('@/types').LiveSource>) =>
    api.put<{ data: import('@/types').LiveSource }>(`/admin/live/sources/${id}`, data),

  // 删除直播源（管理员）
  deleteSource: (id: string) =>
    api.delete<{ message: string }>(`/admin/live/sources/${id}`),

  // 检测直播源（管理员）
  checkSource: (id: string) =>
    api.post<{ status: string }>(`/admin/live/sources/${id}/check`),

  // 批量检测（管理员）
  batchCheck: () =>
    api.post<{ data: Record<string, string> }>('/admin/live/sources/batch-check'),

  // 导入M3U（管理员）
  importM3U: (data: { name: string; url?: string; file_path?: string }) =>
    api.post<{ data: import('@/types').LivePlaylist; count: number; message: string }>('/admin/live/playlists/import', data),

  // 获取播放列表（管理员）
  listPlaylists: () =>
    api.get<{ data: import('@/types').LivePlaylist[] }>('/admin/live/playlists'),

  // 删除播放列表（管理员）
  deletePlaylist: (id: string) =>
    api.delete<{ message: string }>(`/admin/live/playlists/${id}`),

  // 开始录制
  startRecording: (sourceId: string, title: string) =>
    api.post<{ data: import('@/types').LiveRecording; message: string }>('/live/recordings', { source_id: sourceId, title }),

  // 停止录制
  stopRecording: (id: string) =>
    api.post<{ message: string }>(`/live/recordings/${id}/stop`),

  // 获取录制列表
  listRecordings: (page = 1, size = 20) =>
    api.get<{ data: import('@/types').LiveRecording[]; total: number }>('/live/recordings', { params: { page, size } }),

  // 删除录制
  deleteRecording: (id: string) =>
    api.delete<{ message: string }>(`/live/recordings/${id}`),
}

// ==================== V3: 云端同步 ====================
export const cloudSyncApi = {
  // 注册设备
  registerDevice: (data: { device_id: string; device_name: string; device_type: string; platform?: string; app_version?: string }) =>
    api.post<{ data: import('@/types').SyncDevice }>('/sync/devices', data),

  // 获取设备列表
  listDevices: () =>
    api.get<{ data: import('@/types').SyncDevice[] }>('/sync/devices'),

  // 注销设备
  unregisterDevice: (deviceId: string) =>
    api.delete<{ message: string }>(`/sync/devices/${deviceId}`),

  // 同步数据（上传）
  syncData: (data: { device_id: string; data_type: string; data_key: string; data_value: string; version?: number }) =>
    api.post<{ message: string }>('/sync/data', data),

  // 拉取数据（下载）
  pullData: (dataType: string, sinceVersion = 0) =>
    api.get<{ data: import('@/types').SyncRecord[] }>('/sync/data', { params: { data_type: dataType, since: sinceVersion } }),

  // 批量同步
  batchSync: (deviceId: string, records: import('@/types').SyncRecord[]) =>
    api.post<{ success: number; failed: number; message: string }>('/sync/batch', { device_id: deviceId, records }),

  // 全量同步
  fullSync: () =>
    api.get<{ data: Record<string, unknown> }>('/sync/full'),

  // 获取同步配置
  getSyncConfig: () =>
    api.get<{ data: import('@/types').UserSyncConfig }>('/sync/config'),

  // 更新同步配置
  updateSyncConfig: (config: Partial<import('@/types').UserSyncConfig>) =>
    api.put<{ message: string }>('/sync/config', config),

  // 导出数据
  exportData: () =>
    api.get('/sync/export', { responseType: 'blob' }),
}
