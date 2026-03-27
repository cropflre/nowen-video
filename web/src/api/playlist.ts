import api from './client'
import type {
  Playlist,
  CreatePlaylistRequest,
  ListResponse,
} from '@/types'

// ==================== 播放列表 ====================
export const playlistApi = {
  list: () =>
    api.get<ListResponse<Playlist>>('/playlists'),

  create: (data: CreatePlaylistRequest) =>
    api.post<{ data: Playlist }>('/playlists', data),

  detail: (id: string) =>
    api.get<{ data: Playlist }>(`/playlists/${id}`),

  delete: (id: string) =>
    api.delete(`/playlists/${id}`),

  addItem: (playlistId: string, mediaId: string) =>
    api.post(`/playlists/${playlistId}/items/${mediaId}`),

  removeItem: (playlistId: string, mediaId: string) =>
    api.delete(`/playlists/${playlistId}/items/${mediaId}`),
}
