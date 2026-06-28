package com.nowen.video.ui.util

/**
 * 构建 poster URL
 *
 * @param serverUrl 服务器地址
 * @param id 媒体/剧集 ID
 * @param type 类型：media, series, collection, library
 * @return 完整的 poster URL
 */
fun buildPosterUrl(serverUrl: String, id: String, type: String = "media"): String {
    return when (type) {
        "series" -> "$serverUrl/api/series/$id/poster"
        "collection" -> "$serverUrl/api/collections/$id/poster"
        "library" -> "$serverUrl/api/libraries/$id/poster"
        else -> "$serverUrl/api/media/$id/poster"
    }
}

/**
 * 构建缩略图 URL
 *
 * @param serverUrl 服务器地址
 * @param id 媒体 ID
 * @return 完整的缩略图 URL
 */
fun buildThumbnailUrl(serverUrl: String, id: String): String {
    return "$serverUrl/api/media/$id/thumbnail"
}

/**
 * 构建 backdrop URL
 *
 * @param serverUrl 服务器地址
 * @param id 媒体 ID
 * @return 完整的 backdrop URL
 */
fun buildBackdropUrl(serverUrl: String, id: String): String {
    return "$serverUrl/api/media/$id/backdrop"
}
