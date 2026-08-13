package com.nowen.video.v2.core.data

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PlaybackCapabilitiesTest {
    @Test
    fun matchesHevcMimeTypeCaseInsensitively() {
        assertTrue(codecSupportsHevc(arrayOf("video/avc", "VIDEO/HEVC")))
    }

    @Test
    fun rejectsCodecListsWithoutHevc() {
        assertFalse(codecSupportsHevc(arrayOf("video/avc", "video/av01")))
    }
}
