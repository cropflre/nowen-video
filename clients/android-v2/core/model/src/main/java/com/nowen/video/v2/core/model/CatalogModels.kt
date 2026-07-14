package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class PaginatedEnvelope<T>(
    val data: List<T> = emptyList(),
    val total: Int = 0,
    val page: Int = 1,
    val size: Int = 20,
)

@Serializable
data class MediaDetail(
    val id: String,
    val title: String = "",
    @SerialName("orig_title") val originalTitle: String = "",
    val year: Int = 0,
    val overview: String = "",
    @SerialName("poster_path") val posterPath: String = "",
    @SerialName("backdrop_path") val backdropPath: String = "",
    val rating: Double = 0.0,
    val runtime: Int = 0,
    val genres: String = "",
    @SerialName("media_type") val mediaType: String = "movie",
    val duration: Double = 0.0,
    val resolution: String = "",
    @SerialName("video_codec") val videoCodec: String = "",
    @SerialName("audio_codec") val audioCodec: String = "",
    @SerialName("series_id") val seriesId: String = "",
    @SerialName("season_num") val seasonNumber: Int = 0,
    @SerialName("episode_num") val episodeNumber: Int = 0,
    @SerialName("episode_title") val episodeTitle: String = "",
) {
    val displayTitle: String
        get() = if (mediaType == "episode" && episodeNumber > 0) {
            buildString {
                append(title)
                append(" S")
                append(seasonNumber.toString().padStart(2, '0'))
                append("E")
                append(episodeNumber.toString().padStart(2, '0'))
                if (episodeTitle.isNotBlank()) append(" · $episodeTitle")
            }
        } else title
}

@Serializable
data class StreamInfo(
    @SerialName("media_id") val mediaId: String = "",
    val title: String = "",
    val duration: Double = 0.0,
    @SerialName("mime_type") val mimeType: String = "",
    @SerialName("can_direct_play") val canDirectPlay: Boolean = false,
    @SerialName("can_remux") val canRemux: Boolean = false,
    @SerialName("is_preprocessed") val isPreprocessed: Boolean = false,
    @SerialName("direct_play_url") val directPlayUrl: String = "",
    @SerialName("remux_url") val remuxUrl: String = "",
    @SerialName("hls_url") val hlsUrl: String = "",
    @SerialName("preprocessed_url") val preprocessedUrl: String = "",
) {
    val preferredUrl: String
        get() = when {
            isPreprocessed && preprocessedUrl.isNotBlank() -> preprocessedUrl
            canDirectPlay && directPlayUrl.isNotBlank() -> directPlayUrl
            canRemux && remuxUrl.isNotBlank() -> remuxUrl
            hlsUrl.isNotBlank() -> hlsUrl
            directPlayUrl.isNotBlank() -> directPlayUrl
            else -> ""
        }
}

@Serializable
data class ProgressUpdate(
    val position: Double,
    val duration: Double,
)

@Serializable
data class WatchProgress(
    @SerialName("media_id") val mediaId: String = "",
    val position: Double = 0.0,
    val duration: Double = 0.0,
    val completed: Boolean = false,
)