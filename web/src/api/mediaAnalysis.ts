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

export type MediaAnalysisExecutionMode = 'auto' | 'client_preferred' | 'server_only' | 'off'

export interface MediaAnalysisWorkerConfig {
  execution_mode: MediaAnalysisExecutionMode
  modes: MediaAnalysisExecutionMode[]
}

export interface MediaAnalysisWorkerHeartbeat {
  worker_id: string
  kind: 'android' | 'desktop' | 'client' | string
  name: string
  version: string
  capabilities: string[]
  network: string
  charging: boolean
  battery_percent: number
}

export interface MediaAnalysisWorker extends MediaAnalysisWorkerHeartbeat {
  last_seen: string
  state: 'idle' | 'busy' | 'unavailable' | string
  task_id?: string
}

export interface MediaAnalysisWorkerClaim {
  task_id: string
  claim_token: string
  media_id: string
  fingerprint: string
  duration: number
  stream_url: string
  sample_times: number[]
  max_highlights: number
  engine_version: number
  lease_expires_at: string
}

export interface MediaAnalysisWorkerProgress {
  claim_token: string
  stage: string
  progress: number
}

export interface MediaAnalysisWorkerResultItem {
  title?: string
  start_time: number
  end_time: number
  score: number
  analysis_method: string
  thumbnail_base64?: string
  thumbnail_mime?: string
}

export interface MediaAnalysisWorkerComplete {
  claim_token: string
  fingerprint: string
  highlights: MediaAnalysisWorkerResultItem[]
}

export interface MediaAnalysisWorkerFailure {
  claim_token: string
  error: string
}

export const mediaAnalysisApi = {
  getHighlights: (mediaId: string) =>
    api.get<{ data: MediaHighlightList }>(`/media/${mediaId}/highlights`),

  analyzeHighlights: (mediaId: string) =>
    api.post<{ data: MediaAnalysisTask; message: string; execution_mode?: MediaAnalysisExecutionMode }>(`/media/${mediaId}/highlights/analyze`),

  getStatus: (mediaId: string) =>
    api.get<{ data: MediaAnalysisTask | null }>(`/media/${mediaId}/highlights/status`),

  deleteHighlights: (mediaId: string) =>
    api.delete<{ message: string }>(`/media/${mediaId}/highlights`),

  getWorkerConfig: () =>
    api.get<{ data: MediaAnalysisWorkerConfig }>('/admin/media-analysis/config'),

  updateWorkerConfig: (executionMode: MediaAnalysisExecutionMode) =>
    api.put<{ data: Pick<MediaAnalysisWorkerConfig, 'execution_mode'> }>('/admin/media-analysis/config', {
      execution_mode: executionMode,
    }),

  getWorkers: () =>
    api.get<{ data: MediaAnalysisWorker[] }>('/admin/media-analysis/workers'),

  heartbeatWorker: (heartbeat: MediaAnalysisWorkerHeartbeat) =>
    api.post<{ data: MediaAnalysisWorker }>('/media-analysis/workers/heartbeat', heartbeat),

  claimWorkerTask: (heartbeat: MediaAnalysisWorkerHeartbeat) =>
    api.post<{ data?: MediaAnalysisWorkerClaim }>('/media-analysis/workers/claim', heartbeat),

  updateWorkerProgress: (taskId: string, progress: MediaAnalysisWorkerProgress) =>
    api.post<void>(`/media-analysis/workers/tasks/${taskId}/progress`, progress),

  completeWorkerTask: (taskId: string, result: MediaAnalysisWorkerComplete) =>
    api.post<{ message: string }>(`/media-analysis/workers/tasks/${taskId}/complete`, result),

  failWorkerTask: (taskId: string, failure: MediaAnalysisWorkerFailure) =>
    api.post<void>(`/media-analysis/workers/tasks/${taskId}/fail`, failure),
}
