import { useEffect, useRef, useCallback, useState } from 'react'
import { useAuthStore } from '@/stores/auth'

// ==================== 事件类型 ====================

export const WS_EVENTS = {
  // 扫描事件
  SCAN_STARTED: 'scan_started',
  SCAN_PROGRESS: 'scan_progress',
  SCAN_COMPLETED: 'scan_completed',
  SCAN_FAILED: 'scan_failed',
  // 刮削事件
  SCRAPE_STARTED: 'scrape_started',
  SCRAPE_PROGRESS: 'scrape_progress',
  SCRAPE_COMPLETED: 'scrape_completed',
  // 转码事件
  TRANSCODE_STARTED: 'transcode_started',
  TRANSCODE_PROGRESS: 'transcode_progress',
  TRANSCODE_COMPLETED: 'transcode_completed',
  TRANSCODE_FAILED: 'transcode_failed',
  // 本地媒体分析事件
  MEDIA_ANALYSIS_PROGRESS: 'media_analysis_progress',
  MEDIA_ANALYSIS_COMPLETE: 'media_analysis_complete',
  // 统一任务事件
  TASK_UPDATED: 'task_updated',
  // 媒体库变更事件
  LIBRARY_DELETED: 'library_deleted',
  LIBRARY_UPDATED: 'library_updated',
  // 扫描阶段事件
  SCAN_PHASE: 'scan_phase',
  // 文件管理事件
  FILE_IMPORTED: 'file_imported',
  FILE_DELETED: 'file_deleted',
  BATCH_RENAME_COMPLETE: 'batch_rename_complete',
  FILE_SCRAPE_PROGRESS: 'file_scrape_progress',
  // AI 字幕（ASR）事件
  ASR_STARTED: 'asr_started',
  ASR_PROGRESS: 'asr_progress',
  ASR_COMPLETED: 'asr_completed',
  ASR_FAILED: 'asr_failed',
  // 字幕翻译事件
  TRANSLATE_PROGRESS: 'translate_progress',
  TRANSLATE_COMPLETED: 'translate_completed',
  TRANSLATE_FAILED: 'translate_failed',
  // 视频预处理事件
  PREPROCESS_STARTED: 'preprocess_started',
  PREPROCESS_PROGRESS: 'preprocess_progress',
  PREPROCESS_COMPLETED: 'preprocess_completed',
  PREPROCESS_FAILED: 'preprocess_failed',
  PREPROCESS_PAUSED: 'preprocess_paused',
  PREPROCESS_CANCELLED: 'preprocess_cancelled',
  // 字幕预处理事件
  SUB_PREPROCESS_STARTED: 'sub_preprocess_started',
  SUB_PREPROCESS_PROGRESS: 'sub_preprocess_progress',
  SUB_PREPROCESS_COMPLETED: 'sub_preprocess_completed',
  SUB_PREPROCESS_FAILED: 'sub_preprocess_failed',
  SUB_PREPROCESS_SKIPPED: 'sub_preprocess_skipped',
  // 字幕提取事件（P2）
  SUB_EXTRACT_STARTED: 'sub_extract_started',
  SUB_EXTRACT_PROGRESS: 'sub_extract_progress',
  SUB_EXTRACT_COMPLETED: 'sub_extract_completed',
  SUB_EXTRACT_FAILED: 'sub_extract_failed',
  // 成人批量刮削事件
  ADULT_BATCH_STARTED: 'adult_batch_started',
  ADULT_BATCH_PROGRESS: 'adult_batch_progress',
  ADULT_BATCH_COMPLETED: 'adult_batch_completed',
  ADULT_BATCH_FAILED: 'adult_batch_failed',
  ADULT_BATCH_PAUSED: 'adult_batch_paused',
  ADULT_BATCH_RESUMED: 'adult_batch_resumed',
  ADULT_BATCH_CANCELLED: 'adult_batch_cancelled',
  // 成人文件夹懒人刮削事件
  ADULT_FOLDER_BATCH_STARTED: 'adult_folder_batch_started',
  ADULT_FOLDER_BATCH_PROGRESS: 'adult_folder_batch_progress',
  ADULT_FOLDER_BATCH_COMPLETED: 'adult_folder_batch_completed',
  ADULT_FOLDER_BATCH_FAILED: 'adult_folder_batch_failed',
  ADULT_FOLDER_BATCH_CANCELLED: 'adult_folder_batch_cancelled',
  // 文件夹操作事件
  FOLDER_RENAMED: 'folder_renamed',
  FOLDER_DELETED: 'folder_deleted',
  // 懒人入库（一键入库）事件
  INGEST_PROGRESS: 'ingest_progress',
} as const

export type WSEventType = (typeof WS_EVENTS)[keyof typeof WS_EVENTS]

// ==================== 事件数据类型 ====================

export interface ScanProgressData {
  library_id: string
  library_name: string
  phase: 'scanning' | 'scraping'
  current: number
  total: number
  new_found: number
  message: string
}

export interface ScanPhaseData {
  library_id: string
  library_name: string
  phase: 'scanning' | 'scraping' | 'ai_organizing' | 'merging' | 'matching' | 'cleaning' | 'completed'
  step_current: number
  step_total: number
  current?: number
  total?: number
  message: string
}

export interface ScrapeProgressData {
  library_id: string
  library_name: string
  current: number
  total: number
  success: number
  failed: number
  media_title: string
  message: string
}

export interface TranscodeProgressData {
  task_id: string
  media_id: string
  title: string
  quality: string
  progress: number
  speed: string
  message: string
}

export interface MediaAnalysisProgressData {
  task_id: string
  media_id: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'interrupted'
  stage: string
  progress: number
  error?: string
}

export interface TaskLifecycleUpdatedData {
  kind: 'scan' | 'scrape' | 'transcode'
  source_id?: string
  status: 'queued' | 'running' | 'completed' | 'failed' | 'cancelled'
  source_event: string
}

export interface TaskActionUpdatedData {
  id: string
  kind: 'scan' | 'scrape' | 'transcode'
  source_id: string
  action: 'retry' | 'rollback'
  accepted: boolean
  message: string
}

export type TaskUpdatedData = TaskLifecycleUpdatedData | TaskActionUpdatedData

export interface LibraryChangedData {
  library_id: string
  library_name: string
  action: string
  message: string
}

export interface WSMessage {
  type: WSEventType
  data: ScanProgressData | ScanPhaseData | ScrapeProgressData | TranscodeProgressData | MediaAnalysisProgressData | TaskUpdatedData
  timestamp: number
}

// ==================== 事件监听器类型 ====================

type WSEventHandler = (data: any) => void

// ==================== WebSocket Hook ====================

interface UseWebSocketOptions {
  /** 自动重连（默认 true） */
  autoReconnect?: boolean
  /** 重连间隔毫秒（默认 3000） */
  reconnectInterval?: number
  /** 最大重连次数（默认 10） */
  maxRetries?: number
}

interface UseWebSocketReturn {
  /** 是否已连接 */
  connected: boolean
  /** 订阅事件 */
  on: (event: WSEventType, handler: WSEventHandler) => void
  /** 取消订阅 */
  off: (event: WSEventType, handler: WSEventHandler) => void
  /** 最后一条消息 */
  lastMessage: WSMessage | null
}

export function useWebSocket(options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const {
    autoReconnect = true,
    reconnectInterval = 3000,
    maxRetries = 10,
  } = options

  const wsRef = useRef<WebSocket | null>(null)
  const retriesRef = useRef(0)
  const listenersRef = useRef<Map<string, Set<WSEventHandler>>>(new Map())
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [connected, setConnected] = useState(false)
  const [lastMessage, setLastMessage] = useState<WSMessage | null>(null)

  const token = useAuthStore((s) => s.token)
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)

  const on = useCallback((event: WSEventType, handler: WSEventHandler) => {
    if (!listenersRef.current.has(event)) {
      listenersRef.current.set(event, new Set())
    }
    listenersRef.current.get(event)!.add(handler)
  }, [])

  const off = useCallback((event: WSEventType, handler: WSEventHandler) => {
    listenersRef.current.get(event)?.delete(handler)
  }, [])

  const dispatchEvent = useCallback((msg: WSMessage) => {
    const handlers = listenersRef.current.get(msg.type)
    if (handlers) {
      handlers.forEach((handler) => {
        try {
          handler(msg.data)
        } catch (e) {
          console.error('[WS] 事件处理器错误:', e)
        }
      })
    }
  }, [])

  const connect = useCallback(() => {
    if (!token || !isAuthenticated) return
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/ws?token=${encodeURIComponent(token)}`

    try {
      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        if (ws !== wsRef.current) {
          ws.close()
          return
        }
        console.log('[WS] 已连接')
        setConnected(true)
        retriesRef.current = 0
      }

      ws.onmessage = (event) => {
        try {
          const messages = event.data.split('\n')
          messages.forEach((msgStr: string) => {
            if (!msgStr.trim()) return
            const msg: WSMessage = JSON.parse(msgStr)
            setLastMessage(msg)
            dispatchEvent(msg)
          })
        } catch (e) {
          console.error('[WS] 消息解析失败:', e)
        }
      }

      ws.onclose = (event) => {
        if (ws !== wsRef.current) return
        console.log('[WS] 连接关闭:', event.code, event.reason)
        setConnected(false)
        wsRef.current = null

        if (autoReconnect && retriesRef.current < maxRetries && isAuthenticated) {
          retriesRef.current++
          console.log(`[WS] ${reconnectInterval}ms 后重连 (${retriesRef.current}/${maxRetries})`)
          reconnectTimerRef.current = setTimeout(connect, reconnectInterval)
        }
      }

      ws.onerror = (error) => {
        if (ws !== wsRef.current) return
        console.error('[WS] 连接错误:', error)
      }
    } catch (e) {
      console.error('[WS] 创建连接失败:', e)
    }
  }, [token, isAuthenticated, autoReconnect, reconnectInterval, maxRetries, dispatchEvent])

  useEffect(() => {
    if (isAuthenticated && token) {
      connect()
    }

    return () => {
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
      if (wsRef.current) {
        const ws = wsRef.current
        wsRef.current = null
        ws.onclose = null
        ws.onerror = null
        if (ws.readyState === WebSocket.CONNECTING) {
          ws.onopen = () => ws.close()
        } else {
          ws.onopen = null
          ws.close()
        }
      }
    }
  }, [isAuthenticated, token, connect])

  return { connected, on, off, lastMessage }
}
