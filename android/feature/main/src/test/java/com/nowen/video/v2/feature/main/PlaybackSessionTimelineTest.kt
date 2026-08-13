package com.nowen.video.v2.feature.main

import org.junit.Assert.assertEquals
import org.junit.Test

class PlaybackSessionTimelineTest {
    @Test
    fun generationRelativePositionMapsToAbsoluteMediaTimeline() {
        assertEquals(
            3_615_000L,
            absolutePlaybackPositionMs(
                sessionManaged = true,
                generationOffsetMs = 3_600_000L,
                relativePositionMs = 15_000L,
            ),
        )
    }

    @Test
    fun legacyPlaybackKeepsOriginalPosition() {
        assertEquals(
            15_000L,
            absolutePlaybackPositionMs(
                sessionManaged = false,
                generationOffsetMs = 3_600_000L,
                relativePositionMs = 15_000L,
            ),
        )
    }

    @Test
    fun absoluteTargetConvertsBackToGenerationRelativePosition() {
        assertEquals(
            25_000L,
            relativePlaybackPositionMs(
                sessionManaged = true,
                generationOffsetMs = 3_600_000L,
                absolutePositionMs = 3_625_000L,
            ),
        )
    }

    @Test
    fun targetsAreClampedToMediaDuration() {
        assertEquals(7_200_000L, clampPlaybackTargetMs(9_000_000L, 7_200_000L))
        assertEquals(0L, clampPlaybackTargetMs(-1_000L, 7_200_000L))
    }
}
