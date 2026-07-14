package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

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
