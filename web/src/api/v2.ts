import api from './client'

// ==================== V2: 多用户配置文件 ====================
export const userProfileApi = {
  list: () => api.get<{ data: import('@/types').UserProfile[] }>('/admin/profiles'),
  create: (profile: Partial<import('@/types').UserProfile>) => api.post<{ data: import('@/types').UserProfile }>('/admin/profiles', profile),
  get: (id: string) => api.get<{ data: import('@/types').UserProfile }>(`/admin/profiles/${id}`),
  update: (id: string, updates: Partial<import('@/types').UserProfile>) => api.put(`/admin/profiles/${id}`, updates),
  delete: (id: string) => api.delete(`/admin/profiles/${id}`),
  switch: (id: string, pin?: string) => api.post<{ data: import('@/types').UserProfile }>(`/admin/profiles/${id}/switch`, { pin }),
  getWatchLogs: (id: string, days = 7) => api.get<{ data: import('@/types').ProfileWatchLog[] }>(`/admin/profiles/${id}/watch-logs`, { params: { days } }),
  getDailyUsage: (id: string, days = 30) => api.get<{ data: import('@/types').ProfileDailyUsage[] }>(`/admin/profiles/${id}/usage`, { params: { days } }),
  getStats: (id: string) => api.get<{ data: Record<string, unknown> }>(`/admin/profiles/${id}/stats`),
}

// ==================== V2: 离线下载 ====================
export const offlineDownloadApi = {
  create: (data: { media_id: string; title: string; file_size: number; file_path: string; quality: string }) =>
    api.post<{ data: import('@/types').DownloadTask }>('/admin/downloads', data),
  batchCreate: (items: { media_id: string; title: string; file_size: number; file_path: string; quality: string }[]) =>
    api.post<{ data: import('@/types').DownloadTask[] }>('/admin/downloads/batch', { items }),
  list: (status?: string) => api.get<{ data: import('@/types').DownloadTask[] }>('/admin/downloads', { params: { status } }),
  getQueue: () => api.get<{ data: import('@/types').DownloadQueueInfo }>('/admin/downloads/queue'),
  cancel: (id: string) => api.post(`/admin/downloads/${id}/cancel`),
  pause: (id: string) => api.post(`/admin/downloads/${id}/pause`),
  resume: (id: string) => api.post(`/admin/downloads/${id}/resume`),
  delete: (id: string) => api.delete(`/admin/downloads/${id}`),
}

// ==================== V2: 插件系统 ====================
export const pluginApi = {
  list: () => api.get<{ data: import('@/types').PluginInfo[] }>('/admin/plugins'),
  get: (id: string) => api.get<{ data: import('@/types').PluginInfo; manifest: import('@/types').PluginManifest }>(`/admin/plugins/${id}`),
  enable: (id: string) => api.post(`/admin/plugins/${id}/enable`),
  disable: (id: string) => api.post(`/admin/plugins/${id}/disable`),
  uninstall: (id: string) => api.delete(`/admin/plugins/${id}`),
  updateConfig: (id: string, config: Record<string, unknown>) => api.put(`/admin/plugins/${id}/config`, config),
  scan: () => api.post<{ data: import('@/types').PluginManifest[] }>('/admin/plugins/scan'),
}

// ==================== V2: 音乐库 ====================
export const musicApi = {
  listTracks: (params: { library_id?: string; page?: number; size?: number; sort?: string }) =>
    api.get<{ data: import('@/types').MusicTrack[]; total: number }>('/music/tracks', { params }),
  listAlbums: (params: { library_id?: string; page?: number; size?: number }) =>
    api.get<{ data: import('@/types').MusicAlbum[]; total: number }>('/music/albums', { params }),
  getAlbum: (id: string) => api.get<{ data: import('@/types').MusicAlbum }>(`/music/albums/${id}`),
  search: (q: string, limit = 20) => api.get<{ data: import('@/types').MusicTrack[] }>('/music/search', { params: { q, limit } }),
  getLyrics: (id: string) => api.get<{ data: string }>(`/music/tracks/${id}/lyrics`),
  toggleLove: (id: string) => api.post<{ loved: boolean }>(`/music/tracks/${id}/love`),
  scan: (libraryId: string, path: string) => api.post<{ count: number }>('/admin/music/scan', { library_id: libraryId, path }),
  listPlaylists: () => api.get<{ data: import('@/types').MusicPlaylist[] }>('/music/playlists'),
  createPlaylist: (name: string) => api.post<{ data: import('@/types').MusicPlaylist }>('/music/playlists', { name }),
  getPlaylist: (id: string) => api.get<{ data: import('@/types').MusicPlaylist }>(`/music/playlists/${id}`),
  addToPlaylist: (id: string, trackIds: string[]) => api.post(`/music/playlists/${id}/tracks`, { track_ids: trackIds }),
}

// ==================== V2: 图片库 ====================
export const photoApi = {
  list: (params: { library_id?: string; page?: number; size?: number; sort?: string; album_id?: string; tag?: string; favorite?: string }) =>
    api.get<{ data: import('@/types').Photo[]; total: number }>('/photos', { params }),
  get: (id: string) => api.get<{ data: import('@/types').Photo }>(`/photos/${id}`),
  listAlbums: () => api.get<{ data: import('@/types').PhotoAlbum[] }>('/photos/albums'),
  createAlbum: (name: string, description?: string) => api.post<{ data: import('@/types').PhotoAlbum }>('/photos/albums', { name, description }),
  addToAlbum: (albumId: string, photoIds: string[]) => api.post(`/photos/albums/${albumId}/photos`, { photo_ids: photoIds }),
  toggleFavorite: (id: string) => api.post<{ is_favorite: boolean }>(`/photos/${id}/favorite`),
  setRating: (id: string, rating: number) => api.post(`/photos/${id}/rating`, { rating }),
  search: (q: string, limit = 50) => api.get<{ data: import('@/types').Photo[] }>('/photos/search', { params: { q, limit } }),
  getStats: (libraryId?: string) => api.get<{ data: Record<string, unknown> }>('/photos/stats', { params: { library_id: libraryId } }),
  scan: (libraryId: string, path: string) => api.post<{ count: number }>('/admin/photos/scan', { library_id: libraryId, path }),
}

// ==================== V2: 联邦架构 ====================
export const federationApi = {
  listNodes: () => api.get<{ data: import('@/types').ServerNode[] }>('/admin/federation/nodes'),
  registerNode: (data: { name: string; url: string; api_key: string; role: string }) =>
    api.post<{ data: import('@/types').ServerNode }>('/admin/federation/nodes', data),
  removeNode: (id: string) => api.delete(`/admin/federation/nodes/${id}`),
  syncNode: (id: string, type = 'full') => api.post<{ data: import('@/types').SyncTask }>(`/admin/federation/nodes/${id}/sync`, null, { params: { type } }),
  getStats: () => api.get<{ data: import('@/types').FederationStats }>('/admin/federation/stats'),
  getSyncTasks: (nodeId?: string) => api.get<{ data: import('@/types').SyncTask[] }>('/admin/federation/sync-tasks', { params: { node_id: nodeId } }),
  searchShared: (q: string, page = 1, size = 20) =>
    api.get<{ data: import('@/types').SharedMedia[]; total: number }>('/federation/search', { params: { q, page, size } }),
  getSharedStream: (id: string) => api.get<{ stream_url: string }>(`/federation/stream/${id}`),
}

// ==================== V2: ABR 自适应码率 ====================
export const abrApi = {
  getStatus: () => api.get<{ data: import('@/types').ABRStatus }>('/admin/abr/status'),
  getGPUInfo: () => api.get<{ data: import('@/types').GPUInfo }>('/admin/abr/gpu'),
  cleanCache: (mediaId?: string) => api.delete('/admin/abr/cache', { params: { media_id: mediaId } }),
}
