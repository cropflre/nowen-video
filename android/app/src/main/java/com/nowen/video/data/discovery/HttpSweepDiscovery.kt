package com.nowen.video.data.discovery

import android.content.Context
import android.net.wifi.WifiManager
import android.util.Log
import kotlinx.coroutines.*
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.sync.Semaphore
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONObject
import java.net.Inet4Address
import java.net.NetworkInterface
import java.util.concurrent.TimeUnit

/**
 * HTTP 端口扫描发现
 * 遍历局域网 IP 段，通过调用 /api/auth/status 识别 NowenVideo 服务器
 */
class HttpSweepDiscovery(private val context: Context) {

    companion object {
        private const val TAG = "HttpSweepDiscovery"

        /** 扫描的目标端口列表 */
        private val SCAN_PORTS = listOf(8080, 80, 8443, 443, 3000, 9090)

        /** 最大并发连接数 */
        private const val MAX_CONCURRENT = 50

        /** 单次连接超时（毫秒） */
        private const val CONNECT_TIMEOUT_MS = 800L

        /** 单次读取超时（毫秒） */
        private const val READ_TIMEOUT_MS = 1500L
    }

    /** 专用 OkHttpClient：超短超时，不跟随重定向 */
    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(CONNECT_TIMEOUT_MS, TimeUnit.MILLISECONDS)
        .readTimeout(READ_TIMEOUT_MS, TimeUnit.MILLISECONDS)
        .writeTimeout(READ_TIMEOUT_MS, TimeUnit.MILLISECONDS)
        .followRedirects(false)
        .followSslRedirects(false)
        .retryOnConnectionFailure(false)
        .build()

    /**
     * 开始 HTTP 端口扫描，返回发现结果的 Flow。
     * 扫描完成后 Flow 自动关闭。
     */
    fun discover(): Flow<DiscoveredServer> = callbackFlow {
        val subnet = getLocalSubnet()
        if (subnet == null) {
            Log.w(TAG, "无法获取本机局域网地址，跳过 HTTP 扫描")
            channel.close()
            return@callbackFlow
        }

        Log.i(TAG, "开始 HTTP 扫描，子网: $subnet.0/24，端口: $SCAN_PORTS")

        val semaphore = Semaphore(MAX_CONCURRENT)
        val jobs = mutableListOf<Job>()

        // 在单独的协程中执行扫描，完成后关闭 channel
        val scanJob = launch(Dispatchers.IO) {
            // 扫描 1~254
            for (i in 1..254) {
                val ip = "$subnet.$i"
                for (port in SCAN_PORTS) {
                    val job = launch {
                        semaphore.acquire()
                        try {
                            val server = probeServer(ip, port)
                            if (server != null) {
                                trySend(server)
                            }
                        } finally {
                            semaphore.release()
                        }
                    }
                    jobs.add(job)
                }
            }

            // 等待所有扫描完成
            jobs.joinAll()
            Log.i(TAG, "HTTP 扫描完成")
            channel.close()
        }

        awaitClose {
            scanJob.cancel()
            jobs.forEach { it.cancel() }
        }
    }

    /**
     * 探测单个 IP:Port 是否为 NowenVideo 服务器
     */
    private fun probeServer(ip: String, port: Int): DiscoveredServer? {
        val url = "http://$ip:$port/api/auth/status"
        return try {
            val request = Request.Builder()
                .url(url)
                .get()
                .build()

            httpClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) return null

                val body = response.body?.string() ?: return null
                val json = JSONObject(body)

                // 验证响应格式：必须包含 data.initialized 字段（NowenVideo 特征）
                val data = json.optJSONObject("data") ?: return null
                if (!data.has("initialized")) return null

                // 提取服务器信息
                val serverName = data.optString("server_name", "NowenVideo")
                val version = data.optString("version", "")

                Log.i(TAG, "HTTP 发现服务器: $ip:$port ($serverName)")

                DiscoveredServer(
                    name = serverName,
                    host = ip,
                    port = port,
                    version = version,
                    source = DiscoverySource.HTTP_SWEEP,
                    url = "http://$ip:$port"
                )
            }
        } catch (e: Exception) {
            // 连接失败是正常的（大部分 IP 不是我们的服务器）
            null
        }
    }

    /**
     * 获取本机局域网 IP 的前三段（如 "192.168.1"）
     */
    private fun getLocalSubnet(): String? {
        // 方式1：通过 WifiManager 获取
        try {
            val wifiManager = context.applicationContext.getSystemService(Context.WIFI_SERVICE) as? WifiManager
            val dhcpInfo = wifiManager?.dhcpInfo
            if (dhcpInfo != null && dhcpInfo.ipAddress != 0) {
                val ip = dhcpInfo.ipAddress
                val ipStr = "${ip and 0xFF}.${ip shr 8 and 0xFF}.${ip shr 16 and 0xFF}"
                if (ipStr != "0.0.0") {
                    return ipStr
                }
            }
        } catch (e: Exception) {
            Log.d(TAG, "WifiManager 获取 IP 失败: ${e.message}")
        }

        // 方式2：通过 NetworkInterface 获取
        try {
            val interfaces = NetworkInterface.getNetworkInterfaces()
            while (interfaces.hasMoreElements()) {
                val iface = interfaces.nextElement()
                if (iface.isLoopback || !iface.isUp) continue

                val addrs = iface.inetAddresses
                while (addrs.hasMoreElements()) {
                    val addr = addrs.nextElement()
                    if (addr is Inet4Address && !addr.isLoopbackAddress) {
                        val parts = addr.hostAddress?.split(".") ?: continue
                        if (parts.size == 4) {
                            return "${parts[0]}.${parts[1]}.${parts[2]}"
                        }
                    }
                }
            }
        } catch (e: Exception) {
            Log.d(TAG, "NetworkInterface 获取 IP 失败: ${e.message}")
        }

        return null
    }
}
