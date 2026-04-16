package com.nowen.video.data.remote

import com.nowen.video.data.local.TokenManager
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.*
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton

/**
 * WebSocket 事件数据模型
 */
@Serializable
data class WSEvent(
    val type: String,
    val data: kotlinx.serialization.json.JsonElement? = null,
    val timestamp: Long = 0
)

@Serializable
data class ScanProgressData(
    val library_id: String = "",
    val library_name: String = "",
    val phase: String = "",
    val current: Int = 0,
    val total: Int = 0,
    val new_found: Int = 0,
    val cleaned: Int = 0,
    val message: String = ""
)

@Serializable
data class ScrapeProgressData(
    val library_id: String = "",
    val library_name: String = "",
    val current: Int = 0,
    val total: Int = 0,
    val success: Int = 0,
    val failed: Int = 0,
    val media_title: String = "",
    val message: String = ""
)

@Serializable
data class TranscodeProgressData(
    val task_id: String = "",
    val media_id: String = "",
    val title: String = "",
    val quality: String = "",
    val progress: Double = 0.0,
    val speed: String = "",
    val message: String = ""
)

/**
 * WebSocket 连接状态
 */
enum class WSConnectionState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    RECONNECTING
}

/**
 * WebSocket 客户端管理器
 * 负责与后端 /api/ws 端点建立 WebSocket 连接，接收实时事件推送
 *
 * 支持功能：
 * - 自动重连（指数退避）
 * - 心跳保活
 * - 事件流分发
 */
@Singleton
class WebSocketManager @Inject constructor(
    private val tokenManager: TokenManager,
    private val json: Json
) {
    companion object {
        private const val NORMAL_CLOSURE = 1000
        private const val MAX_RECONNECT_DELAY = 30_000L // 最大重连间隔 30 秒
        private const val INITIAL_RECONNECT_DELAY = 1_000L // 初始重连间隔 1 秒
    }

    private var webSocket: WebSocket? = null
    private var client: OkHttpClient? = null
    private var reconnectJob: Job? = null
    private var reconnectAttempt = 0
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    // 连接状态
    private val _connectionState = MutableStateFlow(WSConnectionState.DISCONNECTED)
    val connectionState = _connectionState.asStateFlow()

    // 事件流 — 所有 WebSocket 事件都通过此 Flow 分发
    private val _events = MutableSharedFlow<WSEvent>(extraBufferCapacity = 64)
    val events = _events.asSharedFlow()

    // 特定类型的事件流（便于 UI 层订阅）
    private val _scanProgress = MutableSharedFlow<ScanProgressData>(extraBufferCapacity = 16)
    val scanProgress = _scanProgress.asSharedFlow()

    private val _scrapeProgress = MutableSharedFlow<ScrapeProgressData>(extraBufferCapacity = 16)
    val scrapeProgress = _scrapeProgress.asSharedFlow()

    private val _transcodeProgress = MutableSharedFlow<TranscodeProgressData>(extraBufferCapacity = 16)
    val transcodeProgress = _transcodeProgress.asSharedFlow()

    /**
     * 建立 WebSocket 连接
     */
    fun connect() {
        if (_connectionState.value == WSConnectionState.CONNECTED ||
            _connectionState.value == WSConnectionState.CONNECTING) {
            return
        }

        scope.launch {
            try {
                val serverUrl = tokenManager.getServerUrl() ?: return@launch
                val token = tokenManager.getToken() ?: return@launch

                // 清理 URL 中可能存在的非法字符（如不可见 Unicode 字符）
                val cleanUrl = serverUrl.replace(Regex("[^\\x20-\\x7E]"), "").trim()
                if (cleanUrl.isBlank()) return@launch

                // 将 http(s) 转换为 ws(s)
                val wsUrl = cleanUrl
                    .replace("https://", "wss://")
                    .replace("http://", "ws://")
                    .trimEnd('/') + "/api/ws?token=$token"

                _connectionState.value = WSConnectionState.CONNECTING

                client = OkHttpClient.Builder()
                    .readTimeout(0, TimeUnit.MILLISECONDS) // WebSocket 不设读超时
                    .pingInterval(30, TimeUnit.SECONDS)    // 30 秒心跳
                    .build()

                val request = Request.Builder()
                    .url(wsUrl)
                    .build()

                client?.newWebSocket(request, createWebSocketListener())
            } catch (e: Exception) {
                // URL 解析失败等异常，不应导致闪退
                _connectionState.value = WSConnectionState.DISCONNECTED
            }
        }
    }

    /**
     * 断开 WebSocket 连接
     */
    fun disconnect() {
        reconnectJob?.cancel()
        reconnectJob = null
        reconnectAttempt = 0
        webSocket?.close(NORMAL_CLOSURE, "用户主动断开")
        webSocket = null
        _connectionState.value = WSConnectionState.DISCONNECTED
    }

    /**
     * 重新连接（带指数退避）
     */
    private fun scheduleReconnect() {
        reconnectJob?.cancel()
        reconnectJob = scope.launch {
            _connectionState.value = WSConnectionState.RECONNECTING
            val delay = (INITIAL_RECONNECT_DELAY * (1 shl reconnectAttempt.coerceAtMost(5)))
                .coerceAtMost(MAX_RECONNECT_DELAY)
            reconnectAttempt++
            delay(delay)
            connect()
        }
    }

    private fun createWebSocketListener() = object : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            this@WebSocketManager.webSocket = webSocket
            _connectionState.value = WSConnectionState.CONNECTED
            reconnectAttempt = 0 // 重置重连计数
        }

        override fun onMessage(webSocket: WebSocket, text: String) {
            try {
                // 支持批量消息（以换行分隔）
                text.split("\n").forEach { line ->
                    if (line.isBlank()) return@forEach
                    val event = json.decodeFromString<WSEvent>(line)
                    scope.launch {
                        _events.emit(event)
                        dispatchEvent(event)
                    }
                }
            } catch (e: Exception) {
                // 解析失败，忽略
            }
        }

        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
            webSocket.close(NORMAL_CLOSURE, null)
        }

        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            this@WebSocketManager.webSocket = null
            _connectionState.value = WSConnectionState.DISCONNECTED
            if (code != NORMAL_CLOSURE) {
                scheduleReconnect()
            }
        }

        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
            this@WebSocketManager.webSocket = null
            _connectionState.value = WSConnectionState.DISCONNECTED
            scheduleReconnect()
        }
    }

    /**
     * 将事件分发到特定类型的 Flow
     */
    private suspend fun dispatchEvent(event: WSEvent) {
        val dataStr = event.data?.toString() ?: return

        try {
            when (event.type) {
                "scan_started", "scan_progress", "scan_completed", "scan_failed", "scan_phase" -> {
                    val data = json.decodeFromString<ScanProgressData>(dataStr)
                    _scanProgress.emit(data)
                }
                "scrape_started", "scrape_progress", "scrape_completed" -> {
                    val data = json.decodeFromString<ScrapeProgressData>(dataStr)
                    _scrapeProgress.emit(data)
                }
                "transcode_started", "transcode_progress", "transcode_completed", "transcode_failed" -> {
                    val data = json.decodeFromString<TranscodeProgressData>(dataStr)
                    _transcodeProgress.emit(data)
                }
            }
        } catch (_: Exception) {
            // 解析特定数据失败，忽略
        }
    }

    /**
     * 释放资源
     */
    fun destroy() {
        disconnect()
        scope.cancel()
        client?.dispatcher?.executorService?.shutdown()
    }
}
