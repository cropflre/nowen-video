package com.nowen.video.v2.core.data

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.ApiEnvelope
import com.nowen.video.v2.core.model.CollectionWithMedia
import com.nowen.video.v2.core.model.FavoriteRecord
import com.nowen.video.v2.core.model.MediaPerson
import com.nowen.video.v2.core.model.MovieCollection
import com.nowen.video.v2.core.model.PaginatedEnvelope
import com.nowen.video.v2.core.model.Person
import com.nowen.video.v2.core.model.PersonMediaResponse
import com.nowen.video.v2.core.model.WatchHistoryRecord
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Response
import retrofit2.Retrofit
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

private const val SOCIAL_CATALOG_PLACEHOLDER = "placeholder.invalid"

interface SocialCatalogApi {
    @GET("users/me/favorites")
    suspend fun favorites(
        @Query("page") page: Int = 1,
        @Query("size") size: Int = 50,
    ): PaginatedEnvelope<FavoriteRecord>

    @POST("users/me/favorites/{mediaId}")
    suspend fun addFavorite(@Path("mediaId") mediaId: String): Response<Unit>

    @DELETE("users/me/favorites/{mediaId}")
    suspend fun removeFavorite(@Path("mediaId") mediaId: String): Response<Unit>

    @GET("users/me/favorites/{mediaId}/check")
    suspend fun favoriteStatus(@Path("mediaId") mediaId: String): ApiEnvelope<Boolean>

    @GET("users/me/history")
    suspend fun history(
        @Query("page") page: Int = 1,
        @Query("size") size: Int = 50,
    ): PaginatedEnvelope<WatchHistoryRecord>

    @DELETE("users/me/history/{mediaId}")
    suspend fun deleteHistory(@Path("mediaId") mediaId: String): Response<Unit>

    @DELETE("users/me/history")
    suspend fun clearHistory(): Response<Unit>

    @GET("collections")
    suspend fun collections(
        @Query("page") page: Int = 1,
        @Query("size") size: Int = 100,
        @Query("sort") sort: String = "name_asc",
    ): PaginatedEnvelope<MovieCollection>

    @GET("collections/{id}")
    suspend fun collection(@Path("id") id: String): ApiEnvelope<CollectionWithMedia>

    @GET("media/{id}/collection")
    suspend fun mediaCollection(@Path("id") id: String): ApiEnvelope<CollectionWithMedia?>

    @GET("media/{id}/persons")
    suspend fun mediaPersons(@Path("id") id: String): ApiEnvelope<List<MediaPerson>>

    @GET("persons/{id}")
    suspend fun person(@Path("id") id: String): ApiEnvelope<Person>

    @GET("persons/{id}/media")
    suspend fun personMedia(@Path("id") id: String): PersonMediaResponse
}

@Module
@InstallIn(SingletonComponent::class)
object SocialCatalogNetworkModule {
    @Provides
    @Singleton
    fun socialCatalogApi(client: OkHttpClient, json: Json): SocialCatalogApi =
        Retrofit.Builder()
            .baseUrl("https://$SOCIAL_CATALOG_PLACEHOLDER/api/")
            .client(client)
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(SocialCatalogApi::class.java)
}

@Singleton
class SocialCatalogRepository @Inject constructor(
    private val api: SocialCatalogApi,
) {
    suspend fun favorites(): Result<PaginatedEnvelope<FavoriteRecord>> = call { api.favorites() }

    suspend fun favoriteStatus(mediaId: String): Result<Boolean> = call {
        api.favoriteStatus(mediaId).data
    }

    suspend fun setFavorite(mediaId: String, favorite: Boolean): Result<Unit> = call {
        val response = if (favorite) api.addFavorite(mediaId) else api.removeFavorite(mediaId)
        val accepted = response.isSuccessful ||
            (favorite && response.code() == 409) ||
            (!favorite && response.code() == 404)
        if (!accepted) throw ServerException(response.code(), "收藏状态更新失败")
    }

    suspend fun history(): Result<PaginatedEnvelope<WatchHistoryRecord>> = call { api.history() }

    suspend fun deleteHistory(mediaId: String): Result<Unit> = call {
        val response = api.deleteHistory(mediaId)
        if (!response.isSuccessful && response.code() != 404) {
            throw ServerException(response.code(), "删除观看记录失败")
        }
    }

    suspend fun clearHistory(): Result<Unit> = call {
        val response = api.clearHistory()
        if (!response.isSuccessful) throw ServerException(response.code(), "清空观看历史失败")
    }

    suspend fun collections(): Result<PaginatedEnvelope<MovieCollection>> = call { api.collections() }

    suspend fun collection(id: String): Result<CollectionWithMedia> = call { api.collection(id).data }

    suspend fun mediaCollection(mediaId: String): Result<CollectionWithMedia?> = call {
        api.mediaCollection(mediaId).data
    }

    suspend fun mediaPersons(mediaId: String): Result<List<MediaPerson>> = call {
        api.mediaPersons(mediaId).data.sortedBy(MediaPerson::sortOrder)
    }

    suspend fun person(id: String): Result<Person> = call { api.person(id).data }

    suspend fun personMedia(id: String): Result<PersonMediaResponse> = call { api.personMedia(id) }

    private suspend fun <T> call(block: suspend () -> T): Result<T> =
        runCatching { block() }.recoverCatching { error -> throw mapApiError(error) }
}
