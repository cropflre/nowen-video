import api from './client'
import { withToken } from './stream'
import type {
  SubtitleInfo,
  ASRTask,
  TranslatedSubtitle,
  ASRServiceStatus,
  ExtractedSubtitleFile,
} from '@/types'

// ==================== 字幕 ====================
export const subtitleApi = {
  getTracks: (mediaId: string) =>
    api.get<{ data: SubtitleInfo }>(`/subtitle/${mediaId}/tracks`),

  getExtractUrl: (mediaId: string, index: number) =>
    withToken(`/api/subtitle/${mediaId}/extract/${index}`),

  getExternalUrl: (path: string) =>
    withToken(`/api/subtitle/external?path=${encodeURIComponent(path)}`),

  // AI 字幕生成
  generateAI: (mediaId: string, language?: string) =>
    api.post<{ message: string; data: ASRTask }>(`/subtitle/${mediaId}/ai/generate`, { language: language || '' }),

  getAIStatus: (mediaId: string) =>
    api.get<{ data: ASRTask }>(`/subtitle/${mediaId}/ai/status`),

  getAISubtitleUrl: (mediaId: string) =>
    withToken(`/api/subtitle/${mediaId}/ai/serve`),

  deleteAI: (mediaId: string) =>
    api.delete(`/subtitle/${mediaId}/ai`),

  // Phase 4: 字幕翻译
  translate: (mediaId: string, targetLang: string) =>
    api.post<{ message: string; data: ASRTask }>(`/subtitle/${mediaId}/translate`, { target_lang: targetLang }),

  getTranslateStatus: (mediaId: string, lang?: string) =>
    api.get<{ data: ASRTask | TranslatedSubtitle[] }>(`/subtitle/${mediaId}/translate/status`, { params: lang ? { lang } : {} }),

  getTranslatedSubtitleUrl: (mediaId: string, lang: string) =>
    withToken(`/api/subtitle/${mediaId}/translate/${lang}/serve`),

  listTranslated: (mediaId: string) =>
    api.get<{ data: TranslatedSubtitle[] }>(`/subtitle/${mediaId}/translate/status`),

  // ASR 服务状态
  getASRStatus: () =>
    api.get<{ data: ASRServiceStatus }>('/asr/status'),

  // P0: 批量字幕提取导出
  extractAll: (mediaId: string, format?: string, tracks?: number[]) =>
    api.post<{ message: string; data: { files: ExtractedSubtitleFile[]; success: number; failed: number; total: number } }>(
      `/subtitle/${mediaId}/extract-all`, { format: format || 'srt', tracks: tracks || [] }
    ),

  // P2: 异步字幕提取（大文件，通过 WebSocket 推送进度）
  extractAllAsync: (mediaId: string, title?: string, format?: string, tracks?: number[]) =>
    api.post<{ message: string }>(
      `/subtitle/${mediaId}/extract-all/async`, { format: format || 'srt', tracks: tracks || [], title: title || '' }
    ),

  // 下载已提取的字幕文件
  getDownloadUrl: (path: string) =>
    withToken(`/api/subtitle/download?path=${encodeURIComponent(path)}`),
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
