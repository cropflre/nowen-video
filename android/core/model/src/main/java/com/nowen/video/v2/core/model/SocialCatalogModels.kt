package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class FavoriteRecord(
    val id: String = "",
    @SerialName("user_id") val userId: String = "",
    @SerialName("media_id") val mediaId: String = "",
    @SerialName("created_at") val createdAt: String = "",
    val media: MediaCard = MediaCard(),
)

@Serializable
data class WatchHistoryRecord(
    val id: String = "",
    @SerialName("user_id") val userId: String = "",
    @SerialName("media_id") val mediaId: String = "",
    val position: Double = 0.0,
    val duration: Double = 0.0,
    val completed: Boolean = false,
    @SerialName("updated_at") val updatedAt: String = "",
    @SerialName("created_at") val createdAt: String = "",
    val media: MediaCard = MediaCard(),
) {
    val normalizedProgress: Float
        get() = if (duration <= 0.0) 0f else (position / duration).toFloat().coerceIn(0f, 1f)

    val progressLabel: String
        get() = when {
            completed -> "已看完"
            duration <= 0.0 -> "观看记录"
            else -> "已观看 ${(normalizedProgress * 100).toInt()}%"
        }
}

@Serializable
data class MovieCollection(
    val id: String = "",
    val name: String = "",
    val overview: String = "",
    @SerialName("poster_path") val posterPath: String = "",
    @SerialName("tmdb_coll_id") val tmdbCollectionId: Int = 0,
    @SerialName("media_count") val mediaCount: Int = 0,
    @SerialName("file_count") val fileCount: Int = 0,
    @SerialName("auto_matched") val autoMatched: Boolean = true,
    @SerialName("year_range") val yearRange: String = "",
)

@Serializable
data class CollectionMediaItem(
    val id: String = "",
    val title: String = "",
    @SerialName("orig_title") val originalTitle: String = "",
    val year: Int = 0,
    val premiered: String = "",
    val rating: Double = 0.0,
    @SerialName("poster_path") val posterPath: String = "",
    val runtime: Int = 0,
    val overview: String = "",
    val genres: String = "",
    @SerialName("is_current") val isCurrent: Boolean = false,
    val resolution: String = "",
    @SerialName("video_codec") val videoCodec: String = "",
)

@Serializable
data class CollectionWithMedia(
    val collection: MovieCollection = MovieCollection(),
    val media: List<CollectionMediaItem> = emptyList(),
)

@Serializable
data class Person(
    val id: String = "",
    val name: String = "",
    @SerialName("orig_name") val originalName: String = "",
    @SerialName("profile_url") val profileUrl: String = "",
    @SerialName("tmdb_id") val tmdbId: Int = 0,
)

@Serializable
data class MediaPerson(
    val id: String = "",
    @SerialName("media_id") val mediaId: String = "",
    @SerialName("series_id") val seriesId: String = "",
    @SerialName("person_id") val personId: String = "",
    val role: String = "",
    val character: String = "",
    @SerialName("sort_order") val sortOrder: Int = 0,
    val person: Person = Person(),
) {
    val roleLabel: String
        get() = when (role.lowercase()) {
            "director" -> "导演"
            "writer" -> "编剧"
            "actor" -> character.takeIf(String::isNotBlank)?.let { "饰 $it" } ?: "演员"
            else -> character.ifBlank { role.ifBlank { "演职人员" } }
        }
}

@Serializable
data class PersonMediaResponse(
    val media: List<MediaCard> = emptyList(),
    val series: List<MediaCard> = emptyList(),
)
