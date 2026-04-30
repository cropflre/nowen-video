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
  ADULT_BATCH_PAUSED: 'adult_batch_paused',
  ADULT_BATCH_RESUMED: 'adult_batch_resumed',
  ADULT_BATCH_CANCELLED: 'adult_batch_cancelled',
  // 文件夹操作事件
  FOLDER_RENAMED: 'folder_renamed',
  FOLDER_DELETED: 'folder_deleted',
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
  phase: 'scanning' | 'scraping' | 'merging' | 'matching' | 'cleaning' | 'completed'
  step_current: number
  step_total: number
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

export interface LibraryChangedData {
  library_id: string
  library_name: string
  action: string
  message: string
}

export interface WSMessage {
  type: WSEventType
  data: ScanProgressData | ScrapeProgressData | TranscodeProgressData
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

  // 注册事件监听
  const on = useCallback((event: WSEventType, handler: WSEventHandler) => {
    if (!listenersRef.current.has(event)) {
      listenersRef.current.set(event, new Set())
    }
    listenersRef.current.get(event)!.add(handler)
  }, [])

  // 取消事件监听
  const off = useCallback((event: WSEventType, handler: WSEventHandler) => {
    listenersRef.current.get(event)?.delete(handler)
  }, [])

  // 分发事件
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

  // 建立WebSocket连接
  const connect = useCallback(() => {
    if (!token || !isAuthenticated) return
    if (wsRef.current?.readyState === WebSocket.OPEN) return

    // 构建WebSocket URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/ws?token=${encodeURIComponent(token)}`

    try {
      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        console.log('[WS] 已连接')
        setConnected(true)
        retriesRef.current = 0
      }

      ws.onmessage = (event) => {
        try {
          // 支持批量消息（用换行分隔）
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
        console.log('[WS] 连接关闭:', event.code, event.reason)
        setConnected(false)
        wsRef.current = null

        // 自动重连
        if (autoReconnect && retriesRef.current < maxRetries && isAuthenticated) {
          retriesRef.current++
          console.log(`[WS] ${reconnectInterval}ms 后重连 (${retriesRef.current}/${maxRetries})`)
          reconnectTimerRef.current = setTimeout(connect, reconnectInterval)
        }
      }

      ws.onerror = (error) => {
        console.error('[WS] 连接错误:', error)
      }
    } catch (e) {
      console.error('[WS] 创建连接失败:', e)
    }
  }, [token, isAuthenticated, autoReconnect, reconnectInterval, maxRetries, dispatchEvent])

  // 连接管理
  useEffect(() => {
    if (isAuthenticated && token) {
      connect()
    }

    return () => {
      // 清理
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
      }
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
    }
  }, [isAuthenticated, token, connect])

  return { connected, on, off, lastMessage }
}
