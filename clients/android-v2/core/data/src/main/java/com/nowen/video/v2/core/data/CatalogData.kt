package com.nowen.video.v2.core.data

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.ApiEnvelope
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.PaginatedEnvelope
import com.nowen.video.v2.core.model.StreamInfo
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import java.io.IOException
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.HttpException
import retrofit2.Retrofit
import retrofit2.http.GET
import retrofit2.http.Path
import retrofit2.http.Query

private const val CATALOG_PLACEHOLDER = "placeholder.invalid"

interface CatalogApi {
    @GET("media/mixed")
    suspend fun media(
        @Query("page") page: Int,
        @Query("size") size: Int,
        @Query("library_id") libraryId: String? = null,
    ): PaginatedEnvelope<MediaCard>

    @GET("media/{id}")
    suspend fun detail(@Path("id") id: String): ApiEnvelope<MediaDetail>

    @GET("stream/{id}/info")
    suspend fun stream(@Path("id") id: String): ApiEnvelope<StreamInfo>
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
) {
    suspend fun media(
        page: Int = 1,
        size: Int = 60,
        libraryId: String? = null,
    ): Result<PaginatedEnvelope<MediaCard>> = call {
        api.media(page.coerceAtLeast(1), size.coerceIn(1, 200), libraryId)
    }

    suspend fun detail(id: String): Result<MediaDetail> = call {
        api.detail(id).data
    }

    suspend fun stream(id: String): Result<StreamInfo> = call {
        api.stream(id).data
    }

    private suspend fun <T> call(block: suspend () -> T): Result<T> =
        runCatching { block() }.recoverCatching { error ->
            when (error) {
                is HttpException -> {
                    if (error.code() == 401) throw UnauthorizedException()
                    throw ServerException(error.code(), error.message())
                }
                is IOException -> throw NetworkException(error.message ?: "网络不可用")
                else -> throw error
            }
        }
}
