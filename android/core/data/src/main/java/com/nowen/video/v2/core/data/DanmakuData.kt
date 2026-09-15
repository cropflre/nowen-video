package com.nowen.video.v2.core.data

import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.DanmakuAnime
import com.nowen.video.v2.core.model.DanmakuComment
import com.nowen.video.v2.core.model.DanmakuCommentResponse
import com.nowen.video.v2.core.model.DanmakuEpisodesResponse
import com.nowen.video.v2.core.model.DanmakuMatch
import com.nowen.video.v2.core.model.DanmakuMatchResponse
import com.nowen.video.v2.core.model.DanmakuCue
import com.nowen.video.v2.core.model.toCue
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Qualifier
import javax.inject.Singleton
import kotlinx.coroutines.flow.first
import kotlinx.serialization.json.Json
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Retrofit
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Query
import retrofit2.http.Url

@Qualifier
@Retention(AnnotationRetention.BINARY)
annotation class DanmakuHttpClient

@Qualifier
@Retention(AnnotationRetention.BINARY)
annotation class DanmakuRetrofit

@Qualifier
@Retention(AnnotationRetention.BINARY)
annotation class DanmakuApiService

interface DanmakuApi {
    @GET
    suspend fun searchAnime(@Url url: String, @Query("keyword") keyword: String): DanmakuSearchEnvelope

    @POST
    suspend fun match(@Url url: String, @Body request: DanmakuMatchRequest): DanmakuMatchResponse

    @GET
    suspend fun searchEpisodes(
        @Url url: String,
        @Query("anime") anime: String,
        @Query("episode") episode: String? = null,
    ): DanmakuEpisodesResponse

    @GET
    suspend fun bangumi(@Url url: String): DanmakuBangumiEnvelope

    @GET
    suspend fun comment(
        @Url url: String,
        @Query("format") format: String = "json",
        @Query("duration") duration: Boolean = true,
    ): DanmakuCommentResponse

    @GET
    suspend fun commentByUrl(
        @Url url: String,
        @Query("url") videoUrl: String,
        @Query("format") format: String = "json",
        @Query("duration") duration: Boolean = true,
    ): DanmakuCommentResponse
}

@kotlinx.serialization.Serializable
data class DanmakuSearchEnvelope(
    val errorCode: Int = 0,
    val success: Boolean = true,
    val errorMessage: String = "",
    val animes: List<DanmakuAnime> = emptyList(),
)

@kotlinx.serialization.Serializable
data class DanmakuBangumiEnvelope(
    val errorCode: Int = 0,
    val success: Boolean = true,
    val errorMessage: String = "",
    val bangumi: kotlinx.serialization.json.JsonObject? = null,
)

@kotlinx.serialization.Serializable
data class DanmakuMatchRequest(
    val fileName: String,
    val fileHash: String = "",
    val fileSize: Long = 0L,
    val videoDuration: Double = 0.0,
    val matchMode: String = "fileNameOnly",
)

@Module
@InstallIn(SingletonComponent::class)
object DanmakuNetworkModule {
    @Provides
    @Singleton
    @DanmakuHttpClient
    fun danmakuHttpClient(): OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(12, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .writeTimeout(15, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
        .build()

    @Provides
    @Singleton
    @DanmakuRetrofit
    fun danmakuRetrofit(
        @DanmakuHttpClient client: OkHttpClient,
        json: Json,
    ): Retrofit = Retrofit.Builder()
        .baseUrl("https://danmaku.invalid/")
        .client(client)
        .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
        .build()

    @Provides
    @Singleton
    @DanmakuApiService
    fun danmakuApi(@DanmakuRetrofit retrofit: Retrofit): DanmakuApi = retrofit.create(DanmakuApi::class.java)
}

@Singleton
class DanmakuRepository @Inject constructor(
    @DanmakuApiService private val api: DanmakuApi,
    private val preferencesStore: DanmakuPreferencesStore,
) {
    suspend fun searchAnime(keyword: String): Result<List<DanmakuAnime>> = call {
        val response = api.searchAnime(endpoint("search/anime"), keyword.trim())
        response.requireSuccess()
        response.animes
    }

    suspend fun searchEpisodes(anime: String, episode: String? = null): Result<DanmakuEpisodesResponse> = call {
        val response = api.searchEpisodes(endpoint("search/episodes"), anime.trim(), episode?.trim()?.takeIf(String::isNotBlank))
        response.requireSuccess()
        response
    }

    suspend fun match(fileName: String, duration: Double = 0.0): Result<List<DanmakuMatch>> = call {
        val response = api.match(endpoint("match"), DanmakuMatchRequest(fileName = fileName, videoDuration = duration))
        response.requireSuccess()
        response.matches
    }

    suspend fun commentsForId(commentId: Long, shiftMs: Long = 0L): Result<List<DanmakuCue>> = call {
        val response = api.comment(endpoint("comment/$commentId"), duration = true)
        response.requireSuccess()
        response.comments.toCues(shiftMs)
    }

    suspend fun commentsForUrl(url: String, shiftMs: Long = 0L): Result<List<DanmakuCue>> = call {
        val response = api.commentByUrl(endpoint("comment"), videoUrl = url, duration = true)
        response.requireSuccess()
        response.comments.toCues(shiftMs)
    }

    suspend fun loadForMedia(title: String, season: Int, episode: Int, duration: Double): Result<List<DanmakuCue>> {
        val preferences = preferencesStore.preferences.first()
        if (preferences.apiBaseUrl.isBlank()) return Result.success(emptyList())
        val fileName = buildString {
            append(title.trim())
            if (season > 0 && episode > 0) append(" S${season.toString().padStart(2, '0')}E${episode.toString().padStart(2, '0')}")
        }
        val matches = match(fileName, duration).getOrElse { return Result.failure(it) }
        val selected = selectBestMatch(matches, title, episode) ?: return Result.success(emptyList())
        val preferenceOffsetMs = preferences.offsetMs.toLong()
        val shiftMs = (selected.shift * 1_000.0).toLong() + preferenceOffsetMs
        return when {
            selected.url.isNotBlank() -> commentsForUrl(selected.url, shiftMs)
            selected.episodeId > 0L -> commentsForId(selected.episodeId, shiftMs)
            else -> Result.success(emptyList())
        }
    }

    private suspend fun endpoint(path: String): String {
        val preferences = preferencesStore.preferences.first()
        return buildDanmakuEndpoint(preferences.apiBaseUrl, preferences.apiToken, path)
    }

    private suspend fun <T> call(block: suspend () -> T): Result<T> = runCatching { block() }
}

private fun DanmakuSearchEnvelope.requireSuccess() {
    if (!success || errorCode != 0) error(errorMessage.ifBlank { "弹幕作品搜索失败" })
}

private fun DanmakuEpisodesResponse.requireSuccess() {
    if (!success || errorCode != 0) error(errorMessage.ifBlank { "弹幕剧集搜索失败" })
}

private fun DanmakuMatchResponse.requireSuccess() {
    if (!success || errorCode != 0) error(errorMessage.ifBlank { "弹幕自动匹配失败" })
}

private fun DanmakuCommentResponse.requireSuccess() {
    if (!success || errorCode != 0) error(errorMessage.ifBlank { "弹幕加载失败" })
}

internal fun buildDanmakuEndpoint(baseUrl: String, token: String, path: String): String {
    val base = baseUrl.trim().trimEnd('/').toHttpUrlOrNull()
        ?: error("弹幕 API 地址无效")
    val segments = base.pathSegments.filter(String::isNotBlank).toMutableList()
    val normalized = segments.joinToString("/")
    if (!normalized.endsWith("api/v2")) {
        segments.removeAll { it == "api" || it == "v2" }
        segments += listOf("api", "v2")
    }
    val tokenValue = token.trim()
    if (tokenValue.isNotBlank() && segments.firstOrNull() != tokenValue) {
        segments.add(0, tokenValue)
    }
    val endpointSegments = segments + path.trim('/').split('/').filter(String::isNotBlank)
    return base.newBuilder()
        .encodedPath("/")
        .query(null)
        .fragment(null)
        .apply { endpointSegments.forEach(::addPathSegment) }
        .build()
        .toString()
}

private fun selectBestMatch(
    matches: List<DanmakuMatch>,
    title: String,
    episode: Int,
): DanmakuMatch? {
    val normalizedTitle = title.trim().lowercase().replace(Regex("\\s+"), "")
    return matches
        .asSequence()
        .filter { it.episodeId > 0L || it.url.isNotBlank() }
        .map { candidate ->
            val candidateTitle = candidate.animeTitle.trim().lowercase().replace(Regex("\\s+"), "")
            val titleScore = when {
                candidateTitle == normalizedTitle -> 3
                candidateTitle.contains(normalizedTitle) || normalizedTitle.contains(candidateTitle) -> 2
                else -> 0
            }
            val episodeScore = if (episode <= 0 || candidate.episodeTitle.contains(episode.toString())) 1 else 0
            candidate to (titleScore + episodeScore)
        }
        .filter { (_, score) -> score > 0 }
        .maxByOrNull { (_, score) -> score }
        ?.first
}

private fun List<DanmakuComment>.toCues(shiftMs: Long): List<DanmakuCue> =
    mapIndexedNotNull { index, comment -> comment.toCue(index, shiftMs = shiftMs) }
        .distinctBy { "${it.effectiveTimeMs}:${it.text}" }
        .sortedBy(DanmakuCue::effectiveTimeMs)
        .take(5_000)
