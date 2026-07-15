package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class SeriesInfo(
    val id: String,
    @SerialName("library_id") val libraryId: String = "",
    val title: String = "",
    @SerialName("orig_title") val originalTitle: String = "",
    val year: Int = 0,
    val overview: String = "",
    @SerialName("poster_path") val posterPath: String = "",
    @SerialName("backdrop_path") val backdropPath: String = "",
    val rating: Double = 0.0,
    val genres: String = "",
    @SerialName("season_count") val seasonCount: Int = 0,
    @SerialName("episode_count") val episodeCount: Int = 0,
    val country: String = "",
    val language: String = "",
    val studio: String = "",
    val episodes: List<MediaDetail> = emptyList(),
) {
    val displayTitle: String
        get() = title.ifBlank { originalTitle.ifBlank { "未命名剧集" } }

    val metadataLabel: String
        get() = listOfNotNull(
            year.takeIf { it > 0 }?.toString(),
            seasonCount.takeIf { it > 0 }?.let { "$it 季" },
            episodeCount.takeIf { it > 0 }?.let { "$it 集" },
            rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
        ).joinToString(" · ")

    val genreList: List<String>
        get() = genres
            .split(',', '，', '/', '|')
            .map(String::trim)
            .filter(String::isNotBlank)
            .distinct()
}

@Serializable
data class SeasonInfo(
    @SerialName("season_num") val seasonNumber: Int = 0,
    @SerialName("episode_count") val episodeCount: Int = 0,
    val episodes: List<MediaDetail> = emptyList(),
) {
    val label: String
        get() = when (seasonNumber) {
            0 -> "特别篇"
            else -> "第 $seasonNumber 季"
        }

    fun normalized(): SeasonInfo = copy(
        episodes = episodes.sortedWith(
            compareBy<MediaDetail> { it.episodeNumber }
                .thenBy { it.displayTitle },
        ),
    )
}

data class SeriesBundle(
    val series: SeriesInfo,
    val seasons: List<SeasonInfo>,
    val persons: List<MediaPerson> = emptyList(),
) {
    val firstEpisode: MediaDetail?
        get() = seasons
            .asSequence()
            .sortedWith(compareBy<SeasonInfo> { it.seasonNumber == 0 }.thenBy { it.seasonNumber })
            .flatMap { it.episodes.asSequence() }
            .firstOrNull()
}

val MediaDetail.seriesEpisodeLabel: String
    get() = when {
        seasonNumber == 0 && episodeNumber > 0 -> "特别篇 $episodeNumber"
        episodeNumber > 0 -> "第 $episodeNumber 集"
        else -> displayTitle
    }

val MediaDetail.seriesEpisodeSubtitle: String
    get() = listOfNotNull(
        episodeTitle.takeIf(String::isNotBlank),
        duration.takeIf { it > 0 }?.let { "${(it / 60).toInt().coerceAtLeast(1)} 分钟" },
        resolution.takeIf(String::isNotBlank),
    ).joinToString(" · ")
