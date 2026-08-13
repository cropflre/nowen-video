package com.nowen.video.v2.core.data

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class ProgressContractTest {
    @Test
    fun `normalizes valid playback progress`() {
        val progress = normalizeProgress(position = 42.5, duration = 120.0)
        requireNotNull(progress)
        assertEquals(42.5, progress.position, 0.0001)
        assertEquals(120.0, progress.duration, 0.0001)
    }

    @Test
    fun `rejects progress without a playable duration`() {
        assertNull(normalizeProgress(position = 10.0, duration = 0.0))
        assertNull(normalizeProgress(position = 0.0, duration = 120.0))
        assertNull(normalizeProgress(position = Double.NaN, duration = 120.0))
    }

    @Test
    fun `clamps progress beyond media duration`() {
        val progress = normalizeProgress(position = 200.0, duration = 120.0)
        requireNotNull(progress)
        assertEquals(120.0, progress.position, 0.0001)
    }

    @Test
    fun `completed media restarts instead of resuming at credits`() {
        assertEquals(0.0, effectiveResumePosition(position = 95.0, duration = 100.0), 0.0001)
        assertEquals(0.0, effectiveResumePosition(position = 40.0, duration = 100.0, completed = true), 0.0001)
        assertEquals(40.0, effectiveResumePosition(position = 40.0, duration = 100.0), 0.0001)
    }
}