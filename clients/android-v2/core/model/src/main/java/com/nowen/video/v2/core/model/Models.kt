package com.nowen.video.v2.core.model

import kotlinx.serialization.KSerializer
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.descriptors.SerialDescriptor
import kotlinx.serialization.encoding.Decoder
import kotlinx.serialization.encoding.Encoder
import kotlinx.serialization.json.JsonDecoder
import kotlinx.serialization.json.JsonEncoder
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.doubleOrNull
import kotlinx.serialization.json.jsonPrimitive

@Serializable
data class ServerProfile(
    val id: String,
    val name: String,
    val baseUrl: String,
    val lastConnectedAt: Long = 0,
    val serverVersion: String? = null,
    val allowCleartext: Boolean = false,
)

@Serializable
data class UserProfile(
    val id: String,
    val username: String,
    val nickname: String = "",
    val role: String = "user",
    @SerialName("must_change_pwd") val mustChangePassword: Boolean = false,
)

data class SessionSnapshot(
    val servers: List<ServerProfile> = emptyList(),
    val activeServerId: String? = null,
    val user: UserProfile? = null,
    val token: String? = null,
    val initialized: Boolean = false,
) {
    val activeServer: ServerProfile? get() = servers.firstOrNull { it.id == activeServerId }
    val isAuthenticated: Boolean get() = activeServer != null && user != null && !token.isNullOrBlank()
}

@Serializable
data class LoginRequest(val username: String, val password: String)

@Serializable
data class PasswordChangeRequest(
    @SerialName("old_password") val oldPassword: String,
    @SerialName("new_password") val newPassword: String,
)

@Serializable
data class TokenResponse(
    val token: String,
    @SerialName("expires_at") val expiresAt: Long = 0,
    val user: UserProfile,
    @SerialName("must_change_password") val mustChangePassword: Boolean = false,
)

@Serializable
data class PasswordChangeResponse(val message: String = "", val data: TokenResponse? = null)

@Serializable
data class ApiEnvelope<T>(val data: T)

@Serializable
data class InitStatusEnvelope(val data: InitStatus)

@Serializable
data class InitStatus(
    val initialized: Boolean = true,
    @SerialName("registration_open") val registrationOpen: Boolean = false,
    @SerialName("invite_required") val inviteRequired: Boolean = false,
    @SerialName("server_name") val serverName: String = "Nowen Video",
    val version: String = "",
)

@Serializable
data class LibrarySummary(
    val id: String,
    val name: String,
    val type: String = "",
    @SerialName("media_count") val mediaCount: Int = 0,
)

/**
 * 首页、搜索和续播共用的轻量媒体模型。
 *
 * 服务端目前存在三种合法响应形态：
 * 1. 普通媒体对象；
 * 2. `{ type, media|series }` 混合列表项；
 * 3. `{ position, duration, media }` 续播记录。
 *
 * 自定义序列化器统一解包，避免每个页面重复兼容历史 API 契约。
 */
@Serializable(with = MediaCardSerializer::class)
data class MediaCard(
    val id: String = "",
    val title: String = "",
    val name: String = "",
    val type: String = "",
    val year: Int? = null,
    val poster: String? = null,
    val posterUrl: String? = null,
    val posterPath: String? = null,
    val progress: Double = 0.0,
    val progressPercent: Double = 0.0,
    val mediaId: String? = null,
) {
    val displayTitle: String get() = title.ifBlank { name.ifBlank { "未命名媒体" } }
    val resolvedId: String get() = mediaId?.takeIf { it.isNotBlank() } ?: id
    val resolvedPoster: String? get() = posterUrl ?: posterPath ?: poster
    val normalizedProgress: Float get() = when {
        progressPercent > 1 -> (progressPercent / 100).toFloat()
        progress > 1 -> (progress / 100).toFloat()
        progressPercent > 0 -> progressPercent.toFloat()
        else -> progress.toFloat()
    }.coerceIn(0f, 1f)
}

@Serializable
private data class MediaCardPayload(
    val id: String = "",
    val title: String = "",
    val name: String = "",
    val type: String = "",
    @SerialName("media_type") val mediaType: String = "",
    val year: Int? = null,
    val poster: String? = null,
    @SerialName("poster_url") val posterUrl: String? = null,
    @SerialName("poster_path") val posterPath: String? = null,
    val progress: Double = 0.0,
    @SerialName("progress_percent") val progressPercent: Double = 0.0,
    @SerialName("media_id") val mediaId: String? = null,
)

object MediaCardSerializer : KSerializer<MediaCard> {
    override val descriptor: SerialDescriptor = MediaCardPayload.serializer().descriptor

    override fun deserialize(decoder: Decoder): MediaCard {
        val jsonDecoder = decoder as? JsonDecoder
            ?: error("MediaCard 仅支持 JSON 反序列化")
        val root = jsonDecoder.decodeJsonElement() as? JsonObject
            ?: return MediaCard()
        val nestedMedia = root["media"] as? JsonObject
        val nestedSeries = root["series"] as? JsonObject
        val payloadObject = nestedMedia ?: nestedSeries ?: root
        val payload = jsonDecoder.json.decodeFromJsonElement(MediaCardPayload.serializer(), payloadObject)

        val outerPosition = root.doubleValue("position")
        val outerDuration = root.doubleValue("duration")
        val historyProgress = if (outerDuration > 0) outerPosition / outerDuration else 0.0
        val outerType = root["type"]?.jsonPrimitive?.content.orEmpty()
        val outerMediaId = root["media_id"]?.jsonPrimitive?.content

        return MediaCard(
            id = payload.id,
            title = payload.title,
            name = payload.name,
            type = outerType.ifBlank { payload.type.ifBlank { payload.mediaType } },
            year = payload.year,
            poster = payload.poster,
            posterUrl = payload.posterUrl,
            posterPath = payload.posterPath,
            progress = historyProgress.takeIf { it > 0 } ?: payload.progress,
            progressPercent = payload.progressPercent,
            mediaId = payload.mediaId ?: outerMediaId,
        )
    }

    override fun serialize(encoder: Encoder, value: MediaCard) {
        val jsonEncoder = encoder as? JsonEncoder
            ?: error("MediaCard 仅支持 JSON 序列化")
        val payload = MediaCardPayload(
            id = value.id,
            title = value.title,
            name = value.name,
            type = value.type,
            year = value.year,
            poster = value.poster,
            posterUrl = value.posterUrl,
            posterPath = value.posterPath,
            progress = value.progress,
            progressPercent = value.progressPercent,
            mediaId = value.mediaId,
        )
        jsonEncoder.encodeJsonElement(
            jsonEncoder.json.encodeToJsonElement(MediaCardPayload.serializer(), payload),
        )
    }
}

private fun JsonObject.doubleValue(name: String): Double =
    this[name]?.jsonPrimitive?.doubleOrNull ?: 0.0

data class HomeContent(
    val libraries: List<LibrarySummary> = emptyList(),
    val continueWatching: List<MediaCard> = emptyList(),
    val recent: List<MediaCard> = emptyList(),
)
