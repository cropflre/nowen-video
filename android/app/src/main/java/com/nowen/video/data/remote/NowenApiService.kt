package com.nowen.video.data.remote

import com.nowen.video.data.model.*
import retrofit2.Response
import retrofit2.http.*

/**
 * Nowen Video 后端 API 接口定义
 * 对应后端 main.go 中注册的所有路由
 *
 * 注意：后端几乎所有接口都使用 gin.H{"data": ...} 包裹返回数据，
 * 因此大部分接口返回类型为 ApiResponse<T> 或 ApiPaginatedResponse<T>
 */
interface NowenApiService {

    // ==================== 认证（公开） ====================
    // 登录/注册直接返回 TokenResponse（不包裹在 data 中）

    @POST("auth/login")
    suspend fun login(@Body request: LoginRequest): TokenResponse

    @POST("auth/register")
    suspend fun register(@Body request: RegisterRequest): TokenResponse

    @GET("auth/status")
    suspend fun getInitStatus(): InitStatusWrapper

    // ==================== 认证（需登录） ====================

    @POST("auth/refresh")
    suspend fun refreshToken(): TokenResponse

    // ==================== 媒体库 ====================

    @GET("libraries")
    suspend fun getLibraries(): ApiResponse<List<Library>>

    // ==================== 媒体内容 ====================

    @GET("media")
    suspend fun getMediaList(
        @Query("library_id") libraryId: String? = null,
        @Query("type") type: String? = null,
        @Query("page") page: Int = 1,
        @Query("size") size: Int = 20
    ): ApiPaginatedResponse<Media>

    @GET("media/{id}")
    suspend fun getMediaDetail(@Path("id") id: String): ApiResponse<Media>

    @GET("media/{id}/enhanced")
    suspend fun getMediaDetailEnhanced(@Path("id") id: String): ApiResponse<Media>

    @GET("media/recent")
    suspend fun getRecentMedia(
        @Query("limit") limit: Int = 20
    ): ApiResponse<List<Media>>

    @GET("media/recent/mixed")
    suspend fun getRecentMixed(
        @Query("limit") limit: Int = 20
    ): ApiResponse<List<MixedItem>>

    @GET("media/mixed")
    suspend fun getMediaMixed(
        @Query("library_id") libraryId: String? = null,
        @Query("page") page: Int = 1,
        @Query("size") size: Int = 20
    ): ApiPaginatedResponse<MixedItem>

    @GET("media/continue")
    suspend fun getContinueWatching(): ApiResponse<List<WatchHistory>>

    // ==================== 剧集 ====================

    @GET("series")
    suspend fun getSeriesList(
        @Query("library_id") libraryId: String? = null
    ): ApiPaginatedResponse<Series>

    @GET("series/{id}")
    suspend fun getSeriesDetail(@Path("id") id: String): ApiResponse<Series>

    @GET("series/{id}/seasons")
    suspend fun getSeasons(@Path("id") id: String): ApiResponse<List<Season>>

    @GET("series/{id}/seasons/{season}")
    suspend fun getSeasonEpisodes(
        @Path("id") id: String,
        @Path("season") season: Int
    ): ApiResponse<List<Media>>

    @GET("series/{id}/next")
    suspend fun getNextEpisode(@Path("id") id: String): ApiResponse<Media?>

    // ==================== 流媒体 ====================

    @GET("stream/{id}/info")
    suspend fun getStreamInfo(@Path("id") id: String): ApiResponse<StreamInfo>

    // ==================== 搜索 ====================

    @GET("search")
    suspend fun search(
        @Query("q") query: String,
        @Query("type") type: String? = null,
        @Query("page") page: Int = 1,
        @Query("size") size: Int = 20
    ): ApiPaginatedResponse<Media>

    @GET("search/mixed")
    suspend fun searchMixed(@Query("q") query: String): SearchResult

    // ==================== 用户 ====================

    @GET("users/me")
    suspend fun getProfile(): ApiResponse<User>

    @PUT("users/me/progress/{mediaId}")
    suspend fun updateProgress(
        @Path("mediaId") mediaId: String,
        @Body progress: ProgressUpdate
    )

    @GET("users/me/progress/{mediaId}")
    suspend fun getProgress(@Path("mediaId") mediaId: String): ApiResponse<WatchHistory?>

    @GET("users/me/history")
    suspend fun getHistory(): ApiPaginatedResponse<WatchHistory>

    @GET("users/me/favorites")
    suspend fun getFavorites(): ApiPaginatedResponse<Favorite>

    @POST("users/me/favorites/{mediaId}")
    suspend fun addFavorite(@Path("mediaId") mediaId: String): Response<Unit>

    @DELETE("users/me/favorites/{mediaId}")
    suspend fun removeFavorite(@Path("mediaId") mediaId: String): Response<Unit>

    @GET("users/me/favorites/{mediaId}/check")
    suspend fun checkFavorite(@Path("mediaId") mediaId: String): ApiResponse<Boolean>

    // 观看历史管理
    @DELETE("users/me/history/{mediaId}")
    suspend fun deleteHistory(@Path("mediaId") mediaId: String)

    @DELETE("users/me/history")
    suspend fun clearHistory()

    // ==================== 字幕 ====================

    @GET("subtitle/{id}/tracks")
    suspend fun getSubtitleTracks(@Path("id") id: String): ApiResponse<SubtitleTracksResponse>

    @GET("subtitle/{id}/extract/{index}")
    suspend fun extractSubtitleTrack(
        @Path("id") id: String,
        @Path("index") index: Int
    ): okhttp3.ResponseBody

    // AI 字幕生成
    @POST("subtitle/{id}/ai/generate")
    suspend fun generateAISubtitle(
        @Path("id") id: String,
        @Body request: Map<String, String>
    ): ApiResponse<ASRTask>

    @GET("subtitle/{id}/ai/status")
    suspend fun getAISubtitleStatus(@Path("id") id: String): ApiResponse<ASRTask>

    // 字幕翻译
    @POST("subtitle/{id}/translate")
    suspend fun translateSubtitle(
        @Path("id") id: String,
        @Body request: Map<String, String>
    ): ApiResponse<ASRTask>

    @GET("subtitle/{id}/translate/status")
    suspend fun getTranslateStatus(@Path("id") id: String): ApiResponse<List<TranslatedSubtitle>>

    // 字幕在线搜索
    @GET("subtitle/{id}/search")
    suspend fun searchSubtitles(
        @Path("id") id: String,
        @Query("language") language: String? = null,
        @Query("title") title: String? = null,
        @Query("year") year: Int? = null,
        @Query("type") type: String? = null
    ): ApiResponse<List<SubtitleSearchResult>>

    @POST("subtitle/{id}/download")
    suspend fun downloadSubtitle(
        @Path("id") id: String,
        @Body request: Map<String, String>
    ): ApiResponse<SubtitleDownloadResult>

    // ASR 服务状态
    @GET("asr/status")
    suspend fun getASRStatus(): ApiResponse<Map<String, @kotlinx.serialization.Serializable Any>>

    // ==================== 合集 ====================

    @GET("collections")
    suspend fun getCollections(): ApiPaginatedResponse<MovieCollection>

    @GET("collections/{id}")
    suspend fun getCollectionDetail(@Path("id") id: String): ApiResponse<MovieCollection>

    @GET("collections/search")
    suspend fun searchCollections(@Query("keyword") keyword: String): ApiResponse<List<MovieCollection>>

    @GET("media/{id}/collection")
    suspend fun getMediaCollection(@Path("id") mediaId: String): ApiResponse<CollectionWithMedia?>

    // ==================== 书签 ====================

    @POST("bookmarks")
    suspend fun createBookmark(@Body request: CreateBookmarkRequest): ApiResponse<Bookmark>

    @GET("bookmarks")
    suspend fun getBookmarks(): ApiPaginatedResponse<Bookmark>

    @GET("bookmarks/media/{mediaId}")
    suspend fun getMediaBookmarks(@Path("mediaId") mediaId: String): ApiResponse<List<Bookmark>>

    @DELETE("bookmarks/{id}")
    suspend fun deleteBookmark(@Path("id") id: String)

    // ==================== 推荐 ====================

    @GET("recommend")
    suspend fun getRecommendations(): ApiResponse<List<Media>>

    @GET("recommend/similar/{mediaId}")
    suspend fun getSimilarMedia(@Path("mediaId") mediaId: String): ApiResponse<List<Media>>
}
