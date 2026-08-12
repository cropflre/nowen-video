package com.nowen.video.v2.core.model

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class OfflineDownloadModelsTest {
    @Test
    fun progressIsClampedToAvailableRange() {
        val record = sampleRecord(downloaded = 150L, total = 100L)
        assertEquals(1f, record.progress)
    }

    @Test
    fun zeroLengthDownloadHasZeroProgress() {
        val record = sampleRecord(downloaded = 50L, total = 0L)
        assertEquals(0f, record.progress)
    }

    @Test
    fun policyNormalizesUnsafeQuota() {
        val tooSmall = OfflineDownloadPolicy(maxBytes = 1L).normalized()
        val tooLarge = OfflineDownloadPolicy(maxBytes = Long.MAX_VALUE).normalized()
        assertEquals(2L * 1024L * 1024L * 1024L, tooSmall.maxBytes)
        assertEquals(200L * 1024L * 1024L * 1024L, tooLarge.maxBytes)
    }

    @Test
    fun storageStatsIncludePartialFiles() {
        val stats = OfflineStorageStats(
            completedBytes = 100L,
            partialBytes = 50L,
            quotaBytes = 200L,
        )
        assertEquals(150L, stats.usedBytes)
        assertEquals(50L, stats.remainingQuotaBytes)
        assertTrue(stats.quotaProgress in 0.74f..0.76f)
    }

    private fun sampleRecord(downloaded: Long, total: Long) = OfflineDownloadRecord(
        id = "download",
        serverId = "server",
        userId = "user",
        mediaId = "media",
        title = "Movie",
        sourceUrl = "https://example.test/movie.mp4",
        fileName = "movie.mp4",
        partialPath = "/tmp/movie.mp4.part",
        localPath = "/tmp/movie.mp4",
        totalBytes = total,
        downloadedBytes = downloaded,
        createdAtEpochMs = 1L,
        updatedAtEpochMs = 1L,
    )
}
