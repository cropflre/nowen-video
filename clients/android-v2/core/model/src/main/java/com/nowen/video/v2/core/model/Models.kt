package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

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

@Serializable data class LoginRequest(val username: String, val password: String)

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

@Serializable data class PasswordChangeResponse(val message: String = "", val data: TokenResponse? = null)
@Serializable data class ApiEnvelope<T>(val data: T)
@Serializable data class InitStatusEnvelope(val data: InitStatus)

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

@Serializable
data class MediaCard(
    val id: String = "",
    val title: String = "",
    val name: String = "",
    val type: String = "",
    val year: Int? = null,
    val poster: String? = null,
    @SerialName("poster_url") val posterUrl: String? = null,
    val progress: Double = 0.0,
    @SerialName("progress_percent") val progressPercent: Double = 0.0,
    @SerialName("media_id") val mediaId: String? = null,
) {
    val displayTitle: String get() = title.ifBlank { name.ifBlank { "未命名媒体" } }
    val resolvedId: String get() = mediaId?.takeIf { it.isNotBlank() } ?: id
    val resolvedPoster: String? get() = posterUrl ?: poster
    val normalizedProgress: Float get() = when {
        progressPercent > 1 -> (progressPercent / 100).toFloat()
        progress > 1 -> (progress / 100).toFloat()
        progressPercent > 0 -> progressPercent.toFloat()
        else -> progress.toFloat()
    }.coerceIn(0f, 1f)
}

data class HomeContent(
    val libraries: List<LibrarySummary> = emptyList(),
    val continueWatching: List<MediaCard> = emptyList(),
    val recent: List<MediaCard> = emptyList(),
)
