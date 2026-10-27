import api from './client'
import type {
  LoginRequest,
  RegisterRequest,
  TokenResponse,
} from '@/types'

// ==================== 认证 ====================
export const authApi = {
  createPairing: (device_name: string) => api.post<{ data: PairingRequest }>('/auth/device-pairing', { device_name }, { timeout: 10000 }),
  pollPairing: (id: string, secret: string) => api.post<{ data: { status: string; authorization?: TokenResponse } }>('/auth/device-pairing/status', { id, secret }, { timeout: 10000 }),
  cancelPairing: (id: string, secret: string) => api.post('/auth/device-pairing/cancel', { id, secret }, { timeout: 5000 }),
  inspectPairing: (code: string) => api.post<{ data: { device_name: string; expires_at: string } }>('/users/me/device-pairing/inspect', { code }, { timeout: 10000 }),
  approvePairing: (code: string) => api.post('/users/me/device-pairing/approve', { code }, { timeout: 10000 }),
  devices: () => api.get<{ data: LoginDevice[]; current_id: string }>('/users/me/devices', { timeout: 10000 }),
  revokeDevice: (id: string) => api.delete('/users/me/devices/' + encodeURIComponent(id), { timeout: 10000 }),
  revokeAllDevices: () => api.post('/users/me/devices/revoke-all', {}, { timeout: 10000 }),
  logout: () => api.post<{ server_revoked: boolean }>('/auth/logout', {}, { timeout: 5000 }),
  deviceStatus: () => api.get('/users/me/device-status', { timeout: 10000 }),
  login: (data: LoginRequest) =>
    api.post<TokenResponse>('/auth/login', data),

  register: (data: RegisterRequest) =>
    api.post<TokenResponse>('/auth/register', data),

  refreshToken: () =>
    api.post<TokenResponse>('/auth/refresh'),

  /** 获取系统初始化状态（公开接口，无需认证） */
  getStatus: () =>
    api.get<{ data: { initialized: boolean; registration_open: boolean } }>('/auth/status'),

  /** 修改密码（需要认证，成功时返回替换当前会话的新 Token） */
  changePassword: (oldPassword: string, newPassword: string) =>
    api.put<{ message: string; data?: TokenResponse }>('/auth/password', {
      old_password: oldPassword,
      new_password: newPassword,
    }),
}

export interface PairingRequest {
  id: string
  secret: string
  code: string
  device_name: string
  expires_in_ms: number
}
export interface LoginDevice {
  id: string
  name: string
  created_at: string
  last_seen_at: string
  expires_at: string
}
