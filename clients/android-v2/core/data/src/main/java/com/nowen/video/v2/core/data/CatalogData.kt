package com.nowen.video.v2.core.data

import androidx.paging.Pager
import androidx.paging.PagingConfig
import androidx.paging.PagingData
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.ApiEnvelope
import com.nowen.video.v2.core.model.CreatePlaybackSessionRequest
import com.nowen.video.v2.core.model.LibraryFilter
import com.nowen.video.v2.core.model.LibrarySummary
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.NullableMediaDetailEnvelope
import com.nowen.video.v2.core.model.PaginatedEnvelope
import com.nowen.video.v2.core.model.PlaybackSessionHeartbeatRequest
import com.nowen.video.v2.core.model.PlaybackSessionResult
import com.nowen.video.v2.core.model.ProgressUpdate
import com.nowen.video.v2.core.model.RestartPlaybackSessionRequest
import com.nowen.video.v2.core.model.StreamInfo
import com.nowen.video.v2.core.model.SubtitleTracksResponse
import com.nowen.video.v2.core.model.WatchProgress
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import java.io.IOException
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.withTimeout
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.HttpException
import retrofit2.Response
import retrofit2.Retrofit
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

private const val CATALOG_PLACEHOLDER = "placeholder.invalid"
internal const val LIBRARY_PAGE_SIZE = 36
private const val PLAYBACK_SESSION_READY_TIMEOUT_MS = 15_000L
private const val PLAYBACK_SESSION_POLL_INTERVAL_MS = 250L

interface CatalogApi {
    @GET("media/mixed")
    suspend fun media(
        @Query("page") page: Int,
        @Query("size") size: Int,
        @Query("library_id") libraryId: String? = null,
        @Query("type") contentType: String? = null,
        @Query("genre") genre: String? = null,
        @Query("q") query: String? = null,
        @Query("year_from") yearFrom: Int? = null,
        @Query("year_to") yearTo: Int? = null,
        @Query("sort") sort: String? = null,
        @Query("order") order: String? = null,
    ): PaginatedEnvelope<MediaCard>

    @GET("libraries")
    suspend fun libraries(): ApiEnvelope<List<LibrarySummary>>

    @GET("media/{id}")
    suspend fun detail(@Path("id") id: String): ApiEnvelope<MediaDetail>

    @GET("stream/{id}/info")
    suspend fun stream(
        @Path("id") id: String,
        @Query("supports_direct") supportsDirect: Boolean,
        @Query("supports_remux") supportsRemux: Boolean,
        @Query("supports_hevc") supportsHevc: Boolean,
    ): ApiEnvelope<StreamInfo>

    @POST("playback/sessions")
    suspend fun createPlaybackSession(
        @Body request: CreatePlaybackSessionRequest,
    ): ApiEnvelope<PlaybackSessionResult>

    @GET("playback/sessions/{sessionId}/status")
    suspend fun playbackSessionStatus(
        @Path("sessionId") sessionId: String,
    ): ApiEnvelope<PlaybackSessionResult>

    @POST("playback/sessions/{sessionId}/heartbeat")
    suspend fun heartbeatPlaybackSession(
        @Path("sessionId") sessionId: String,
        @Body request: PlaybackSessionHeartbeatRequest,
    ): ApiEnvelope<PlaybackSessionResult>

    @POST("playback/sessions/{sessionId}/restart")
    suspend fun restartPlaybackSession(
        @Path("sessionId") sessionId: String,
        @Body request: RestartPlaybackSessionRequest,
    ): ApiEnvelope<PlaybackSessionResult>

    @DELETE("playback/sessions/{sessionId}")
    suspend fun closePlaybackSession(
        @Path("sessionId") sessionId: String,
        @Query("reason") reason: String,
    ): Response<Unit>

    @GET("subtitle/{id}/tracks")
    suspend fun subtitles(@Path("id") id: String): ApiEnvelope<SubtitleTracksResponse>

    @GET("series/{id}/next")
    suspend fun nextEpisode(
        @Path("id") seriesId: String,
        @Query("season") season: Int,
        @Query("episode") episode: Int,
    ): NullableMediaDetailEnvelope

    @PUT("users/me/progress/{mediaId}")
    suspend fun updateProgress(
        @Path("mediaId") mediaId: String,
        @Body progress: ProgressUpdate,
    )

    @GET("users/me/progress/{mediaId}")
    suspend fun progress(@Path("mediaId") mediaId: String): ApiEnvelope<WatchProgress?>
}

@Module
@InstallIn(SingletonComponent::class)
object CatalogNetworkModule {
    @Provides
    @Singleton
    fun catalogApi(client: OkHttpClient, json: Json): CatalogApi =
        Retrofit.Builder()
            .baseUrl("https://$CATALOG_PLACEHOLDER/api/")
            .client(client)
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(CatalogApi::class.java)
}

@Singleton
class CatalogRepository @Inject constructor(
    private val api: CatalogApi,
    private val playbackCapabilities: PlaybackCapabilityProvider,
) {
    fun pagedMedia(filter: LibraryFilter): Flow<PagingData<MediaCard>> = Pager(
        config = PagingConfig(
            pageSize = LIBRARY_PAGE_SIZE,
            initialLoadSize = LIBRARY_PAGE_SIZE,
            prefetchDistance = 12,
            enablePlaceholders = false,
        ),
        pagingSourceFactory = { CatalogPagingSource(api, filter.normalized()) },
    ).flow

    suspend fun media(
        page: Int = 1,
        size: Int = 60,
        libraryId: String? = null,
    ): Result<PaginatedEnvelope<MediaCard>> = call {
        api.media(page.coerceAtLeast(1), size.coerceIn(1, 200), libraryId)
    }

    suspend fun libraries(): Result<List<LibrarySummary>> = call {
        api.libraries().data
    }

    suspend fun detail(id: String): Result<MediaDetail> = call {
        api.detail(id).data
    }

    suspend fun stream(id: String): Result<StreamInfo> = call {
        val capabilities = playbackCapabilities.current()
        api.stream(
            id = id,
            supportsDirect = capabilities.supportsDirectPlay,
            supportsRemux = capabilities.supportsRemux,
            supportsHevc = capabilities.supportsHevc,
        ).data
    }

    suspend fun createPlaybackSession(
        request: CreatePlaybackSessionRequest,
    ): Result<PlaybackSessionResult> = call {
        awaitPlaybackSessionReady(api.createPlaybackSession(request).data)
    }

    suspend fun restartPlaybackSession(
        sessionId: String,
        request: RestartPlaybackSessionRequest,
    ): Result<PlaybackSessionResult> = call {
        awaitPlaybackSessionReady(api.restartPlaybackSession(sessionId, request).data)
    }

    suspend fun heartbeatPlaybackSession(
        sessionId: String,
        request: PlaybackSessionHeartbeatRequest,
    ): Result<PlaybackSessionResult> = call {
        api.heartbeatPlaybackSession(sessionId, request).data
    }

    suspend fun closePlaybackSession(sessionId: String, reason: String): Result<Unit> = call {
        val response = api.closePlaybackSession(sessionId, reason.ifBlank { "android_client_closed" })
        if (!response.isSuccessful && response.code() != 404) {
            throw ServerException(response.code(), "播放会话关闭失败")
        }
    }

    suspend fun subtitles(id: String): Result<SubtitleTracksResponse> = call {
        api.subtitles(id).data
    }

    suspend fun nextEpisode(
        seriesId: String,
        season: Int,
        episode: Int,
    ): Result<MediaDetail?> = call {
        api.nextEpisode(seriesId, season, episode).data
    }

    private suspend fun awaitPlaybackSessionReady(
        initial: PlaybackSessionResult,
        timeoutMs: Long = PLAYBACK_SESSION_READY_TIMEOUT_MS,
    ): PlaybackSessionResult = withTimeout(timeoutMs) {
        var current = initial
        while (!current.firstSegmentReady || current.playlistUrl.isBlank()) {
            if (current.isTerminalFailure) {
                throw IllegalStateException(
                    current.failureMessage.ifBlank { "实时转码会话启动失败" },
                )
            }
            val sessionId = current.session.id
            if (sessionId.isBlank()) {
                throw IllegalStateException("服务器未返回播放会话标识")
            }
            delay(PLAYBACK_SESSION_POLL_INTERVAL_MS)
            current = api.playbackSessionStatus(sessionId).data
        }
        current
    }

    private suspend fun <T> call(block: suspend () -> T): Result<T> =
        runCatching { block() }.recoverCatching { error ->
            throw mapApiError(error)
        }
}

internal fun mapApiError(error: Throwable): Throwable = when (error) {
    is HttpException -> {
        if (error.code() == 401) UnauthorizedException()
        else ServerException(error.code(), error.message())
    }
    is IOException -> NetworkException(error.message ?: "网络不可用")
    else -> error
}
