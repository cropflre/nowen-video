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

    suspend fun getRecentMedia(limit: Int = 20): Result<List<Media>> = runCatching {
        api.getRecentMedia(limit).data
    }

    suspend fun getRecentMixed(limit: Int = 20): Result<List<Media>> = runCatching {
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
        api.getFavorites().data
    }

    suspend fun addFavorite(mediaId: String): Result<Unit> = runCatching {
        api.addFavorite(mediaId)
    }

    suspend fun removeFavorite(mediaId: String): Result<Unit> = runCatching {
        api.removeFavorite(mediaId)
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

    suspend fun getSubtitleTracks(mediaId: String): Result<List<SubtitleTrack>> = runCatching {
        val resp = api.getSubtitleTracks(mediaId).data
        resp.embedded + resp.external
    }

    // ==================== 合集 ====================

    suspend fun getCollections(): Result<List<MovieCollection>> = runCatching {
        api.getCollections().data
    }

    suspend fun getCollectionDetail(id: String): Result<MovieCollection> = runCatching {
        api.getCollectionDetail(id).data
    }

    suspend fun getMediaCollection(mediaId: String): Result<MovieCollection?> = runCatching {
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
