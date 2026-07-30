import api from './client'

export interface ServerCapability {
  available: boolean
  enabled: boolean
  configured: boolean
  configurable: boolean
  requires_restart: boolean
  pending_restart: boolean
  mode?: string
}

export interface ServerProfileManifest {
  schema_version: number
  profile: 'lite' | 'full' | 'unknown'
  capabilities: Record<string, ServerCapability>
}

export interface ServerHealthData {
  status: string
  version: string
  server_name: string
  profile?: string
  schema_version?: number
  capabilities?: Record<string, ServerCapability>
  features?: Record<string, unknown>
}

export const serverApi = {
  capabilities: () => api.get<{ data: ServerProfileManifest }>('/capabilities'),
  health: () => api.get<ServerHealthData & { data?: ServerHealthData }>('/health'),
}
