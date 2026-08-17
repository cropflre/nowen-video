package com.nowen.video.v2.feature.main

import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.SeasonInfo
import com.nowen.video.v2.core.model.SeriesBundle
import com.nowen.video.v2.core.model.SeriesInfo
import com.nowen.video.v2.core.model.WatchHistoryRecord
import org.junit.Assert.assertEquals
import org.junit.Test

class SeriesContinueWatchingTest {
    private val episodes = listOf(
        episode("e1", 1),
        episode("e2", 2),
        episode("e3", 3),
    )
    private val bundle = SeriesBundle(
        series = SeriesInfo(id = "series-1", title = "测试剧集", seasonCount = 1, episodeCount = 3),
        seasons = listOf(SeasonInfo(seasonNumber = 1, episodeCount = 3, episodes = episodes)),
    )

    @Test
    fun `优先继续最近未看完的单集`() {
        val state = SeriesDetailUiState(
            loading = false,
            bundle = bundle,
            history = listOf(
                history("e2", position = 600.0, duration = 1200.0, completed = false),
                history("e1", position = 1200.0, duration = 1200.0, completed = true),
            ),
        )

        assertEquals("e2", state.continueEpisode?.id)
        assertEquals("继续播放 S01E02", state.continueActionLabel)
        assertEquals(1, state.watchedCount)
        assertEquals(1, state.inProgressCount)
    }

    @Test
    fun `没有进行中记录时选择第一集未看完单集`() {
        val state = SeriesDetailUiState(
            loading = false,
            bundle = bundle,
            history = listOf(history("e1", 1200.0, 1200.0, completed = true)),
        )

        assertEquals("e2", state.continueEpisode?.id)
        assertEquals("播放 S01E02", state.continueActionLabel)
    }

    @Test
    fun `全部看完后回到第一集重新播放`() {
        val state = SeriesDetailUiState(
            loading = false,
            bundle = bundle,
            history = episodes.map { history(it.id, 1200.0, 1200.0, completed = true) },
        )

        assertEquals("e1", state.continueEpisode?.id)
        assertEquals("重新播放 S01E01", state.continueActionLabel)
        assertEquals(3, state.watchedCount)
    }

    private fun episode(id: String, number: Int) = MediaDetail(
        id = id,
        title = "测试剧集",
        mediaType = "episode",
        seriesId = "series-1",
        seasonNumber = 1,
        episodeNumber = number,
        episodeTitle = "第 $number 集",
        duration = 1200.0,
    )

    private fun history(
        mediaId: String,
        position: Double,
        duration: Double,
        completed: Boolean,
    ) = WatchHistoryRecord(
        id = "history-$mediaId",
        mediaId = mediaId,
        position = position,
        duration = duration,
        completed = completed,
        media = MediaCard(id = mediaId, mediaId = mediaId),
    )
}
