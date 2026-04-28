package com.nowen.video.data.discovery

import android.content.Context
import android.util.Log
import kotlinx.coroutines.*
import kotlinx.coroutines.flow.*
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 服务器发现统一管理器
 * 同时启动 mDNS 和 HTTP 扫描两种发现方式，合并去重结果
 */
@Singleton
class ServerDiscoveryManager @Inject constructor(
    private val context: Context
) {
    companion object {
        private const val TAG = "ServerDiscovery"

        /** 发现超时时间（秒） */
        private const val DISCOVERY_TIMEOUT_SECONDS = 15L
    }

    private val mdnsDiscovery = MdnsDiscovery(context)
    private val httpSweepDiscovery = HttpSweepDiscovery(context)

    /** 当前发现状态 */
    private val _discoveryState = MutableStateFlow(DiscoveryState())
    val discoveryState: StateFlow<DiscoveryState> = _discoveryState.asStateFlow()

    /** 当前发现任务 */
    private var discoveryJob: Job? = null

    /**
     * 开始设备发现（mDNS + HTTP 扫描并行）
     * 调用前会自动取消上一次未完成的发现
     */
    fun startDiscovery(scope: CoroutineScope) {
        // 取消上一次发现
        stopDiscovery()

        _discoveryState.value = DiscoveryState(isScanning = true, servers = emptyList(), error = null)

        discoveryJob = scope.launch {
            val discoveredServers = mutableMapOf<String, DiscoveredServer>()

            try {
                // 使用 withTimeout 限制总发现时间
                withTimeout(DISCOVERY_TIMEOUT_SECONDS * 1000) {
                    // 并行启动两种发现方式
                    val mdnsFlow = mdnsDiscovery.discover()
                        .catch { e ->
                            Log.w(TAG, "mDNS 发现异常（已降级到 HTTP 扫描）: ${e.message}")
                        }

                    val httpFlow = httpSweepDiscovery.discover()
                        .catch { e ->
                            Log.w(TAG, "HTTP 扫描异常: ${e.message}")
                        }

                    // 合并两个 Flow
                    merge(mdnsFlow, httpFlow).collect { server ->
                        val key = server.uniqueKey
                        val existing = discoveredServers[key]

                        if (existing == null) {
                            // 新发现的服务器
                            discoveredServers[key] = server
                        } else if (server.source == DiscoverySource.MDNS && existing.source == DiscoverySource.HTTP_SWEEP) {
                            // mDNS 结果优先级更高（信息更丰富）
                            discoveredServers[key] = server
                        }

                        // 更新状态
                        _discoveryState.value = _discoveryState.value.copy(
                            servers = discoveredServers.values.toList().sortedBy { it.host }
                        )
                    }
                }
            } catch (e: TimeoutCancellationException) {
                Log.i(TAG, "发现超时，已收集 ${discoveredServers.size} 个服务器")
            } catch (e: CancellationException) {
                Log.i(TAG, "发现已取消")
                throw e
            } catch (e: Exception) {
                Log.e(TAG, "发现异常: ${e.message}")
                _discoveryState.value = _discoveryState.value.copy(
                    error = "扫描失败: ${e.message}"
                )
            } finally {
                _discoveryState.value = _discoveryState.value.copy(isScanning = false)
            }
        }
    }

    /**
     * 停止设备发现
     */
    fun stopDiscovery() {
        discoveryJob?.cancel()
        discoveryJob = null
    }
}

/**
 * 发现状态
 */
data class DiscoveryState(
    /** 是否正在扫描 */
    val isScanning: Boolean = false,
    /** 已发现的服务器列表 */
    val servers: List<DiscoveredServer> = emptyList(),
    /** 错误信息 */
    val error: String? = null
)
