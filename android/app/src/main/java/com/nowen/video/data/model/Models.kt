package com.nowen.video.data.model

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

// ==================== 通用 API 响应包装 ====================

/**
 * 后端统一响应格式：{"data": T, ...}
 * 几乎所有后端接口都使用 gin.H{"data": ...} 包裹返回数据
 */
@Serializable
data class ApiResponse<T>(
    val data: T
)

/**
 * 后端分页响应格式：{"data": [...], "total": N, "page": P, "size": S}
 * 注意：后端使用 "size" 字段名
 */
@Serializable
data class ApiPaginatedResponse<T>(
    val data: List<T>,
    val total: Int = 0,
    val page: Int = 1,
    val size: Int = 20
)

// ==================== 认证 ====================

@Serializable
data class LoginRequest(
    val username: String,
    val password: String
)

@Serializable
data class RegisterRequest(
    val username: String,
    val password: String,
    @SerialName("invite_code") val inviteCode: String = ""
)

@Serializable
data class TokenResponse(
    val token: String,
    @SerialName("expires_at") val expiresAt: Long,
    val user: User
)

@Serializable
data class InitStatus(
    val initialized: Boolean,
    @SerialName("registration_open") val registrationOpen: Boolean
)

@Serializable
data class InitStatusWrapper(
    val data: InitStatus
)

// ==================== 用户 ====================

@Serializable
data class User(
    val id: String,
    val username: String,
    val role: String = "user",
    val avatar: String = "",
    @SerialName("created_at") val createdAt: String = ""
)

// ==================== 媒体库 ====================

@Serializable
data class Library(
    val id: String,
    val name: String,
    val path: String = "",
    val type: String = "movie",
    @SerialName("last_scan") val lastScan: String? = null,
    @SerialName("created_at") val createdAt: String = ""
)

// ==================== 媒体内容 ====================

@Serializable
data class Media(
    val id: String,
    @SerialName("library_id") val libraryId: String = "",
    val title: String,
    @SerialName("orig_title") val origTitle: String = "",
    val year: Int = 0,
    val overview: String = "",
    @SerialName("poster_path") val posterPath: String = "",
    @SerialName("backdrop_path") val backdropPath: String = "",
    val rating: Double = 0.0,
    val runtime: Int = 0,
    val genres: String = "",
    @SerialName("file_path") val filePath: String = "",
    @SerialName("file_size") val fileSize: Long = 0,
    @SerialName("media_type") val mediaType: String = "movie",
    @SerialName("video_codec") val videoCodec: String = "",
    @SerialName("audio_codec") val audioCodec: String = "",
    val resolution: String = "",
    val duration: Double = 0.0,
    @SerialName("tmdb_id") val tmdbId: Int = 0,
    @SerialName("imdb_id") val imdbId: String = "",
    val country: String = "",
    val language: String = "",
    val tagline: String = "",
    val studio: String = "",
    @SerialName("trailer_url") val trailerUrl: String = "",
    // 剧集字段
    @SerialName("series_id") val seriesId: String = "",
    @SerialName("season_num") val seasonNum: Int = 0,
    @SerialName("episode_num") val episodeNum: Int = 0,
    @SerialName("episode_title") val episodeTitle: String = "",
    // 合集
    @SerialName("collection_id") val collectionId: String = "",
    @SerialName("created_at") val createdAt: String = ""
)

// ==================== 剧集 ====================

@Serializable
data class Series(
    val id: String,
    @SerialName("library_id") val libraryId: String = "",
    val title: String,
    @SerialName("orig_title") val origTitle: String = "",
    val year: Int = 0,
    val overview: String = "",
    @SerialName("poster_path") val posterPath: String = "",
    @SerialName("backdrop_path") val backdropPath: String = "",
    val rating: Double = 0.0,
    val genres: String = "",
    @SerialName("season_count") val seasonCount: Int = 0,
    @SerialName("episode_count") val episodeCount: Int = 0,
    @SerialName("tmdb_id") val tmdbId: Int = 0,
    val country: String = "",
    val studio: String = "",
    val episodes: List<Media>? = null,
    @SerialName("created_at") val createdAt: String = ""
)

@Serializable
data class Season(
    @SerialName("season_num") val seasonNum: Int,
    @SerialName("episode_count") val episodeCount: Int,
    val episodes: List<Media>? = null
)

// ==================== 流媒体 ====================

@Serializable
data class StreamInfo(
    @SerialName("media_id") val mediaId: String = "",
    val title: String = "",
    val duration: Double = 0.0,
    @SerialName("file_size") val fileSize: Long = 0,
    @SerialName("video_codec") val videoCodec: String = "",
    @SerialName("audio_codec") val audioCodec: String = "",
    val resolution: String = "",
    @SerialName("can_direct_play") val canDirectPlay: Boolean = false,
    @SerialName("can_remux") val canRemux: Boolean = false,
    val preprocessed: Boolean = false,
    @SerialName("mime_type") val mimeType: String = "",
    @SerialName("direct_url") val directUrl: String = "",
    @SerialName("remux_url") val remuxUrl: String = "",
    @SerialName("hls_url") val hlsUrl: String = "",
    @SerialName("preprocess_url") val preprocessUrl: String = ""
)

// ==================== 搜索 ====================

@Serializable
data class SearchResult(
    val media: List<Media> = emptyList(),
    val series: List<Series> = emptyList(),
    @SerialName("media_total") val mediaTotal: Int = 0,
    @SerialName("series_total") val seriesTotal: Int = 0
)

// ==================== 观看历史 ====================

@Serializable
data class WatchHistory(
    val id: String,
    @SerialName("user_id") val userId: String = "",
    @SerialName("media_id") val mediaId: String = "",
    val position: Double = 0.0,
    val duration: Double = 0.0,
    val completed: Boolean = false,
    @SerialName("updated_at") val updatedAt: String = "",
    val media: Media? = null
)

@Serializable
data class ProgressUpdate(
    val position: Double,
    val duration: Double,
    val completed: Boolean = false
)

// ==================== 字幕 ====================

@Serializable
data class SubtitleTrack(
    val index: Int,
    val language: String = "",
    val title: String = "",
    val codec: String = "",
    val forced: Boolean = false,
    @SerialName("is_default") val isDefault: Boolean = false,
    @SerialName("is_external") val isExternal: Boolean = false,
    @SerialName("file_path") val filePath: String = ""
)

/**
 * 字幕轨道响应（后端返回 {"data": {"embedded": [...], "external": [...]}}）
 */
@Serializable
data class SubtitleTracksResponse(
    val embedded: List<SubtitleTrack> = emptyList(),
    val external: List<SubtitleTrack> = emptyList()
)

// ==================== 合集 ====================

@Serializable
data class MovieCollection(
    val id: String,
    val name: String,
    val overview: String = "",
    @SerialName("poster_path") val posterPath: String = "",
    @SerialName("backdrop_path") val backdropPath: String = "",
    @SerialName("media_count") val mediaCount: Int = 0,
    val media: List<Media>? = null,
    @SerialName("tmdb_id") val tmdbId: Int = 0,
    @SerialName("created_at") val createdAt: String = ""
)

// ==================== 书签 ====================

@Serializable
data class Bookmark(
    val id: String,
    @SerialName("user_id") val userId: String = "",
    @SerialName("media_id") val mediaId: String = "",
    val position: Double = 0.0,
    val title: String = "",
    val note: String = "",
    @SerialName("created_at") val createdAt: String = ""
)

@Serializable
data class CreateBookmarkRequest(
    @SerialName("media_id") val mediaId: String,
    val position: Double,
    val title: String = "",
    val note: String = ""
)

// ==================== 推荐 ====================

@Serializable
data class RecommendItem(
    val media: Media,
    val reason: String = ""
)

// ==================== 分页 ====================

@Serializable
data class PaginatedResponse<T>(
    val data: List<T>,
    val total: Int = 0,
    val page: Int = 1,
    val size: Int = 20
)
