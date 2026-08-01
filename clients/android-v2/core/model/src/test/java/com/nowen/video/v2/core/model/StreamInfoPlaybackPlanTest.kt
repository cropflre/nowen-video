package com.nowen.video.v2.core.model

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class StreamInfoPlaybackPlanTest {
    @Test
    fun plannedUrlTakesPriorityOverLegacyFields() {
        val stream = StreamInfo(
            canDirectPlay = true,
            directPlayUrl = "/api/stream/media/direct",
            hlsUrl = "/api/stream/media/master.m3u8",
            playbackPlan = PlaybackPlan(
                mediaId = "media",
                method = "remux",
                url = "/api/stream/media/remux",
                reasonCode = "container_remux",
                reason = "编码兼容，仅需转换容器",
                fallbackMethod = "transcode",
                fallbackUrl = "/api/stream/media/master.m3u8",
            ),
        )

        assertEquals("/api/stream/media/remux", stream.preferredUrl)
        assertEquals("/api/stream/media/master.m3u8", stream.fallbackUrl)
        assertEquals("remux", stream.playbackMethod)
        assertEquals("无损封装转换", stream.playbackMethodLabel)
        assertEquals("container_remux", stream.playbackReasonCode)
    }

    @Test
    fun startupStreamPlanUsesBridgeAndKeepsRuntimeFallback() {
        val bridge = "/api/stream/media/startup-720p/stream.m3u8"
        val fallback = "/api/stream/media/master.m3u8"
        val stream = StreamInfo(
            hlsUrl = fallback,
            playbackPlan = PlaybackPlan(
                mediaId = "media",
                method = "startup_stream",
                url = bridge,
                reasonCode = "startup_artifact_ready",
                reason = "已命中预生成启动流",
                requiresTranscode = true,
                fallbackMethod = "transcode",
                fallbackUrl = fallback,
                startupStream = PlaybackStartupStream(
                    profileId = "720p",
                    durationMs = 30_000,
                    playlistUrl = bridge,
                    continuationMode = "event_bridge_v1",
                    discontinuityAtHandoff = true,
                ),
            ),
        )

        assertEquals(bridge, stream.preferredUrl)
        assertEquals(fallback, stream.fallbackUrl)
        assertEquals("startup_stream", stream.playbackMethod)
        assertEquals("启动流秒开", stream.playbackMethodLabel)
        assertTrue(stream.playbackPlan?.startupStream?.discontinuityAtHandoff == true)
    }

    @Test
    fun blankPlanUrlFallsBackToLegacySelection() {
        val stream = StreamInfo(
            canDirectPlay = true,
            directPlayUrl = "/api/stream/media/direct",
            hlsUrl = "/api/stream/media/master.m3u8",
            playbackPlan = PlaybackPlan(method = "direct"),
        )

        assertEquals("/api/stream/media/direct", stream.preferredUrl)
    }

    @Test
    fun oldServerResponseKeepsLegacyBehavior() {
        val stream = StreamInfo(
            canRemux = true,
            remuxUrl = "/api/stream/media/remux",
            hlsUrl = "/api/stream/media/master.m3u8",
        )

        assertEquals("/api/stream/media/remux", stream.preferredUrl)
        assertEquals("remux", stream.playbackMethod)
        assertEquals("legacy_stream_info", stream.playbackReasonCode)
    }
}
