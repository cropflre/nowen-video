import api from './client'

export interface AgentRef { id: string; kind: 'media' | 'series' }
export interface AgentCard extends AgentRef { title: string; year: number; rating: number; duration?: number; position?: number }
export interface AgentSearch { query: string; genre: string; type: string; year_min: number; year_max: number; min_rating: number; max_minutes: number; unwatched: boolean }
export type AgentActionKind = 'play' | 'favorite' | 'unfavorite' | 'pause' | 'resume' | 'volume_up' | 'volume_down' | 'mute' | 'unmute' | 'large_text_on' | 'large_text_off' | 'subtitle_zh' | 'subtitle_en' | 'subtitle_ja' | 'subtitle_auto' | 'subtitle_off' | 'inspect_tasks' | 'prepare_scan' | 'prepare_metadata'
export interface AgentAction { kind: AgentActionKind; target?: AgentRef; library_id?: string }
export interface AgentManagement { library_count: number; media_count: number; needs_metadata: number; needs_rating: number; libraries: { id: string; name: string; media_count: number }[] }
export interface AgentReply { message: string; cards: AgentCard[]; action?: AgentAction; search?: AgentSearch; management?: AgentManagement; metadata_candidates?: boolean; scan_libraries?: { id: string; name: string }[]; mode: 'basic' | 'ai' | 'fallback' }
export interface AgentRun {
  id: string; library_id: string; library_name: string; approval_digest: string;
  kind: 'discover_movies' | 'fill_movie_metadata';
  status: 'draft' | 'running' | 'completed' | 'failed' | 'cancelled' | 'expired' | 'interrupted' | 'reverted';
  media_id?: string; media_title?: string; source_title?: string; source_url?: string;
  changes?: { field: string; label: string; before: string; after: string }[];
  paths: string[]; effect: string; min_file_size_mb: number; file_filter: boolean;
  new_count: number; message: string; expires_at: string; created_at: string;
}
export const viewerAgentApi = {
  runs: () => api.get<{ data: AgentRun[] }>('/admin/agent/runs', { timeout: 10000 }),
  prepareScan: (library_id: string) => api.post<{ data: AgentRun }>('/admin/agent/runs', { library_id }, { timeout: 10000 }),
  prepareMetadata: (media_id: string) => api.post<{ data: AgentRun }>('/admin/agent/runs/metadata', { media_id }, { timeout: 12000 }),
  undoMetadata: (run: AgentRun) => api.post<{ data: AgentRun }>(`/admin/agent/runs/${encodeURIComponent(run.id)}/undo`, { approval_digest: run.approval_digest }, { timeout: 10000 }),
  confirmScan: (run: AgentRun) => api.post<{ data: AgentRun }>(`/admin/agent/runs/${encodeURIComponent(run.id)}/confirm`, { approval_digest: run.approval_digest }, { timeout: 10000 }),
  cancelScan: (run: AgentRun) => api.post<{ data: AgentRun }>(`/admin/agent/runs/${encodeURIComponent(run.id)}/cancel`, {}, { timeout: 10000 }),
  chat: (message: string, context: AgentRef[], current_media_id: string, search: AgentSearch | undefined, signal: AbortSignal) =>
    api.post<{ data: AgentReply }>('/agent/chat', { message, context, current_media_id, search }, { signal, timeout: 15000 }),
}
