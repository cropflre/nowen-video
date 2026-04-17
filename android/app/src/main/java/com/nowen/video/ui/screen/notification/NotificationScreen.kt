package com.nowen.video.ui.screen.notification

import androidx.compose.animation.*
import androidx.compose.animation.core.*
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.remote.*
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 实时通知页面 — 赛博朋克风格
 * 展示扫描/刮削/转码等后台任务进度
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NotificationScreen(
    onBack: () -> Unit,
    viewModel: NotificationViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(Unit) { viewModel.startListening() }
    DisposableEffect(Unit) { onDispose { viewModel.stopListening() } }

    Box(Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    title = {
                        Text(
                            "后台任务",
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
                    actions = {
                        // 赛博朋克连接状态指示
                        CyberConnectionBadge(uiState.connectionState)
                    },
                    colors = TopAppBarDefaults.topAppBarColors(
                        containerColor = MaterialTheme.colorScheme.scrim.copy(alpha = 0.85f)
                    )
                )
            }
        ) { padding ->
            if (uiState.tasks.isEmpty()) {
                // 空状态
                Box(
                    modifier = Modifier.fillMaxSize().padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        // 脉冲动画图标
                        val infiniteTransition = rememberInfiniteTransition(label = "empty_pulse")
                        val pulseAlpha by infiniteTransition.animateFloat(
                            initialValue = 0.3f, targetValue = 0.7f,
                            animationSpec = infiniteRepeatable(
                                tween(2000, easing = EaseInOutCubic),
                                RepeatMode.Reverse
                            ), label = "pulse_a"
                        )
                        Icon(
                            Icons.Default.Notifications,
                            contentDescription = null,
                            modifier = Modifier.size(72.dp),
                            tint = MaterialTheme.colorScheme.primary.copy(alpha = pulseAlpha)
                        )
                        Spacer(Modifier.height(16.dp))
                        Text(
                            "暂无后台任务",
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                        Text(
                            "扫描、刮削、转码等任务进度将在此显示",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.outline
                        )
                    }
                }
            } else {
                LazyColumn(
                    modifier = Modifier.fillMaxSize().padding(padding),
                    contentPadding = PaddingValues(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    items(uiState.tasks) { task ->
                        CyberTaskCard(task)
                    }
                }
            }
        }
    }
}

/**
 * 赛博朋克连接状态徽章
 */
@Composable
private fun CyberConnectionBadge(state: WSConnectionState) {
    val stateColor = when (state) {
        WSConnectionState.CONNECTED -> ElectricGreen
        WSConnectionState.CONNECTING, WSConnectionState.RECONNECTING -> AmberGold
        WSConnectionState.DISCONNECTED -> MaterialTheme.colorScheme.error
    }
    val stateText = when (state) {
        WSConnectionState.CONNECTED -> "已连接"
        WSConnectionState.CONNECTING -> "连接中..."
        WSConnectionState.RECONNECTING -> "重连中..."
        WSConnectionState.DISCONNECTED -> "未连接"
    }

    // 连接状态呼吸动画
    val infiniteTransition = rememberInfiniteTransition(label = "conn_badge")
    val glowAlpha by infiniteTransition.animateFloat(
        initialValue = 0.15f, targetValue = 0.35f,
        animationSpec = infiniteRepeatable(
            tween(1500, easing = EaseInOutCubic),
            RepeatMode.Reverse
        ), label = "conn_glow"
    )

    Surface(
        shape = RoundedCornerShape(12.dp),
        color = stateColor.copy(alpha = glowAlpha),
        modifier = Modifier
            .padding(end = 8.dp)
            .border(1.dp, stateColor.copy(alpha = 0.3f), RoundedCornerShape(12.dp))
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(5.dp)
        ) {
            // 发光小圆点
            Box(
                modifier = Modifier
                    .size(8.dp)
                    .clip(CircleShape)
                    .background(stateColor)
            )
            Text(
                stateText,
                style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Medium),
                color = stateColor
            )
        }
    }
}

/**
 * 赛博朋克任务卡片
 */
@Composable
private fun CyberTaskCard(task: TaskInfo) {
    val colorScheme = MaterialTheme.colorScheme
    val taskColor = when (task.type) {
        TaskType.SCAN -> colorScheme.primary
        TaskType.SCRAPE -> colorScheme.secondary
        TaskType.TRANSCODE -> ElectricGreen
    }
    val statusColor = when (task.status) {
        TaskStatus.RUNNING -> taskColor
        TaskStatus.COMPLETED -> ElectricGreen
        TaskStatus.FAILED -> colorScheme.error
    }

    Box(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(14.dp))
            .background(
                Brush.verticalGradient(
                    listOf(
                        taskColor.copy(alpha = 0.04f),
                        colorScheme.surface.copy(alpha = 0.95f)
                    )
                )
            )
            .border(
                1.dp,
                Brush.verticalGradient(
                    listOf(
                        taskColor.copy(alpha = 0.2f),
                        taskColor.copy(alpha = 0.05f)
                    )
                ),
                RoundedCornerShape(14.dp)
            )
            .animateContentSize()
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp)
            ) {
                // 任务类型图标
                Box(
                    modifier = Modifier
                        .size(40.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(taskColor.copy(alpha = 0.1f))
                        .border(1.dp, taskColor.copy(alpha = 0.2f), RoundedCornerShape(10.dp)),
                    contentAlignment = Alignment.Center
                ) {
                    Icon(
                        when (task.type) {
                            TaskType.SCAN -> Icons.Default.FolderOpen
                            TaskType.SCRAPE -> Icons.Default.CloudDownload
                            TaskType.TRANSCODE -> Icons.Default.Transform
                        },
                        contentDescription = null,
                        tint = taskColor,
                        modifier = Modifier.size(22.dp)
                    )
                }

                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = task.title,
                        style = MaterialTheme.typography.titleSmall.copy(fontWeight = FontWeight.Medium),
                        color = colorScheme.onSurface
                    )
                    if (task.message.isNotBlank()) {
                        Text(
                            text = task.message,
                            style = MaterialTheme.typography.bodySmall,
                            color = colorScheme.onSurfaceVariant,
                            maxLines = 1
                        )
                    }
                }

                // 赛博朋克状态标签
                Surface(
                    shape = RoundedCornerShape(6.dp),
                    color = statusColor.copy(alpha = 0.12f),
                    modifier = Modifier.border(
                        0.5.dp, statusColor.copy(alpha = 0.3f), RoundedCornerShape(6.dp)
                    )
                ) {
                    Text(
                        text = when (task.status) {
                            TaskStatus.RUNNING -> "进行中"
                            TaskStatus.COMPLETED -> "已完成"
                            TaskStatus.FAILED -> "失败"
                        },
                        style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Medium),
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp),
                        color = statusColor
                    )
                }
            }

            // 赛博朋克进度条
            if (task.status == TaskStatus.RUNNING && task.total > 0) {
                Spacer(Modifier.height(12.dp))
                val progress = (task.current.toFloat() / task.total).coerceIn(0f, 1f)

                // 进度条背景
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(6.dp)
                        .clip(RoundedCornerShape(3.dp))
                        .background(colorScheme.surfaceVariant)
                ) {
                    // 发光进度条
                    Box(
                        modifier = Modifier
                            .fillMaxWidth(progress)
                            .fillMaxHeight()
                            .clip(RoundedCornerShape(3.dp))
                            .background(
                                Brush.horizontalGradient(
                                    listOf(taskColor, taskColor.copy(alpha = 0.7f))
                                )
                            )
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth().padding(top = 4.dp),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Text(
                        "${task.current} / ${task.total}",
                        style = MaterialTheme.typography.labelSmall,
                        color = colorScheme.outline
                    )
                    Text(
                        "${(progress * 100).toInt()}%",
                        style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Medium),
                        color = taskColor
                    )
                }
            }
        }
    }
}

// ==================== 数据模型 ====================

enum class TaskType { SCAN, SCRAPE, TRANSCODE }
enum class TaskStatus { RUNNING, COMPLETED, FAILED }

data class TaskInfo(
    val id: String,
    val type: TaskType,
    val title: String,
    val message: String = "",
    val status: TaskStatus = TaskStatus.RUNNING,
    val current: Int = 0,
    val total: Int = 0,
    val timestamp: Long = System.currentTimeMillis()
)

// ==================== ViewModel ====================

data class NotificationUiState(
    val connectionState: WSConnectionState = WSConnectionState.DISCONNECTED,
    val tasks: List<TaskInfo> = emptyList()
)

@HiltViewModel
class NotificationViewModel @Inject constructor(
    private val webSocketManager: WebSocketManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(NotificationUiState())
    val uiState = _uiState.asStateFlow()

    fun startListening() {
        webSocketManager.connect()

        viewModelScope.launch {
            webSocketManager.connectionState.collect { state ->
                _uiState.value = _uiState.value.copy(connectionState = state)
            }
        }

        viewModelScope.launch {
            webSocketManager.scanProgress.collect { data ->
                updateTask(
                    id = "scan_${data.library_id}",
                    type = TaskType.SCAN,
                    title = "扫描: ${data.library_name}",
                    message = data.message,
                    current = data.current,
                    total = data.total,
                    isCompleted = data.phase == "completed"
                )
            }
        }

        viewModelScope.launch {
            webSocketManager.scrapeProgress.collect { data ->
                updateTask(
                    id = "scrape_${data.library_id}",
                    type = TaskType.SCRAPE,
                    title = "刮削: ${data.library_name}",
                    message = data.message.ifBlank { "正在刮削: ${data.media_title}" },
                    current = data.current,
                    total = data.total,
                    isCompleted = data.current >= data.total && data.total > 0
                )
            }
        }

        viewModelScope.launch {
            webSocketManager.transcodeProgress.collect { data ->
                updateTask(
                    id = "transcode_${data.task_id}",
                    type = TaskType.TRANSCODE,
                    title = "转码: ${data.title}",
                    message = "${data.quality} · ${data.speed}",
                    current = data.progress.toInt(),
                    total = 100,
                    isCompleted = data.progress >= 100
                )
            }
        }

        viewModelScope.launch {
            webSocketManager.events.collect { event ->
                when (event.type) {
                    "scan_failed" -> markTaskFailed("scan_")
                    "transcode_failed" -> markTaskFailed("transcode_")
                }
            }
        }
    }

    fun stopListening() {}

    private fun updateTask(id: String, type: TaskType, title: String, message: String, current: Int, total: Int, isCompleted: Boolean) {
        val tasks = _uiState.value.tasks.toMutableList()
        val index = tasks.indexOfFirst { it.id == id }
        val task = TaskInfo(id = id, type = type, title = title, message = message,
            status = if (isCompleted) TaskStatus.COMPLETED else TaskStatus.RUNNING,
            current = current, total = total)
        if (index >= 0) tasks[index] = task else tasks.add(0, task)
        _uiState.value = _uiState.value.copy(tasks = tasks)
    }

    private fun markTaskFailed(idPrefix: String) {
        val tasks = _uiState.value.tasks.map { task ->
            if (task.id.startsWith(idPrefix) && task.status == TaskStatus.RUNNING)
                task.copy(status = TaskStatus.FAILED) else task
        }
        _uiState.value = _uiState.value.copy(tasks = tasks)
    }
}
