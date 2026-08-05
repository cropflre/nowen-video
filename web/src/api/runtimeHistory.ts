import api from './client'

export interface RuntimeHistoryRetentionPolicy {
  metadata_mode: string
  automatic_metadata_prune: boolean
  artifact_content: string
  cleanup_evidence: string
  sensitive_fields_hidden: string[]
}

export interface RuntimeHistoryItem {
  id: string
  legacy_task_id?: string
  media_id: string
  media_title?: string
  intent: string
  profile_id?: string
  status: string
  desired_state?: string
  priority: number
  start_ms: number
  duration_ms: number
  session_id?: string
  planner_version?: string
  encoding_plan_hash?: string
  timestamp_plan_hash?: string
  timeline_origin_ms: number
  attempt_count: number
  artifact_count: number
  artifact_bytes: number
  last_backend?: string
  last_error_code?: string
  last_error_message?: string
  integrity_state: 'active_residual' | 'legacy_projection_linked' | 'execution_record_only' | string
  created_at: string
  updated_at: string
  claimed_at?: string
  completed_at?: string
}

export interface RuntimeHistoryAttempt {
  id: string
  number: number
  backend?: string
  status: string
  exit_code: number
  signal?: string
  error_code?: string
  error_message?: string
  stderr_tail?: string
  started_at?: string
  completed_at?: string
  created_at: string
}

export interface RuntimeHistoryArtifact {
  id: string
  attempt_id?: string
  kind: string
  profile_id?: string
  status: string
  size_bytes: number
  duration_ms: number
  attestation_status?: string
  attestation_hash?: string
  error_code?: string
  error_message?: string
  cleanup_state?: string
  cleanup_attempts: number
  cleanup_error_code?: string
  cleanup_error_message?: string
  cleanup_completed_at?: string
  cleanup_disposition?: string
  cleanup_original_path?: string
  cleanup_rollback_until?: string
  published_at?: string
  expires_at?: string
  created_at: string
}

export interface RuntimeHistoryList {
  items: RuntimeHistoryItem[]
  total: number
  page: number
  page_size: number
  total_pages: number
  generated_at: string
  retention: RuntimeHistoryRetentionPolicy
}

export interface RuntimeHistoryDetail {
  job: RuntimeHistoryItem
  attempts: RuntimeHistoryAttempt[]
  artifacts: RuntimeHistoryArtifact[]
  retention: RuntimeHistoryRetentionPolicy
}

export interface RuntimeHistoryLegacyMigration {
  source: string
  generation: number
  status: string
  target_rows: number
  scanned_rows: number
  imported_jobs: number
  artifacts_queued: number
  artifacts_blocked: number
  missing_paths: number
  failure_count: number
  consecutive_failures: number
  last_error_code?: string
  last_error_message?: string
  cursor_updated_at?: string
  cursor_id?: string
  high_water_updated_at?: string
  high_water_id?: string
  completed_at?: string
  source_retire_after?: string
  next_source_check_at?: string
  retirement_eligible: boolean
}

export interface RuntimeHistorySummary {
  jobs: number
  attempts: number
  artifacts: number
  legacy_tasks: number
  orphan_legacy_tasks: number
  artifact_bytes: number
  by_status: Record<string, number>
  oldest_at?: string
  newest_at?: string
  generated_at: string
  retention: RuntimeHistoryRetentionPolicy
  legacy_migration?: RuntimeHistoryLegacyMigration
}

export const runtimeHistoryApi = {
  list: (params?: {
    page?: number
    page_size?: number
    status?: string
    intent?: string
    media_id?: string
    search?: string
    from?: string
    to?: string
  }) => api.get<{ data: RuntimeHistoryList }>('/admin/runtime-history', { params }),
  summary: () => api.get<{ data: RuntimeHistorySummary }>('/admin/runtime-history/summary'),
  detail: (id: string) => api.get<{ data: RuntimeHistoryDetail }>(`/admin/runtime-history/jobs/${encodeURIComponent(id)}`),
}
