package com.nowen.video.v2.core.model

import org.junit.Assert.assertEquals
import org.junit.Test

class SocialCatalogModelsTest {
    @Test
    fun `history progress is clamped and labelled`() {
        val history = WatchHistoryRecord(position = 75.0, duration = 100.0)
        assertEquals(0.75f, history.normalizedProgress)
        assertEquals("已观看 75%", history.progressLabel)

        val overrun = WatchHistoryRecord(position = 130.0, duration = 100.0)
        assertEquals(1f, overrun.normalizedProgress)
    }

    @Test
    fun `completed history uses completed label`() {
        assertEquals("已看完", WatchHistoryRecord(completed = true).progressLabel)
    }

    @Test
    fun `credit role labels prefer character for actors`() {
        assertEquals("导演", MediaPerson(role = "director").roleLabel)
        assertEquals("饰 Neo", MediaPerson(role = "actor", character = "Neo").roleLabel)
        assertEquals("演员", MediaPerson(role = "actor").roleLabel)
    }
}
