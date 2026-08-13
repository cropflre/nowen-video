package com.nowen.video.v2.core.model

import kotlinx.serialization.json.Json
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class SeriesModelsTest {
    private val json = Json { ignoreUnknownKeys = true }

    @Test
    fun `decodes series detail and season episodes`() {
        val series = json.decodeFromString<SeriesInfo>(
            """
            {
              "id":"series-1",
              "title":"三体",
              "orig_title":"Three-Body",
              "year":2023,
              "season_count":2,
              "episode_count":30,
              "genres":"科幻, 剧情",
              "unknown_field":"ignored"
            }
            """.trimIndent(),
        )
        val season = json.decodeFromString<SeasonInfo>(
            """
            {
              "season_num":1,
              "episode_count":2,
              "episodes":[
                {"id":"ep-2","title":"三体","media_type":"episode","season_num":1,"episode_num":2,"episode_title":"科学边界"},
                {"id":"ep-1","title":"三体","media_type":"episode","season_num":1,"episode_num":1,"episode_title":"倒计时"}
              ]
            }
            """.trimIndent(),
        ).normalized()

        assertEquals("三体", series.displayTitle)
        assertEquals("2023 · 2 季 · 30 集", series.metadataLabel)
        assertEquals(listOf("科幻", "剧情"), series.genreList)
        assertEquals(listOf("ep-1", "ep-2"), season.episodes.map(MediaDetail::id))
    }

    @Test
    fun `labels regular seasons specials and episodes`() {
        assertEquals("第 2 季", SeasonInfo(seasonNumber = 2).label)
        assertEquals("特别篇", SeasonInfo(seasonNumber = 0).label)
        assertEquals(
            "第 8 集",
            MediaDetail(id = "ep", mediaType = "episode", seasonNumber = 1, episodeNumber = 8).seriesEpisodeLabel,
        )
        assertEquals(
            "特别篇 3",
            MediaDetail(id = "sp", mediaType = "episode", seasonNumber = 0, episodeNumber = 3).seriesEpisodeLabel,
        )
    }

    @Test
    fun `chooses first regular episode before specials`() {
        val special = MediaDetail(id = "special", mediaType = "episode", seasonNumber = 0, episodeNumber = 1)
        val regular = MediaDetail(id = "regular", mediaType = "episode", seasonNumber = 1, episodeNumber = 1)
        val bundle = SeriesBundle(
            series = SeriesInfo(id = "series"),
            seasons = listOf(
                SeasonInfo(seasonNumber = 0, episodes = listOf(special)),
                SeasonInfo(seasonNumber = 1, episodes = listOf(regular)),
            ),
        )

        assertEquals("regular", bundle.firstEpisode?.id)
        assertTrue(bundle.firstEpisode?.seasonNumber == 1)
    }
}
