package com.nowen.video.v2.core.model

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class DanmakuModelsTest {
    @Test
    fun `p and m become cue in milliseconds`() {
        val cue = DanmakuComment(p = "12.345,1,25", m = " hello ", cid = 9L).toCue(0)
        assertEquals(12_345L, cue?.timeMs)
        assertEquals("hello", cue?.text)
        assertEquals("9", cue?.id)
    }

    @Test
    fun `t and content aliases are accepted`() {
        val cue = DanmakuComment(t = "3.5", content = "content").toCue(2)
        assertEquals(3_500L, cue?.timeMs)
        assertEquals("content", cue?.text)
    }

    @Test
    fun `blank and invalid comments are ignored`() {
        assertNull(DanmakuComment(p = "1", m = " ").toCue(0))
        assertNull(DanmakuComment(p = "-1", m = "bad").toCue(0))
        assertNull(DanmakuComment(p = "NaN", m = "bad").toCue(0))
    }

    @Test
    fun `shift is applied only to effective display time`() {
        val cue = DanmakuComment(p = "1", m = "shifted").toCue(0, shiftMs = -2_000L)
        assertEquals(1_000L, cue?.effectiveTimeMs)
    }
}
