package com.nowen.video.v2.feature.main

import android.net.Uri
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.TrackGroup
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.ui.AspectRatioFrameLayout
import com.nowen.video.v2.core.model.SubtitleTrack

internal data class PlayerTrackChoice(
    val group: TrackGroup,
    val trackIndex: Int,
    val label: String,
    val selected: Boolean,
)

internal fun extractTrackChoices(tracks: Tracks, trackType: Int): List<PlayerTrackChoice> =
    tracks.groups
        .filter { it.type == trackType }
        .flatMapIndexed { groupIndex, group ->
            (0 until group.length)
                .filter { group.isTrackSupported(it) }
                .map { trackIndex ->
                    PlayerTrackChoice(
                        group = group.mediaTrackGroup,
                        trackIndex = trackIndex,
                        label = playerTrackLabel(
                            format = group.getTrackFormat(trackIndex),
                            trackType = trackType,
                            ordinal = groupIndex + trackIndex + 1,
                        ),
                        selected = group.isTrackSelected(trackIndex),
                    )
                }
        }

internal fun playerTrackLabel(format: Format, trackType: Int, ordinal: Int): String {
    val language = format.language?.takeIf { it.isNotBlank() && it != "und" }
    val base = format.label?.takeIf { it.isNotBlank() }
        ?: language
        ?: if (trackType == C.TRACK_TYPE_AUDIO) "音轨 $ordinal" else "字幕 $ordinal"
    val detail = when (trackType) {
        C.TRACK_TYPE_AUDIO -> listOfNotNull(
            format.channelCount.takeIf { it > 0 }?.let { "${it}ch" },
            format.sampleMimeType?.substringAfterLast('/')?.uppercase(),
        ).joinToString(" · ")
        C.TRACK_TYPE_TEXT -> format.sampleMimeType?.substringAfterLast('/')?.uppercase().orEmpty()
        else -> ""
    }
    return if (detail.isBlank()) base else "$base · $detail"
}

internal fun applyTrackChoice(
    tracks: androidx.media3.common.TrackSelectionParameters,
    trackType: Int,
    choice: PlayerTrackChoice?,
): androidx.media3.common.TrackSelectionParameters {
    val builder = tracks.buildUpon()
        .setTrackTypeDisabled(trackType, false)
        .clearOverridesOfType(trackType)
    if (choice != null) {
        builder.setOverrideForType(TrackSelectionOverride(choice.group, choice.trackIndex))
    }
    return builder.build()
}

internal fun disableTextTracks(
    tracks: androidx.media3.common.TrackSelectionParameters,
): androidx.media3.common.TrackSelectionParameters = tracks.buildUpon()
    .clearOverridesOfType(C.TRACK_TYPE_TEXT)
    .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true)
    .build()

internal fun resizeModeForPreference(mode: Int): Int = when (mode) {
    1 -> AspectRatioFrameLayout.RESIZE_MODE_ZOOM
    2 -> AspectRatioFrameLayout.RESIZE_MODE_FILL
    else -> AspectRatioFrameLayout.RESIZE_MODE_FIT
}

internal fun subtitleMimeType(format: String, path: String = ""): String {
    val resolved = format.ifBlank { path.substringAfterLast('.', "") }.lowercase()
    return when (resolved) {
        "vtt", "webvtt" -> MimeTypes.TEXT_VTT
        "ass", "ssa" -> MimeTypes.TEXT_SSA
        "ttml", "xml" -> MimeTypes.APPLICATION_TTML
        "subrip", "srt" -> MimeTypes.APPLICATION_SUBRIP
        else -> MimeTypes.APPLICATION_SUBRIP
    }
}

internal fun externalSubtitleConfigurations(
    baseUrl: String?,
    tracks: List<SubtitleTrack>,
): List<MediaItem.SubtitleConfiguration> {
    if (baseUrl.isNullOrBlank()) return emptyList()
    return tracks.mapNotNull { track ->
        val path = track.sourcePath.takeIf { it.isNotBlank() } ?: return@mapNotNull null
        val uri = Uri.parse(baseUrl).buildUpon()
            .appendEncodedPath("api/subtitle/external")
            .appendQueryParameter("path", path)
            .appendQueryParameter("format", "raw")
            .build()
        MediaItem.SubtitleConfiguration.Builder(uri)
            .setMimeType(subtitleMimeType(track.format, path))
            .setLanguage(track.language.ifBlank { "und" })
            .setLabel(track.displayLabel)
            .build()
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun PlayerSettingsSheet(
    onDismiss: () -> Unit,
    playbackSpeed: Float,
    onPlaybackSpeedChange: (Float) -> Unit,
    resizeMode: Int,
    onResizeModeChange: (Int) -> Unit,
    autoPlayNext: Boolean,
    onAutoPlayNextChange: (Boolean) -> Unit,
    audioTracks: List<PlayerTrackChoice>,
    onAudioTrackSelected: (PlayerTrackChoice?) -> Unit,
    subtitleTracks: List<PlayerTrackChoice>,
    subtitlesDisabled: Boolean,
    onSubtitleTrackSelected: (PlayerTrackChoice?) -> Unit,
) {
    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(bottom = 28.dp),
        ) {
            Text(
                text = "播放设置",
                style = MaterialTheme.typography.headlineSmall,
                modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp),
            )

            SettingsSectionTitle("播放速度")
            ChoiceRow {
                listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 1.75f, 2f).forEach { speed ->
                    FilterChip(
                        selected = playbackSpeed == speed,
                        onClick = { onPlaybackSpeedChange(speed) },
                        label = { Text(if (speed == 1f) "正常" else "${speed}x") },
                    )
                }
            }

            SettingsSectionTitle("画面比例")
            ChoiceRow {
                listOf(0 to "适应", 1 to "裁切", 2 to "拉伸").forEach { (mode, label) ->
                    FilterChip(
                        selected = resizeMode == mode,
                        onClick = { onResizeModeChange(mode) },
                        label = { Text(label) },
                    )
                }
            }

            HorizontalDivider(Modifier.padding(vertical = 8.dp))
            ListItem(
                headlineContent = { Text("自动播放下一集") },
                supportingContent = { Text("当前剧集结束后显示 5 秒倒计时并自动续播") },
                trailingContent = {
                    Switch(
                        checked = autoPlayNext,
                        onCheckedChange = onAutoPlayNextChange,
                    )
                },
            )

            HorizontalDivider(Modifier.padding(vertical = 8.dp))
            SettingsSectionTitle("音轨")
            ChoiceRow {
                FilterChip(
                    selected = audioTracks.none(PlayerTrackChoice::selected),
                    onClick = { onAudioTrackSelected(null) },
                    label = { Text("自动") },
                )
                audioTracks.forEach { track ->
                    FilterChip(
                        selected = track.selected,
                        onClick = { onAudioTrackSelected(track) },
                        label = { Text(track.label) },
                    )
                }
            }

            SettingsSectionTitle("字幕")
            ChoiceRow {
                FilterChip(
                    selected = subtitlesDisabled,
                    onClick = { onSubtitleTrackSelected(null) },
                    label = { Text("关闭") },
                )
                subtitleTracks.forEach { track ->
                    FilterChip(
                        selected = !subtitlesDisabled && track.selected,
                        onClick = { onSubtitleTrackSelected(track) },
                        label = { Text(track.label) },
                    )
                }
            }
            if (subtitleTracks.isEmpty()) {
                Text(
                    text = "当前媒体没有可选字幕",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodyMedium,
                    modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp),
                )
            }
            Spacer(Modifier.height(12.dp))
        }
    }
}

@Composable
private fun SettingsSectionTitle(title: String) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleMedium,
        modifier = Modifier.padding(start = 20.dp, end = 20.dp, top = 16.dp, bottom = 8.dp),
    )
}

@Composable
private fun ChoiceRow(content: @Composable Row.() -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState())
            .padding(horizontal = 20.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        content = content,
    )
}
