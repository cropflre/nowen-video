package com.nowen.video.v2.core.data

import android.graphics.Bitmap
import android.media.MediaMetadataRetriever
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.decodeFromJsonElement
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Response
import retrofit2.Retrofit
import retrofit2.http.Body
import retrofit2.http.POST
import retrofit2.http.Path

const val MEDIA_COMPUTE_JOB_CHAPTER_DETECT_V1 = "chapter_detect_v1"
const val MEDIA_COMPUTE_CAPABILITY_CHAPTER_DETECT_V1 = "chapter_detect_v1"

private const val CHAPTER_WORKER_HOST = "placeholder.invalid"
private const val CHAPTER_SIGNATURE_WIDTH = 32
private const val CHAPTER_SIGNATURE_HEIGHT = 18

@Serializable
data class MediaComputeChapterDetectInput(
    @SerialName("media_id") val mediaId: String = "",
    val fingerprint: String = "",
    val duration: Double = 0.0,
    @SerialName("stream_url") val streamUrl: String = "",
    @SerialName("sample_times") val sampleTimes: List<Double> = emptyList(),
    @SerialName("probe_gap_seconds") val probeGapSeconds: Double = 3.0,
    @SerialName("min_chapter_seconds") val minChapterSeconds: Double = 60.0,
    @SerialName("max_chapters") val maxChapters: Int = 12,
    @SerialName("capture_width") val captureWidth: Int = 240,
    @SerialName("engine_version") val engineVersion: Int = 1,
)

@Serializable
data class MediaComputeChapterCandidate(
    val time: Double,
    val score: Double,
)

@Serializable
data class MediaComputeChapterDetectResult(
    val fingerprint: String,
    val candidates: List<MediaComputeChapterCandidate>,
)

@Serializable
data class MediaComputeChapterComplete(
    @SerialName("claim_token") val claimToken: String,
    @SerialName("job_type") val jobType: String = MEDIA_COMPUTE_JOB_CHAPTER_DETECT_V1,
    val result: MediaComputeChapterDetectResult,
)

interface ChapterComputeApi {
    @POST("media-analysis/workers/tasks/{taskId}/progress")
    suspend fun progress(
        @Path("taskId") taskId: String,
        @Body request: HighlightWorkerProgress,
    ): Response<Unit>

    @POST("media-analysis/workers/tasks/{taskId}/complete")
    suspend fun complete(
        @Path("taskId") taskId: String,
        @Body request: MediaComputeChapterComplete,
    ): Response<Unit>
}

@Module
@InstallIn(SingletonComponent::class)
object ChapterComputeNetworkModule {
    @Provides
    @Singleton
    fun chapterComputeApi(client: OkHttpClient, json: Json): ChapterComputeApi = Retrofit.Builder()
        .baseUrl("https://$CHAPTER_WORKER_HOST/api/")
        .client(client)
        .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
        .build()
        .create(ChapterComputeApi::class.java)
}

@Singleton
class ChapterComputeExecutor @Inject constructor(
    private val api: ChapterComputeApi,
    private val json: Json,
) {
    suspend fun process(serverBaseUrl: String, token: String, claim: MediaComputeTaskClaim) = withContext(Dispatchers.IO) {
        if (claim.requiredCapability != MEDIA_COMPUTE_CAPABILITY_CHAPTER_DETECT_V1) {
            error("Android 媒体计算节点缺少章节检测能力：${claim.requiredCapability}")
        }
        val element = claim.input ?: error("服务器没有返回 chapter_detect_v1 input")
        val input = runCatching { json.decodeFromJsonElement<MediaComputeChapterDetectInput>(element) }
            .getOrElse { error("服务器返回的 chapter_detect_v1 input 格式无效") }
        if (input.fingerprint.isBlank() || input.duration <= 0.0 || input.sampleTimes.isEmpty()) {
            error("服务器没有返回可用的章节检测采样计划")
        }
        val streamUrl = requireNotNull(UrlNormalizer.apiUrl(serverBaseUrl, input.streamUrl)) {
            "服务器返回的章节检测媒体流地址无效"
        }
        val headers = mapOf("Authorization" to "Bearer $token")
        val gap = input.probeGapSeconds.coerceIn(1.0, 8.0)
        val retriever = MediaMetadataRetriever()
        try {
            retriever.setDataSource(streamUrl, headers)
            val candidates = ArrayList<MediaComputeChapterCandidate>(input.sampleTimes.size)
            input.sampleTimes.forEachIndexed { index, center ->
                if (!center.isFinite() || center <= 0.0 || center >= input.duration) {
                    error("服务器返回了无效章节检测时间点")
                }
                val before = extractFrame(retriever, max(0.0, center - gap))
                    ?: error("Android 无法提取章节检测前帧")
                val after = extractFrame(retriever, min(input.duration, center + gap))
                    ?: error("Android 无法提取章节检测后帧")
                val score = try {
                    signatureDifference(frameSignature(before), frameSignature(after))
                } finally {
                    before.recycle()
                    after.recycle()
                }
                candidates += MediaComputeChapterCandidate(time = center, score = score)
                if ((index + 1) % 4 == 0 || index == input.sampleTimes.lastIndex) {
                    runCatching {
                        api.progress(
                            claim.taskId,
                            HighlightWorkerProgress(
                                claimToken = claim.claimToken,
                                stage = "client_chapter_probe",
                                progress = 8.0 + 84.0 * (index + 1).toDouble() / input.sampleTimes.size.toDouble(),
                            ),
                        )
                    }
                }
            }
            if (candidates.size != input.sampleTimes.size) {
                error("Android 章节检测没有完成全部采样点")
            }
            val completed = api.complete(
                claim.taskId,
                MediaComputeChapterComplete(
                    claimToken = claim.claimToken,
                    result = MediaComputeChapterDetectResult(
                        fingerprint = input.fingerprint,
                        candidates = candidates,
                    ),
                ),
            )
            if (!completed.isSuccessful) {
                error("服务器拒绝 Android 章节检测结果：HTTP ${completed.code()}")
            }
        } finally {
            retriever.release()
        }
    }

    private fun extractFrame(retriever: MediaMetadataRetriever, seconds: Double): Bitmap? {
        val micros = (seconds.coerceAtLeast(0.0) * 1_000_000.0).toLong()
        return retriever.getFrameAtTime(micros, MediaMetadataRetriever.OPTION_CLOSEST_SYNC)
            ?: retriever.getFrameAtTime(micros, MediaMetadataRetriever.OPTION_CLOSEST)
    }

    private fun frameSignature(source: Bitmap): DoubleArray {
        val bitmap = Bitmap.createScaledBitmap(source, CHAPTER_SIGNATURE_WIDTH, CHAPTER_SIGNATURE_HEIGHT, true)
        try {
            val pixels = IntArray(CHAPTER_SIGNATURE_WIDTH * CHAPTER_SIGNATURE_HEIGHT)
            bitmap.getPixels(pixels, 0, CHAPTER_SIGNATURE_WIDTH, 0, 0, CHAPTER_SIGNATURE_WIDTH, CHAPTER_SIGNATURE_HEIGHT)
            return DoubleArray(pixels.size) { index ->
                val color = pixels[index]
                val r = (color shr 16) and 0xff
                val g = (color shr 8) and 0xff
                val b = color and 0xff
                0.2126 * r + 0.7152 * g + 0.0722 * b
            }
        } finally {
            if (bitmap !== source) bitmap.recycle()
        }
    }

    private fun signatureDifference(before: DoubleArray, after: DoubleArray): Double {
        if (before.isEmpty() || before.size != after.size) return 0.0
        var total = 0.0
        for (index in before.indices) total += abs(before[index] - after[index])
        return (total / before.size / 255.0).coerceIn(0.0, 1.0)
    }
}
