package com.nowen.video.v2.feature.main

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.nio.charset.StandardCharsets
import java.nio.file.Files
import java.nio.file.Path

class PlayerGestureRulesTest {
    @Test
    fun `double tap seeks backward on left and forward on right`() {
        assertEquals(-10_000L, playerDoubleTapSeekDeltaMs(20f, 200f))
        assertEquals(10_000L, playerDoubleTapSeekDeltaMs(180f, 200f))
        assertEquals(10_000L, playerDoubleTapSeekDeltaMs(100f, 200f))
    }

    @Test
    fun `long press boost uses configured speed without reducing permanent speed`() {
        assertEquals(2f, temporaryBoostSpeed(1f))
        assertEquals(3f, temporaryBoostSpeed(1f, 3f))
        assertEquals(5f, temporaryBoostSpeed(1f, 5f))
        assertEquals(8f, temporaryBoostSpeed(1f, 8f))
        assertEquals(3f, temporaryBoostSpeed(3f, 2f))
        assertEquals(8f, temporaryBoostSpeed(8f, 5f))
    }

    @Test
    fun `horizontal seek maps drag direction and distance`() {
        assertEquals(-30_000L, horizontalSeekDeltaMs(-500f, 1000f))
        assertEquals(30_000L, horizontalSeekDeltaMs(500f, 1000f))
        assertEquals(0L, horizontalSeekDeltaMs(100f, 0f))
    }

    @Test
    fun `vertical controls clamp brightness and volume`() {
        assertEquals(1f, verticalControlDelta(0.5f, -1000f, 100f))
        assertEquals(0f, verticalControlDelta(0.5f, 1000f, 100f))
        assertEquals(10, verticalVolumeDelta(5, -100f, 100f, 10))
        assertEquals(0, verticalVolumeDelta(5, 100f, 100f, 10))
    }

    @Test
    fun `speed step follows supported discrete values`() {
        assertEquals(1.5f, neighborPlaybackSpeed(1.25f, 1))
        assertEquals(1f, neighborPlaybackSpeed(1.25f, -1))
        assertEquals(3f, neighborPlaybackSpeed(2.25f, 1))
        assertEquals(2f, neighborPlaybackSpeed(2.25f, -1))
        assertEquals(8f, neighborPlaybackSpeed(8f, 1))
        assertEquals(0.5f, neighborPlaybackSpeed(0.5f, -1))
    }

    @Test
    fun `timeline controls stay above Android navigation gestures`() {
        val source = readPlayerControlsSource()
        val timelineStart = source.indexOf("val preview = seekPreviewMs ?: positionMs")
        val timelineEnd = source.indexOf("private fun FixedPlayerSlot", startIndex = timelineStart)

        assertTrue("未找到播放器时间轴区域", timelineStart >= 0 && timelineEnd > timelineStart)
        val timelineSource = source.substring(timelineStart, timelineEnd)
        assertTrue(
            "播放器时间轴必须避开 Android 导航手势区域",
            timelineSource.contains(".windowInsetsPadding(WindowInsets.navigationBars)"),
        )
    }

    @Test
    fun `playback resource resolver rejects missing base and external origins`() {
        assertEquals(null, resolveServerResource(null, "/api/stream/movie"))
        assertEquals(null, resolveServerResource("https://nowen.example", "https://cdn.example/movie"))
        assertEquals(
            "https://nowen.example/api/stream/movie",
            resolveServerResource("https://nowen.example", "/api/stream/movie"),
        )
        assertEquals(
            "HTTPS://NOWEN.EXAMPLE/api/stream/movie",
            resolveServerResource("https://nowen.example", "HTTPS://NOWEN.EXAMPLE/api/stream/movie"),
        )
    }

    private fun readPlayerControlsSource(): String {
        val relativeSource = Path.of(
            "src", "main", "java", "com", "nowen", "video", "v2", "feature", "main", "PlayerControls.kt",
        )
        val candidates = listOf(
            relativeSource,
            Path.of("feature", "main").resolve(relativeSource),
            Path.of("android", "feature", "main").resolve(relativeSource),
        )
        val sourceFile = candidates.firstOrNull(Files::isRegularFile)
            ?: error("找不到 PlayerControls.kt：${candidates.joinToString()}")
        return String(Files.readAllBytes(sourceFile), StandardCharsets.UTF_8)
    }
}