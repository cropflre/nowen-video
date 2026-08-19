package com.nowen.video.v2.core.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

@Serializable
data class MediaCommentUser(
    val id: String = "",
    val username: String = "",
    val role: String = "user",
    val avatar: String = "",
    val nickname: String = "",
)

@Serializable
data class MediaComment(
    val id: String = "",
    @SerialName("user_id") val userId: String = "",
    @SerialName("media_id") val mediaId: String = "",
    val content: String = "",
    val rating: Double = 0.0,
    val username: String = "",
    val nickname: String = "",
    @SerialName("created_at") val createdAt: String = "",
    val user: MediaCommentUser? = null,
) {
    val displayName: String
        get() = user?.nickname?.takeIf(String::isNotBlank)
            ?: user?.username?.takeIf(String::isNotBlank)
            ?: nickname.takeIf(String::isNotBlank)
            ?: username.takeIf(String::isNotBlank)
            ?: "用户"
}

@Serializable
data class MediaCommentList(
    val data: List<MediaComment> = emptyList(),
    val total: Int = 0,
    val page: Int = 1,
    val size: Int = 10,
    @SerialName("avg_rating") val averageRating: Double = 0.0,
    @SerialName("rating_count") val ratingCount: Int = 0,
)

@Serializable
data class CreateMediaCommentRequest(
    val content: String,
    val rating: Int? = null,
)
