import api from './client'
import { withToken } from './stream'
import type {
  SubtitleInfo,
} from '@/types'

// ==================== 字幕 ====================
export const subtitleApi = {
  getTracks: (mediaId: string) =>
    api.get<{ data: SubtitleInfo }>(`/subtitle/${mediaId}/tracks`),

  getExtractUrl: (mediaId: string, index: number) =>
    withToken(`/api/subtitle/${mediaId}/extract/${index}`),

  getExternalUrl: (path: string) =>
    withToken(`/api/subtitle/external?path=${encodeURIComponent(path)}`),
}

// ==================== 字幕在线搜索 ====================
export const subtitleSearchApi = {
  // 搜索字幕
  search: (mediaId: string, params: { language?: string; title?: string; year?: number; type?: string }) =>
    api.get<{ data: import('@/types').SubtitleSearchResult[] }>(`/subtitle/${mediaId}/search`, { params }),

  // 下载字幕
  download: (mediaId: string, fileId: string) =>
    api.post<{ message: string; data: import('@/types').SubtitleDownloadResult }>(`/subtitle/${mediaId}/download`, { file_id: fileId }),
}
