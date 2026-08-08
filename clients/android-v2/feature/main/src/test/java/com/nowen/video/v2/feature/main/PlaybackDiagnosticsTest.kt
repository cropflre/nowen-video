package com.nowen.video.v2.feature.main

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PlaybackDiagnosticsTest {
    @Test
    fun `fallback is allowed for io parsing and decoder failures`() {
        assertTrue(shouldAttemptPlaybackFallback(2_001, "direct", "hls", false))
        assertTrue(shouldAttemptPlaybackFallback(3_001, "direct", "hls", false))
        assertTrue(shouldAttemptPlaybackFallback(4_001, "remux", "hls", false))
    }

    @Test
    fun `fallback is attempted only once and requires a distinct url`() {
        assertFalse(shouldAttemptPlaybackFallback(2_001, "direct", "hls", true))
        assertFalse(shouldAttemptPlaybackFallback(2_001, "direct", "", false))
        assertFalse(shouldAttemptPlaybackFallback(2_001, "same", "same", false))
    }

    @Test
    fun `fallback ignores unrelated error groups`() {
        assertFalse(shouldAttemptPlaybackFallback(1_000, "direct", "hls", false))
        assertFalse(shouldAttemptPlaybackFallback(6_001, "direct", "hls", false))
    }

    @Test
    fun `playback methods have stable diagnostic labels`() {
        assertEquals("直接播放", playbackMethodLabel("direct"))
        assertEquals("无损封装转换", playbackMethodLabel("REMUX"))
        assertEquals("视频直通·音频兼容转换", playbackMethodLabel("smart_remux"))
        assertEquals("兼容转码", playbackMethodLabel("transcode"))
        assertEquals("自动选择", playbackMethodLabel("unknown"))
    }
}
