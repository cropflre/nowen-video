import api from './client'

export const MEDIA_COMPUTE_PROTOCOL_VERSION = 2
export const MEDIA_COMPUTE_JOB_HIGHLIGHT_V1 = 'highlight_v1'

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
  client_protocol_version?: number
  last_seen: string
  state: 'idle' | 'busy' | 'unavailable' | string
  task_id?: string
  current_job_type?: string
}

export interface MediaComputeHighlightInput {
  media_id: string
  fingerprint: string
  duration: number
  stream_url: string
  sample_times: number[]
  max_highlights: number
  engine_version: number
}

export interface MediaAnalysisWorkerClaim {
  // Media Compute Node V2 envelope.
  protocol_version?: number
  job_type?: string
  required_capability?: string
  input?: MediaComputeHighlightInput

  task_id: string
  claim_token: string
  lease_expires_at: string

  // V1 compatibility fields. The server intentionally keeps them while released
  // Android/Desktop clients migrate to the V2 input envelope.
  media_id: string
  fingerprint: string
  duration: number
  stream_url: string
  sample_times: number[]
  max_highlights: number
  engine_version: number
}

export type MediaComputeTaskClaim = MediaAnalysisWorkerClaim

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

  // The historical URL is intentionally retained as a compatibility transport.
  // Its response is already a Media Compute Node V2 task envelope.
  claimWorkerTask: (heartbeat: MediaAnalysisWorkerHeartbeat) =>
    api.post<{ data?: MediaComputeTaskClaim }>('/media-analysis/workers/claim', heartbeat),

  updateWorkerProgress: (taskId: string, progress: MediaAnalysisWorkerProgress) =>
    api.post<void>(`/media-analysis/workers/tasks/${taskId}/progress`, progress),

  completeWorkerTask: (taskId: string, result: MediaAnalysisWorkerComplete) =>
    api.post<{ message: string }>(`/media-analysis/workers/tasks/${taskId}/complete`, result),

  failWorkerTask: (taskId: string, failure: MediaAnalysisWorkerFailure) =>
    api.post<void>(`/media-analysis/workers/tasks/${taskId}/fail`, failure),
}
