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
data class PlaybackPlan(
    @SerialName("media_id") val mediaId: String = "",
    val method: String = "",
    val url: String = "",
    @SerialName("reason_code") val reasonCode: String = "",
    val reason: String = "",
    @SerialName("requires_transcode") val requiresTranscode: Boolean = false,
    @SerialName("fallback_method") val fallbackMethod: String = "",
    @SerialName("fallback_url") val fallbackUrl: String = "",
    @SerialName("client_capabilities") val clientCapabilities: PlaybackClientCapabilities = PlaybackClientCapabilities(),
) {
    val methodLabel: String
        get() = when (method.lowercase()) {
            "direct" -> "直接播放"
            "remux" -> "无损封装转换"
            "smart_remux" -> "视频直通·音频兼容转换"
            "transcode" -> "兼容转码"
            else -> "自动选择"
        }
}

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
