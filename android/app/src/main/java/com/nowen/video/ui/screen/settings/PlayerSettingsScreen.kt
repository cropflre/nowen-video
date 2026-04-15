package com.nowen.video.ui.screen.settings

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.PlayerPreferences
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 播放器高级设置页面
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun PlayerSettingsScreen(
    onBack: () -> Unit,
    viewModel: PlayerSettingsViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(Unit) {
        viewModel.loadSettings()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("播放器设置") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            // ==================== 播放控制 ====================
            Text(
                "播放控制",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(vertical = 8.dp)
            )

            // 默认播放速度
            var showSpeedDialog by remember { mutableStateOf(false) }
            ListItem(
                headlineContent = { Text("默认播放速度") },
                supportingContent = { Text("${uiState.playbackSpeed}x") },
                leadingContent = { Icon(Icons.Default.Speed, contentDescription = null) },
                modifier = Modifier.clickableListItem { showSpeedDialog = true }
            )

            if (showSpeedDialog) {
                val speeds = listOf(0.5f, 0.75f, 1.0f, 1.25f, 1.5f, 1.75f, 2.0f, 2.5f, 3.0f)
                AlertDialog(
                    onDismissRequest = { showSpeedDialog = false },
                    title = { Text("选择播放速度") },
                    text = {
                        Column {
                            speeds.forEach { speed ->
                                ListItem(
                                    headlineContent = { Text("${speed}x") },
                                    leadingContent = {
                                        RadioButton(
                                            selected = uiState.playbackSpeed == speed,
                                            onClick = {
                                                viewModel.setPlaybackSpeed(speed)
                                                showSpeedDialog = false
                                            }
                                        )
                                    },
                                    modifier = Modifier.clickableListItem {
                                        viewModel.setPlaybackSpeed(speed)
                                        showSpeedDialog = false
                                    }
                                )
                            }
                        }
                    },
                    confirmButton = {
                        TextButton(onClick = { showSpeedDialog = false }) { Text("取消") }
                    }
                )
            }

            // 优先播放模式
            var showPlayModeDialog by remember { mutableStateOf(false) }
            val playModeLabels = listOf("自动选择", "直接播放", "Remux", "HLS 转码")
            ListItem(
                headlineContent = { Text("优先播放模式") },
                supportingContent = { Text(playModeLabels[uiState.preferredPlayMode]) },
                leadingContent = { Icon(Icons.Default.PlayCircle, contentDescription = null) },
                modifier = Modifier.clickableListItem { showPlayModeDialog = true }
            )

            if (showPlayModeDialog) {
                AlertDialog(
                    onDismissRequest = { showPlayModeDialog = false },
                    title = { Text("选择播放模式") },
                    text = {
                        Column {
                            playModeLabels.forEachIndexed { index, label ->
                                ListItem(
                                    headlineContent = { Text(label) },
                                    supportingContent = {
                                        Text(
                                            when (index) {
                                                0 -> "根据设备能力自动选择最佳模式"
                                                1 -> "直接播放原始文件，最低延迟"
                                                2 -> "转封装为 MP4，兼容性更好"
                                                3 -> "服务器端转码，兼容所有格式"
                                                else -> ""
                                            },
                                            style = MaterialTheme.typography.bodySmall
                                        )
                                    },
                                    leadingContent = {
                                        RadioButton(
                                            selected = uiState.preferredPlayMode == index,
                                            onClick = {
                                                viewModel.setPreferredPlayMode(index)
                                                showPlayModeDialog = false
                                            }
                                        )
                                    },
                                    modifier = Modifier.clickableListItem {
                                        viewModel.setPreferredPlayMode(index)
                                        showPlayModeDialog = false
                                    }
                                )
                            }
                        }
                    },
                    confirmButton = {
                        TextButton(onClick = { showPlayModeDialog = false }) { Text("取消") }
                    }
                )
            }

            // 快进步长
            var showSeekDialog by remember { mutableStateOf(false) }
            ListItem(
                headlineContent = { Text("快进/快退步长") },
                supportingContent = { Text("${uiState.seekStep} 秒") },
                leadingContent = { Icon(Icons.Default.FastForward, contentDescription = null) },
                modifier = Modifier.clickableListItem { showSeekDialog = true }
            )

            if (showSeekDialog) {
                val steps = listOf(5, 10, 15, 30, 60)
                AlertDialog(
                    onDismissRequest = { showSeekDialog = false },
                    title = { Text("选择快进步长") },
                    text = {
                        Column {
                            steps.forEach { step ->
                                ListItem(
                                    headlineContent = { Text("$step 秒") },
                                    leadingContent = {
                                        RadioButton(
                                            selected = uiState.seekStep == step,
                                            onClick = {
                                                viewModel.setSeekStep(step)
                                                showSeekDialog = false
                                            }
                                        )
                                    },
                                    modifier = Modifier.clickableListItem {
                                        viewModel.setSeekStep(step)
                                        showSeekDialog = false
                                    }
                                )
                            }
                        }
                    },
                    confirmButton = {
                        TextButton(onClick = { showSeekDialog = false }) { Text("取消") }
                    }
                )
            }

            HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

            // ==================== 画面设置 ====================
            Text(
                "画面设置",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(vertical = 8.dp)
            )

            // 画面比例
            var showAspectDialog by remember { mutableStateOf(false) }
            val aspectLabels = listOf("自适应", "填充屏幕", "16:9", "4:3")
            ListItem(
                headlineContent = { Text("默认画面比例") },
                supportingContent = { Text(aspectLabels[uiState.aspectRatio]) },
                leadingContent = { Icon(Icons.Default.AspectRatio, contentDescription = null) },
                modifier = Modifier.clickableListItem { showAspectDialog = true }
            )

            if (showAspectDialog) {
                AlertDialog(
                    onDismissRequest = { showAspectDialog = false },
                    title = { Text("选择画面比例") },
                    text = {
                        Column {
                            aspectLabels.forEachIndexed { index, label ->
                                ListItem(
                                    headlineContent = { Text(label) },
                                    leadingContent = {
                                        RadioButton(
                                            selected = uiState.aspectRatio == index,
                                            onClick = {
                                                viewModel.setAspectRatio(index)
                                                showAspectDialog = false
                                            }
                                        )
                                    },
                                    modifier = Modifier.clickableListItem {
                                        viewModel.setAspectRatio(index)
                                        showAspectDialog = false
                                    }
                                )
                            }
                        }
                    },
                    confirmButton = {
                        TextButton(onClick = { showAspectDialog = false }) { Text("取消") }
                    }
                )
            }

            // 解码器优先级
            var showDecoderDialog by remember { mutableStateOf(false) }
            val decoderLabels = listOf("自动", "硬件解码优先", "软件解码优先")
            ListItem(
                headlineContent = { Text("解码器优先级") },
                supportingContent = { Text(decoderLabels[uiState.decoderPriority]) },
                leadingContent = { Icon(Icons.Default.Memory, contentDescription = null) },
                modifier = Modifier.clickableListItem { showDecoderDialog = true }
            )

            if (showDecoderDialog) {
                AlertDialog(
                    onDismissRequest = { showDecoderDialog = false },
                    title = { Text("选择解码器") },
                    text = {
                        Column {
                            decoderLabels.forEachIndexed { index, label ->
                                ListItem(
                                    headlineContent = { Text(label) },
                                    supportingContent = {
                                        Text(
                                            when (index) {
                                                0 -> "系统自动选择最佳解码器"
                                                1 -> "优先使用 GPU 硬件加速，省电高效"
                                                2 -> "使用 CPU 软件解码，兼容性最好"
                                                else -> ""
                                            },
                                            style = MaterialTheme.typography.bodySmall
                                        )
                                    },
                                    leadingContent = {
                                        RadioButton(
                                            selected = uiState.decoderPriority == index,
                                            onClick = {
                                                viewModel.setDecoderPriority(index)
                                                showDecoderDialog = false
                                            }
                                        )
                                    },
                                    modifier = Modifier.clickableListItem {
                                        viewModel.setDecoderPriority(index)
                                        showDecoderDialog = false
                                    }
                                )
                            }
                        }
                    },
                    confirmButton = {
                        TextButton(onClick = { showDecoderDialog = false }) { Text("取消") }
                    }
                )
            }

            HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

            // ==================== 字幕设置 ====================
            Text(
                "字幕设置",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(vertical = 8.dp)
            )

            // 自动加载字幕
            ListItem(
                headlineContent = { Text("自动加载字幕") },
                supportingContent = { Text("播放时自动加载可用字幕") },
                leadingContent = { Icon(Icons.Default.Subtitles, contentDescription = null) },
                trailingContent = {
                    Switch(
                        checked = uiState.autoLoadSubtitle,
                        onCheckedChange = { viewModel.setAutoLoadSubtitle(it) }
                    )
                }
            )

            // 默认字幕语言
            var showSubLangDialog by remember { mutableStateOf(false) }
            val subLangOptions = listOf("chi" to "中文", "eng" to "英文", "jpn" to "日文", "kor" to "韩文")
            val currentLangLabel = subLangOptions.find { it.first == uiState.subtitleLanguage }?.second ?: uiState.subtitleLanguage
            ListItem(
                headlineContent = { Text("默认字幕语言") },
                supportingContent = { Text(currentLangLabel) },
                leadingContent = { Icon(Icons.Default.Language, contentDescription = null) },
                modifier = Modifier.clickableListItem { showSubLangDialog = true }
            )

            if (showSubLangDialog) {
                AlertDialog(
                    onDismissRequest = { showSubLangDialog = false },
                    title = { Text("选择字幕语言") },
                    text = {
                        Column {
                            subLangOptions.forEach { (code, label) ->
                                ListItem(
                                    headlineContent = { Text(label) },
                                    leadingContent = {
                                        RadioButton(
                                            selected = uiState.subtitleLanguage == code,
                                            onClick = {
                                                viewModel.setSubtitleLanguage(code)
                                                showSubLangDialog = false
                                            }
                                        )
                                    },
                                    modifier = Modifier.clickableListItem {
                                        viewModel.setSubtitleLanguage(code)
                                        showSubLangDialog = false
                                    }
                                )
                            }
                        }
                    },
                    confirmButton = {
                        TextButton(onClick = { showSubLangDialog = false }) { Text("取消") }
                    }
                )
            }

            HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

            // ==================== 手势控制 ====================
            Text(
                "手势控制",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(vertical = 8.dp)
            )

            // 启用手势
            ListItem(
                headlineContent = { Text("启用手势控制") },
                supportingContent = { Text("滑动调节亮度、音量和进度") },
                leadingContent = { Icon(Icons.Default.TouchApp, contentDescription = null) },
                trailingContent = {
                    Switch(
                        checked = uiState.gestureEnabled,
                        onCheckedChange = { viewModel.setGestureEnabled(it) }
                    )
                }
            )

            // 手势灵敏度
            val sensitivityLabels = listOf("低", "中", "高")
            var showSensitivityDialog by remember { mutableStateOf(false) }
            ListItem(
                headlineContent = { Text("手势灵敏度") },
                supportingContent = { Text(sensitivityLabels[uiState.gestureSensitivity]) },
                leadingContent = { Icon(Icons.Default.Tune, contentDescription = null) },
                modifier = Modifier.clickableListItem { showSensitivityDialog = true }
            )

            if (showSensitivityDialog) {
                AlertDialog(
                    onDismissRequest = { showSensitivityDialog = false },
                    title = { Text("选择灵敏度") },
                    text = {
                        Column {
                            sensitivityLabels.forEachIndexed { index, label ->
                                ListItem(
                                    headlineContent = { Text(label) },
                                    leadingContent = {
                                        RadioButton(
                                            selected = uiState.gestureSensitivity == index,
                                            onClick = {
                                                viewModel.setGestureSensitivity(index)
                                                showSensitivityDialog = false
                                            }
                                        )
                                    },
                                    modifier = Modifier.clickableListItem {
                                        viewModel.setGestureSensitivity(index)
                                        showSensitivityDialog = false
                                    }
                                )
                            }
                        }
                    },
                    confirmButton = {
                        TextButton(onClick = { showSensitivityDialog = false }) { Text("取消") }
                    }
                )
            }

            HorizontalDivider(modifier = Modifier.padding(vertical = 8.dp))

            // ==================== 其他 ====================
            Text(
                "其他",
                style = MaterialTheme.typography.titleSmall,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(vertical = 8.dp)
            )

            // 自动播放下一集
            ListItem(
                headlineContent = { Text("自动播放下一集") },
                supportingContent = { Text("剧集播放完毕后自动播放下一集") },
                leadingContent = { Icon(Icons.Default.SkipNext, contentDescription = null) },
                trailingContent = {
                    Switch(
                        checked = uiState.autoPlayNext,
                        onCheckedChange = { viewModel.setAutoPlayNext(it) }
                    )
                }
            )

            // 记住播放位置
            ListItem(
                headlineContent = { Text("记住播放位置") },
                supportingContent = { Text("下次打开时从上次位置继续播放") },
                leadingContent = { Icon(Icons.Default.Bookmark, contentDescription = null) },
                trailingContent = {
                    Switch(
                        checked = uiState.rememberPosition,
                        onCheckedChange = { viewModel.setRememberPosition(it) }
                    )
                }
            )

            Spacer(modifier = Modifier.height(32.dp))
        }
    }
}

/**
 * 扩展函数：使 ListItem 可点击
 */
private fun Modifier.clickableListItem(onClick: () -> Unit): Modifier {
    return this.then(Modifier.padding(0.dp))
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

        // 监听各项设置变化
        viewModelScope.launch {
            playerPreferences.playbackSpeedFlow.collect {
                _uiState.value = _uiState.value.copy(playbackSpeed = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.aspectRatioFlow.collect {
                _uiState.value = _uiState.value.copy(aspectRatio = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.decoderPriorityFlow.collect {
                _uiState.value = _uiState.value.copy(decoderPriority = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.subtitleLanguageFlow.collect {
                _uiState.value = _uiState.value.copy(subtitleLanguage = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.autoLoadSubtitleFlow.collect {
                _uiState.value = _uiState.value.copy(autoLoadSubtitle = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.gestureEnabledFlow.collect {
                _uiState.value = _uiState.value.copy(gestureEnabled = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.gestureSensitivityFlow.collect {
                _uiState.value = _uiState.value.copy(gestureSensitivity = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.seekStepFlow.collect {
                _uiState.value = _uiState.value.copy(seekStep = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.autoPlayNextFlow.collect {
                _uiState.value = _uiState.value.copy(autoPlayNext = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.rememberPositionFlow.collect {
                _uiState.value = _uiState.value.copy(rememberPosition = it)
            }
        }
        viewModelScope.launch {
            playerPreferences.preferredPlayModeFlow.collect {
                _uiState.value = _uiState.value.copy(preferredPlayMode = it)
            }
        }
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
