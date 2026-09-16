package com.nowen.video.v2.feature.main

import com.nowen.video.v2.core.model.MediaCard
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.nio.file.Files
import java.nio.file.Path

class MediaDetailRoutingTest {
    @Test
    fun `recommendations retain movie and series identities`() {
        assertTrue(MediaCard(id = "series-1", type = "series").isSeries)
        assertFalse(MediaCard(id = "movie-1", type = "movie").isSeries)
        assertFalse(MediaCard(id = "episode-1", type = "episode").isSeries)
    }

    @Test
    fun `recommendations pass cards while episode links keep IDs`() {
        fun source(name: String): String {
            val relative = Path.of("src", "main", "java", "com", "nowen", "video", "v2", "feature", "main", name)
            val candidates = listOf(relative, Path.of("feature", "main").resolve(relative), Path.of("android", "feature", "main").resolve(relative))
            return String(Files.readAllBytes(candidates.firstOrNull(Files::isRegularFile) ?: error("Missing $name")), java.nio.charset.StandardCharsets.UTF_8)
        }
        val detail = source("MediaDetailScreen.kt")
        val shell = source("MainShell.kt")
        assertTrue(detail.contains("onRecommendationClick: (MediaCard) -> Unit"))
        assertTrue(detail.contains("onClick = { onRecommendationClick(item) }"))
        assertTrue(detail.contains("onClick = { onMediaClick(item) }"))
        assertTrue(shell.contains("onRecommendationClick = ::openCatalogDetail"))
        assertTrue(detail.contains("onEpisodeClick = onMediaClick"))
    }
}
