import api from './client'

export const MEDIA_COMPUTE_PROTOCOL_VERSION = 2
export const MEDIA_COMPUTE_JOB_HIGHLIGHT_V1 = 'highlight_v1'
export const MEDIA_COMPUTE_CAPABILITY_HIGHLIGHT_V1 = 'highlight_v1'
export const MEDIA_COMPUTE_JOB_PREVIEW_THUMBNAIL_V1 = 'preview_thumbnail_v1'
export const MEDIA_COMPUTE_CAPABILITY_PREVIEW_THUMBNAIL_V1 = 'preview_thumbnail_v1'
export const MEDIA_COMPUTE_JOB_CHAPTER_DETECT_V1 = 'chapter_detect_v1'
export const MEDIA_COMPUTE_CAPABILITY_CHAPTER_DETECT_V1 = 'chapter_detect_v1'

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

export interface MediaComputePreviewThumbnailInput {
  media_id: string
  highlight_id: string
  fingerprint: string
  stream_url: string
  frame_times: number[]
  max_width: number
  frame_rate: number
}

export interface MediaComputePreviewFrame {
  time: number
  mime: string
  data_base64: string
}

export interface MediaComputePreviewThumbnailResult {
  fingerprint: string
  highlight_id: string
  frames: MediaComputePreviewFrame[]
}

export interface MediaComputeChapterDetectInput {
  media_id: string
  fingerprint: string
  duration: number
  stream_url: string
  sample_times: number[]
  probe_gap_seconds: number
  min_chapter_seconds: number
  max_chapters: number
  capture_width: number
  engine_version: number
}

export interface MediaComputeChapterCandidate {
  time: number
  score: number
}

export interface MediaComputeChapterDetectResult {
  fingerprint: string
  candidates: MediaComputeChapterCandidate[]
}

export interface MediaComputeTaskClaim<TInput = unknown> {
  protocol_version?: number
  job_type?: string
  required_capability?: string
  task_id: string
  claim_token: string
  input?: TInput
  lease_expires_at?: string

  // V1 compatibility mirror. Only highlight_v1 should ever populate these.
  media_id?: string
  fingerprint?: string
  duration?: number
  stream_url?: string
  sample_times?: number[]
  max_highlights?: number
  engine_version?: number
}

export type MediaAnalysisWorkerClaim = MediaComputeTaskClaim<MediaComputeHighlightInput>

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

export interface MediaComputeHighlightResult {
  fingerprint: string
  highlights: MediaAnalysisWorkerResultItem[]
}

export interface MediaComputeTaskComplete<TResult = unknown> {
  claim_token: string
  job_type: string
  result: TResult
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

  // Historical transport URL, V2 generic claim envelope.
  claimWorkerTask: (heartbeat: MediaAnalysisWorkerHeartbeat) =>
    api.post<{ data?: MediaComputeTaskClaim }>('/media-analysis/workers/claim', heartbeat),

  updateWorkerProgress: (taskId: string, progress: MediaAnalysisWorkerProgress) =>
    api.post<void>(`/media-analysis/workers/tasks/${taskId}/progress`, progress),

  // V1 compatibility call kept for already-shipped clients and any old web bundle.
  completeWorkerTask: (taskId: string, result: MediaAnalysisWorkerComplete) =>
    api.post<{ message: string }>(`/media-analysis/workers/tasks/${taskId}/complete`, result),

  // V2 uses the same transport path but sends a job-scoped generic result envelope.
  completeComputeTask: <TResult>(taskId: string, result: MediaComputeTaskComplete<TResult>) =>
    api.post<{ message: string }>(`/media-analysis/workers/tasks/${taskId}/complete`, result),

  failWorkerTask: (taskId: string, failure: MediaAnalysisWorkerFailure) =>
    api.post<void>(`/media-analysis/workers/tasks/${taskId}/fail`, failure),
}
