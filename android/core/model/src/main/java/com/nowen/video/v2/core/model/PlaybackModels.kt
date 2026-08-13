package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class PlaybackClientCapabilities(
    @SerialName("user_agent") val userAgent: String = "",
    @SerialName("supports_direct_play") val supportsDirectPlay: Boolean = true,
    @SerialName("supports_remux") val supportsRemux: Boolean = true,
    @SerialName("supports_hevc") val supportsHevc: Boolean = false,
    @SerialName("force_transcode") val forceTranscode: Boolean = false,
    @SerialName("max_bitrate") val maxBitrate: Int = 0,
)

@Serializable
data class PlaybackStartupStream(
    @SerialName("profile_id") val profileId: String = "",
    @SerialName("duration_ms") val durationMs: Long = 0,
    @SerialName("playlist_url") val playlistUrl: String = "",
    @SerialName("continuation_mode") val continuationMode: String = "",
    @SerialName("discontinuity_at_handoff") val discontinuityAtHandoff: Boolean = false,
    @SerialName("encoding_plan_version") val encodingPlanVersion: String = "",
    @SerialName("encoding_plan_hash") val encodingPlanHash: String = "",
)

@Serializable
data class PlaybackSessionTemplate(
    @SerialName("create_url") val createUrl: String = "/api/playback/sessions",
    @SerialName("profile_id") val profileId: String = "auto",
    @SerialName("max_bitrate") val maxBitrate: Int = 0,
)

@Serializable
data class PlaybackPlan(
    @SerialName("media_id") val mediaId: String = "",
    val method: String = "",
    val url: String = "",
    @SerialName("reason_code") val reasonCode: String = "",
    val reason: String = "",
    @SerialName("requires_transcode") val requiresTranscode: Boolean = false,
    @SerialName("session_required") val sessionRequired: Boolean = false,
    @SerialName("session_template") val sessionTemplate: PlaybackSessionTemplate? = null,
    @SerialName("fallback_method") val fallbackMethod: String = "",
    @SerialName("fallback_url") val fallbackUrl: String = "",
    @SerialName("client_capabilities") val clientCapabilities: PlaybackClientCapabilities = PlaybackClientCapabilities(),
    @SerialName("startup_stream") val startupStream: PlaybackStartupStream? = null,
) {
    val methodLabel: String
        get() = when (method.lowercase()) {
            "direct" -> "直接播放"
            "remux" -> "无损封装转换"
            "smart_remux" -> "视频直通·音频兼容转换"
            "startup_stream" -> "启动流秒开"
            "transcode" -> "兼容转码"
            else -> "自动选择"
        }

    val requiresEphemeralSession: Boolean
        get() = method.equals("transcode", ignoreCase = true) && sessionRequired
}

@Serializable
data class PlaybackGenerationSnapshot(
    val id: Long = 0,
    @SerialName("session_id") val sessionId: String = "",
    val state: String = "",
    @SerialName("profile_id") val profileId: String = "",
    @SerialName("start_position_ms") val startPositionMs: Long = 0,
    @SerialName("audio_track") val audioTrack: Int = 0,
    @SerialName("subtitle_track") val subtitleTrack: Int = -1,
    @SerialName("burn_subtitle") val burnSubtitle: Boolean = false,
    @SerialName("max_bitrate") val maxBitrate: Int = 0,
    val reason: String = "",
    val backend: String = "",
    @SerialName("transcoded_ms") val transcodedMs: Long = 0,
    @SerialName("ahead_ms") val aheadMs: Long = 0,
    val suspended: Boolean = false,
    val speed: String = "",
    @SerialName("error_code") val errorCode: String = "",
    @SerialName("error_message") val errorMessage: String = "",
)

@Serializable
data class PlaybackSessionSnapshot(
    val id: String = "",
    @SerialName("user_id") val userId: String = "",
    @SerialName("media_id") val mediaId: String = "",
    val state: String = "",
    val paused: Boolean = false,
    @SerialName("position_ms") val positionMs: Long = 0,
    @SerialName("buffered_end_ms") val bufferedEndMs: Long = 0,
    @SerialName("current_generation_id") val currentGenerationId: Long = 0,
    @SerialName("pending_generation_id") val pendingGenerationId: Long = 0,
    @SerialName("close_reason") val closeReason: String = "",
    val generation: PlaybackGenerationSnapshot? = null,
)

@Serializable
data class PlaybackSessionResult(
    val session: PlaybackSessionSnapshot = PlaybackSessionSnapshot(),
    @SerialName("playlist_url") val playlistUrl: String = "",
    @SerialName("status_url") val statusUrl: String = "",
    @SerialName("heartbeat_interval_sec") val heartbeatIntervalSec: Int = 15,
    @SerialName("first_segment_ready") val firstSegmentReady: Boolean = false,
    @SerialName("startup_ms") val startupMs: Long = 0,
) {
    val failureMessage: String
        get() = session.generation?.errorMessage.orEmpty()

    val isTerminalFailure: Boolean
        get() = session.state == "failed" || session.state == "closed" || session.state == "expired"
}

@Serializable
data class CreatePlaybackSessionRequest(
    @SerialName("media_id") val mediaId: String,
    @SerialName("profile_id") val profileId: String = "auto",
    @SerialName("start_position_ms") val startPositionMs: Long = 0,
    @SerialName("audio_track") val audioTrack: Int = 0,
    @SerialName("subtitle_track") val subtitleTrack: Int = -1,
    @SerialName("burn_subtitle") val burnSubtitle: Boolean = false,
    @SerialName("max_bitrate") val maxBitrate: Int = 0,
)

@Serializable
data class RestartPlaybackSessionRequest(
    @SerialName("profile_id") val profileId: String = "auto",
    @SerialName("start_position_ms") val startPositionMs: Long,
    @SerialName("audio_track") val audioTrack: Int = 0,
    @SerialName("subtitle_track") val subtitleTrack: Int = -1,
    @SerialName("burn_subtitle") val burnSubtitle: Boolean = false,
    @SerialName("max_bitrate") val maxBitrate: Int = 0,
    val reason: String = "seek",
)

@Serializable
data class PlaybackSessionHeartbeatRequest(
    @SerialName("generation_id") val generationId: Long,
    @SerialName("position_ms") val positionMs: Long,
    @SerialName("buffered_end_ms") val bufferedEndMs: Long,
    val paused: Boolean,
)

@Serializable
data class SubtitleTrack(
    val index: Int = -1,
    val language: String = "",
    val title: String = "",
    val codec: String = "",
    val forced: Boolean = false,
    val bitmap: Boolean = false,
    @SerialName("default") val isDefault: Boolean = false,
    val filename: String = "",
    val format: String = "",
    val path: String = "",
) {
    val isExternal: Boolean get() = path.isNotBlank() || filename.isNotBlank()
    val sourcePath: String get() = path.ifBlank { filename }
    val displayLabel: String
        get() = title.ifBlank {
            language.ifBlank {
                filename.ifBlank { if (index >= 0) "字幕 ${index + 1}" else "外挂字幕" }
            }
        }
}

@Serializable
data class SubtitleTracksResponse(
    val embedded: List<SubtitleTrack> = emptyList(),
    val external: List<SubtitleTrack> = emptyList(),
)
