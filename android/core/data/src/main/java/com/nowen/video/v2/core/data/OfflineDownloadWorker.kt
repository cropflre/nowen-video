package com.nowen.video.v2.core.data

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Context
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.SystemClock
import androidx.core.app.NotificationCompat
import androidx.work.CoroutineWorker
import androidx.work.ForegroundInfo
import androidx.work.WorkerParameters
import androidx.work.workDataOf
import com.nowen.video.v2.core.model.OfflineDownloadRecord
import com.nowen.video.v2.core.model.OfflineDownloadStatus
import java.io.File
import java.io.FileOutputStream
import java.io.IOException
import java.util.concurrent.TimeUnit
import kotlinx.coroutines.CancellationException
import kotlinx.serialization.json.Json
import okhttp3.OkHttpClient
import okhttp3.Request

private const val DOWNLOAD_NOTIFICATION_CHANNEL = "nowen_v2_offline_downloads"
private const val DOWNLOAD_PROGRESS_BYTES = "downloaded_bytes"
private const val DOWNLOAD_TOTAL_BYTES = "total_bytes"
private const val PROGRESS_UPDATE_INTERVAL_MS = 900L
private const val MIN_DEVICE_FREE_BYTES = 64L * 1024L * 1024L
private const val MAX_RETRY_ATTEMPTS = 4

class OfflineDownloadWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {
    private val json = offlineWorkerJson()
    private val store = OfflineDownloadStore(appContext, json)
    private val vault = CredentialVault(appContext)
    private val client = OkHttpClient.Builder()
        .connectTimeout(20, TimeUnit.SECONDS)
        .readTimeout(45, TimeUnit.SECONDS)
        .writeTimeout(45, TimeUnit.SECONDS)
        .retryOnConnectionFailure(true)
        .build()

    override suspend fun doWork(): Result {
        val id = inputData.getString(OFFLINE_DOWNLOAD_ID_KEY) ?: return Result.failure()
        val record = store.find(id) ?: return Result.success()
        if (record.status == OfflineDownloadStatus.Paused) return Result.success()
        if (record.status == OfflineDownloadStatus.Completed && File(record.localPath).isFile) {
            return Result.success()
        }

        return try {
            setForeground(downloadForegroundInfo(record, record.progress))
            download(record)
            Result.success()
        } catch (cancelled: CancellationException) {
            val latest = store.find(id)
            if (latest?.status == OfflineDownloadStatus.Downloading) {
                store.update(id) {
                    it.copy(
                        status = OfflineDownloadStatus.Queued,
                        downloadedBytes = File(it.partialPath).length(),
                        updatedAtEpochMs = System.currentTimeMillis(),
                    )
                }
            }
            throw cancelled
        } catch (error: IOException) {
            val retry = runAttemptCount < MAX_RETRY_ATTEMPTS
            store.update(id) {
                it.copy(
                    status = if (retry) OfflineDownloadStatus.Queued else OfflineDownloadStatus.Failed,
                    downloadedBytes = File(it.partialPath).length(),
                    error = error.message ?: "下载网络错误",
                    updatedAtEpochMs = System.currentTimeMillis(),
                )
            }
            if (retry) Result.retry() else Result.failure()
        } catch (error: Throwable) {
            store.update(id) {
                it.copy(
                    status = OfflineDownloadStatus.Failed,
                    downloadedBytes = File(it.partialPath).length(),
                    error = error.message ?: "下载失败",
                    updatedAtEpochMs = System.currentTimeMillis(),
                )
            }
            Result.failure()
        }
    }

    private suspend fun download(initialRecord: OfflineDownloadRecord) {
        val token = vault.readToken(initialRecord.serverId)
            ?: throw IllegalStateException("登录凭据已失效，请重新登录后继续下载")
        val partFile = File(initialRecord.partialPath).apply { parentFile?.mkdirs() }
        val finalFile = File(initialRecord.localPath).apply { parentFile?.mkdirs() }
        var existingBytes = partFile.takeIf(File::isFile)?.length() ?: 0L

        val request = Request.Builder()
            .url(initialRecord.sourceUrl)
            .header("Accept-Encoding", "identity")
            .header("Authorization", "Bearer $token")
            .apply {
                if (existingBytes > 0L) header("Range", "bytes=$existingBytes-")
            }
            .build()

        client.newCall(request).execute().use { response ->
            if (response.code == 416 && existingBytes > 0L) {
                val knownTotal = parseContentRangeTotal(response.header("Content-Range"))
                    ?: initialRecord.totalBytes.takeIf { it > 0L }
                if (knownTotal != null && existingBytes >= knownTotal) {
                    completeDownload(initialRecord, partFile, finalFile, knownTotal)
                    return
                }
            }
            if (!response.isSuccessful) {
                throw IOException("下载请求失败：HTTP ${response.code}")
            }
            val body = response.body ?: throw IOException("服务器未返回下载内容")
            val append = existingBytes > 0L && response.code == 206
            if (existingBytes > 0L && !append) {
                partFile.delete()
                existingBytes = 0L
            }

            val totalBytes = parseContentRangeTotal(response.header("Content-Range"))
                ?: body.contentLength().takeIf { it >= 0L }?.plus(existingBytes)
                ?: initialRecord.totalBytes
            val otherUsage = store.recordsNow()
                .asSequence()
                .filterNot { it.id == initialRecord.id }
                .sumOf(::recordDiskUsage)
            val policy = store.policyNow()
            if (totalBytes > 0L && otherUsage + totalBytes > policy.maxBytes) {
                throw IllegalStateException("该视频会超过离线空间上限，请提高上限或清理旧下载")
            }

            var downloaded = existingBytes
            var lastPersistedAt = 0L
            store.update(initialRecord.id) {
                it.copy(
                    status = OfflineDownloadStatus.Downloading,
                    totalBytes = totalBytes.coerceAtLeast(0L),
                    downloadedBytes = downloaded,
                    error = "",
                    updatedAtEpochMs = System.currentTimeMillis(),
                )
            }

            body.byteStream().use { input ->
                FileOutputStream(partFile, append).buffered().use { output ->
                    val buffer = ByteArray(DEFAULT_BUFFER_SIZE * 4)
                    while (true) {
                        if (isStopped) throw CancellationException("下载任务已停止")
                        val count = input.read(buffer)
                        if (count < 0) break
                        val projected = otherUsage + downloaded + count
                        if (projected > policy.maxBytes) {
                            throw IllegalStateException("已达到离线空间上限，下载已暂停在当前进度")
                        }
                        if (offlineRoot(applicationContext).usableSpace < MIN_DEVICE_FREE_BYTES + count) {
                            throw IllegalStateException("设备存储空间不足，下载已暂停在当前进度")
                        }
                        output.write(buffer, 0, count)
                        downloaded += count

                        val now = SystemClock.elapsedRealtime()
                        if (now - lastPersistedAt >= PROGRESS_UPDATE_INTERVAL_MS) {
                            persistProgress(initialRecord.id, downloaded, totalBytes)
                            setProgress(
                                workDataOf(
                                    DOWNLOAD_PROGRESS_BYTES to downloaded,
                                    DOWNLOAD_TOTAL_BYTES to totalBytes,
                                ),
                            )
                            val latest = store.find(initialRecord.id) ?: initialRecord
                            setForeground(
                                downloadForegroundInfo(
                                    latest.copy(downloadedBytes = downloaded, totalBytes = totalBytes),
                                    if (totalBytes > 0L) downloaded.toFloat() / totalBytes.toFloat() else 0f,
                                ),
                            )
                            lastPersistedAt = now
                        }
                    }
                    output.flush()
                }
            }
            persistProgress(initialRecord.id, downloaded, totalBytes)
            if (totalBytes > 0L && downloaded < totalBytes) {
                throw IOException("下载提前结束：已接收 $downloaded / $totalBytes 字节")
            }
            completeDownload(initialRecord, partFile, finalFile, downloaded)
        }
    }

    private suspend fun persistProgress(id: String, downloaded: Long, total: Long) {
        store.update(id) {
            it.copy(
                status = OfflineDownloadStatus.Downloading,
                downloadedBytes = downloaded,
                totalBytes = total.coerceAtLeast(0L),
                error = "",
                updatedAtEpochMs = System.currentTimeMillis(),
            )
        }
    }

    private suspend fun completeDownload(
        record: OfflineDownloadRecord,
        partFile: File,
        finalFile: File,
        completedBytes: Long,
    ) {
        if (finalFile.exists() && !finalFile.delete()) {
            throw IOException("无法替换旧的离线文件")
        }
        if (!partFile.renameTo(finalFile)) {
            partFile.copyTo(finalFile, overwrite = true)
            if (!partFile.delete()) partFile.deleteOnExit()
        }
        store.update(record.id) {
            it.copy(
                status = OfflineDownloadStatus.Completed,
                downloadedBytes = completedBytes,
                totalBytes = completedBytes,
                error = "",
                updatedAtEpochMs = System.currentTimeMillis(),
            )
        }
    }

    private fun downloadForegroundInfo(record: OfflineDownloadRecord, progress: Float): ForegroundInfo {
        ensureDownloadChannel(applicationContext)
        val percent = (progress.coerceIn(0f, 1f) * 100).toInt()
        val notification = NotificationCompat.Builder(applicationContext, DOWNLOAD_NOTIFICATION_CHANNEL)
            .setSmallIcon(android.R.drawable.stat_sys_download)
            .setContentTitle(record.title)
            .setContentText(if (record.totalBytes > 0L) "离线下载 $percent%" else "正在准备离线下载")
            .setOnlyAlertOnce(true)
            .setOngoing(true)
            .setProgress(100, percent, record.totalBytes <= 0L)
            .setCategory(NotificationCompat.CATEGORY_PROGRESS)
            .build()
        val notificationId = record.id.hashCode() and Int.MAX_VALUE
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            ForegroundInfo(notificationId, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
        } else {
            ForegroundInfo(notificationId, notification)
        }
    }
}

class OfflineDownloadRecoveryWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {
    override suspend fun doWork(): Result {
        val store = OfflineDownloadStore(applicationContext, offlineWorkerJson())
        val vault = CredentialVault(applicationContext)
        val policy = store.policyNow()
        store.recordsNow().forEach { record ->
            when (record.status) {
                OfflineDownloadStatus.Queued,
                OfflineDownloadStatus.Downloading,
                -> {
                    if (vault.readToken(record.serverId) != null) {
                        val queued = store.update(record.id) {
                            it.copy(
                                status = OfflineDownloadStatus.Queued,
                                downloadedBytes = File(it.partialPath).length(),
                                updatedAtEpochMs = System.currentTimeMillis(),
                            )
                        } ?: return@forEach
                        enqueueOfflineWork(applicationContext, queued, policy)
                    }
                }
                OfflineDownloadStatus.Completed -> {
                    if (!File(record.localPath).isFile) {
                        store.update(record.id) {
                            it.copy(
                                status = OfflineDownloadStatus.Failed,
                                error = "离线文件已丢失，请重新下载",
                                updatedAtEpochMs = System.currentTimeMillis(),
                            )
                        }
                    }
                }
                OfflineDownloadStatus.Paused,
                OfflineDownloadStatus.Failed,
                -> Unit
            }
        }
        return Result.success()
    }
}

class OfflineDownloadMaintenanceWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {
    override suspend fun doWork(): Result {
        val store = OfflineDownloadStore(applicationContext, offlineWorkerJson())
        val records = store.recordsNow()
        val knownFiles = records.flatMap { listOf(it.partialPath, it.localPath) }.toSet()
        val staleBefore = System.currentTimeMillis() - TimeUnit.DAYS.toMillis(14)

        records.forEach { record ->
            if (record.status == OfflineDownloadStatus.Completed && !File(record.localPath).isFile) {
                store.update(record.id) {
                    it.copy(
                        status = OfflineDownloadStatus.Failed,
                        error = "离线文件已被系统或用户清理",
                        updatedAtEpochMs = System.currentTimeMillis(),
                    )
                }
            }
            if (record.status == OfflineDownloadStatus.Failed && record.updatedAtEpochMs < staleBefore) {
                File(record.partialPath).delete()
                store.update(record.id) {
                    it.copy(downloadedBytes = 0L, updatedAtEpochMs = System.currentTimeMillis())
                }
            }
        }

        offlineRoot(applicationContext)
            .walkTopDown()
            .filter(File::isFile)
            .filter { it.absolutePath !in knownFiles && it.lastModified() < staleBefore }
            .forEach(File::delete)
        return Result.success()
    }
}

private fun recordDiskUsage(record: OfflineDownloadRecord): Long =
    (File(record.localPath).takeIf(File::isFile)?.length() ?: 0L) +
        (File(record.partialPath).takeIf(File::isFile)?.length() ?: 0L)

private fun ensureDownloadChannel(context: Context) {
    if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
    val manager = context.getSystemService(Service.NOTIFICATION_SERVICE) as NotificationManager
    manager.createNotificationChannel(
        NotificationChannel(
            DOWNLOAD_NOTIFICATION_CHANNEL,
            "离线下载",
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = "显示 Nowen Video 离线视频的下载进度"
            setShowBadge(false)
        },
    )
}

private fun offlineWorkerJson(): Json = Json {
    ignoreUnknownKeys = true
    coerceInputValues = true
    explicitNulls = false
    encodeDefaults = true
}
