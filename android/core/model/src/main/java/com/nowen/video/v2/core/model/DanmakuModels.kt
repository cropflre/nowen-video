package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class DanmakuAnime(
    @SerialName("animeId") val animeId: Long = 0L,
    @SerialName("bangumiId") val bangumiId: String = "",
    @SerialName("animeTitle") val animeTitle: String = "",
    val type: String = "",
    val typeDescription: String = "",
    @SerialName("imageUrl") val imageUrl: String = "",
    @SerialName("startDate") val startDate: String = "",
    @SerialName("episodeCount") val episodeCount: Int = 0,
    val rating: Double = 0.0,
    val source: String = "",
)

@Serializable
data class DanmakuMatch(
    @SerialName("episodeId") val episodeId: Long = 0L,
    @SerialName("animeId") val animeId: Long = 0L,
    @SerialName("animeTitle") val animeTitle: String = "",
    @SerialName("episodeTitle") val episodeTitle: String = "",
    val type: String = "",
    val typeDescription: String = "",
    val shift: Double = 0.0,
    @SerialName("imageUrl") val imageUrl: String = "",
    val url: String = "",
)

@Serializable
data class DanmakuSearchResponse(
    val errorCode: Int = 0,
    val success: Boolean = true,
    val errorMessage: String = "",
    val animes: List<DanmakuAnime> = emptyList(),
)

@Serializable
data class DanmakuMatchResponse(
    val errorCode: Int = 0,
    val success: Boolean = true,
    val errorMessage: String = "",
    val isMatched: Boolean = false,
    val matches: List<DanmakuMatch> = emptyList(),
)

@Serializable
data class DanmakuEpisode(
    @SerialName("episodeId") val episodeId: Long = 0L,
    @SerialName("episodeTitle") val episodeTitle: String = "",
    val url: String = "",
)

@Serializable
data class DanmakuEpisodesGroup(
    @SerialName("animeId") val animeId: Long = 0L,
    @SerialName("animeTitle") val animeTitle: String = "",
    val type: String = "",
    val typeDescription: String = "",
    val episodes: List<DanmakuEpisode> = emptyList(),
)

@Serializable
data class DanmakuEpisodesResponse(
    val errorCode: Int = 0,
    val success: Boolean = true,
    val errorMessage: String = "",
    val hasMore: Boolean = false,
    val animes: List<DanmakuEpisodesGroup> = emptyList(),
)

@Serializable
data class DanmakuComment(
    val p: String = "",
    val m: String = "",
    val cid: Long = 0L,
    val t: String = "",
    val content: String = "",
)

@Serializable
data class DanmakuCommentResponse(
    val errorCode: Int = 0,
    val success: Boolean = true,
    val errorMessage: String = "",
    val count: Int = 0,
    val comments: List<DanmakuComment> = emptyList(),
    val videoDuration: Double = 0.0,
)

data class DanmakuCue(
    val id: String,
    val timeMs: Long,
    val text: String,
    val mode: Int = 1,
    val color: Int = 0xFFFFFFFF.toInt(),
    val source: String = "",
    val shiftMs: Long = 0L,
) {
    val effectiveTimeMs: Long get() = (timeMs + shiftMs).coerceAtLeast(0L)
}

fun parseDanmakuTimeMs(value: String): Long? {
    val seconds = value.substringBefore(',').trim().toDoubleOrNull() ?: return null
    if (!seconds.isFinite() || seconds < 0.0) return null
    return (seconds * 1000.0).toLong().coerceAtLeast(0L)
}

fun DanmakuComment.toCue(index: Int, source: String = "", shiftMs: Long = 0L): DanmakuCue? {
    val text = m.trim().ifBlank { content.trim() }
    if (text.isBlank()) return null
    val timeMs = parseDanmakuTimeMs(p.ifBlank { t }) ?: return null
    return DanmakuCue(
        id = if (cid > 0L) cid.toString() else "$timeMs:$index:$text",
        timeMs = timeMs,
        text = text.take(300),
        source = source,
        shiftMs = shiftMs,
    )
}
