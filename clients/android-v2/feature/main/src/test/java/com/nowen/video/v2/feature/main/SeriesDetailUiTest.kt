package com.nowen.video.v2.feature.main

import com.nowen.video.v2.core.model.SeasonInfo
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SeriesDetailUiTest {
    @Test
    fun `prefers first regular season over specials`() {
        assertEquals(
            1,
            initialSeasonNumber(
                listOf(
                    SeasonInfo(seasonNumber = 0),
                    SeasonInfo(seasonNumber = 1),
                    SeasonInfo(seasonNumber = 2),
                ),
            ),
        )
        assertEquals(0, initialSeasonNumber(listOf(SeasonInfo(seasonNumber = 0))))
        assertNull(initialSeasonNumber(emptyList()))
    }

    @Test
    fun `builds authenticated same-origin series image routes`() {
        assertEquals(
            "https://video.example.com/api/series/series-1/poster",
            seriesPosterUrl("https://video.example.com/", "series-1"),
        )
        assertEquals(
            "https://video.example.com/api/series/series-1/backdrop",
            seriesBackdropUrl("https://video.example.com", "series-1"),
        )
        assertEquals(
            "https://video.example.com/api/media/episode-1/poster",
            mediaPosterUrl("https://video.example.com", "episode-1"),
        )
    }
}
