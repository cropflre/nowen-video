package com.nowen.video.v2.feature.main

import androidx.media3.common.MimeTypes
import androidx.media3.ui.AspectRatioFrameLayout
import org.junit.Assert.assertEquals
import org.junit.Test

class PlayerSettingsTest {
    @Test
    fun `maps common subtitle formats for Media3`() {
        assertEquals(MimeTypes.TEXT_VTT, subtitleMimeType("vtt"))
        assertEquals(MimeTypes.TEXT_SSA, subtitleMimeType("ass"))
        assertEquals(MimeTypes.APPLICATION_SUBRIP, subtitleMimeType("", "/media/movie.zh-CN.srt"))
        assertEquals(MimeTypes.APPLICATION_TTML, subtitleMimeType("ttml"))
    }

    @Test
    fun `maps persisted resize modes to PlayerView`() {
        assertEquals(AspectRatioFrameLayout.RESIZE_MODE_FIT, resizeModeForPreference(0))
        assertEquals(AspectRatioFrameLayout.RESIZE_MODE_ZOOM, resizeModeForPreference(1))
        assertEquals(AspectRatioFrameLayout.RESIZE_MODE_FILL, resizeModeForPreference(2))
        assertEquals(AspectRatioFrameLayout.RESIZE_MODE_FIT, resizeModeForPreference(99))
    }
}
