import api from './client'

// ==================== V2.1: WebDAV 存储管理 ====================

export interface WebDAVConfig {
  enabled: boolean
  server_url: string
  username: string
  password: string
  base_path: string
  timeout: number
  enable_pool: boolean
  pool_size: number
  enable_cache: boolean
  cache_ttl_hours: number
  max_retries: number
  retry_interval: number
}

export interface WebDAVStatus {
  enabled: boolean
  server_url: string
  base_path: string
  client_count: number
  connected?: boolean
  error?: string
}

// ==================== V2.3: Alist 聚合网盘 ====================

export interface AlistConfig {
  enabled: boolean
  server_url: string
  username: string
  password: string
  token: string
  base_path: string
  timeout: number
  enable_cache: boolean
  cache_ttl_hours: number
  read_block_size_mb: number
  read_block_count: number
}

export interface AlistStatus {
  enabled: boolean
  server_url: string
  base_path: string
  connected: boolean
}

export interface TestAlistRequest {
  server_url: string
  username?: string
  password?: string
  token?: string
  base_path?: string
}

// ==================== V2.3: S3 兼容对象存储 ====================

export interface S3Config {
  enabled: boolean
  endpoint: string
  region: string
  access_key: string
  secret_key: string
  bucket: string
  base_path: string
  path_style: boolean
  timeout: number
  enable_cache: boolean
  cache_ttl_hours: number
  read_block_size_mb: number
  read_block_count: number
}

export interface S3Status {
  enabled: boolean
  endpoint: string
  bucket: string
  region: string
  path_style: boolean
  connected: boolean
}

export interface TestS3Request {
  endpoint: string
  region?: string
  access_key: string
  secret_key?: string
  bucket: string
  base_path?: string
  path_style?: boolean
}

// ==================== 聚合状态 ====================

export interface StorageStatus {
  webdav: WebDAVStatus
  local: {
    enabled: boolean
    type: string
  }
  alist?: AlistStatus
  s3?: S3Status
}

export interface TestWebDAVRequest {
  server_url: string
  username?: string
  password?: string
  base_path?: string
}

export type StorageReadOptions = {
  allowCachedOnError?: boolean
}

let cachedAlistConfig: AlistConfig | null = null
let cachedS3Config: S3Config | null = null
let cachedStorageStatus: StorageStatus | null = null

export const storageApi = {
  // ---------- WebDAV ----------
  getWebDAVConfig: () =>
    api.get<{ data: WebDAVConfig }>('/admin/storage/webdav'),
  updateWebDAVConfig: (data: Partial<WebDAVConfig>) =>
    api.put<{ message: string }>('/admin/storage/webdav', data),
  testWebDAVConnection: (data: TestWebDAVRequest) =>
    api.post<{ message: string }>('/admin/storage/webdav/test', data),
  getWebDAVStatus: () =>
    api.get<{ data: WebDAVStatus }>('/admin/storage/webdav/status'),
  registerWebDAVLibrary: (libraryId: string) =>
    api.post<{ message: string }>('/admin/storage/webdav/libraries/register', {
      library_id: libraryId,
    }),

  // ---------- V2.3: Alist ----------
  getAlistConfig: async (options: StorageReadOptions = {}) => {
    try {
      const response = await api.get<{ data: AlistConfig }>('/admin/storage/alist')
      cachedAlistConfig = response.data.data
      return response
    } catch (error) {
      if (options.allowCachedOnError !== false && cachedAlistConfig) {
        return { data: { data: cachedAlistConfig } }
      }
      throw error
    }
  },
  updateAlistConfig: async (data: Partial<AlistConfig>) => {
    const response = await api.put<{ message: string }>('/admin/storage/alist', data)
    if (cachedAlistConfig) {
      const safeData: Partial<AlistConfig> = { ...data }
      delete safeData.password
      delete safeData.token
      cachedAlistConfig = { ...cachedAlistConfig, ...safeData }
    }
    return response
  },
  testAlistConnection: (data: TestAlistRequest) =>
    api.post<{ message: string }>('/admin/storage/alist/test', data),

  // ---------- V2.3: S3 ----------
  getS3Config: async (options: StorageReadOptions = {}) => {
    try {
      const response = await api.get<{ data: S3Config }>('/admin/storage/s3')
      cachedS3Config = response.data.data
      return response
    } catch (error) {
      if (options.allowCachedOnError !== false && cachedS3Config) {
        return { data: { data: cachedS3Config } }
      }
      throw error
    }
  },
  updateS3Config: async (data: Partial<S3Config>) => {
    const response = await api.put<{ message: string }>('/admin/storage/s3', data)
    if (cachedS3Config) {
      const safeData: Partial<S3Config> = { ...data }
      delete safeData.secret_key
      cachedS3Config = { ...cachedS3Config, ...safeData }
    }
    return response
  },
  testS3Connection: (data: TestS3Request) =>
    api.post<{ message: string }>('/admin/storage/s3/test', data),

  // ---------- 聚合状态 ----------
  getStorageStatus: async (options: StorageReadOptions = {}) => {
    try {
      const response = await api.get<{ data: StorageStatus }>('/admin/storage/status')
      cachedStorageStatus = response.data.data
      return response
    } catch (error) {
      if (options.allowCachedOnError !== false && cachedStorageStatus) {
        return { data: { data: cachedStorageStatus } }
      }
      throw error
    }
  },
}
