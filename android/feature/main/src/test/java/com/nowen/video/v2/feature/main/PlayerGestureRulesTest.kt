package com.nowen.video.v2.feature.main

import org.junit.Assert.assertEquals
import org.junit.Test

class PlayerGestureRulesTest {
    @Test
    fun `double tap seeks backward on left and forward on right`() {
        assertEquals(-10_000L, playerDoubleTapSeekDeltaMs(20f, 200f))
        assertEquals(10_000L, playerDoubleTapSeekDeltaMs(180f, 200f))
        assertEquals(10_000L, playerDoubleTapSeekDeltaMs(100f, 200f))
    }

    @Test
    fun `long press boost never reduces existing permanent speed`() {
        assertEquals(2f, temporaryBoostSpeed(1f))
        assertEquals(2f, temporaryBoostSpeed(1.75f))
        assertEquals(3f, temporaryBoostSpeed(3f))
        assertEquals(8f, temporaryBoostSpeed(8f))
    }
}
