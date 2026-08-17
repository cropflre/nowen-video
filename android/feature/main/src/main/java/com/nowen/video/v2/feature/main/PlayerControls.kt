package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.LocalViewConfiguration
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

private const val LONG_PRESS_BOOST_SPEED = 2f

@Composable
internal fun PlayerGestureLayer(
    currentSpeed: Float,
    enabled: Boolean,
    onTap: () -> Unit,
    onSeekBy: (Long) -> Unit,
    onBoostStart: (Float) -> Unit,
    onBoostEnd: (Float) -> Unit,
    modifier: Modifier = Modifier,
) {
    val viewConfiguration = LocalViewConfiguration.current
    val hapticFeedback = LocalHapticFeedback.current

    Box(
        modifier = modifier.pointerInput(currentSpeed, enabled) {
            if (!enabled) return@pointerInput
            detectTapGestures(
                onTap = { onTap() },
                onDoubleTap = { offset ->
                    onSeekBy(if (offset.x < size.width / 2f) -10_000L else 10_000L)
                },
                onLongPress = {},
                onPress = {
                    coroutineScope {
                        val restoreSpeed = currentSpeed
                        var boosting = false
                        val boostJob = launch {
                            delay(viewConfiguration.longPressTimeoutMillis.toLong())
                            boosting = true
                            hapticFeedback.performHapticFeedback(HapticFeedbackType.LongPress)
                            onBoostStart(maxOf(restoreSpeed, LONG_PRESS_BOOST_SPEED))
                        }
                        try {
                            tryAwaitRelease()
                        } finally {
                            boostJob.cancel()
                            if (boosting) onBoostEnd(restoreSpeed)
                        }
                    }
                },
            )
        },
    )
}

@Composable
internal fun NowenPlayerControls(
    visible: Boolean,
    title: String,
    isPlaying: Boolean,
    positionMs: Long,
    durationMs: Long,
    seekPreviewMs: Long?,
    playbackSpeed: Float,
    boostingSpeed: Float?,
    seekingEnabled: Boolean,
    onBack: () -> Unit,
    onSettings: () -> Unit,
    onPlayPause: () -> Unit,
    onSeekBy: (Long) -> Unit,
    onSeekFractionChange: (Float) -> Unit,
    onSeekFinished: () -> Unit,
    onSpeedClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Box(modifier = modifier.fillMaxSize()) {
        if (visible) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(
                        Brush.verticalGradient(
                            0f to Color.Black.copy(alpha = 0.72f),
                            0.28f to Color.Transparent,
                            0.68f to Color.Transparent,
                            1f to Color.Black.copy(alpha = 0.82f),
                        ),
                    ),
            )

            Row(
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .fillMaxWidth()
                    .windowInsetsPadding(WindowInsets.statusBars)
                    .padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                GlassIconButton(onClick = onBack) {
                    Icon(
                        Icons.AutoMirrored.Filled.ArrowBack,
                        contentDescription = "退出播放",
                        tint = Color.White,
                    )
                }
                Spacer(Modifier.width(10.dp))
                Text(
                    text = title,
                    color = Color.White,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
                Spacer(Modifier.width(10.dp))
                GlassIconButton(onClick = onSettings) {
                    Icon(Icons.Default.Settings, contentDescription = "播放设置", tint = Color.White)
                }
            }

            Row(
                modifier = Modifier.align(Alignment.Center),
                horizontalArrangement = Arrangement.spacedBy(28.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                SeekButton(label = "↶ 10", onClick = { onSeekBy(-10_000L) })
                Surface(
                    shape = MaterialTheme.shapes.extraLarge,
                    color = Color.White,
                    shadowElevation = 10.dp,
                ) {
                    IconButton(onClick = onPlayPause) {
                        Icon(
                            imageVector = if (isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
                            contentDescription = if (isPlaying) "暂停" else "播放",
                            tint = Color.Black,
                        )
                    }
                }
                SeekButton(label = "10 ↷", onClick = { onSeekBy(10_000L) })
            }

            val preview = seekPreviewMs ?: positionMs
            Column(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .fillMaxWidth()
                    .padding(horizontal = 18.dp, vertical = 18.dp),
            ) {
                Slider(
                    value = if (durationMs > 0L) {
                        (preview.toDouble() / durationMs.toDouble()).coerceIn(0.0, 1.0).toFloat()
                    } else {
                        0f
                    },
                    onValueChange = onSeekFractionChange,
                    onValueChangeFinished = onSeekFinished,
                    enabled = seekingEnabled && durationMs > 0L,
                )
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        text = formatPlaybackTime(preview),
                        color = Color.White,
                        style = MaterialTheme.typography.bodyMedium,
                    )
                    Text(
                        text = " / ${formatPlaybackTime(durationMs)}",
                        color = Color.White.copy(alpha = 0.62f),
                        style = MaterialTheme.typography.bodyMedium,
                    )
                    Spacer(Modifier.weight(1f))
                    TextButton(onClick = onSpeedClick) {
                        Text(
                            text = speedDisplayLabel(playbackSpeed),
                            color = Color.White,
                            fontWeight = FontWeight.SemiBold,
                        )
                    }
                }
            }
        }

        if (boostingSpeed != null) {
            Surface(
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .windowInsetsPadding(WindowInsets.statusBars)
                    .padding(top = 62.dp),
                shape = MaterialTheme.shapes.extraLarge,
                color = Color.Black.copy(alpha = 0.72f),
            ) {
                Text(
                    text = "» ${speedDisplayLabel(boostingSpeed)} 快速播放",
                    color = Color.White,
                    style = MaterialTheme.typography.labelLarge,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 9.dp),
                )
            }
        }
    }
}

@Composable
private fun GlassIconButton(
    onClick: () -> Unit,
    content: @Composable () -> Unit,
) {
    Surface(
        shape = MaterialTheme.shapes.large,
        color = Color.Black.copy(alpha = 0.46f),
    ) {
        IconButton(onClick = onClick) { content() }
    }
}

@Composable
private fun SeekButton(label: String, onClick: () -> Unit) {
    Surface(
        shape = MaterialTheme.shapes.extraLarge,
        color = Color.Black.copy(alpha = 0.46f),
    ) {
        TextButton(onClick = onClick) {
            Text(label, color = Color.White, fontWeight = FontWeight.Bold)
        }
    }
}

internal fun speedDisplayLabel(speed: Float): String =
    if (speed == 1f) "1.0×" else "${speed.toString().removeSuffix(".0")}×"
