import api from './client'

export type UnifiedTaskKind = 'scan' | 'scrape' | 'artifact_cleanup' | 'legacy_artifact_migration' | 'legacy_projection_migration' | 'storage_incident'
export type UnifiedTaskStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'
export type UnifiedTaskAction = 'retry' | 'rollback'

export interface UnifiedTask {
  id: string
  kind: UnifiedTaskKind
  status: UnifiedTaskStatus
  title: string
  subtitle?: string
  message?: string
  progress: number
  source_id?: string
  actions: UnifiedTaskAction[]
  created_at?: string
  updated_at?: string
  started_at?: string
  completed_at?: string
}

export interface TaskCenterSummary {
  total: number
  active: number
  by_status: Record<string, number>
  by_kind: Record<string, number>
  generated_at: string
}

export interface TaskCenterSnapshot {
  tasks: UnifiedTask[]
  summary: TaskCenterSummary
}

export interface TaskActionResult {
  id: string
  kind: UnifiedTaskKind
  source_id: string
  action: UnifiedTaskAction
  accepted: boolean
  message: string
}

export const taskCenterApi = {
  list: (params?: { active?: boolean; limit?: number }) =>
    api.get<{ data: TaskCenterSnapshot }>('/admin/tasks', { params }),
  action: (kind: UnifiedTaskKind, sourceId: string, action: UnifiedTaskAction) =>
    api.post<{ data: TaskActionResult }>(`/admin/tasks/${kind}/${encodeURIComponent(sourceId)}/${action}`),
}
