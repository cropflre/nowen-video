package com.nowen.video.data.repository

import com.nowen.video.data.model.*
import com.nowen.video.data.remote.NowenApiService
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 媒体内容仓储 — 处理电影、剧集、搜索等
 * 负责从 ApiResponse 包装中提取实际数据
 */
@Singleton
class MediaRepository @Inject constructor(
    private val api: NowenApiService
) {
    // ==================== 媒体库 ====================

    suspend fun getLibraries(): Result<List<Library>> = runCatching {
        api.getLibraries().data
    }

    // ==================== 媒体列表 ====================

    suspend fun getMediaList(
        libraryId: String? = null,
        type: String? = null,
        page: Int = 1,
        limit: Int = 20
    ): Result<PaginatedResponse<Media>> = runCatching {
        val resp = api.getMediaList(libraryId, type, page, limit)
        PaginatedResponse(data = resp.data, total = resp.total, page = resp.page, size = resp.size)
    }

    suspend fun getMediaMixed(
        libraryId: String? = null,
        page: Int = 1,
        limit: Int = 20
    ): Result<PaginatedResponse<MixedItem>> = runCatching {
        val resp = api.getMediaMixed(libraryId, page, limit)
        PaginatedResponse(data = resp.data, total = resp.total, page = resp.page, size = resp.size)
    }

    suspend fun getRecentMedia(limit: Int = 20): Result<List<Media>> = runCatching {
        api.getRecentMedia(limit).data
    }

    suspend fun getRecentMixed(limit: Int = 20): Result<List<MixedItem>> = runCatching {
        api.getRecentMixed(limit).data
    }

    suspend fun getContinueWatching(): Result<List<WatchHistory>> = runCatching {
        api.getContinueWatching().data
    }

    // ==================== 媒体详情 ====================

    suspend fun getMediaDetail(id: String): Result<Media> = runCatching {
        api.getMediaDetail(id).data
    }

    suspend fun getMediaDetailEnhanced(id: String): Result<Media> = runCatching {
        api.getMediaDetailEnhanced(id).data
    }

    // ==================== 剧集 ====================

    suspend fun getSeriesList(libraryId: String? = null): Result<List<Series>> = runCatching {
        api.getSeriesList(libraryId).data
    }

    suspend fun getSeriesDetail(id: String): Result<Series> = runCatching {
        api.getSeriesDetail(id).data
    }

    suspend fun getSeasons(seriesId: String): Result<List<Season>> = runCatching {
        api.getSeasons(seriesId).data
    }

    suspend fun getSeasonEpisodes(seriesId: String, season: Int): Result<List<Media>> = runCatching {
        api.getSeasonEpisodes(seriesId, season).data
    }

    suspend fun getNextEpisode(seriesId: String): Result<Media?> = runCatching {
        api.getNextEpisode(seriesId).data
    }

    // ==================== 搜索 ====================

    suspend fun search(
        query: String,
        type: String? = null,
        page: Int = 1,
        limit: Int = 20
    ): Result<PaginatedResponse<Media>> = runCatching {
        val resp = api.search(query, type, page, limit)
        PaginatedResponse(data = resp.data, total = resp.total, page = resp.page, size = resp.size)
    }

    suspend fun searchMixed(query: String): Result<SearchResult> = runCatching {
        api.searchMixed(query)
    }

    // ==================== 流媒体 ====================

    suspend fun getStreamInfo(mediaId: String): Result<StreamInfo> = runCatching {
        api.getStreamInfo(mediaId).data
    }

    // ==================== 用户操作 ====================

    suspend fun updateProgress(mediaId: String, position: Double, duration: Double, completed: Boolean = false): Result<Unit> = runCatching {
        api.updateProgress(mediaId, ProgressUpdate(position, duration, completed))
    }

    suspend fun getProgress(mediaId: String): Result<WatchHistory?> = runCatching {
        api.getProgress(mediaId).data
    }

    suspend fun getHistory(): Result<List<WatchHistory>> = runCatching {
        api.getHistory().data
    }

    suspend fun getFavorites(): Result<List<Media>> = runCatching {
        api.getFavorites().data.map { it.media }
    }

    suspend fun addFavorite(mediaId: String): Result<Unit> = runCatching {
        val response = api.addFavorite(mediaId)
        if (!response.isSuccessful && response.code() != 409) {
            throw retrofit2.HttpException(response)
        }
        // 201 Created 或 409 Conflict（已收藏）都视为成功
    }

    suspend fun removeFavorite(mediaId: String): Result<Unit> = runCatching {
        val response = api.removeFavorite(mediaId)
        if (!response.isSuccessful) {
            throw retrofit2.HttpException(response)
        }
    }

    suspend fun checkFavorite(mediaId: String): Result<Boolean> = runCatching {
        api.checkFavorite(mediaId).data
    }

    // ==================== 观看历史管理 ====================

    suspend fun deleteHistory(mediaId: String): Result<Unit> = runCatching {
        api.deleteHistory(mediaId)
    }

    suspend fun clearHistory(): Result<Unit> = runCatching {
        api.clearHistory()
    }

    // ==================== 字幕 ====================

    suspend fun getSubtitleTracks(mediaId: String): Result<SubtitleTracksResponse> = runCatching {
        api.getSubtitleTracks(mediaId).data
    }

    // AI 字幕生成
    suspend fun generateAISubtitle(mediaId: String, language: String = ""): Result<ASRTask> = runCatching {
        api.generateAISubtitle(mediaId, mapOf("language" to language)).data
    }

    suspend fun getAISubtitleStatus(mediaId: String): Result<ASRTask> = runCatching {
        api.getAISubtitleStatus(mediaId).data
    }

    // 字幕翻译
    suspend fun translateSubtitle(mediaId: String, targetLang: String): Result<ASRTask> = runCatching {
        api.translateSubtitle(mediaId, mapOf("target_lang" to targetLang)).data
    }

    suspend fun getTranslatedSubtitles(mediaId: String): Result<List<TranslatedSubtitle>> = runCatching {
        api.getTranslateStatus(mediaId).data
    }

    // 字幕在线搜索
    suspend fun searchSubtitles(
        mediaId: String,
        language: String? = null,
        title: String? = null,
        year: Int? = null,
        type: String? = null
    ): Result<List<SubtitleSearchResult>> = runCatching {
        api.searchSubtitles(mediaId, language, title, year, type).data
    }

    suspend fun downloadSubtitle(mediaId: String, fileId: String): Result<SubtitleDownloadResult> = runCatching {
        api.downloadSubtitle(mediaId, mapOf("file_id" to fileId)).data
    }

    // ==================== 合集 ====================

    suspend fun getCollections(): Result<List<MovieCollection>> = runCatching {
        api.getCollections().data
    }

    suspend fun getCollectionDetail(id: String): Result<MovieCollection> = runCatching {
        api.getCollectionDetail(id).data
    }

    suspend fun getMediaCollection(mediaId: String): Result<CollectionWithMedia?> = runCatching {
        api.getMediaCollection(mediaId).data
    }

    // ==================== 书签 ====================

    suspend fun createBookmark(mediaId: String, position: Double, title: String = "", note: String = ""): Result<Bookmark> = runCatching {
        api.createBookmark(CreateBookmarkRequest(mediaId, position, title, note)).data
    }

    suspend fun getBookmarks(): Result<List<Bookmark>> = runCatching {
        api.getBookmarks().data
    }

    suspend fun getMediaBookmarks(mediaId: String): Result<List<Bookmark>> = runCatching {
        api.getMediaBookmarks(mediaId).data
    }

    suspend fun deleteBookmark(id: String): Result<Unit> = runCatching {
        api.deleteBookmark(id)
    }

    // ==================== 推荐 ====================

    suspend fun getRecommendations(): Result<List<Media>> = runCatching {
        api.getRecommendations().data
    }

    suspend fun getSimilarMedia(mediaId: String): Result<List<Media>> = runCatching {
        api.getSimilarMedia(mediaId).data
    }
}
