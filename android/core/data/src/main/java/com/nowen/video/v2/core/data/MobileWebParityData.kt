package com.nowen.video.v2.core.data

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.ApiEnvelope
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MediaHighlightList
import com.nowen.video.v2.core.model.RecommendedMediaCard
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.http.GET
import retrofit2.http.Path
import retrofit2.http.Query

/**
 * Web 移动端已经使用的内容接口。
 * Android 复用同一服务端契约，避免为了视觉对齐再造一套数据源。
 */
interface MobileWebParityApi {
    @GET("recommend")
    suspend fun recommendations(@Query("limit") limit: Int = 20): ApiEnvelope<List<RecommendedMediaCard>>

    @GET("recommend/similar/{mediaId}")
    suspend fun similar(
        @Path("mediaId") mediaId: String,
        @Query("limit") limit: Int = 12,
    ): ApiEnvelope<List<RecommendedMediaCard>>

    @GET("media/{mediaId}/highlights")
    suspend fun highlights(@Path("mediaId") mediaId: String): ApiEnvelope<MediaHighlightList>
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
}
