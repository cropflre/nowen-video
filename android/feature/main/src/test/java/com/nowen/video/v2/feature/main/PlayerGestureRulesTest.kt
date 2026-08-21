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
    fun `long press boost never reduces existing permanent speed`() {
        assertEquals(2f, temporaryBoostSpeed(1f))
        assertEquals(2f, temporaryBoostSpeed(1.75f))
        assertEquals(3f, temporaryBoostSpeed(3f))
        assertEquals(8f, temporaryBoostSpeed(8f))
    }

    @Test
    fun `timeline controls stay above Android navigation gestures`() {
        val source = readPlayerControlsSource()
        val timelineStart = source.indexOf("val preview = seekPreviewMs ?: positionMs")
        val timelineEnd = source.indexOf("if (boostingSpeed", startIndex = timelineStart)

        assertTrue("未找到播放器时间轴区域", timelineStart >= 0 && timelineEnd > timelineStart)
        val timelineSource = source.substring(timelineStart, timelineEnd)
        assertTrue(
            "播放器时间轴必须避开 Android 导航手势区域",
            timelineSource.contains(".windowInsetsPadding(WindowInsets.navigationBars)"),
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
