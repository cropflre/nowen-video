package com.nowen.video.v2.core.model

import org.junit.Assert.assertEquals
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
