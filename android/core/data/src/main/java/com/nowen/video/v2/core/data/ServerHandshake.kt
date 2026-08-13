package com.nowen.video.v2.core.data

import com.nowen.video.v2.core.model.InitStatusEnvelope
import java.io.IOException
import kotlinx.serialization.json.Json
import okhttp3.Request
import okhttp3.Response

internal const val SERVER_HANDSHAKE_PATH = "api/auth/status"

internal data class ServerHandshake(
    val serverName: String,
    val version: String,
)

internal fun buildServerHandshakeRequest(baseUrl: String): Request {
    val url = UrlNormalizer.apiUrl(baseUrl, SERVER_HANDSHAKE_PATH)
        ?: throw IllegalArgumentException("服务器地址无效，请输入 HTTP、HTTPS、域名或 IP 地址")
    return Request.Builder()
        .url(url)
        .header("Accept", "application/json")
        .build()
}

internal fun decodeServerHandshake(payload: String, json: Json): ServerHandshake {
    if (payload.isBlank()) {
        throw IOException("服务器响应为空，无法确认这是 Nowen Video 服务")
    }
    val status = runCatching {
        json.decodeFromString<InitStatusEnvelope>(payload).data
    }.getOrElse { error ->
        throw IOException("服务器响应格式不兼容，请确认该地址指向 Nowen Video 服务", error)
    }
    return ServerHandshake(
        serverName = status.serverName.ifBlank { "Nowen Video" },
        version = status.version,
    )
}

internal fun Response.readServerHandshake(json: Json): ServerHandshake {
    if (!isSuccessful) {
        throw IOException("服务器返回 HTTP $code，请确认地址和端口是否正确")
    }
    return decodeServerHandshake(body?.string().orEmpty(), json)
}
