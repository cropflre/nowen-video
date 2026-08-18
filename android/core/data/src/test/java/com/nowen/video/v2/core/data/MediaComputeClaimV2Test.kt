package com.nowen.video.v2.core.data

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlinx.serialization.json.Json

class MediaComputeClaimV2Test {
    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    @Test
    fun `v2 highlight claim decodes generic envelope and nested input`() {
        val claim = json.decodeFromString<HighlightWorkerClaim>(
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
        assertEquals("media-1", claim.input?.mediaId)
        assertEquals(listOf(120.0, 360.0), claim.input?.sampleTimes)
    }

    @Test
    fun `v1 flattened claim remains decodable`() {
        val claim = json.decodeFromString<HighlightWorkerClaim>(
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
