package com.nowen.video.v2.core.data

import org.junit.Assert.assertEquals
import org.junit.Test

class DanmakuDataTest {
    @Test
    fun `endpoint adds api v2`() {
        assertEquals(
            "https://example.com/api/v2/comment/12",
            buildDanmakuEndpoint("https://example.com/", "", "comment/12"),
        )
    }

    @Test
    fun `endpoint preserves deployment prefix and token`() {
        assertEquals(
            "https://example.com/token/service/api/v2/match",
            buildDanmakuEndpoint("https://example.com/service", "token", "match"),
        )
    }

    @Test
    fun `endpoint does not duplicate api v2`() {
        assertEquals(
            "https://example.com/api/v2/search/anime",
            buildDanmakuEndpoint("https://example.com/api/v2/", "", "/search/anime"),
        )
    }
}
