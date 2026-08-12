package com.nowen.video.v2.core.model

import kotlinx.serialization.Serializable

@Serializable
enum class DiscoverySource {
    MDNS,
    HTTP_SWEEP,
    QR,
}

@Serializable
data class DiscoveredServer(
    val name: String,
    val host: String,
    val port: Int,
    val version: String = "",
    val source: DiscoverySource,
    val url: String,
) {
    val uniqueKey: String get() = url.trim().trimEnd('/').lowercase()
}

@Serializable
data class ServerQrPayload(
    val url: String,
    val name: String = "",
)
