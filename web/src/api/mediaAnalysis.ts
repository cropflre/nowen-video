import api from './client'

export interface MediaHighlight {
  id: string
  media_id: string
  title: string
  start_time: number
  end_time: number
  score: number
  tags: string
  source: string
  analysis_method: string
  thumbnail_url?: string
  preview_url?: string
  version: number
}

export interface MediaHighlightList {
  highlights: MediaHighlight[]
  stale: boolean
}

export interface MediaAnalysisTask {
  id: string
  media_id: string
  task_type: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'interrupted' | string
  stage: string
  progress: number
  result?: string
  error?: string
  started_at?: string | null
  completed_at?: string | null
  created_at?: string
  updated_at?: string
}

export const mediaAnalysisApi = {
  getHighlights: (mediaId: string) =>
    api.get<{ data: MediaHighlightList }>(`/media/${mediaId}/highlights`),

  analyzeHighlights: (mediaId: string) =>
    api.post<{ data: MediaAnalysisTask; message: string }>(`/media/${mediaId}/highlights/analyze`),

  getStatus: (mediaId: string) =>
    api.get<{ data: MediaAnalysisTask | null }>(`/media/${mediaId}/highlights/status`),

  deleteHighlights: (mediaId: string) =>
    api.delete<{ message: string }>(`/media/${mediaId}/highlights`),
}
