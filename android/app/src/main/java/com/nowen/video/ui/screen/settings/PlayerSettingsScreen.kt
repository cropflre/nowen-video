package com.nowen.video.ui.screen.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AspectRatio
import androidx.compose.material.icons.filled.Bookmark
import androidx.compose.material.icons.filled.FastForward
import androidx.compose.material.icons.filled.Language
import androidx.compose.material.icons.filled.Memory
import androidx.compose.material.icons.filled.PlayCircle
import androidx.compose.material.icons.filled.SkipNext
import androidx.compose.material.icons.filled.Speed
import androidx.compose.material.icons.filled.Subtitles
import androidx.compose.material.icons.filled.TouchApp
import androidx.compose.material.icons.filled.Tune
import androidx.compose.material3.RadioButton
import androidx.compose.material3.RadioButtonDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.PlayerPreferences
import com.nowen.video.ui.component.mobile.MobilePageHeader
import com.nowen.video.ui.component.mobile.SelectionOption
import com.nowen.video.ui.component.mobile.SettingsGroup
import com.nowen.video.ui.component.mobile.SettingsRow
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 播放器设置 UI 状态
 */
data class PlayerSettingsUiState(
    val playbackSpeed: Float = 1.0f,
    val aspectRatio: Int = 0,
    val decoderPriority: Int = 0,
    val subtitleLanguage: String = "chi",
    val autoLoadSubtitle: Boolean = true,
    val gestureEnabled: Boolean = true,
    val gestureSensitivity: Int = 1,
    val seekStep: Int = 10,
    val autoPlayNext: Boolean = true,
    val rememberPosition: Boolean = true,
    val preferredPlayMode: Int = 0,
)

/**
 * 播放器设置 ViewModel
 */
@HiltViewModel
class PlayerSettingsViewModel @Inject constructor(
    private val playerPreferences: PlayerPreferences
) : ViewModel() {

    private val _uiState = MutableStateFlow(PlayerSettingsUiState())
    val uiState = _uiState.asStateFlow()

    fun loadSettings() {
        viewModelScope.launch {
            _uiState.value = PlayerSettingsUiState(
                playbackSpeed = playerPreferences.getPlaybackSpeed(),
                aspectRatio = playerPreferences.getAspectRatio(),
                decoderPriority = playerPreferences.getDecoderPriority(),
                autoPlayNext = true,
                rememberPosition = true
            )
        }

        viewModelScope.launch { playerPreferences.playbackSpeedFlow.collect { _uiState.value = _uiState.value.copy(playbackSpeed = it) } }
        viewModelScope.launch { playerPreferences.aspectRatioFlow.collect { _uiState.value = _uiState.value.copy(aspectRatio = it) } }
        viewModelScope.launch { playerPreferences.decoderPriorityFlow.collect { _uiState.value = _uiState.value.copy(decoderPriority = it) } }
        viewModelScope.launch { playerPreferences.subtitleLanguageFlow.collect { _uiState.value = _uiState.value.copy(subtitleLanguage = it) } }
        viewModelScope.launch { playerPreferences.autoLoadSubtitleFlow.collect { _uiState.value = _uiState.value.copy(autoLoadSubtitle = it) } }
        viewModelScope.launch { playerPreferences.gestureEnabledFlow.collect { _uiState.value = _uiState.value.copy(gestureEnabled = it) } }
        viewModelScope.launch { playerPreferences.gestureSensitivityFlow.collect { _uiState.value = _uiState.value.copy(gestureSensitivity = it) } }
        viewModelScope.launch { playerPreferences.seekStepFlow.collect { _uiState.value = _uiState.value.copy(seekStep = it) } }
        viewModelScope.launch { playerPreferences.autoPlayNextFlow.collect { _uiState.value = _uiState.value.copy(autoPlayNext = it) } }
        viewModelScope.launch { playerPreferences.rememberPositionFlow.collect { _uiState.value = _uiState.value.copy(rememberPosition = it) } }
        viewModelScope.launch { playerPreferences.preferredPlayModeFlow.collect { _uiState.value = _uiState.value.copy(preferredPlayMode = it) } }
    }

    fun setPlaybackSpeed(speed: Float) = viewModelScope.launch { playerPreferences.setPlaybackSpeed(speed) }
    fun setAspectRatio(ratio: Int) = viewModelScope.launch { playerPreferences.setAspectRatio(ratio) }
    fun setDecoderPriority(priority: Int) = viewModelScope.launch { playerPreferences.setDecoderPriority(priority) }
    fun setSubtitleLanguage(lang: String) = viewModelScope.launch { playerPreferences.setSubtitleLanguage(lang) }
    fun setAutoLoadSubtitle(enabled: Boolean) = viewModelScope.launch { playerPreferences.setAutoLoadSubtitle(enabled) }
    fun setGestureEnabled(enabled: Boolean) = viewModelScope.launch { playerPreferences.setGestureEnabled(enabled) }
    fun setGestureSensitivity(sensitivity: Int) = viewModelScope.launch { playerPreferences.setGestureSensitivity(sensitivity) }
    fun setSeekStep(step: Int) = viewModelScope.launch { playerPreferences.setSeekStep(step) }
    fun setAutoPlayNext(enabled: Boolean) = viewModelScope.launch { playerPreferences.setAutoPlayNext(enabled) }
    fun setRememberPosition(enabled: Boolean) = viewModelScope.launch { playerPreferences.setRememberPosition(enabled) }
    fun setPreferredPlayMode(mode: Int) = viewModelScope.launch { playerPreferences.setPreferredPlayMode(mode) }
}

/**
 * 播放器高级设置页面 — Hills Pro 风格
 */
@Composable
fun PlayerSettingsScreen(
    onBack: () -> Unit,
    viewModel: PlayerSettingsViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()

    // 进入页面时加载设置
    LaunchedEffect(Unit) {
        viewModel.loadSettings()
    }

    // 选择弹窗状态
    var showSpeedDialog by remember { mutableStateOf(false) }
    var showPlayModeDialog by remember { mutableStateOf(false) }
    var showSeekDialog by remember { mutableStateOf(false) }
    var showAspectDialog by remember { mutableStateOf(false) }
    var showDecoderDialog by remember { mutableStateOf(false) }
    var showSubLangDialog by remember { mutableStateOf(false) }
    var showSensitivityDialog by remember { mutableStateOf(false) }

    // 选项定义
    val speedOptions = listOf(0.5f, 0.75f, 1.0f, 1.25f, 1.5f, 1.75f, 2.0f, 2.5f, 3.0f)
    val playModeLabels = listOf("自动选择", "直接播放", "Remux", "HLS 转码")
    val playModeDescs = listOf(
        "根据设备能力自动选择最佳模式",
        "直接播放原始文件，最低延迟",
        "转封装为 MP4，兼容性更好",
        "服务器端转码，兼容所有格式"
    )
    val seekSteps = listOf(5, 10, 15, 30, 60)
    val aspectLabels = listOf("自适应", "填充屏幕", "16:9", "4:3")
    val decoderLabels = listOf("自动", "硬件解码优先", "软件解码优先")
    val decoderDescs = listOf(
        "系统自动选择最佳解码器",
        "优先使用 GPU 硬件加速，省电高效",
        "使用 CPU 软件解码，兼容性最好"
    )
    val subLangOptions = listOf("chi" to "中文", "eng" to "英文", "jpn" to "日文", "kor" to "韩文")
    val sensitivityLabels = listOf("低", "中", "高")

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
    ) {
        // 页面标题
        item {
            MobilePageHeader(
                title = "播放器设置",
                onBack = onBack,
            )
        }

        // 播放控制分组
        item {
            SettingsGroup(
                title = "播放控制",
                rows = listOf(
                    SettingsRow.Action(
                        icon = Icons.Default.Speed,
                        title = "默认播放速度",
                        status = "${uiState.playbackSpeed}x",
                        onClick = { showSpeedDialog = true },
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.PlayCircle,
                        title = "优先播放模式",
                        status = playModeLabels[uiState.preferredPlayMode],
                        onClick = { showPlayModeDialog = true },
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.FastForward,
                        title = "快进/快退步长",
                        status = "${uiState.seekStep} 秒",
                        onClick = { showSeekDialog = true },
                    ),
                ),
            )
        }

        // 画面设置分组
        item {
            SettingsGroup(
                title = "画面设置",
                rows = listOf(
                    SettingsRow.Action(
                        icon = Icons.Default.AspectRatio,
                        title = "默认画面比例",
                        status = aspectLabels[uiState.aspectRatio],
                        onClick = { showAspectDialog = true },
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Memory,
                        title = "解码器优先级",
                        status = decoderLabels[uiState.decoderPriority],
                        onClick = { showDecoderDialog = true },
                    ),
                ),
            )
        }

        // 字幕设置分组
        item {
            SettingsGroup(
                title = "字幕设置",
                rows = listOf(
                    SettingsRow.Switch(
                        icon = Icons.Default.Subtitles,
                        title = "自动加载字幕",
                        subtitle = "播放时自动加载可用字幕",
                        checked = uiState.autoLoadSubtitle,
                        onCheckedChange = { viewModel.setAutoLoadSubtitle(it) },
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Language,
                        title = "默认字幕语言",
                        status = subLangOptions.find { it.first == uiState.subtitleLanguage }?.second ?: uiState.subtitleLanguage,
                        onClick = { showSubLangDialog = true },
                    ),
                ),
            )
        }

        // 手势控制分组
        item {
            SettingsGroup(
                title = "手势控制",
                rows = listOf(
                    SettingsRow.Switch(
                        icon = Icons.Default.TouchApp,
                        title = "启用手势控制",
                        subtitle = "滑动调节亮度、音量和进度",
                        checked = uiState.gestureEnabled,
                        onCheckedChange = { viewModel.setGestureEnabled(it) },
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Tune,
                        title = "手势灵敏度",
                        status = sensitivityLabels[uiState.gestureSensitivity],
                        onClick = { showSensitivityDialog = true },
                    ),
                ),
            )
        }

        // 其他分组
        item {
            SettingsGroup(
                title = "其他",
                rows = listOf(
                    SettingsRow.Switch(
                        icon = Icons.Default.SkipNext,
                        title = "自动播放下一集",
                        subtitle = "剧集播放完毕后自动播放下一集",
                        checked = uiState.autoPlayNext,
                        onCheckedChange = { viewModel.setAutoPlayNext(it) },
                    ),
                    SettingsRow.Switch(
                        icon = Icons.Default.Bookmark,
                        title = "记住播放位置",
                        subtitle = "下次打开时从上次位置继续播放",
                        checked = uiState.rememberPosition,
                        onCheckedChange = { viewModel.setRememberPosition(it) },
                    ),
                ),
            )
        }

        // 底部间距
        item {
            Spacer(modifier = Modifier.height(MobileSpacing.xl))
        }
    }

    // 选择弹窗
    if (showSpeedDialog) {
        SelectionBottomSheet(
            title = "选择播放速度",
            options = speedOptions.map { SelectionOption(it.toString(), "${it}x") },
            selectedValue = uiState.playbackSpeed.toString(),
            onSelect = { viewModel.setPlaybackSpeed(it.toFloat()); showSpeedDialog = false },
            onDismiss = { showSpeedDialog = false },
        )
    }

    if (showPlayModeDialog) {
        SelectionBottomSheet(
            title = "选择播放模式",
            options = playModeLabels.mapIndexed { index, label ->
                SelectionOption(index.toString(), label)
            },
            selectedValue = uiState.preferredPlayMode.toString(),
            onSelect = { viewModel.setPreferredPlayMode(it.toInt()); showPlayModeDialog = false },
            onDismiss = { showPlayModeDialog = false },
        )
    }

    if (showSeekDialog) {
        SelectionBottomSheet(
            title = "选择快进步长",
            options = seekSteps.map { SelectionOption(it.toString(), "$it 秒") },
            selectedValue = uiState.seekStep.toString(),
            onSelect = { viewModel.setSeekStep(it.toInt()); showSeekDialog = false },
            onDismiss = { showSeekDialog = false },
        )
    }

    if (showAspectDialog) {
        SelectionBottomSheet(
            title = "选择画面比例",
            options = aspectLabels.mapIndexed { index, label ->
                SelectionOption(index.toString(), label)
            },
            selectedValue = uiState.aspectRatio.toString(),
            onSelect = { viewModel.setAspectRatio(it.toInt()); showAspectDialog = false },
            onDismiss = { showAspectDialog = false },
        )
    }

    if (showDecoderDialog) {
        SelectionBottomSheet(
            title = "选择解码器",
            options = decoderLabels.mapIndexed { index, label ->
                SelectionOption(index.toString(), label)
            },
            selectedValue = uiState.decoderPriority.toString(),
            onSelect = { viewModel.setDecoderPriority(it.toInt()); showDecoderDialog = false },
            onDismiss = { showDecoderDialog = false },
        )
    }

    if (showSubLangDialog) {
        SelectionBottomSheet(
            title = "选择字幕语言",
            options = subLangOptions.map { SelectionOption(it.first, it.second) },
            selectedValue = uiState.subtitleLanguage,
            onSelect = { viewModel.setSubtitleLanguage(it); showSubLangDialog = false },
            onDismiss = { showSubLangDialog = false },
        )
    }

    if (showSensitivityDialog) {
        SelectionBottomSheet(
            title = "选择灵敏度",
            options = sensitivityLabels.mapIndexed { index, label ->
                SelectionOption(index.toString(), label)
            },
            selectedValue = uiState.gestureSensitivity.toString(),
            onSelect = { viewModel.setGestureSensitivity(it.toInt()); showSensitivityDialog = false },
            onDismiss = { showSensitivityDialog = false },
        )
    }
}

/**
 * 选择底部弹窗
 */
@Composable
private fun SelectionBottomSheet(
    title: String,
    options: List<SelectionOption>,
    selectedValue: String,
    onSelect: (String) -> Unit,
    onDismiss: () -> Unit,
) {
    // 使用 ModalBottomSheet 或 AlertDialog
    // 这里先用 AlertDialog 实现，后续可改为 BottomSheet
    androidx.compose.material3.AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            androidx.compose.material3.Text(
                text = title,
                color = com.nowen.video.ui.theme.MobileColors.Text,
                fontWeight = androidx.compose.ui.text.font.FontWeight.Bold,
            )
        },
        containerColor = com.nowen.video.ui.theme.MobileColors.Bg,
        shape = androidx.compose.foundation.shape.RoundedCornerShape(com.nowen.video.ui.theme.MobileRadius.xl),
        text = {
            androidx.compose.foundation.layout.Column(
                verticalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(4.dp),
            ) {
                options.forEach { option ->
                    val isSelected = option.value == selectedValue
                    androidx.compose.foundation.layout.Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clip(androidx.compose.foundation.shape.RoundedCornerShape(com.nowen.video.ui.theme.MobileRadius.md))
                            .then(
                                if (isSelected) Modifier.background(com.nowen.video.ui.theme.MobileColors.PrimarySoft)
                                else Modifier
                            )
                            .clickable { onSelect(option.value) }
                            .padding(12.dp),
                        verticalAlignment = androidx.compose.ui.Alignment.CenterVertically,
                        horizontalArrangement = androidx.compose.foundation.layout.Arrangement.spacedBy(12.dp),
                    ) {
                        androidx.compose.material3.RadioButton(
                            selected = isSelected,
                            onClick = { onSelect(option.value) },
                            colors = androidx.compose.material3.RadioButtonDefaults.colors(
                                selectedColor = com.nowen.video.ui.theme.MobileColors.Primary,
                                unselectedColor = com.nowen.video.ui.theme.MobileColors.Muted,
                            ),
                        )
                        androidx.compose.material3.Text(
                            text = option.label,
                            color = if (isSelected) com.nowen.video.ui.theme.MobileColors.Primary else com.nowen.video.ui.theme.MobileColors.Text,
                            fontWeight = if (isSelected) androidx.compose.ui.text.font.FontWeight.Medium else androidx.compose.ui.text.font.FontWeight.Normal,
                        )
                    }
                }
            }
        },
        confirmButton = {
            androidx.compose.material3.TextButton(onClick = onDismiss) {
                androidx.compose.material3.Text(
                    text = "取消",
                    color = com.nowen.video.ui.theme.MobileColors.Primary,
                )
            }
        },
    )
}
