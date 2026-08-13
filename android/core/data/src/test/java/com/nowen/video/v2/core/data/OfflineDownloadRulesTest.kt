package com.nowen.video.v2.core.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class OfflineDownloadRulesTest {
    @Test
    fun relativeStreamPathUsesActiveServer() {
        assertEquals(
            "https://video.example/api/stream/1",
            resolveOfflineSource("https://video.example/", "/api/stream/1"),
        )
    }

    @Test
    fun absoluteStreamUrlIsPreserved() {
        assertEquals(
            "https://cdn.example/movie.mp4",
            resolveOfflineSource("https://video.example", "https://cdn.example/movie.mp4"),
        )
    }

    @Test
    fun hlsSourcesAreRejectedForRangeDownload() {
        assertTrue(isHlsDownload("https://example.test/video.m3u8", "application/octet-stream"))
        assertTrue(isHlsDownload("https://example.test/stream", "application/vnd.apple.mpegurl"))
        assertFalse(isHlsDownload("https://example.test/video.mp4", "video/mp4"))
    }

    @Test
    fun extensionPrefersMimeThenUrl() {
        assertEquals("mkv", inferOfflineExtension("https://example.test/file.bin", "video/x-matroska"))
        assertEquals("webm", inferOfflineExtension("https://example.test/movie.webm?token=1", ""))
        assertEquals("mp4", inferOfflineExtension("https://example.test/stream", ""))
    }

    @Test
    fun contentRangeTotalParsesKnownAndUnknownSizes() {
        assertEquals(1_000L, parseContentRangeTotal("bytes 200-999/1000"))
        assertEquals(1_000L, parseContentRangeTotal("bytes */1000"))
        assertNull(parseContentRangeTotal("bytes 0-1/*"))
        assertNull(parseContentRangeTotal(null))
    }
}
