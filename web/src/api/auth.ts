import api from './client'
import type {
  LoginRequest,
  RegisterRequest,
  TokenResponse,
} from '@/types'

// ==================== 认证 ====================
export const authApi = {
  login: (data: LoginRequest) =>
    api.post<TokenResponse>('/auth/login', data),

  register: (data: RegisterRequest) =>
    api.post<TokenResponse>('/auth/register', data),

  refreshToken: () =>
    api.post<TokenResponse>('/auth/refresh'),

  /** 获取系统初始化状态（公开接口，无需认证） */
  getStatus: () =>
    api.get<{ data: { initialized: boolean; registration_open: boolean } }>('/auth/status'),

  /** 修改密码（需要认证） */
  changePassword: (oldPassword: string, newPassword: string) =>
    api.put<{ message: string }>('/auth/password', { old_password: oldPassword, new_password: newPassword }),
}
