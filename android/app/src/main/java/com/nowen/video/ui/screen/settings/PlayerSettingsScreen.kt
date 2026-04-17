package com.nowen.video.ui.screen.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.PlayerPreferences
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 播放器高级设置页面 — 赛博朋克风格
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PlayerSettingsScreen(
    onBack: () -> Unit,
    viewModel: PlayerSettingsViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    val colorScheme = MaterialTheme.colorScheme

    LaunchedEffect(Unit) { viewModel.loadSettings() }

    Box(Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    title = {
                        Text(
                            "播放器设置",
                            color = MaterialTheme.colorScheme.primary,
                            style = MaterialTheme.typography.titleLarge.copy(
                                letterSpacing = 1.sp,
                                fontWeight = FontWeight.Bold
                            )
                        )
                    },
                    navigationIcon = {
                        IconButton(onClick = onBack) {
                            Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = MaterialTheme.colorScheme.primary)
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(
                        containerColor = MaterialTheme.colorScheme.scrim.copy(alpha = 0.85f)
                    )
                )
            }
        ) { padding ->
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .verticalScroll(rememberScrollState())
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp)
            ) {
                // ==================== 播放控制 ====================
                CyberSectionTitle("播放控制", MaterialTheme.colorScheme.primary)

                Box(Modifier.fillMaxWidth().glassMorphism(cornerRadius = 14.dp)) {
                    Column {
                        // 默认播放速度
                        var showSpeedDialog by remember { mutableStateOf(false) }
                        CyberClickItem(
                            icon = Icons.Default.Speed,
                            iconColor = MaterialTheme.colorScheme.primary,
                            title = "默认播放速度",
                            value = "${uiState.playbackSpeed}x",
                            onClick = { showSpeedDialog = true }
                        )
                        if (showSpeedDialog) {
                            CyberSelectionDialog(
                                title = "选择播放速度",
                                options = listOf(0.5f, 0.75f, 1.0f, 1.25f, 1.5f, 1.75f, 2.0f, 2.5f, 3.0f).map { "${it}x" },
                                selectedIndex = listOf(0.5f, 0.75f, 1.0f, 1.25f, 1.5f, 1.75f, 2.0f, 2.5f, 3.0f).indexOf(uiState.playbackSpeed),
                                onSelect = { idx ->
                                    viewModel.setPlaybackSpeed(listOf(0.5f, 0.75f, 1.0f, 1.25f, 1.5f, 1.75f, 2.0f, 2.5f, 3.0f)[idx])
                                    showSpeedDialog = false
                                },
                                onDismiss = { showSpeedDialog = false }
                            )
                        }

                        CyberItemDivider()

                        // 优先播放模式
                        var showPlayModeDialog by remember { mutableStateOf(false) }
                        val playModeLabels = listOf("自动选择", "直接播放", "Remux", "HLS 转码")
                        CyberClickItem(
                            icon = Icons.Default.PlayCircle,
                            iconColor = ElectricGreen,
                            title = "优先播放模式",
                            value = playModeLabels[uiState.preferredPlayMode],
                            onClick = { showPlayModeDialog = true }
                        )
                        if (showPlayModeDialog) {
                            val playModeDescs = listOf(
                                "根据设备能力自动选择最佳模式",
                                "直接播放原始文件，最低延迟",
                                "转封装为 MP4，兼容性更好",
                                "服务器端转码，兼容所有格式"
                            )
                            CyberSelectionDialog(
                                title = "选择播放模式",
                                options = playModeLabels,
                                descriptions = playModeDescs,
                                selectedIndex = uiState.preferredPlayMode,
                                onSelect = { viewModel.setPreferredPlayMode(it); showPlayModeDialog = false },
                                onDismiss = { showPlayModeDialog = false }
                            )
                        }

                        CyberItemDivider()

                        // 快进步长
                        var showSeekDialog by remember { mutableStateOf(false) }
                        CyberClickItem(
                            icon = Icons.Default.FastForward,
                            iconColor = MaterialTheme.colorScheme.secondary,
                            title = "快进/快退步长",
                            value = "${uiState.seekStep} 秒",
                            onClick = { showSeekDialog = true }
                        )
                        if (showSeekDialog) {
                            val steps = listOf(5, 10, 15, 30, 60)
                            CyberSelectionDialog(
                                title = "选择快进步长",
                                options = steps.map { "$it 秒" },
                                selectedIndex = steps.indexOf(uiState.seekStep),
                                onSelect = { viewModel.setSeekStep(steps[it]); showSeekDialog = false },
                                onDismiss = { showSeekDialog = false }
                            )
                        }
                    }
                }

                // ==================== 画面设置 ====================
                CyberSectionTitle("画面设置", colorScheme.secondary)

                Box(Modifier.fillMaxWidth().glassMorphism(cornerRadius = 14.dp)) {
                    Column {
                        // 画面比例
                        var showAspectDialog by remember { mutableStateOf(false) }
                        val aspectLabels = listOf("自适应", "填充屏幕", "16:9", "4:3")
                        CyberClickItem(
                            icon = Icons.Default.AspectRatio,
                            iconColor = colorScheme.primary,
                            title = "默认画面比例",
                            value = aspectLabels[uiState.aspectRatio],
                            onClick = { showAspectDialog = true }
                        )
                        if (showAspectDialog) {
                            CyberSelectionDialog(
                                title = "选择画面比例",
                                options = aspectLabels,
                                selectedIndex = uiState.aspectRatio,
                                onSelect = { viewModel.setAspectRatio(it); showAspectDialog = false },
                                onDismiss = { showAspectDialog = false }
                            )
                        }

                        CyberItemDivider()

                        // 解码器优先级
                        var showDecoderDialog by remember { mutableStateOf(false) }
                        val decoderLabels = listOf("自动", "硬件解码优先", "软件解码优先")
                        CyberClickItem(
                            icon = Icons.Default.Memory,
                            iconColor = ElectricGreen,
                            title = "解码器优先级",
                            value = decoderLabels[uiState.decoderPriority],
                            onClick = { showDecoderDialog = true }
                        )
                        if (showDecoderDialog) {
                            val decoderDescs = listOf(
                                "系统自动选择最佳解码器",
                                "优先使用 GPU 硬件加速，省电高效",
                                "使用 CPU 软件解码，兼容性最好"
                            )
                            CyberSelectionDialog(
                                title = "选择解码器",
                                options = decoderLabels,
                                descriptions = decoderDescs,
                                selectedIndex = uiState.decoderPriority,
                                onSelect = { viewModel.setDecoderPriority(it); showDecoderDialog = false },
                                onDismiss = { showDecoderDialog = false }
                            )
                        }
                    }
                }

                // ==================== 字幕设置 ====================
                CyberSectionTitle("字幕设置", ElectricGreen)

                Box(Modifier.fillMaxWidth().glassMorphism(cornerRadius = 14.dp)) {
                    Column {
                        // 自动加载字幕
                        CyberSwitchItem(
                            icon = Icons.Default.Subtitles,
                            iconColor = ElectricGreen,
                            title = "自动加载字幕",
                            subtitle = "播放时自动加载可用字幕",
                            checked = uiState.autoLoadSubtitle,
                            onCheckedChange = { viewModel.setAutoLoadSubtitle(it) }
                        )

                        CyberItemDivider()

                        // 默认字幕语言
                        var showSubLangDialog by remember { mutableStateOf(false) }
                        val subLangOptions = listOf("chi" to "中文", "eng" to "英文", "jpn" to "日文", "kor" to "韩文")
                        val currentLangLabel = subLangOptions.find { it.first == uiState.subtitleLanguage }?.second ?: uiState.subtitleLanguage
                        CyberClickItem(
                            icon = Icons.Default.Language,
                            iconColor = colorScheme.secondary,
                            title = "默认字幕语言",
                            value = currentLangLabel,
                            onClick = { showSubLangDialog = true }
                        )
                        if (showSubLangDialog) {
                            CyberSelectionDialog(
                                title = "选择字幕语言",
                                options = subLangOptions.map { it.second },
                                selectedIndex = subLangOptions.indexOfFirst { it.first == uiState.subtitleLanguage },
                                onSelect = { viewModel.setSubtitleLanguage(subLangOptions[it].first); showSubLangDialog = false },
                                onDismiss = { showSubLangDialog = false }
                            )
                        }
                    }
                }

                // ==================== 手势控制 ====================
                CyberSectionTitle("手势控制", AmberGold)

                Box(Modifier.fillMaxWidth().glassMorphism(cornerRadius = 14.dp)) {
                    Column {
                        // 启用手势
                        CyberSwitchItem(
                            icon = Icons.Default.TouchApp,
                            iconColor = AmberGold,
                            title = "启用手势控制",
                            subtitle = "滑动调节亮度、音量和进度",
                            checked = uiState.gestureEnabled,
                            onCheckedChange = { viewModel.setGestureEnabled(it) }
                        )

                        CyberItemDivider()

                        // 手势灵敏度
                        var showSensitivityDialog by remember { mutableStateOf(false) }
                        val sensitivityLabels = listOf("低", "中", "高")
                        CyberClickItem(
                            icon = Icons.Default.Tune,
                            iconColor = colorScheme.error,
                            title = "手势灵敏度",
                            value = sensitivityLabels[uiState.gestureSensitivity],
                            onClick = { showSensitivityDialog = true }
                        )
                        if (showSensitivityDialog) {
                            CyberSelectionDialog(
                                title = "选择灵敏度",
                                options = sensitivityLabels,
                                selectedIndex = uiState.gestureSensitivity,
                                onSelect = { viewModel.setGestureSensitivity(it); showSensitivityDialog = false },
                                onDismiss = { showSensitivityDialog = false }
                            )
                        }
                    }
                }

                // ==================== 其他 ====================
                CyberSectionTitle("其他", MaterialTheme.colorScheme.outline)

                Box(Modifier.fillMaxWidth().glassMorphism(cornerRadius = 14.dp)) {
                    Column {
                        // 自动播放下一集
                        CyberSwitchItem(
                            icon = Icons.Default.SkipNext,
                            iconColor = colorScheme.primary,
                            title = "自动播放下一集",
                            subtitle = "剧集播放完毕后自动播放下一集",
                            checked = uiState.autoPlayNext,
                            onCheckedChange = { viewModel.setAutoPlayNext(it) }
                        )

                        CyberItemDivider()

                        // 记住播放位置
                        CyberSwitchItem(
                            icon = Icons.Default.Bookmark,
                            iconColor = colorScheme.secondary,
                            title = "记住播放位置",
                            subtitle = "下次打开时从上次位置继续播放",
                            checked = uiState.rememberPosition,
                            onCheckedChange = { viewModel.setRememberPosition(it) }
                        )
                    }
                }

                Spacer(Modifier.height(32.dp))
            }
        }
    }
}

// ==================== 赛博朋克播放器设置组件 ====================

@Composable
private fun CyberSectionTitle(title: String, color: Color) {
    Text(
        title,
        style = MaterialTheme.typography.labelMedium.copy(
            letterSpacing = 2.sp,
            fontWeight = FontWeight.Bold
        ),
        color = color.copy(alpha = 0.7f)
    )
}

@Composable
private fun CyberClickItem(
    icon: ImageVector,
    iconColor: Color,
    title: String,
    value: String,
    onClick: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp)
    ) {
        Box(
            modifier = Modifier
                .size(36.dp)
                .clip(RoundedCornerShape(8.dp))
                .background(iconColor.copy(alpha = 0.1f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(icon, null, tint = iconColor, modifier = Modifier.size(20.dp))
        }
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.bodyLarge.copy(fontWeight = FontWeight.Medium), color = MaterialTheme.colorScheme.onSurface)
            Text(value, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.outline)
        }
        Icon(Icons.Default.ChevronRight, null, tint = MaterialTheme.colorScheme.outline, modifier = Modifier.size(20.dp))
    }
}

@Composable
private fun CyberSwitchItem(
    icon: ImageVector,
    iconColor: Color,
    title: String,
    subtitle: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp)
    ) {
        Box(
            modifier = Modifier
                .size(36.dp)
                .clip(RoundedCornerShape(8.dp))
                .background(iconColor.copy(alpha = 0.1f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(icon, null, tint = iconColor, modifier = Modifier.size(20.dp))
        }
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.bodyLarge.copy(fontWeight = FontWeight.Medium), color = MaterialTheme.colorScheme.onSurface)
            Text(subtitle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.outline)
        }
        Switch(
            checked = checked,
            onCheckedChange = onCheckedChange,
            colors = SwitchDefaults.colors(
                checkedThumbColor = Color.White,
                checkedTrackColor = MaterialTheme.colorScheme.primary,
                checkedBorderColor = MaterialTheme.colorScheme.primary,
                uncheckedThumbColor = MaterialTheme.colorScheme.outline,
                uncheckedTrackColor = MaterialTheme.colorScheme.surfaceVariant,
                uncheckedBorderColor = MaterialTheme.colorScheme.outline
            )
        )
    }
}

@Composable
private fun CyberItemDivider() {
    HorizontalDivider(
        modifier = Modifier.padding(horizontal = 16.dp),
        thickness = 0.5.dp,
        color = MaterialTheme.colorScheme.outline.copy(alpha = 0.5f)
    )
}

@Composable
private fun CyberSelectionDialog(
    title: String,
    options: List<String>,
    descriptions: List<String>? = null,
    selectedIndex: Int,
    onSelect: (Int) -> Unit,
    onDismiss: () -> Unit
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title, color = MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Bold) },
        containerColor = MaterialTheme.colorScheme.surface,
        shape = CyberDialogShape,
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                options.forEachIndexed { index, label ->
                    val isSelected = selectedIndex == index
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .clip(RoundedCornerShape(10.dp))
                            .then(
                                if (isSelected) Modifier
                                    .background(MaterialTheme.colorScheme.primary.copy(alpha = 0.08f))
                                    .border(1.dp, MaterialTheme.colorScheme.primary.copy(alpha = 0.2f), RoundedCornerShape(10.dp))
                                else Modifier
                            )
                            .clickable { onSelect(index) }
                            .padding(12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        RadioButton(
                            selected = isSelected,
                            onClick = { onSelect(index) },
                            colors = RadioButtonDefaults.colors(
                                selectedColor = MaterialTheme.colorScheme.primary,
                                unselectedColor = MaterialTheme.colorScheme.outline
                            )
                        )
                        Column(Modifier.weight(1f)) {
                            Text(
                                label,
                                color = if (isSelected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface,
                                style = MaterialTheme.typography.bodyMedium.copy(
                                    fontWeight = if (isSelected) FontWeight.Medium else FontWeight.Normal
                                )
                            )
                            if (descriptions != null && index < descriptions.size) {
                                Text(
                                    descriptions[index],
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.outline
                                )
                            }
                        }
                    }
                }
            }
        },
        confirmButton = {
            TextButton(onClick = onDismiss) { Text("取消", color = MaterialTheme.colorScheme.primary) }
        }
    )
}

// ==================== ViewModel ====================

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
    val preferredPlayMode: Int = 0
)

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
