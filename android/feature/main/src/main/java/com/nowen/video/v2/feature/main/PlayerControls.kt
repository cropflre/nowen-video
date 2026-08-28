package com.nowen.video.v2.feature.main

import android.content.res.Configuration
import android.os.SystemClock
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.AspectRatio
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PictureInPictureAlt
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.QueueMusic
import androidx.compose.material.icons.filled.Remove
import androidx.compose.material.icons.filled.SkipNext
import androidx.compose.material.icons.filled.SkipPrevious
import androidx.compose.material.icons.filled.Subtitles
import androidx.compose.material3.Icon
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Slider
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.input.pointer.PointerEventPass
import androidx.compose.ui.input.pointer.PointerInputChange
import androidx.compose.ui.input.pointer.PointerType
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.platform.LocalViewConfiguration
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.delay
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.launch

private const val LONG_PRESS_BOOST_SPEED = 2f
private const val DOUBLE_TAP_SEEK_MS = 10_000L
private const val HORIZONTAL_SEEK_RANGE_MS = 60_000L
private const val DOUBLE_TAP_TIMEOUT_MS = 300L
private val PLAYER_CONTROL_SIZE = 48.dp
private val PLAYER_ICON_SIZE = 18.dp
private val PLAYER_SIDE_GUTTER = 25.dp
private val PLAYER_TITLE_START = 12.dp
private val PLAYER_TIME_WIDTH = 96.dp
private val PLAYER_RAIL_WIDTH = 48.dp

internal fun playerDoubleTapSeekDeltaMs(x: Float, width: Float): Long =
    if (x < width / 2f) -DOUBLE_TAP_SEEK_MS else DOUBLE_TAP_SEEK_MS

internal fun temporaryBoostSpeed(currentSpeed: Float, configuredSpeed: Float = LONG_PRESS_BOOST_SPEED): Float =
    maxOf(currentSpeed, configuredSpeed.coerceIn(2f, 8f))

internal fun horizontalSeekDeltaMs(deltaX: Float, width: Float): Long {
    if (width <= 0f) return 0L
    return (deltaX / width * HORIZONTAL_SEEK_RANGE_MS).toLong()
}

internal fun verticalControlDelta(current: Float, deltaY: Float, height: Float): Float {
    if (height <= 0f) return current.coerceIn(0f, 1f)
    return (current - deltaY / height).coerceIn(0f, 1f)
}

internal fun verticalVolumeDelta(current: Int, deltaY: Float, height: Float, maximum: Int): Int {
    if (height <= 0f || maximum <= 0) return current.coerceIn(0, maximum)
    return (current - (deltaY / height * maximum).toInt()).coerceIn(0, maximum)
}

@Composable
private fun rememberPictureInPictureMode(): Boolean {
    val context = LocalContext.current
    val host = remember(context) { context.findPlaybackPictureInPictureHost() }
    val fallback = remember { MutableStateFlow(false) }
    val inPictureInPictureMode by (host?.pictureInPictureMode ?: fallback).collectAsState()
    return inPictureInPictureMode
}

@Composable
internal fun PlayerGestureLayer(
    currentSpeed: Float,
    longPressBoostSpeed: Float,
    enabled: Boolean,
    onTap: () -> Unit,
    onSeekBy: (Long) -> Unit,
    onSeekDelta: (Long) -> Unit = onSeekBy,
    onSeekFinished: () -> Unit = {},
    onBrightnessChange: (Float) -> Unit = {},
    onVolumeChange: (Float) -> Unit = {},
    onBoostStart: (Float) -> Unit,
    onBoostEnd: (Float) -> Unit,
    modifier: Modifier = Modifier,
) {
    val viewConfiguration = LocalViewConfiguration.current
    val hapticFeedback = LocalHapticFeedback.current
    val inPictureInPictureMode = rememberPictureInPictureMode()

    Box(
        modifier = modifier.pointerInput(currentSpeed, longPressBoostSpeed, enabled, inPictureInPictureMode) {
            if (!enabled || inPictureInPictureMode) return@pointerInput
            var lastTapTimeMs = 0L
            awaitEachGesture {
                val down = awaitFirstDown(requireUnconsumed = false)
                val start = down.position
                var last = start
                var mode: GestureMode? = null
                var moved = false
                var released = false
                var seekDeltaMs = 0L
                var boostStarted = false
                while (!released) {
                    val event = if (mode == null && !boostStarted) {
                        withTimeoutOrNull(viewConfiguration.longPressTimeoutMillis.toLong()) {
                            awaitPointerEvent(PointerEventPass.Main)
                        }
                    } else {
                        awaitPointerEvent(PointerEventPass.Main)
                    }
                    if (event == null) {
                        mode = GestureMode.BOOST
                        boostStarted = true
                        hapticFeedback.performHapticFeedback(HapticFeedbackType.LongPress)
                        onBoostStart(temporaryBoostSpeed(currentSpeed, longPressBoostSpeed))
                        continue
                    }
                    val change = event.changes.firstOrNull { it.id == down.id } ?: break
                    if (!change.pressed) {
                        released = true
                        break
                    }
                    val delta = change.position - last
                    val total = change.position - start
                    if (mode == null && total.getDistance() > viewConfiguration.touchSlop) {
                        moved = true
                        mode = if (kotlin.math.abs(total.x) >= kotlin.math.abs(total.y)) {
                            GestureMode.SEEK
                        } else if (start.x < size.width / 2f) {
                            GestureMode.BRIGHTNESS
                        } else {
                            GestureMode.VOLUME
                        }
                    }
                    when (mode) {
                        GestureMode.SEEK -> seekDeltaMs += horizontalSeekDeltaMs(delta.x, size.width.toFloat())
                        GestureMode.BRIGHTNESS -> onBrightnessChange(delta.y / size.height.toFloat())
                        GestureMode.VOLUME -> onVolumeChange(delta.y / size.height.toFloat())
                        else -> Unit
                    }
                    last = change.position
                    change.consume()
                }
                if (mode == GestureMode.SEEK && seekDeltaMs != 0L) onSeekDelta(seekDeltaMs)
                if (boostStarted) onBoostEnd(currentSpeed)
                if (!moved && mode == null) {
                    val now = SystemClock.elapsedRealtime()
                    if (now - lastTapTimeMs <= DOUBLE_TAP_TIMEOUT_MS) {
                        onSeekBy(playerDoubleTapSeekDeltaMs(start.x, size.width.toFloat()))
                        lastTapTimeMs = 0L
                    } else {
                        onTap()
                        lastTapTimeMs = now
                    }
                }
            }
        },
    )
}

private enum class GestureMode { SEEK, BRIGHTNESS, VOLUME, BOOST }

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
    onAspectClick: () -> Unit = {},
    resizeMode: Int = 0,
    onResizeModeChange: (Int) -> Unit = {},
    hasNextEpisode: Boolean = false,
    onNextEpisode: () -> Unit = {},
    onPlayPause: () -> Unit,
    onSeekBy: (Long) -> Unit,
    onSeekFractionChange: (Float) -> Unit,
    onSeekFinished: () -> Unit,
    onSpeedClick: () -> Unit,
    onPictureInPicture: () -> Unit = {},
    pictureInPictureAvailable: Boolean = false,
    hasPreviousEpisode: Boolean = false,
    onPreviousEpisode: () -> Unit = {},
    hasAudioTracks: Boolean = true,
    onAudioClick: () -> Unit = onSettings,
    hasSubtitleTracks: Boolean = true,
    onSubtitleClick: () -> Unit = onSettings,
    hasEpisodeList: Boolean = false,
    onEpisodeListClick: () -> Unit = {},
    onSpeedDecrease: () -> Unit = {},
    onSpeedIncrease: () -> Unit = {},
    episodeLabel: String = title,
    modifier: Modifier = Modifier,
) {
    val inPictureInPictureMode = rememberPictureInPictureMode()
    val landscape = LocalConfiguration.current.orientation == Configuration.ORIENTATION_LANDSCAPE
    var showAspectMenu by remember { mutableStateOf(false) }

    Box(modifier = modifier.fillMaxSize()) {
        AnimatedVisibility(
            visible = visible && !inPictureInPictureMode,
            enter = fadeIn(tween(150)) + slideInVertically(tween(150)) { it / 16 },
            exit = fadeOut(tween(120)) + slideOutVertically(tween(120)) { it / 16 },
        ) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(
                        Brush.verticalGradient(
                            0f to Color.Black.copy(alpha = 0.66f),
                            0.22f to Color.Transparent,
                            0.70f to Color.Transparent,
                            1f to Color.Black.copy(alpha = 0.84f),
                        ),
                    ),
            ) {
                PlayerTopBar(
                    title = title,
                    onBack = onBack,
                    onSettings = onSettings,
                    onAspectClick = {
                        showAspectMenu = !showAspectMenu
                        onAspectClick()
                    },
                    resizeMode = resizeMode,
                    onResizeModeChange = { mode ->
                        showAspectMenu = false
                        onResizeModeChange(mode)
                    },
                    showAspectMenu = showAspectMenu,
                    onPictureInPicture = onPictureInPicture,
                    pictureInPictureAvailable = pictureInPictureAvailable,
                    landscape = landscape,
                    modifier = Modifier.align(Alignment.TopCenter),
                )

                PlayerBottomControls(
                    episodeLabel = episodeLabel,
                    isPlaying = isPlaying,
                    onPlayPause = onPlayPause,
                    positionMs = positionMs,
                    durationMs = durationMs,
                    seekPreviewMs = seekPreviewMs,
                    seekingEnabled = seekingEnabled,
                    landscape = landscape,
                    hasPreviousEpisode = hasPreviousEpisode,
                    onPreviousEpisode = onPreviousEpisode,
                    hasNextEpisode = hasNextEpisode,
                    onNextEpisode = onNextEpisode,
                    hasAudioTracks = hasAudioTracks,
                    onAudioClick = onAudioClick,
                    hasSubtitleTracks = hasSubtitleTracks,
                    onSubtitleClick = onSubtitleClick,
                    hasEpisodeList = hasEpisodeList,
                    onEpisodeListClick = onEpisodeListClick,
                    playbackSpeed = playbackSpeed,
                    onSpeedClick = onSpeedClick,
                    onSpeedDecrease = onSpeedDecrease,
                    onSpeedIncrease = onSpeedIncrease,
                    onSeekFractionChange = onSeekFractionChange,
                    onSeekFinished = onSeekFinished,
                    modifier = Modifier.fillMaxSize(),
                )

                if (!landscape) {
                    Surface(
                        modifier = Modifier.align(Alignment.Center),
                        shape = CircleShape,
                        color = Color.Black.copy(alpha = 0.48f),
                    ) {
                        PlayerIconButton(onClick = onPlayPause) {
                            Icon(
                                imageVector = if (isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
                                contentDescription = if (isPlaying) "暂停" else "播放",
                                tint = Color.White,
                                modifier = Modifier.size(30.dp),
                            )
                        }
                    }
                }
            }
        }

        if (boostingSpeed != null && !inPictureInPictureMode) {
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
private fun PlayerTopBar(
    title: String,
    onBack: () -> Unit,
    onSettings: () -> Unit,
    onAspectClick: () -> Unit,
    resizeMode: Int,
    onResizeModeChange: (Int) -> Unit,
    showAspectMenu: Boolean,
    onPictureInPicture: () -> Unit,
    pictureInPictureAvailable: Boolean,
    landscape: Boolean,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .then(if (landscape) Modifier else Modifier.windowInsetsPadding(WindowInsets.safeDrawing))
            .padding(
                start = if (landscape) 0.dp else 10.dp,
                top = if (landscape) 16.dp else 0.dp,
                end = if (landscape) 0.dp else 10.dp,
            ),
    ) {
        PlayerIconButton(
            onClick = onBack,
            modifier = Modifier.align(Alignment.TopStart),
        ) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "退出播放", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
        }
        if (!landscape) {
            Text(
                text = title,
                color = Color.White,
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier
                    .align(Alignment.TopCenter)
                    .padding(horizontal = 80.dp, vertical = 14.dp),
            )
        }
        Row(
            modifier = Modifier.align(Alignment.TopEnd),
            horizontalArrangement = Arrangement.spacedBy(0.dp),
        ) {
            Box {
                PlayerIconButton(onClick = onAspectClick) {
                    Icon(Icons.Default.AspectRatio, contentDescription = "画面比例", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
                DropdownMenu(
                    expanded = showAspectMenu,
                    onDismissRequest = { onResizeModeChange(resizeMode) },
                ) {
                    listOf("适应" to 0, "裁切" to 1, "拉伸" to 2).forEach { (label, mode) ->
                        DropdownMenuItem(
                            text = { Text(label) },
                            onClick = { onResizeModeChange(mode) },
                        )
                    }
                }
            }
            if (pictureInPictureAvailable) {
                PlayerIconButton(onClick = onPictureInPicture) {
                    Icon(Icons.Default.PictureInPictureAlt, contentDescription = "画中画", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
            }
            PlayerIconButton(onClick = onSettings) {
                Icon(Icons.Default.MoreVert, contentDescription = "更多播放选项", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
            }
        }
    }
}

@Composable
private fun PlayerBottomControls(
    episodeLabel: String,
    isPlaying: Boolean,
    onPlayPause: () -> Unit,
    positionMs: Long,
    durationMs: Long,
    seekPreviewMs: Long?,
    seekingEnabled: Boolean,
    landscape: Boolean,
    hasPreviousEpisode: Boolean,
    onPreviousEpisode: () -> Unit,
    hasNextEpisode: Boolean,
    onNextEpisode: () -> Unit,
    hasAudioTracks: Boolean,
    onAudioClick: () -> Unit,
    hasSubtitleTracks: Boolean,
    onSubtitleClick: () -> Unit,
    hasEpisodeList: Boolean,
    onEpisodeListClick: () -> Unit,
    playbackSpeed: Float,
    onSpeedClick: () -> Unit,
    onSpeedDecrease: () -> Unit,
    onSpeedIncrease: () -> Unit,
    onSeekFractionChange: (Float) -> Unit,
    onSeekFinished: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val preview = seekPreviewMs ?: positionMs
    Box(modifier = modifier.fillMaxSize()) {
        if (landscape) {
            Text(
                text = episodeLabel,
                color = Color.White,
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.SemiBold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier
                    .align(Alignment.BottomStart)
                    .padding(start = PLAYER_TITLE_START, bottom = 98.dp),
            )
            Row(
                modifier = Modifier
                    .align(Alignment.BottomStart)
                    .fillMaxWidth()
                    .padding(start = PLAYER_TITLE_START, end = 8.dp, bottom = 44.dp)
                    .height(48.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    text = "${formatPlaybackTime(preview)} / ${formatPlaybackTime(durationMs)}",
                    color = Color.White,
                    style = MaterialTheme.typography.bodySmall,
                    maxLines = 1,
                    modifier = Modifier.width(PLAYER_TIME_WIDTH),
                )
                Slider(
                    value = if (durationMs > 0L) {
                        (preview.toDouble() / durationMs.toDouble()).coerceIn(0.0, 1.0).toFloat()
                    } else 0f,
                    onValueChange = onSeekFractionChange,
                    onValueChangeFinished = onSeekFinished,
                    enabled = seekingEnabled && durationMs > 0L,
                    modifier = Modifier.weight(1f),
                )
            }
            Row(
                modifier = Modifier
                    .align(Alignment.BottomStart)
                    .fillMaxWidth()
                    .height(48.dp)
                    .padding(start = 0.dp, bottom = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                FixedPlayerSlot(enabled = hasPreviousEpisode, onClick = onPreviousEpisode) {
                    Icon(Icons.Default.SkipPrevious, contentDescription = "上一集", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
                FixedPlayerSlot(enabled = true, onClick = onPlayPause) {
                    Icon(
                        imageVector = if (isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
                        contentDescription = if (isPlaying) "暂停" else "播放",
                        tint = Color.White,
                        modifier = Modifier.size(PLAYER_ICON_SIZE),
                    )
                }
                FixedPlayerSlot(enabled = hasNextEpisode, onClick = onNextEpisode) {
                    Icon(Icons.Default.SkipNext, contentDescription = "下一集", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
                Spacer(Modifier.weight(1f))
                FixedPlayerSlot(enabled = hasAudioTracks, onClick = onAudioClick) {
                    Icon(Icons.Default.QueueMusic, contentDescription = "音频", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
                FixedPlayerSlot(enabled = hasSubtitleTracks, onClick = onSubtitleClick) {
                    Icon(Icons.Default.Subtitles, contentDescription = "字幕", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
                FixedPlayerSlot(enabled = hasEpisodeList, onClick = onEpisodeListClick) {
                    Icon(Icons.Default.QueueMusic, contentDescription = "选集", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
            }
            Column(
                modifier = Modifier
                    .align(Alignment.TopEnd)
                    .padding(top = 122.dp, end = PLAYER_SIDE_GUTTER)
                    .width(48.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                PlayerIconButton(onClick = onSpeedIncrease) {
                    Icon(Icons.Default.Add, contentDescription = "提高倍速", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
                TextButton(onClick = onSpeedClick) {
                    Text(
                        text = speedDisplayLabel(playbackSpeed),
                        color = Color.White,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
                PlayerIconButton(onClick = onSpeedDecrease) {
                    Icon(Icons.Default.Remove, contentDescription = "降低倍速", tint = Color.White, modifier = Modifier.size(PLAYER_ICON_SIZE))
                }
            }
        } else {
            Column(
                modifier = Modifier
                    .align(Alignment.BottomStart)
                    .fillMaxWidth(),
            ) {
                Text(
                    text = episodeLabel,
                    color = Color.White,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.padding(start = 18.dp, end = 18.dp, bottom = 2.dp),
                )
                Row(
                    modifier = Modifier.fillMaxWidth().height(48.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        text = "${formatPlaybackTime(preview)} / ${formatPlaybackTime(durationMs)}",
                        color = Color.White,
                        style = MaterialTheme.typography.bodySmall,
                        maxLines = 1,
                        modifier = Modifier.width(94.dp),
                    )
                    Slider(
                        value = if (durationMs > 0L) {
                            (preview.toDouble() / durationMs.toDouble()).coerceIn(0.0, 1.0).toFloat()
                        } else 0f,
                        onValueChange = onSeekFractionChange,
                        onValueChangeFinished = onSeekFinished,
                        enabled = seekingEnabled && durationMs > 0L,
                        modifier = Modifier.weight(1f),
                    )
                }
            }
            Surface(
                modifier = Modifier.align(Alignment.Center),
                shape = CircleShape,
                color = Color.Black.copy(alpha = 0.48f),
            ) {
                PlayerIconButton(onClick = onPlayPause) {
                    Icon(
                        imageVector = if (isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
                        contentDescription = if (isPlaying) "暂停" else "播放",
                        tint = Color.White,
                        modifier = Modifier.size(30.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun FixedPlayerSlot(
    enabled: Boolean,
    onClick: () -> Unit,
    content: @Composable () -> Unit,
) {
    IconButton(
        onClick = onClick,
        enabled = enabled,
        modifier = Modifier.size(PLAYER_CONTROL_SIZE),
    ) { content() }
}

@Composable
private fun PlayerIconButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
) {
    IconButton(onClick = onClick, modifier = modifier.size(PLAYER_CONTROL_SIZE)) { content() }
}

internal fun speedDisplayLabel(speed: Float): String =
    if (speed == 1f) "1.0×" else "${speed.toString().removeSuffix(".0")}×"
