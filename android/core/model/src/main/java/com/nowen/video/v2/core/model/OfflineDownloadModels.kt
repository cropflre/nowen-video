package com.nowen.video.v2.core.model

import kotlinx.serialization.Serializable

const val DEFAULT_OFFLINE_QUOTA_BYTES: Long = 20L * 1024L * 1024L * 1024L

@Serializable
enum class OfflineDownloadStatus {
    Queued,
    Downloading,
    Paused,
    Completed,
    Failed,
}

@Serializable
data class OfflineDownloadRecord(
    val id: String,
    val serverId: String,
    val userId: String,
    val mediaId: String,
    val title: String,
    val posterPath: String = "",
    val sourceUrl: String,
    val mimeType: String = "",
    val durationSeconds: Double = 0.0,
    val fileName: String,
    val partialPath: String,
    val localPath: String,
    val status: OfflineDownloadStatus = OfflineDownloadStatus.Queued,
    val totalBytes: Long = 0L,
    val downloadedBytes: Long = 0L,
    val error: String = "",
    val createdAtEpochMs: Long,
    val updatedAtEpochMs: Long,
) {
    val progress: Float
        get() = if (totalBytes > 0L) {
            (downloadedBytes.toDouble() / totalBytes.toDouble()).toFloat().coerceIn(0f, 1f)
        } else {
            0f
        }

    val isActive: Boolean
        get() = status == OfflineDownloadStatus.Queued || status == OfflineDownloadStatus.Downloading
}

@Serializable
data class OfflineDownloadPolicy(
    val wifiOnly: Boolean = true,
    val maxBytes: Long = DEFAULT_OFFLINE_QUOTA_BYTES,
) {
    fun normalized(): OfflineDownloadPolicy = copy(
        maxBytes = maxBytes.coerceIn(
            2L * 1024L * 1024L * 1024L,
            200L * 1024L * 1024L * 1024L,
        ),
    )
}

data class OfflineStorageStats(
    val completedBytes: Long = 0L,
    val partialBytes: Long = 0L,
    val quotaBytes: Long = DEFAULT_OFFLINE_QUOTA_BYTES,
    val deviceFreeBytes: Long = 0L,
) {
    val usedBytes: Long get() = completedBytes + partialBytes
    val remainingQuotaBytes: Long get() = (quotaBytes - usedBytes).coerceAtLeast(0L)
    val quotaProgress: Float
        get() = if (quotaBytes > 0L) {
            (usedBytes.toDouble() / quotaBytes.toDouble()).toFloat().coerceIn(0f, 1f)
        } else {
            0f
        }
}
