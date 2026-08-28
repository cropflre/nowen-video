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
    @SerialName("logo_path") val logoPath: String = "",
    @SerialName("tvdb_id") val tvdbId: Int = 0,
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
            rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
            year.takeIf { it > 0 }?.toString(),
            seasonCount.takeIf { it > 0 }?.let { "共 $it 季" },
            episodeCount.takeIf { it > 0 }?.let { "$it 集" },
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
        get() = seasonDisplayName(seasonNumber)

    fun normalized(): SeasonInfo = copy(
        episodes = episodes.sortedWith(
            compareBy<MediaDetail> { it.episodeNumber }
                .thenBy { it.displayTitle },
        ),
    )
}

fun seasonDisplayName(seasonNumber: Int): String {
    if (seasonNumber == 0) return "特别篇"
    if (seasonNumber in 1..10) {
        return "第${listOf("", "一", "二", "三", "四", "五", "六", "七", "八", "九", "十")[seasonNumber]}季"
    }
    return "第${seasonNumber}季"
}

fun episodeDisplayName(seasonNumber: Int, episodeNumber: Int): String =
    if (seasonNumber == 0) "特别篇 ${episodeNumber.coerceAtLeast(0)}"
    else "第 ${episodeNumber.coerceAtLeast(0)} 集"

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

val MediaDetail.userEpisodeTitle: String
    get() {
        val rawTitle = episodeTitle.trim()
        if (rawTitle.isBlank()) return seriesEpisodeLabel
        val marker = Regex("\\bS\\d{1,2}\\s*[:.]?\\s*E\\d{1,2}\\b", RegexOption.IGNORE_CASE).find(rawTitle)
            ?: return rawTitle
        val remainder = rawTitle.removeRange(marker.range).trim().trimStart('-', '·', ':', ' ')
        return remainder.ifBlank { seriesEpisodeLabel }
    }

val MediaDetail.seriesEpisodeSubtitle: String
    get() = listOfNotNull(
        userEpisodeTitle.takeIf { it != seriesEpisodeLabel },
        duration.takeIf { it > 0 }?.let { "${(it / 60).toInt().coerceAtLeast(1)} 分钟" },
        resolution.takeIf(String::isNotBlank),
    ).joinToString(" · ")
