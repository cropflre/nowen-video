package com.nowen.video.v2.core.data

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.decodeFromJsonElement

class MediaComputeClaimV2Test {
    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    @Test
    fun `v2 claim decodes generic envelope before highlight adapter parses input`() {
        val claim = json.decodeFromString<MediaComputeTaskClaim>(
            """
            {
              "protocol_version": 2,
              "job_type": "highlight_v1",
              "required_capability": "highlight_v1",
              "task_id": "task-1",
              "claim_token": "claim-1",
              "input": {
                "media_id": "media-1",
                "fingerprint": "fp-1",
                "duration": 3600,
                "stream_url": "/api/stream/media-1/direct",
                "sample_times": [120, 360],
                "max_highlights": 8,
                "engine_version": 3
              },
              "media_id": "media-1",
              "fingerprint": "fp-1",
              "duration": 3600,
              "stream_url": "/api/stream/media-1/direct",
              "sample_times": [120, 360],
              "max_highlights": 8,
              "engine_version": 3
            }
            """.trimIndent(),
        )

        assertEquals(2, claim.protocolVersion)
        assertEquals("highlight_v1", claim.jobType)
        assertEquals("highlight_v1", claim.requiredCapability)
        val input = json.decodeFromJsonElement<MediaComputeHighlightInput>(assertNotNull(claim.input))
        assertEquals("media-1", input.mediaId)
        assertEquals(listOf(120.0, 360.0), input.sampleTimes)
    }

    @Test
    fun `v2 preview claim decodes second real media compute job`() {
        val claim = json.decodeFromString<MediaComputeTaskClaim>(
            """
            {
              "protocol_version": 2,
              "job_type": "preview_thumbnail_v1",
              "required_capability": "preview_thumbnail_v1",
              "task_id": "preview-task",
              "claim_token": "preview-claim",
              "input": {
                "media_id": "media-1",
                "highlight_id": "highlight-1",
                "fingerprint": "fp-1",
                "stream_url": "/api/stream/media-1/direct",
                "frame_times": [100.25, 100.75, 101.25, 101.75, 102.25],
                "max_width": 420,
                "frame_rate": 2
              }
            }
            """.trimIndent(),
        )

        assertEquals("preview_thumbnail_v1", claim.jobType)
        assertEquals("preview_thumbnail_v1", claim.requiredCapability)
        val input = json.decodeFromJsonElement<MediaComputePreviewThumbnailInput>(assertNotNull(claim.input))
        assertEquals("highlight-1", input.highlightId)
        assertEquals(5, input.frameTimes.size)
        assertEquals(420, input.maxWidth)
        assertEquals(2, input.frameRate)
    }

    @Test
    fun `v1 flattened claim remains decodable`() {
        val claim = json.decodeFromString<MediaComputeTaskClaim>(
            """
            {
              "task_id": "task-legacy",
              "claim_token": "claim-legacy",
              "media_id": "media-legacy",
              "fingerprint": "fp-legacy",
              "duration": 1200,
              "stream_url": "/api/stream/media-legacy/direct",
              "sample_times": [60],
              "max_highlights": 4,
              "engine_version": 3
            }
            """.trimIndent(),
        )

        assertEquals(1, claim.protocolVersion)
        assertEquals("highlight_v1", claim.jobType)
        assertNull(claim.input)
        assertEquals("media-legacy", claim.mediaId)
        assertEquals(listOf(60.0), claim.sampleTimes)
    }
}
