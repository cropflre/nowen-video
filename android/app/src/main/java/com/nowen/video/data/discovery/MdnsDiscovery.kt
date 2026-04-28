package com.nowen.video.data.discovery

import android.content.Context
import android.net.nsd.NsdManager
import android.net.nsd.NsdServiceInfo
import android.util.Log
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow

/**
 * mDNS/DNS-SD 服务发现
 * 使用 Android NsdManager 搜索 _nowen-video._tcp 类型的服务
 */
class MdnsDiscovery(private val context: Context) {

    companion object {
        private const val TAG = "MdnsDiscovery"
        private const val SERVICE_TYPE = "_nowen-video._tcp."
    }

    /**
     * 开始 mDNS 服务发现，返回发现结果的 Flow。
     * Flow 会在发现新服务时发射 DiscoveredServer。
     * 取消 Flow 收集时自动停止发现。
     */
    fun discover(): Flow<DiscoveredServer> = callbackFlow {
        val nsdManager = context.getSystemService(Context.NSD_SERVICE) as NsdManager

        val resolveListener = object : NsdManager.ResolveListener {
            override fun onResolveFailed(serviceInfo: NsdServiceInfo, errorCode: Int) {
                Log.w(TAG, "mDNS 解析失败: ${serviceInfo.serviceName}, errorCode=$errorCode")
            }

            override fun onServiceResolved(serviceInfo: NsdServiceInfo) {
                val host = serviceInfo.host?.hostAddress ?: return
                val port = serviceInfo.port
                val name = serviceInfo.serviceName

                // 从 TXT 记录中提取版本号和服务器名称
                var version = ""
                var serverName = name
                try {
                    val attributes = serviceInfo.attributes
                    attributes["version"]?.let { version = String(it) }
                    attributes["server_name"]?.let { serverName = String(it) }
                } catch (e: Exception) {
                    Log.d(TAG, "读取 TXT 记录失败: ${e.message}")
                }

                val server = DiscoveredServer(
                    name = serverName,
                    host = host,
                    port = port,
                    version = version,
                    source = DiscoverySource.MDNS,
                    url = "http://$host:$port"
                )

                Log.i(TAG, "mDNS 发现服务器: $server")
                trySend(server)
            }
        }

        val discoveryListener = object : NsdManager.DiscoveryListener {
            override fun onDiscoveryStarted(serviceType: String) {
                Log.i(TAG, "mDNS 发现已启动: $serviceType")
            }

            override fun onDiscoveryStopped(serviceType: String) {
                Log.i(TAG, "mDNS 发现已停止: $serviceType")
            }

            override fun onServiceFound(serviceInfo: NsdServiceInfo) {
                Log.d(TAG, "mDNS 发现服务: ${serviceInfo.serviceName}")
                // 解析服务以获取 IP 和端口
                try {
                    nsdManager.resolveService(serviceInfo, resolveListener)
                } catch (e: Exception) {
                    Log.w(TAG, "解析服务失败: ${e.message}")
                }
            }

            override fun onServiceLost(serviceInfo: NsdServiceInfo) {
                Log.d(TAG, "mDNS 服务丢失: ${serviceInfo.serviceName}")
            }

            override fun onStartDiscoveryFailed(serviceType: String, errorCode: Int) {
                Log.e(TAG, "mDNS 启动发现失败: errorCode=$errorCode")
                channel.close()
            }

            override fun onStopDiscoveryFailed(serviceType: String, errorCode: Int) {
                Log.e(TAG, "mDNS 停止发现失败: errorCode=$errorCode")
            }
        }

        try {
            nsdManager.discoverServices(SERVICE_TYPE, NsdManager.PROTOCOL_DNS_SD, discoveryListener)
        } catch (e: Exception) {
            Log.e(TAG, "启动 mDNS 发现异常: ${e.message}")
            channel.close()
        }

        awaitClose {
            try {
                nsdManager.stopServiceDiscovery(discoveryListener)
            } catch (e: Exception) {
                Log.d(TAG, "停止 mDNS 发现异常（可忽略）: ${e.message}")
            }
        }
    }
}
