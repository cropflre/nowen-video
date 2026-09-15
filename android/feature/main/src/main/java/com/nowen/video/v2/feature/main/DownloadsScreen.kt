package com.nowen.video.v2.feature.main

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Storage
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.OfflineDownloadRepository
import com.nowen.video.v2.core.designsystem.HillsChoiceRail
import com.nowen.video.v2.core.designsystem.HillsPrimaryAction
import com.nowen.video.v2.core.designsystem.HillsSecondaryAction
import com.nowen.video.v2.core.designsystem.HillsToggle
import com.nowen.video.v2.core.designsystem.HillsScreen
import com.nowen.video.v2.core.designsystem.HillsState
import com.nowen.video.v2.core.designsystem.HillsTopBar
import com.nowen.video.v2.core.model.OfflineDownloadPolicy
import com.nowen.video.v2.core.model.OfflineDownloadRecord
import com.nowen.video.v2.core.model.OfflineDownloadStatus
import com.nowen.video.v2.core.model.OfflineStorageStats
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch

private val DOWNLOAD_QUOTA_OPTIONS_GIB = listOf(5, 10, 20, 50, 100)

data class DownloadsUiState(
    val records: List<OfflineDownloadRecord> = emptyList(),
    val policy: OfflineDownloadPolicy = OfflineDownloadPolicy(),
    val storage: OfflineStorageStats = OfflineStorageStats(),
    val message: String? = null,
)

@HiltViewModel
class DownloadsViewModel @Inject constructor(
    private val repository: OfflineDownloadRepository,
) : ViewModel() {
    private val message = MutableStateFlow<String?>(null)

    val state: StateFlow<DownloadsUiState> = combine(
        repository.downloads,
        repository.policy,
        repository.storageStats,
        message,
    ) { records, policy, storage, currentMessage ->
        DownloadsUiState(records, policy, storage, currentMessage)
    }.stateIn(
        scope = viewModelScope,
        started = SharingStarted.WhileSubscribed(5_000),
        initialValue = DownloadsUiState(),
    )

    init {
        viewModelScope.launch { repository.reconcileActiveDownloads() }
    }

    fun pause(id: String) = runAction("下载已暂停") { repository.pause(id) }

    fun resume(id: String) = runAction("下载已继续") { repository.resume(id) }

    fun retry(id: String) = runAction("已重新加入下载队列") { repository.retry(id) }

    fun delete(id: String) = runAction("离线文件已删除") { repository.delete(id) }

    fun clearCompleted() {
        viewModelScope.launch {
            repository.clearCompleted()
                .onSuccess { count -> message.value = "已清理 $count 个离线文件" }
                .onFailure { error -> message.value = error.message ?: "清理失败" }
        }
    }

    fun setWifiOnly(enabled: Boolean) {
        viewModelScope.launch {
            repository.setWifiOnly(enabled)
            message.value = if (enabled) "仅在 Wi‑Fi / 非计费网络下载" else "已允许移动网络下载"
        }
    }

    fun setQuotaGiB(gib: Int) {
        viewModelScope.launch {
            repository.setMaxBytes(gib.toLong() * 1024L * 1024L * 1024L)
            message.value = "离线空间上限已设为 $gib GB"
        }
    }

    fun dismissMessage() {
        message.value = null
    }

    private fun runAction(successMessage: String, action: suspend () -> Result<Unit>) {
        viewModelScope.launch {
            action()
                .onSuccess { message.value = successMessage }
                .onFailure { error -> message.value = error.message ?: "操作失败" }
        }
    }
}

@Composable
fun DownloadsScreen(
    modifier: Modifier = Modifier,
    onBack: () -> Unit,
    onPlayOffline: (String) -> Unit,
    viewModel: DownloadsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()

    HillsScreen(
        modifier,
        top = {
            HillsTopBar(
                title = "下载",
                subtitle = "离线媒体与空间管理",
                navigationIcon = Icons.AutoMirrored.Filled.ArrowBack,
                onNavigate = onBack,
            )
        },
    ) { topPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(topPadding)
                .padding(horizontal = 20.dp),
        ) {
            DownloadStoragePanel(
                state = state,
                onWifiOnlyChange = viewModel::setWifiOnly,
                onQuotaChange = viewModel::setQuotaGiB,
                onClearCompleted = viewModel::clearCompleted,
            )
            state.message?.let { currentMessage ->
                Spacer(Modifier.height(10.dp))
                Text(currentMessage, color = MaterialTheme.colorScheme.primary)
            }
            Spacer(Modifier.height(18.dp))
            if (state.records.isEmpty()) {
                HillsState(
                    title = "还没有离线内容",
                    message = "在影片详情页下载媒体，任务会在满足网络和存储条件后自动开始。",
                )
            } else {
                LazyColumn(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                    contentPadding = PaddingValues(bottom = 20.dp),
                ) {
                    items(state.records, key = OfflineDownloadRecord::id) { record ->
                        DownloadTaskCard(
                            record = record,
                            onPause = { viewModel.pause(record.id) },
                            onResume = { viewModel.resume(record.id) },
                            onRetry = { viewModel.retry(record.id) },
                            onPlay = { onPlayOffline(record.mediaId) },
                            onDelete = { viewModel.delete(record.id) },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun DownloadStoragePanel(
    state: DownloadsUiState,
    onWifiOnlyChange: (Boolean) -> Unit,
    onQuotaChange: (Int) -> Unit,
    onClearCompleted: () -> Unit,
) {
    Column(Modifier.fillMaxWidth().padding(vertical = 8.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Default.Storage, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text("离线空间", style = MaterialTheme.typography.titleLarge)
                Text(
                    "已用 ${formatBytes(state.storage.usedBytes)} / ${formatBytes(state.storage.quotaBytes)}",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            HillsSecondaryAction(
                label = "清理已完成",
                onClick = onClearCompleted,
                enabled = state.records.any { it.status == OfflineDownloadStatus.Completed },
                modifier = Modifier.width(118.dp),
            )
        }
        Spacer(Modifier.height(12.dp))
        LinearProgressIndicator(
            progress = { state.storage.quotaProgress },
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(8.dp))
        Text(
            "配额剩余 ${formatBytes(state.storage.remainingQuotaBytes)} · 设备可用 ${formatBytes(state.storage.deviceFreeBytes)}",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(14.dp))
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.weight(1f)) {
                Text("仅 Wi‑Fi 下载", style = MaterialTheme.typography.titleMedium)
                Text(
                    "开启后任务只在非计费网络执行",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            HillsToggle(checked = state.policy.wifiOnly, onCheckedChange = onWifiOnlyChange)
        }
        Spacer(Modifier.height(12.dp))
        HillsChoiceRail(
            options = DOWNLOAD_QUOTA_OPTIONS_GIB.map { it.toString() to "$it GB" },
            selected = (state.policy.maxBytes / (1024L * 1024L * 1024L)).toString(),
            onSelect = { value -> onQuotaChange(value.toInt()) },
        )
    }
}

@Composable
private fun DownloadTaskCard(
    record: OfflineDownloadRecord,
    onPause: () -> Unit,
    onResume: () -> Unit,
    onRetry: () -> Unit,
    onPlay: () -> Unit,
    onDelete: () -> Unit,
) {
    Column(Modifier.fillMaxWidth().padding(vertical = 12.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(
                    record.title,
                    style = MaterialTheme.typography.titleMedium,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    downloadStatusLabel(record.status),
                    color = if (record.status == OfflineDownloadStatus.Failed) {
                        MaterialTheme.colorScheme.error
                    } else {
                        MaterialTheme.colorScheme.onSurfaceVariant
                    },
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
            IconButton(onClick = onDelete) {
                Icon(Icons.Default.Delete, contentDescription = "删除下载")
            }
        }

        if (record.status != OfflineDownloadStatus.Completed || record.totalBytes > 0L) {
            Spacer(Modifier.height(10.dp))
            LinearProgressIndicator(
                progress = { record.progress },
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(6.dp))
            Text(
                if (record.totalBytes > 0L) {
                    "${formatBytes(record.downloadedBytes)} / ${formatBytes(record.totalBytes)} · ${(record.progress * 100).toInt()}%"
                } else {
                    "已接收 ${formatBytes(record.downloadedBytes)}"
                },
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }

        if (record.error.isNotBlank()) {
            Spacer(Modifier.height(8.dp))
            Text(record.error, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium)
        }

        Spacer(Modifier.height(12.dp))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.End,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            val (label, icon, action) = when (record.status) {
                OfflineDownloadStatus.Downloading, OfflineDownloadStatus.Queued -> Triple("暂停", Icons.Default.Pause, onPause)
                OfflineDownloadStatus.Paused -> Triple("继续", Icons.Default.PlayArrow, onResume)
                OfflineDownloadStatus.Failed -> Triple("重试", Icons.Default.Refresh, onRetry)
                OfflineDownloadStatus.Completed -> Triple("离线播放", Icons.Default.PlayArrow, onPlay)
            }
            HillsPrimaryAction(label = label, icon = icon, onClick = action, modifier = Modifier.width(126.dp))
        }
    }
}

internal fun downloadStatusLabel(status: OfflineDownloadStatus): String = when (status) {
    OfflineDownloadStatus.Queued -> "等待网络与存储条件"
    OfflineDownloadStatus.Downloading -> "正在下载"
    OfflineDownloadStatus.Paused -> "已暂停，可断点继续"
    OfflineDownloadStatus.Completed -> "已下载到本机"
    OfflineDownloadStatus.Failed -> "下载失败"
}

internal fun formatBytes(bytes: Long): String {
    val safe = bytes.coerceAtLeast(0L).toDouble()
    val kib = 1024.0
    val mib = kib * 1024.0
    val gib = mib * 1024.0
    return when {
        safe >= gib -> "%.1f GB".format(safe / gib)
        safe >= mib -> "%.1f MB".format(safe / mib)
        safe >= kib -> "%.1f KB".format(safe / kib)
        else -> "${safe.toLong()} B"
    }
}
