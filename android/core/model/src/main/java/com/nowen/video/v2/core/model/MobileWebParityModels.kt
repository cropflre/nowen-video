package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/** Web 移动端详情页“精彩片段”接口模型。 */
@Serializable
data class MediaHighlight(
    val id: String = "",
    @SerialName("media_id") val mediaId: String = "",
    val title: String = "",
    @SerialName("start_time") val startTime: Double = 0.0,
    @SerialName("end_time") val endTime: Double = 0.0,
    val score: Double = 0.0,
    val tags: String = "",
    val source: String = "",
    @SerialName("analysis_method") val analysisMethod: String = "",
    @SerialName("thumbnail_url") val thumbnailUrl: String? = null,
    @SerialName("preview_url") val previewUrl: String? = null,
    val version: Int = 0,
) {
    val durationSeconds: Int
        get() = (endTime - startTime).toInt().coerceAtLeast(1)
}

@Serializable
data class MediaHighlightList(
    val highlights: List<MediaHighlight> = emptyList(),
    val stale: Boolean = false,
)

/** 推荐接口保留 Web 的 score/reason，同时复用安卓已有 MediaCard 解包器。 */
@Serializable
data class RecommendedMediaCard(
    val media: MediaCard = MediaCard(),
    val score: Double = 0.0,
    val reason: String = "",
)

/**
 * Android 首页直接复刻 Web 移动端信息架构：Hero、续播、推荐、最近添加与分类货架。
 * 每个分区允许独立失败，避免某一个接口异常导致整个首页空白。
 */
data class MobileWebHomeContent(
    val recommendations: List<MediaCard> = emptyList(),
    val continueWatching: List<MediaCard> = emptyList(),
    val recent: List<MediaCard> = emptyList(),
    val genreShelves: Map<String, List<MediaCard>> = emptyMap(),
    val unavailableSections: Set<String> = emptySet(),
) {
    val heroItems: List<MediaCard>
        get() = (recommendations.ifEmpty { recent }).distinctBy(MediaCard::resolvedId).take(5)

    val isEmpty: Boolean
        get() = recommendations.isEmpty() && continueWatching.isEmpty() && recent.isEmpty()
}
