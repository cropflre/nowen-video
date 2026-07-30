import api from './client'

export type UnifiedTaskKind = 'scan' | 'scrape' | 'transcode'
export type UnifiedTaskStatus = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'

export interface UnifiedTask {
  id: string
  kind: UnifiedTaskKind
  status: UnifiedTaskStatus
  title: string
  subtitle?: string
  message?: string
  progress: number
  source_id?: string
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

export const taskCenterApi = {
  list: (params?: { active?: boolean; limit?: number }) =>
    api.get<{ data: TaskCenterSnapshot }>('/admin/tasks', { params }),
}
