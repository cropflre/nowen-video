package com.nowen.video.v2.core.data

import com.nowen.video.v2.core.model.InitStatusEnvelope
import java.io.IOException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import okhttp3.OkHttpClient
import okhttp3.Request

private const val HEALTH_PATH = "api/health"
private const val AUTH_STATUS_PATH = "api/auth/status"

internal suspend fun probeServer(
    baseUrl: String,
    client: OkHttpClient,
    json: Json,
): Result<ServerProbe> = withContext(Dispatchers.IO) {
    runCatching {
        val candidate = baseUrl.trim()
        require(candidate.isNotBlank() && candidate.lowercase() !in setOf("http://", "https://")) {
            "服务器地址无效"
        }
        val normalized = requireNotNull(UrlNormalizer.normalize(candidate)) { "服务器地址无效" }
        val directClient = client.newBuilder().apply {
            interceptors().clear()
            networkInterceptors().clear()
        }.build()
        val failures = mutableListOf<String>()

        for (path in listOf(HEALTH_PATH, AUTH_STATUS_PATH)) {
            val url = requireNotNull(UrlNormalizer.apiUrl(normalized, path)) { "服务器地址无效" }
            val response = try {
                directClient.newCall(
                    Request.Builder()
                        .url(url)
                        .header("Accept", "application/json")
                        .build(),
                ).execute()
            } catch (error: IOException) {
                failures += "$path：${error.message ?: "网络不可用"}"
                continue
            }

            response.use {
                if (!it.isSuccessful) {
                    failures += "$path：HTTP ${it.code}"
                    return@use
                }
                val body = it.body?.string().orEmpty()
                val parsed = runCatching {
                    when (path) {
                        HEALTH_PATH -> parseHealthProbe(body, json)
                        else -> parseAuthStatusProbe(body, json)
                    }
                }.getOrElse { error ->
                    failures += "$path：响应格式无效（${error.message ?: error::class.java.simpleName}）"
                    null
                }
                if (parsed != null) return@runCatching parsed
            }
        }

        error(
            if (failures.isEmpty()) {
                "连接服务器失败"
            } else {
                "连接服务器失败：${failures.joinToString("；")}"
            },
        )
    }
}

private fun parseHealthProbe(body: String, json: Json): ServerProbe {
    if (body.isBlank()) return ServerProbe()
    val root = json.parseToJsonElement(body).jsonObject
    val data = root["data"] as? JsonObject
    return ServerProbe(
        serverName = root.text("server_name")
            ?: data?.text("server_name")
            ?: "Nowen Video",
        version = root.text("version")
            ?: data?.text("version")
            ?: "",
    )
}

private fun parseAuthStatusProbe(body: String, json: Json): ServerProbe {
    val status = json.decodeFromString<InitStatusEnvelope>(body).data
    return ServerProbe(
        serverName = status.serverName.ifBlank { "Nowen Video" },
        version = status.version,
    )
}

private fun JsonObject.text(key: String): String? =
    this[key]?.jsonPrimitive?.contentOrNull?.trim()?.takeIf(String::isNotBlank)
