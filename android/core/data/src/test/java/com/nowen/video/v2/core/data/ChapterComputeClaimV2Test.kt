package com.nowen.video.v2.core.data

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotNull
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.decodeFromJsonElement

class ChapterComputeClaimV2Test {
    private val json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
    }

    @Test
    fun `v2 chapter claim decodes distributed candidate plan`() {
        val claim = json.decodeFromString<MediaComputeTaskClaim>(
            """
            {
              "protocol_version": 2,
              "job_type": "chapter_detect_v1",
              "required_capability": "chapter_detect_v1",
              "task_id": "chapter-task",
              "claim_token": "chapter-claim",
              "input": {
                "media_id": "media-1",
                "fingerprint": "fp-1",
                "duration": 3600,
                "stream_url": "/api/stream/media-1/direct",
                "sample_times": [300, 600, 900],
                "probe_gap_seconds": 3,
                "min_chapter_seconds": 120,
                "max_chapters": 8,
                "capture_width": 240,
                "engine_version": 1
              }
            }
            """.trimIndent(),
        )

        assertEquals(MEDIA_COMPUTE_JOB_CHAPTER_DETECT_V1, claim.jobType)
        assertEquals(MEDIA_COMPUTE_CAPABILITY_CHAPTER_DETECT_V1, claim.requiredCapability)
        val input = json.decodeFromJsonElement<MediaComputeChapterDetectInput>(assertNotNull(claim.input))
        assertEquals("media-1", input.mediaId)
        assertEquals("fp-1", input.fingerprint)
        assertEquals(listOf(300.0, 600.0, 900.0), input.sampleTimes)
        assertEquals(3.0, input.probeGapSeconds)
        assertEquals(1, input.engineVersion)
    }
}
