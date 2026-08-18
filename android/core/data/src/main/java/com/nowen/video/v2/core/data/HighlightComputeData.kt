package com.nowen.video.v2.core.data

import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.graphics.Bitmap
import android.media.MediaMetadataRetriever
import android.net.ConnectivityManager
import android.net.NetworkCapabilities
import android.os.BatteryManager
import android.os.Build
import android.os.PowerManager
import android.util.Base64
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.v2.core.model.ApiEnvelope
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import java.io.ByteArrayOutputStream
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min
import kotlin.math.sqrt
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.withContext
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import retrofit2.Response
import retrofit2.Retrofit
import retrofit2.http.Body
import retrofit2.http.POST
import retrofit2.http.Path

private const val HIGHLIGHT_WORKER_HOST = "placeholder.invalid"
private const val HIGHLIGHT_WORKER_IDLE_POLL_MS = 8_000L
private const val HIGHLIGHT_WORKER_INELIGIBLE_POLL_MS = 15_000L
private const val HIGHLIGHT_MIN_BATTERY_PERCENT = 40
private const val HIGHLIGHT_MIN_SEPARATION_SECONDS = 45.0
private const val HIGHLIGHT_THUMBNAIL_MAX_WIDTH = 640
private const val HIGHLIGHT_THUMBNAIL_SOFT_LIMIT = 480 * 1024
private const val MEDIA_COMPUTE_PROTOCOL_VERSION = 2
private const val MEDIA_COMPUTE_JOB_HIGHLIGHT_V1 = "highlight_v1"

@Serializable
data class HighlightWorkerHeartbeat(
    @SerialName("worker_id") val workerId: String,
    val kind: String = "android",
    val name: String,
    val version: String,
    val capabilities: List<String> = listOf(MEDIA_COMPUTE_JOB_HIGHLIGHT_V1),
    val network: String,
    val charging: Boolean,
    @SerialName("battery_percent") val batteryPercent: Int,
)

@Serializable
data class HighlightWorkerClaimRequest(
    @SerialName("worker_id") val workerId: String,
    val kind: String = "android",
    val name: String,
    val version: String,
    val capabilities: List<String> = listOf(MEDIA_COMPUTE_JOB_HIGHLIGHT_V1),
    val network: String,
    val charging: Boolean,
    @SerialName("battery_percent") val batteryPercent: Int,
)

@Serializable
data class MediaComputeHighlightInput(
    @SerialName("media_id") val mediaId: String = "",
    val fingerprint: String = "",
    val duration: Double = 0.0,
    @SerialName("stream_url") val streamUrl: String = "",
    @SerialName("sample_times") val sampleTimes: List<Double> = emptyList(),
    @SerialName("max_highlights") val maxHighlights: Int = 8,
    @SerialName("engine_version") val engineVersion: Int = 3,
)

@Serializable
data class HighlightWorkerClaim(
    @SerialName("protocol_version") val protocolVersion: Int = 1,
    @SerialName("job_type") val jobType: String = MEDIA_COMPUTE_JOB_HIGHLIGHT_V1,
    @SerialName("required_capability") val requiredCapability: String = MEDIA_COMPUTE_JOB_HIGHLIGHT_V1,
    @SerialName("task_id") val taskId: String,
    @SerialName("claim_token") val claimToken: String,
    val input: MediaComputeHighlightInput? = null,
    // V1 compatibility fields. V2 clients prefer input, while older servers still only return these.
    @SerialName("media_id") val mediaId: String = "",
    val fingerprint: String = "",
    val duration: Double = 0.0,
    @SerialName("stream_url") val streamUrl: String = "",
    @SerialName("sample_times") val sampleTimes: List<Double> = emptyList(),
    @SerialName("max_highlights") val maxHighlights: Int = 8,
    @SerialName("engine_version") val engineVersion: Int = 3,
)

@Serializable
data class HighlightWorkerProgress(
    @SerialName("claim_token") val claimToken: String,
    val stage: String,
    val progress: Double,
)

@Serializable
data class HighlightWorkerResultItem(
    val title: String = "",
    @SerialName("start_time") val startTime: Double,
    @SerialName("end_time") val endTime: Double,
    val score: Double,
    @SerialName("analysis_method") val analysisMethod: String = "android_visual_sparse_v1",
    @SerialName("thumbnail_base64") val thumbnailBase64: String = "",
    @SerialName("thumbnail_mime") val thumbnailMime: String = "image/webp",
)

@Serializable
data class HighlightWorkerComplete(
    @SerialName("claim_token") val claimToken: String,
    val fingerprint: String,
    val highlights: List<HighlightWorkerResultItem>,
)

@Serializable
data class HighlightWorkerFailure(
    @SerialName("claim_token") val claimToken: String,
    val error: String,
)

interface HighlightComputeApi {
    // 历史 URL 作为兼容传输层继续保留；响应已升级为 Media Compute Node V2 task envelope。
    @POST("media-analysis/workers/claim")
    suspend fun claim(@Body request: HighlightWorkerClaimRequest): Response<ApiEnvelope<HighlightWorkerClaim>>

    @POST("media-analysis/workers/tasks/{taskId}/progress")
    suspend fun progress(
        @Path("taskId") taskId: String,
        @Body request: HighlightWorkerProgress,
    ): Response<Unit>

    @POST("media-analysis/workers/tasks/{taskId}/complete")
    suspend fun complete(
        @Path("taskId") taskId: String,
        @Body request: HighlightWorkerComplete,
    ): Response<Unit>

    @POST("media-analysis/workers/tasks/{taskId}/fail")
    suspend fun fail(
        @Path("taskId") taskId: String,
        @Body request: HighlightWorkerFailure,
    ): Response<Unit>
}

@Module
@InstallIn(SingletonComponent::class)
object HighlightComputeNetworkModule {
    @Provides
    @Singleton
    fun highlightComputeApi(client: OkHttpClient, json: Json): HighlightComputeApi = Retrofit.Builder()
        .baseUrl("https://$HIGHLIGHT_WORKER_HOST/api/")
        .client(client)
        .addConverterFactory(json.asConverterFactory("application/json".toMediaType()))
        .build()
        .create(HighlightComputeApi::class.java)
}

private data class HighlightDeviceState(
    val eligible: Boolean,
    val network: String,
    val charging: Boolean,
    val batteryPercent: Int,
)

private data class VisualSample(
    val time: Double,
    val rawScore: Double,
    var normalizedScore: Double = 5.5,
)

@Singleton
class HighlightComputeAgent @Inject constructor(
    @ApplicationContext private val context: Context,
    private val api: HighlightComputeApi,
    private val sessionStore: ServerSessionStore,
) {
    private val workerId: String by lazy {
        val preferences = context.getSharedPreferences("nowen_highlight_worker", Context.MODE_PRIVATE)
        preferences.getString("worker_id", null)?.takeIf(String::isNotBlank) ?: UUID.randomUUID().toString().also {
            preferences.edit().putString("worker_id", it).apply()
        }
    }

    /**
     * Media Compute Node V2 当前只在 Activity 前台生命周期内运行。
     * 循环本身不会创建后台常驻服务，因此退出应用后不会继续耗电或偷偷占用移动数据。
     */
    suspend fun runForegroundLoop() {
        while (currentCoroutineContext().isActive) {
            val snapshot = sessionStore.snapshot.value
            if (!snapshot.isAuthenticated || !snapshot.user?.role.equals("admin", ignoreCase = true)) {
                delay(HIGHLIGHT_WORKER_INELIGIBLE_POLL_MS)
                continue
            }
            val server = snapshot.activeServer
            val token = snapshot.token
            if (server == null || token.isNullOrBlank()) {
                delay(HIGHLIGHT_WORKER_INELIGIBLE_POLL_MS)
                continue
            }

            val device = deviceState()
            if (!device.eligible) {
                delay(HIGHLIGHT_WORKER_INELIGIBLE_POLL_MS)
                continue
            }

            val request = HighlightWorkerClaimRequest(
                workerId = workerId,
                name = androidWorkerName(),
                version = "android-v$MEDIA_COMPUTE_PROTOCOL_VERSION/${Build.VERSION.RELEASE}",
                network = device.network,
                charging = device.charging,
                batteryPercent = device.batteryPercent,
            )
            val response = try {
                api.claim(request)
            } catch (cancelled: CancellationException) {
                throw cancelled
            } catch (_: Throwable) {
                delay(HIGHLIGHT_WORKER_IDLE_POLL_MS)
                continue
            }

            if (response.code() == 204) {
                delay(HIGHLIGHT_WORKER_IDLE_POLL_MS)
                continue
            }
            val claim = response.body()?.data
            if (!response.isSuccessful || claim == null) {
                delay(HIGHLIGHT_WORKER_IDLE_POLL_MS)
                continue
            }

            try {
                val currentToken = sessionStore.snapshot.value.token?.takeIf(String::isNotBlank) ?: token
                processClaim(server.baseUrl, currentToken, claim)
            } catch (cancelled: CancellationException) {
                withContext(NonCancellable) {
                    reportFailure(claim, "Android 客户端离开前台，任务已释放")
                }
                throw cancelled
            } catch (error: Throwable) {
                reportFailure(claim, error.message ?: "Android 客户端媒体计算失败")
            }
        }
    }

    private fun resolveHighlightInput(claim: HighlightWorkerClaim): MediaComputeHighlightInput {
        if (claim.protocolVersion >= MEDIA_COMPUTE_PROTOCOL_VERSION && claim.jobType != MEDIA_COMPUTE_JOB_HIGHLIGHT_V1) {
            error("Android 媒体计算节点暂不支持任务类型：${claim.jobType}")
        }
        if (claim.requiredCapability.isNotBlank() && claim.requiredCapability != MEDIA_COMPUTE_JOB_HIGHLIGHT_V1) {
            error("Android 媒体计算节点缺少任务能力：${claim.requiredCapability}")
        }
        return claim.input ?: MediaComputeHighlightInput(
            mediaId = claim.mediaId,
            fingerprint = claim.fingerprint,
            duration = claim.duration,
            streamUrl = claim.streamUrl,
            sampleTimes = claim.sampleTimes,
            maxHighlights = claim.maxHighlights,
            engineVersion = claim.engineVersion,
        )
    }

    private suspend fun processClaim(
        serverBaseUrl: String,
        token: String,
        claim: HighlightWorkerClaim,
    ) = withContext(Dispatchers.IO) {
        val input = resolveHighlightInput(claim)
        if (input.sampleTimes.isEmpty() || input.duration <= 0.0) {
            error("服务器没有返回可用的精彩片段采样计划")
        }
        val streamUrl = requireNotNull(UrlNormalizer.apiUrl(serverBaseUrl, input.streamUrl)) {
            "服务器返回的媒体流地址无效"
        }
        val headers = mapOf("Authorization" to "Bearer $token")
        val retriever = MediaMetadataRetriever()
        try {
            retriever.setDataSource(streamUrl, headers)
            val samples = ArrayList<VisualSample>(input.sampleTimes.size)
            input.sampleTimes.forEachIndexed { index, time ->
                val bitmap = extractFrame(retriever, time) ?: return@forEachIndexed
                try {
                    samples += VisualSample(time = time, rawScore = visualInformationScore(bitmap))
                } finally {
                    bitmap.recycle()
                }
                reportProgress(
                    claim,
                    stage = "client_sampling",
                    progress = 8.0 + 54.0 * (index + 1).toDouble() / input.sampleTimes.size.toDouble(),
                )
            }
            if (samples.isEmpty()) {
                error("Android 硬件解码器无法从该媒体提取采样帧")
            }
            normalizeVisualScores(samples)
            val selected = selectHighlights(samples, input.maxHighlights.coerceIn(1, 8))
            if (selected.isEmpty()) {
                error("客户端没有生成有效的精彩片段候选")
            }

            val results = selected.mapIndexed { index, sample ->
                val frame = extractFrame(retriever, sample.time)
                    ?: error("无法生成精彩片段缩略图")
                val thumbnail = try {
                    encodeThumbnail(frame)
                } finally {
                    frame.recycle()
                }
                val start = max(0.0, sample.time - 10.0)
                val end = min(input.duration, start + 30.0)
                if (end <= start) error("精彩片段时间范围无效")
                reportProgress(
                    claim,
                    stage = "client_thumbnail",
                    progress = 64.0 + 28.0 * (index + 1).toDouble() / selected.size.toDouble(),
                )
                HighlightWorkerResultItem(
                    startTime = start,
                    endTime = end,
                    score = sample.normalizedScore.coerceIn(0.0, 10.0),
                    thumbnailBase64 = Base64.encodeToString(thumbnail.first, Base64.NO_WRAP),
                    thumbnailMime = thumbnail.second,
                )
            }

            val completed = api.complete(
                claim.taskId,
                HighlightWorkerComplete(
                    claimToken = claim.claimToken,
                    fingerprint = input.fingerprint.ifBlank { claim.fingerprint },
                    highlights = results,
                ),
            )
            if (!completed.isSuccessful) {
                error("服务器拒绝客户端精彩片段结果：HTTP ${completed.code()}")
            }
        } finally {
            retriever.release()
        }
    }

    private suspend fun reportProgress(claim: HighlightWorkerClaim, stage: String, progress: Double) {
        runCatching {
            api.progress(
                claim.taskId,
                HighlightWorkerProgress(
                    claimToken = claim.claimToken,
                    stage = stage,
                    progress = progress,
                ),
            )
        }
    }

    private suspend fun reportFailure(claim: HighlightWorkerClaim, message: String) {
        runCatching {
            api.fail(
                claim.taskId,
                HighlightWorkerFailure(claimToken = claim.claimToken, error = message.take(500)),
            )
        }
    }

    private fun extractFrame(retriever: MediaMetadataRetriever, seconds: Double): Bitmap? {
        val micros = (seconds.coerceAtLeast(0.0) * 1_000_000.0).toLong()
        return retriever.getFrameAtTime(micros, MediaMetadataRetriever.OPTION_CLOSEST_SYNC)
            ?: retriever.getFrameAtTime(micros, MediaMetadataRetriever.OPTION_CLOSEST)
    }

    private fun visualInformationScore(source: Bitmap): Double {
        val width = min(96, source.width).coerceAtLeast(1)
        val height = max(1, (source.height.toDouble() * width / source.width.coerceAtLeast(1)).toInt())
        val bitmap = if (source.width == width && source.height == height) source else Bitmap.createScaledBitmap(source, width, height, true)
        try {
            val pixels = IntArray(width * height)
            bitmap.getPixels(pixels, 0, width, 0, 0, width, height)
            if (pixels.isEmpty()) return 0.0
            var sum = 0.0
            val luminance = DoubleArray(pixels.size)
            for (i in pixels.indices) {
                val color = pixels[i]
                val r = (color shr 16) and 0xff
                val g = (color shr 8) and 0xff
                val b = color and 0xff
                val value = 0.2126 * r + 0.7152 * g + 0.0722 * b
                luminance[i] = value
                sum += value
            }
            val mean = sum / luminance.size
            var variance = 0.0
            var edge = 0.0
            var edges = 0
            for (y in 0 until height) {
                for (x in 0 until width) {
                    val index = y * width + x
                    val delta = luminance[index] - mean
                    variance += delta * delta
                    if (x > 0) {
                        edge += abs(luminance[index] - luminance[index - 1])
                        edges++
                    }
                    if (y > 0) {
                        edge += abs(luminance[index] - luminance[index - width])
                        edges++
                    }
                }
            }
            val standardDeviation = sqrt(variance / luminance.size)
            val edgeMean = if (edges > 0) edge / edges else 0.0
            val exposureBalance = 1.0 - min(1.0, abs(mean - 128.0) / 128.0)
            return standardDeviation * 0.52 + edgeMean * 0.38 + exposureBalance * 10.0
        } finally {
            if (bitmap !== source) bitmap.recycle()
        }
    }

    private fun normalizeVisualScores(samples: List<VisualSample>) {
        val minScore = samples.minOf { it.rawScore }
        val maxScore = samples.maxOf { it.rawScore }
        val span = maxScore - minScore
        samples.forEach { sample ->
            val normalized = if (span < 0.001) 0.5 else (sample.rawScore - minScore) / span
            sample.normalizedScore = 5.5 + normalized.coerceIn(0.0, 1.0) * 4.5
        }
    }

    private fun selectHighlights(samples: List<VisualSample>, limit: Int): List<VisualSample> {
        val chosen = ArrayList<VisualSample>(limit)
        for (candidate in samples.sortedByDescending { it.normalizedScore }) {
            if (chosen.all { abs(it.time - candidate.time) >= HIGHLIGHT_MIN_SEPARATION_SECONDS }) {
                chosen += candidate
            }
            if (chosen.size >= limit) break
        }
        return chosen.sortedBy { it.time }
    }

    private fun encodeThumbnail(source: Bitmap): Pair<ByteArray, String> {
        val width = min(HIGHLIGHT_THUMBNAIL_MAX_WIDTH, source.width).coerceAtLeast(1)
        val height = max(1, (source.height.toDouble() * width / source.width.coerceAtLeast(1)).toInt())
        val bitmap = if (source.width == width && source.height == height) source else Bitmap.createScaledBitmap(source, width, height, true)
        try {
            val output = ByteArrayOutputStream()
            val isWebP = Build.VERSION.SDK_INT >= Build.VERSION_CODES.R
            val format = if (isWebP) Bitmap.CompressFormat.WEBP_LOSSY else Bitmap.CompressFormat.JPEG
            val mime = if (isWebP) "image/webp" else "image/jpeg"
            for (quality in intArrayOf(72, 62, 52, 42)) {
                output.reset()
                bitmap.compress(format, quality, output)
                if (output.size() <= HIGHLIGHT_THUMBNAIL_SOFT_LIMIT) break
            }
            return output.toByteArray() to mime
        } finally {
            if (bitmap !== source) bitmap.recycle()
        }
    }

    private fun deviceState(): HighlightDeviceState {
        val connectivity = context.getSystemService(ConnectivityManager::class.java)
        val network = connectivity?.activeNetwork
        val capabilities = network?.let { activeNetwork -> connectivity?.getNetworkCapabilities(activeNetwork) }
        val onWifi = capabilities?.hasTransport(NetworkCapabilities.TRANSPORT_WIFI) == true

        val batteryIntent = context.registerReceiver(null, IntentFilter(Intent.ACTION_BATTERY_CHANGED))
        val status = batteryIntent?.getIntExtra(BatteryManager.EXTRA_STATUS, -1) ?: -1
        val charging = status == BatteryManager.BATTERY_STATUS_CHARGING || status == BatteryManager.BATTERY_STATUS_FULL
        val level = batteryIntent?.getIntExtra(BatteryManager.EXTRA_LEVEL, -1) ?: -1
        val scale = batteryIntent?.getIntExtra(BatteryManager.EXTRA_SCALE, -1) ?: -1
        val batteryPercent = if (level >= 0 && scale > 0) (level * 100 / scale).coerceIn(0, 100) else 100

        val power = context.getSystemService(PowerManager::class.java)
        val powerSave = power?.isPowerSaveMode == true
        val thermalSevere = Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q &&
            power != null && power.currentThermalStatus >= PowerManager.THERMAL_STATUS_SEVERE
        val enoughPower = charging || batteryPercent >= HIGHLIGHT_MIN_BATTERY_PERCENT

        return HighlightDeviceState(
            eligible = onWifi && enoughPower && !powerSave && !thermalSevere,
            network = if (onWifi) "wifi" else "other",
            charging = charging,
            batteryPercent = batteryPercent,
        )
    }

    private fun androidWorkerName(): String = listOf(Build.MANUFACTURER, Build.MODEL)
        .map(String::trim)
        .filter(String::isNotBlank)
        .joinToString(" ")
        .ifBlank { "Android" }
}
