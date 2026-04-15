package com.nowen.video.ui.screen.notification

import androidx.compose.animation.animateContentSize
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
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
import com.nowen.video.data.remote.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 实时通知页面 — 展示扫描/刮削/转码等后台任务进度
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun NotificationScreen(
    onBack: () -> Unit,
    viewModel: NotificationViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(Unit) {
        viewModel.startListening()
    }

    DisposableEffect(Unit) {
        onDispose { viewModel.stopListening() }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("后台任务") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    // 连接状态指示
                    val stateColor = when (uiState.connectionState) {
                        WSConnectionState.CONNECTED -> MaterialTheme.colorScheme.primary
                        WSConnectionState.CONNECTING, WSConnectionState.RECONNECTING ->
                            MaterialTheme.colorScheme.tertiary
                        WSConnectionState.DISCONNECTED -> MaterialTheme.colorScheme.error
                    }
                    val stateText = when (uiState.connectionState) {
                        WSConnectionState.CONNECTED -> "已连接"
                        WSConnectionState.CONNECTING -> "连接中..."
                        WSConnectionState.RECONNECTING -> "重连中..."
                        WSConnectionState.DISCONNECTED -> "未连接"
                    }
                    Surface(
                        shape = RoundedCornerShape(12.dp),
                        color = stateColor.copy(alpha = 0.15f),
                        modifier = Modifier.padding(end = 8.dp)
                    ) {
                        Row(
                            modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(4.dp)
                        ) {
                            Icon(
                                if (uiState.connectionState == WSConnectionState.CONNECTED)
                                    Icons.Default.Wifi else Icons.Default.WifiOff,
                                contentDescription = null,
                                modifier = Modifier.size(14.dp),
                                tint = stateColor
                            )
                            Text(
                                stateText,
                                style = MaterialTheme.typography.labelSmall,
                                color = stateColor
                            )
                        }
                    }
                }
            )
        }
    ) { padding ->
        if (uiState.tasks.isEmpty()) {
            Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Icon(
                        Icons.Default.Notifications,
                        contentDescription = null,
                        modifier = Modifier.size(64.dp),
                        tint = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Spacer(modifier = Modifier.height(16.dp))
                    Text(
                        "暂无后台任务",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        "扫描、刮削、转码等任务进度将在此显示",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
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
                    TaskCard(task)
                }
            }
        }
    }
}

@Composable
private fun TaskCard(task: TaskInfo) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .animateContentSize(),
        shape = RoundedCornerShape(12.dp)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                Icon(
                    when (task.type) {
                        TaskType.SCAN -> Icons.Default.FolderOpen
                        TaskType.SCRAPE -> Icons.Default.CloudDownload
                        TaskType.TRANSCODE -> Icons.Default.Transform
                    },
                    contentDescription = null,
                    tint = when (task.status) {
                        TaskStatus.RUNNING -> MaterialTheme.colorScheme.primary
                        TaskStatus.COMPLETED -> MaterialTheme.colorScheme.tertiary
                        TaskStatus.FAILED -> MaterialTheme.colorScheme.error
                    }
                )
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = task.title,
                        style = MaterialTheme.typography.titleSmall
                    )
                    Text(
                        text = task.message,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                // 状态标签
                Surface(
                    shape = RoundedCornerShape(6.dp),
                    color = when (task.status) {
                        TaskStatus.RUNNING -> MaterialTheme.colorScheme.primary.copy(alpha = 0.15f)
                        TaskStatus.COMPLETED -> MaterialTheme.colorScheme.tertiary.copy(alpha = 0.15f)
                        TaskStatus.FAILED -> MaterialTheme.colorScheme.error.copy(alpha = 0.15f)
                    }
                ) {
                    Text(
                        text = when (task.status) {
                            TaskStatus.RUNNING -> "进行中"
                            TaskStatus.COMPLETED -> "已完成"
                            TaskStatus.FAILED -> "失败"
                        },
                        style = MaterialTheme.typography.labelSmall,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp),
                        color = when (task.status) {
                            TaskStatus.RUNNING -> MaterialTheme.colorScheme.primary
                            TaskStatus.COMPLETED -> MaterialTheme.colorScheme.tertiary
                            TaskStatus.FAILED -> MaterialTheme.colorScheme.error
                        }
                    )
                }
            }

            // 进度条
            if (task.status == TaskStatus.RUNNING && task.total > 0) {
                Spacer(modifier = Modifier.height(8.dp))
                LinearProgressIndicator(
                    progress = { (task.current.toFloat() / task.total).coerceIn(0f, 1f) },
                    modifier = Modifier.fillMaxWidth(),
                )
                Text(
                    text = "${task.current} / ${task.total}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.padding(top = 4.dp)
                )
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

        // 监听连接状态
        viewModelScope.launch {
            webSocketManager.connectionState.collect { state ->
                _uiState.value = _uiState.value.copy(connectionState = state)
            }
        }

        // 监听扫描进度
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

        // 监听刮削进度
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

        // 监听转码进度
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

        // 监听通用事件（处理失败事件）
        viewModelScope.launch {
            webSocketManager.events.collect { event ->
                when (event.type) {
                    "scan_failed" -> markTaskFailed("scan_")
                    "transcode_failed" -> markTaskFailed("transcode_")
                }
            }
        }
    }

    fun stopListening() {
        // 不断开 WebSocket，让它在后台保持连接
    }

    private fun updateTask(
        id: String,
        type: TaskType,
        title: String,
        message: String,
        current: Int,
        total: Int,
        isCompleted: Boolean
    ) {
        val tasks = _uiState.value.tasks.toMutableList()
        val index = tasks.indexOfFirst { it.id == id }
        val task = TaskInfo(
            id = id,
            type = type,
            title = title,
            message = message,
            status = if (isCompleted) TaskStatus.COMPLETED else TaskStatus.RUNNING,
            current = current,
            total = total
        )
        if (index >= 0) {
            tasks[index] = task
        } else {
            tasks.add(0, task) // 新任务插入到顶部
        }
        _uiState.value = _uiState.value.copy(tasks = tasks)
    }

    private fun markTaskFailed(idPrefix: String) {
        val tasks = _uiState.value.tasks.map { task ->
            if (task.id.startsWith(idPrefix) && task.status == TaskStatus.RUNNING) {
                task.copy(status = TaskStatus.FAILED)
            } else task
        }
        _uiState.value = _uiState.value.copy(tasks = tasks)
    }
}
