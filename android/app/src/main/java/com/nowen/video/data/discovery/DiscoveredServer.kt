package com.nowen.video.data.discovery

/**
 * 局域网发现的服务器信息
 */
data class DiscoveredServer(
    /** 服务器显示名称 */
    val name: String,
    /** 服务器 IP 地址 */
    val host: String,
    /** 服务器端口 */
    val port: Int,
    /** 服务器版本号 */
    val version: String = "",
    /** 发现来源 */
    val source: DiscoverySource = DiscoverySource.HTTP_SWEEP,
    /** 完整 URL */
    val url: String = "http://$host:$port"
) {
    /** 唯一标识（用于去重） */
    val uniqueKey: String get() = "$host:$port"
}

/**
 * 发现来源枚举
 */
enum class DiscoverySource {
    /** mDNS/DNS-SD 发现 */
    MDNS,
    /** HTTP 端口扫描发现 */
    HTTP_SWEEP
}
