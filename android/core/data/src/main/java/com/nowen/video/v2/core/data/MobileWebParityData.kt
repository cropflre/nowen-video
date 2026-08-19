package com.nowen.video.v2.core.data

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.ApiEnvelope
import com.nowen.video.v2.core.model.CreateMediaCommentRequest
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MediaComment
import com.nowen.video.v2.core.model.MediaCommentList
import com.nowen.video.v2.core.model.MediaHighlightList
import com.nowen.video.v2.core.model.MobileWebHomeContent
import com.nowen.video.v2.core.model.RecommendedMediaCard
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.async
import kotlinx.coroutines.supervisorScope
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Response
import retrofit2.Retrofit
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

private val WEB_HOME_GENRES = listOf("动画", "喜剧", "冒险", "家庭")

/** Android 复用 Web 移动端已经存在的内容与社交接口。 */
interface MobileWebParityApi {
    @GET("recommend")
    suspend fun recommendations(@Query("limit") limit: Int = 20): ApiEnvelope<List<RecommendedMediaCard>>

    @GET("media/continue")
    suspend fun continueWatching(@Query("limit") limit: Int = 10): ApiEnvelope<List<MediaCard>>

    @GET("media/recent/mixed")
    suspend fun recentMixed(@Query("limit") limit: Int = 24): ApiEnvelope<List<MediaCard>>

    @GET("media/mixed")
    suspend fun mixed(
        @Query("page") page: Int = 1,
        @Query("size") size: Int = 16,
        @Query("genre") genre: String,
        @Query("sort") sort: String = "added",
        @Query("order") order: String = "desc",
    ): ApiEnvelope<List<MediaCard>>

    @GET("recommend/similar/{mediaId}")
    suspend fun similar(
        @Path("mediaId") mediaId: String,
        @Query("limit") limit: Int = 12,
    ): ApiEnvelope<List<RecommendedMediaCard>>

    @GET("media/{mediaId}/highlights")
    suspend fun highlights(@Path("mediaId") mediaId: String): ApiEnvelope<MediaHighlightList>

    @GET("media/{mediaId}/comments")
    suspend fun comments(
        @Path("mediaId") mediaId: String,
        @Query("page") page: Int = 1,
        @Query("size") size: Int = 10,
    ): MediaCommentList

    @POST("media/{mediaId}/comments")
    suspend fun createComment(
        @Path("mediaId") mediaId: String,
        @Body request: CreateMediaCommentRequest,
    ): ApiEnvelope<MediaComment>

    @DELETE("comments/{commentId}")
    suspend fun deleteComment(@Path("commentId") commentId: String): Response<Unit>
}

@Module
@InstallIn(SingletonComponent::class)
object MobileWebParityNetworkModule {
    @Provides
    @Singleton
    fun mobileWebParityApi(client: OkHttpClient, json: Json): MobileWebParityApi = Retrofit.Builder()
        .baseUrl("https://$PLACEHOLDER_HOST/api/")
        .client(client)
        .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
        .build()
        .create(MobileWebParityApi::class.java)
}

@Singleton
class MobileWebParityRepository @Inject constructor(
    private val api: MobileWebParityApi,
) {
    /** 与 Web HomePage 的 Promise.allSettled 语义一致：单个分区失败不拖垮整个首页。 */
    suspend fun home(): Result<MobileWebHomeContent> = runCatching {
        supervisorScope {
            val recommendations = async { runCatching { api.recommendations(12).data } }
            val continuing = async { runCatching { api.continueWatching(10).data } }
            val recent = async { runCatching { api.recentMixed(24).data } }
            val genres = WEB_HOME_GENRES.associateWith { genre ->
                async { runCatching { api.mixed(genre = genre).data } }
            }

            val recommendationResult = recommendations.await()
            val continueResult = continuing.await()
            val recentResult = recent.await()
            val recentItems = recentResult.getOrDefault(emptyList())
            val unavailable = linkedSetOf<String>()

            if (recommendationResult.isFailure) unavailable += "推荐"
            if (continueResult.isFailure) unavailable += "继续观看"
            if (recentResult.isFailure) unavailable += "最近添加"

            val genreShelves = genres.mapValues { (genre, deferred) ->
                val result = deferred.await()
                if (result.isFailure) unavailable += genre
                result.getOrDefault(emptyList()).ifEmpty {
                    recentItems.filter { card -> card.matchesGenre(genre) }
                }
            }

            MobileWebHomeContent(
                recommendations = recommendationResult.getOrDefault(emptyList())
                    .map(RecommendedMediaCard::media)
                    .filter { it.resolvedId.isNotBlank() },
                continueWatching = continueResult.getOrDefault(emptyList())
                    .filter { it.resolvedId.isNotBlank() },
                recent = recentItems.filter { it.resolvedId.isNotBlank() },
                genreShelves = genreShelves.mapValues { (_, items) ->
                    items.filter { it.resolvedId.isNotBlank() }
                },
                unavailableSections = unavailable,
            )
        }
    }.recoverCatching { error ->
        throw mapApiError(error)
    }

    suspend fun recommendations(limit: Int = 20): Result<List<MediaCard>> = runCatching {
        api.recommendations(limit.coerceIn(1, 60)).data
            .map(RecommendedMediaCard::media)
            .filter { it.resolvedId.isNotBlank() }
    }.recoverCatching { error ->
        throw mapApiError(error)
    }

    suspend fun similar(mediaId: String, limit: Int = 12): Result<List<MediaCard>> = runCatching {
        api.similar(mediaId, limit.coerceIn(1, 30)).data
            .map(RecommendedMediaCard::media)
            .filter { it.resolvedId.isNotBlank() }
    }.recoverCatching { error ->
        throw mapApiError(error)
    }

    suspend fun highlights(mediaId: String): Result<MediaHighlightList> = runCatching {
        api.highlights(mediaId).data
    }.recoverCatching { error ->
        throw mapApiError(error)
    }

    suspend fun comments(mediaId: String, page: Int = 1, size: Int = 10): Result<MediaCommentList> = runCatching {
        api.comments(mediaId, page.coerceAtLeast(1), size.coerceIn(1, 30))
    }.recoverCatching { error ->
        throw mapApiError(error)
    }

    suspend fun createComment(mediaId: String, content: String, rating: Int?): Result<MediaComment> = runCatching {
        api.createComment(
            mediaId,
            CreateMediaCommentRequest(content.trim(), rating?.coerceIn(1, 5)),
        ).data
    }.recoverCatching { error ->
        throw mapApiError(error)
    }

    suspend fun deleteComment(commentId: String): Result<Unit> = runCatching {
        val response = api.deleteComment(commentId)
        if (!response.isSuccessful) error("删除评价失败（${response.code()}）")
    }.recoverCatching { error ->
        throw mapApiError(error)
    }
}

private fun MediaCard.matchesGenre(genre: String): Boolean = genres
    .split(',')
    .map(String::trim)
    .filter(String::isNotBlank)
    .any { value -> value == genre || value.contains(genre) }
