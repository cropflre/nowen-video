package com.nowen.video.v2.feature.main

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.foundation.text.BasicText
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.nowen.video.v2.core.model.DanmakuCue

private const val DANMAKU_WINDOW_MS = 8_000L

@Composable
internal fun DanmakuOverlay(
    cues: List<DanmakuCue>,
    positionMs: Long,
    enabled: Boolean,
    mode: String,
    fontSizeSp: Float,
    speed: Float,
    opacity: Float,
    maxVisible: Int,
    modifier: Modifier = Modifier,
) {
    if (!enabled || cues.isEmpty()) return
    val visible = remember(cues, positionMs, mode, speed, maxVisible) {
        cues.asSequence()
            .filter { cue ->
                positionMs >= cue.effectiveTimeMs &&
                    positionMs < cue.effectiveTimeMs + DANMAKU_WINDOW_MS
            }
            .sortedBy(DanmakuCue::effectiveTimeMs)
            .take(maxVisible.coerceIn(1, 30))
            .toList()
    }
    if (visible.isEmpty()) return

    Column(
        modifier = modifier
            .fillMaxWidth()
            .fillMaxHeight(0.42f)
            .padding(horizontal = 16.dp),
        verticalArrangement = if (mode == "bottom") Arrangement.Bottom else Arrangement.Top,
    ) {
        visible.forEach { cue ->
            BasicText(
                text = cue.text,
                style = TextStyle(
                    color = Color.White.copy(alpha = opacity.coerceIn(0.2f, 1f)),
                    fontSize = fontSizeSp.coerceIn(12f, 36f).sp,
                    fontWeight = FontWeight.Medium,
                    shadow = androidx.compose.ui.graphics.Shadow(
                        color = Color.Black.copy(alpha = 0.9f),
                        blurRadius = 6f,
                    ),
                ),
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}
