package com.nowen.video.v2.core.data

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.ApiEnvelope
import com.nowen.video.v2.core.model.MediaPerson
import com.nowen.video.v2.core.model.SeasonInfo
import com.nowen.video.v2.core.model.SeriesBundle
import com.nowen.video.v2.core.model.SeriesInfo
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.http.GET
import retrofit2.http.Path

private const val SERIES_PLACEHOLDER = "placeholder.invalid"

interface SeriesApi {
    @GET("series/{id}")
    suspend fun detail(@Path("id") id: String): ApiEnvelope<SeriesInfo>

    @GET("series/{id}/seasons")
    suspend fun seasons(@Path("id") id: String): ApiEnvelope<List<SeasonInfo>>

    @GET("series/{id}/persons")
    suspend fun persons(@Path("id") id: String): ApiEnvelope<List<MediaPerson>>
}

@Module
@InstallIn(SingletonComponent::class)
object SeriesNetworkModule {
    @Provides
    @Singleton
    fun seriesApi(client: OkHttpClient, json: Json): SeriesApi =
        Retrofit.Builder()
            .baseUrl("https://$SERIES_PLACEHOLDER/api/")
            .client(client)
            .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(SeriesApi::class.java)
}

@Singleton
class SeriesRepository @Inject constructor(
    private val api: SeriesApi,
) {
    suspend fun load(id: String): Result<SeriesBundle> = call {
        coroutineScope {
            val series = async { api.detail(id).data }
            val seasons = async {
                api.seasons(id).data
                    .map(SeasonInfo::normalized)
                    .sortedBy(SeasonInfo::seasonNumber)
            }
            val persons = async {
                runCatching { api.persons(id).data }
                    .getOrDefault(emptyList())
                    .sortedBy(MediaPerson::sortOrder)
            }
            SeriesBundle(
                series = series.await(),
                seasons = seasons.await(),
                persons = persons.await(),
            )
        }
    }

    suspend fun detail(id: String): Result<SeriesInfo> = call { api.detail(id).data }

    suspend fun seasons(id: String): Result<List<SeasonInfo>> = call {
        api.seasons(id).data
            .map(SeasonInfo::normalized)
            .sortedBy(SeasonInfo::seasonNumber)
    }

    private suspend fun <T> call(block: suspend () -> T): Result<T> =
        runCatching { block() }.recoverCatching { error -> throw mapApiError(error) }
}
