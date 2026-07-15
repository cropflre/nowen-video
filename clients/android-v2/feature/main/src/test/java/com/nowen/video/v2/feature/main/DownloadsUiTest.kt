package com.nowen.video.v2.feature.main

import com.nowen.video.v2.core.model.OfflineDownloadStatus
import org.junit.Assert.assertEquals
import org.junit.Test

class DownloadsUiTest {
    @Test
    fun formatsStorageUnits() {
        assertEquals("0 B", formatBytes(0L))
        assertEquals("1.0 KB", formatBytes(1024L))
        assertEquals("1.0 MB", formatBytes(1024L * 1024L))
        assertEquals("1.0 GB", formatBytes(1024L * 1024L * 1024L))
    }

    @Test
    fun pausedDownloadExplainsResumeBehavior() {
        assertEquals("已暂停，可断点继续", downloadStatusLabel(OfflineDownloadStatus.Paused))
        assertEquals("继续下载", downloadActionLabel(OfflineDownloadStatus.Paused))
    }

    @Test
    fun completedDownloadPointsToOfflinePlayback() {
        assertEquals("已下载到本机", downloadStatusLabel(OfflineDownloadStatus.Completed))
        assertEquals("已下载，可在下载页离线播放", downloadActionLabel(OfflineDownloadStatus.Completed))
    }
}
