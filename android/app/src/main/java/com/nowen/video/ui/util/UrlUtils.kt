package com.nowen.video.ui.util

import java.net.URLEncoder

/**
 * 构建 poster URL
 *
 * @param serverUrl 服务器地址
 * @param id 媒体/剧集 ID
 * @param type 类型：media, series, collection, library
 * @param token 认证 token（可选）
 * @return 完整的 poster URL
 */
fun buildPosterUrl(serverUrl: String, id: String, type: String = "media", token: String = ""): String {
    val base = when (type) {
        "series" -> "$serverUrl/api/series/$id/poster"
        "collection" -> "$serverUrl/api/collections/$id/poster"
        "library" -> "$serverUrl/api/libraries/$id/poster"
        else -> "$serverUrl/api/media/$id/poster"
    }

    return if (token.isNotBlank()) {
        "$base?token=${URLEncoder.encode(token, "UTF-8")}"
    } else {
        base
    }
}

/**
 * 构建缩略图 URL
 *
 * @param serverUrl 服务器地址
 * @param id 媒体 ID
 * @param token 认证 token（可选）
 * @return 完整的缩略图 URL
 */
fun buildThumbnailUrl(serverUrl: String, id: String, token: String = ""): String {
    val base = "$serverUrl/api/media/$id/thumbnail"
    return if (token.isNotBlank()) {
        "$base?token=${URLEncoder.encode(token, "UTF-8")}"
    } else {
        base
    }
}

/**
 * 构建 backdrop URL
 *
 * @param serverUrl 服务器地址
 * @param id 媒体 ID
 * @param token 认证 token（可选）
 * @return 完整的 backdrop URL
 */
fun buildBackdropUrl(serverUrl: String, id: String, token: String = ""): String {
    val base = "$serverUrl/api/media/$id/backdrop"
    return if (token.isNotBlank()) {
        "$base?token=${URLEncoder.encode(token, "UTF-8")}"
    } else {
        base
    }
}
