package com.nowen.video.v2.feature.main

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Wifi
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerDiscoveryManager
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.HillsEmptyState
import com.nowen.video.v2.core.designsystem.HillsPressable
import com.nowen.video.v2.core.designsystem.HillsPrimaryAction
import com.nowen.video.v2.core.designsystem.HillsSecondaryAction
import com.nowen.video.v2.core.designsystem.HillsTextField
import com.nowen.video.v2.core.designsystem.NowenMobileMetrics
import com.nowen.video.v2.core.designsystem.NowenMotion
import com.nowen.video.v2.core.model.DiscoveredServer
import com.nowen.video.v2.core.model.DiscoverySource
import com.nowen.video.v2.core.model.ServerProfile
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ServerSetupUiState(
    val servers: List<ServerProfile> = emptyList(),
    val activeServerId: String? = null,
    val discoveredServers: List<DiscoveredServer> = emptyList(),
    val address: String = "",
    val name: String = "",
    val loading: Boolean = false,
    val connectingUrl: String? = null,
    val isScanning: Boolean = false,
    val discoveryError: String? = null,
    val error: String? = null,
)

@HiltViewModel
class ServerSetupViewModel @Inject constructor(
    private val store: ServerSessionStore,
    private val repository: NowenRepository,
    private val discovery: ServerDiscoveryManager,
) : ViewModel() {
    private val form = MutableStateFlow(ServerSetupUiState())
    val state: StateFlow<ServerSetupUiState> = combine(
        store.snapshot,
        form,
        discovery.state,
    ) { session, local, discoveryState ->
        local.copy(
            servers = session.servers,
            activeServerId = session.activeServerId,
            discoveredServers = discoveryState.servers,
            isScanning = discoveryState.isScanning,
            discoveryError = discoveryState.error,
        )
    }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), ServerSetupUiState())

    fun address(value: String) = form.update { it.copy(address = value, error = null) }
    fun name(value: String) = form.update { it.copy(name = value, error = null) }

    fun startDiscovery() = discovery.startDiscovery(viewModelScope)
    fun stopDiscovery() = discovery.stopDiscovery()

    fun connect() {
        val current = form.value
        connectCandidate(current.address, current.name)
    }

    fun addDiscovered(server: DiscoveredServer) {
        connectCandidate(server.url, server.name)
    }

    fun addFromQr(rawValue: String) {
        val payload = discovery.parseQr(rawValue)
        if (payload == null) {
            form.update {
                it.copy(error = "无法识别服务器二维码。支持服务器 URL、Nowen JSON 或 nowen-video://server 链接。")
            }
            return
        }
        form.update { it.copy(address = payload.url, name = payload.name, error = null) }
        connectCandidate(payload.url, payload.name)
    }

    fun activate(serverId: String) {
        viewModelScope.launch { store.activate(serverId) }
    }

    fun remove(serverId: String) {
        viewModelScope.launch { store.remove(serverId) }
    }

    private fun connectCandidate(rawAddress: String, preferredName: String) {
        if (rawAddress.isBlank()) {
            form.update { it.copy(error = "请输入服务器地址") }
            return
        }
        if (form.value.loading) return
        viewModelScope.launch {
            form.update {
                it.copy(
                    loading = true,
                    connectingUrl = rawAddress.trim().trimEnd('/'),
                    error = null,
                )
            }
            repository.probe(rawAddress)
                .onSuccess { probe ->
                    store.saveServer(preferredName.ifBlank { probe.serverName }, rawAddress)
                    form.update {
                        it.copy(
                            loading = false,
                            connectingUrl = null,
                            address = "",
                            name = "",
                            error = null,
                        )
                    }
                    discovery.stopDiscovery()
                }
                .onFailure { error ->
                    form.update {
                        it.copy(
                            loading = false,
                            connectingUrl = null,
                            error = error.message ?: "连接失败",
                        )
                    }
                }
        }
    }

    override fun onCleared() {
        discovery.stopDiscovery()
        super.onCleared()
    }
}

@Composable
fun ServerSetupScreen(
    viewModel: ServerSetupViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    var showAdd by rememberSaveable { mutableStateOf(false) }
    var showScanner by rememberSaveable { mutableStateOf(false) }
    var removalTargetId by rememberSaveable { mutableStateOf<String?>(null) }

    LaunchedEffect(Unit) { viewModel.startDiscovery() }
    DisposableEffect(Unit) {
        onDispose { viewModel.stopDiscovery() }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .windowInsetsPadding(WindowInsets.safeDrawing),
    ) {
        Column(Modifier.fillMaxSize()) {
            ServerHubHeader(onAdd = { showAdd = true })
            if (state.servers.isEmpty()) {
                Box(Modifier.weight(1f).fillMaxWidth()) {
                    HillsEmptyState(
                        title = "还没有服务器",
                        message = "添加 Nowen 服务器，开始你的媒体之旅。",
                        modifier = Modifier
                            .align(Alignment.Center)
                            .padding(horizontal = 42.dp),
                    )
                    if (state.isScanning) {
                        Row(
                            modifier = Modifier
                                .align(Alignment.BottomCenter)
                                .padding(bottom = 138.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            CircularProgressIndicator(Modifier.size(16.dp), strokeWidth = 2.dp)
                            Spacer(Modifier.width(8.dp))
                            Text("正在查找附近服务器", color = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            } else {
                LazyColumn(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(
                        start = NowenMobileMetrics.PageHorizontal,
                        end = NowenMobileMetrics.PageHorizontal,
                        top = 10.dp,
                        bottom = 100.dp,
                    ),
                ) {
                    items(state.servers, key = ServerProfile::id) { server ->
                        HillsServerCard(
                            server = server,
                            selected = server.id == state.activeServerId,
                            onOpen = { viewModel.activate(server.id) },
                            onRemove = { removalTargetId = server.id },
                        )
                    }
                    if (state.discoveredServers.isNotEmpty()) {
                        item { Text("附近发现", style = MaterialTheme.typography.titleLarge) }
                        items(state.discoveredServers, key = DiscoveredServer::uniqueKey) { server ->
                            DiscoveredServerRow(
                                server = server,
                                connecting = state.connectingUrl?.let(::normalizedServerKey) == normalizedServerKey(server.url),
                                onClick = { viewModel.addDiscovered(server) },
                            )
                        }
                    }
                    state.discoveryError?.let { error ->
                        item { Text(error, color = MaterialTheme.colorScheme.error) }
                    }
                }
            }
        }

        FloatingActionButton(
            onClick = { showAdd = true },
            modifier = Modifier
                .align(Alignment.BottomEnd)
                .padding(end = 22.dp, bottom = 24.dp)
                .size(NowenMobileMetrics.FloatingActionSize),
            shape = RoundedCornerShape(18.dp),
            containerColor = MaterialTheme.colorScheme.primary,
            contentColor = MaterialTheme.colorScheme.onPrimary,
        ) {
            Icon(Icons.Default.Add, contentDescription = "添加服务器")
        }
    }

    if (showAdd) {
        AddServerDialog(
            state = state,
            onNameChange = viewModel::name,
            onAddressChange = viewModel::address,
            onConnect = viewModel::connect,
            onScanQr = { showScanner = true },
            onRefreshDiscovery = viewModel::startDiscovery,
            onDiscoveredClick = viewModel::addDiscovered,
            onDismiss = { if (!state.loading) showAdd = false },
        )
    }

    if (showScanner) {
        QrScannerDialog(
            onDismiss = { showScanner = false },
            onResult = { rawValue ->
                showScanner = false
                showAdd = true
                viewModel.addFromQr(rawValue)
            },
        )
    }

    removalTargetId?.let { serverId ->
        val name = state.servers.firstOrNull { it.id == serverId }?.name ?: "这台服务器"
        AlertDialog(
            onDismissRequest = { removalTargetId = null },
            title = { Text("移除服务器？") },
            text = { Text("将移除 $name 的本地登录信息，此操作不会影响服务器上的媒体。") },
            confirmButton = {
                TextButton(onClick = {
                    viewModel.remove(serverId)
                    removalTargetId = null
                }) { Text("移除") }
            },
            dismissButton = { TextButton(onClick = { removalTargetId = null }) { Text("取消") } },
        )
    }
}

@Composable
private fun ServerHubHeader(onAdd: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = NowenMobileMetrics.PageHorizontal, vertical = 18.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text("服务器", style = MaterialTheme.typography.headlineLarge, modifier = Modifier.weight(1f))
        HillsPressable(onClick = onAdd, modifier = Modifier.size(44.dp)) {
            Icon(
                Icons.Default.Add,
                contentDescription = "添加服务器",
                tint = MaterialTheme.colorScheme.onSurface,
                modifier = Modifier.align(Alignment.Center),
            )
        }
    }
}

@Composable
private fun HillsServerCard(
    server: ServerProfile,
    selected: Boolean,
    onOpen: () -> Unit,
    onRemove: () -> Unit,
) {
    var menuOpen by rememberSaveable(server.id) { mutableStateOf(false) }
    HillsPressable(onClick = onOpen, modifier = Modifier.fillMaxWidth()) {
        Surface(
            shape = RoundedCornerShape(NowenMobileMetrics.SectionRadius),
            color = if (selected) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surface,
            border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
        ) {
            Row(
                modifier = Modifier.padding(16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    modifier = Modifier
                        .size(52.dp)
                        .clip(RoundedCornerShape(16.dp))
                        .background(MaterialTheme.colorScheme.primaryContainer),
                    contentAlignment = Alignment.Center,
                ) {
                    Icon(Icons.Default.Dns, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
                }
                Spacer(Modifier.width(14.dp))
                Column(Modifier.weight(1f)) {
                    Text(server.name, style = MaterialTheme.typography.titleLarge, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Spacer(Modifier.height(3.dp))
                    Text(server.baseUrl, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    server.serverVersion?.takeIf(String::isNotBlank)?.let {
                        Spacer(Modifier.height(4.dp))
                        Text("Nowen $it", style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.primary)
                    }
                }
                Box {
                    IconButton(onClick = { menuOpen = true }) {
                        Icon(Icons.Default.MoreVert, contentDescription = "服务器操作")
                    }
                    DropdownMenu(expanded = menuOpen, onDismissRequest = { menuOpen = false }) {
                        DropdownMenuItem(
                            text = { Text("移除") },
                            leadingIcon = { Icon(Icons.Default.DeleteOutline, contentDescription = null) },
                            onClick = {
                                menuOpen = false
                                onRemove()
                            },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun AddServerDialog(
    state: ServerSetupUiState,
    onNameChange: (String) -> Unit,
    onAddressChange: (String) -> Unit,
    onConnect: () -> Unit,
    onScanQr: () -> Unit,
    onRefreshDiscovery: () -> Unit,
    onDiscoveredClick: (DiscoveredServer) -> Unit,
    onDismiss: () -> Unit,
) {
    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .widthIn(max = 440.dp)
                .padding(22.dp)
                .imePadding(),
            shape = RoundedCornerShape(NowenMobileMetrics.ModalRadius),
            color = MaterialTheme.colorScheme.surface,
            border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
        ) {
            Column(Modifier.padding(24.dp)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text("连接服务器", modifier = Modifier.weight(1f), style = MaterialTheme.typography.headlineMedium)
                    IconButton(onClick = onDismiss, enabled = !state.loading) {
                        Icon(Icons.Default.Close, contentDescription = "关闭")
                    }
                }
                Spacer(Modifier.height(18.dp))
                HillsTextField(
                    value = state.address,
                    onValueChange = onAddressChange,
                    placeholder = "服务器地址，例如 https://media.example.com",
                )
                Spacer(Modifier.height(8.dp))
                Text("可以直接填写完整 Nowen 地址，端口和路径包含在地址中。", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Spacer(Modifier.height(14.dp))
                HillsTextField(
                    value = state.name,
                    onValueChange = onNameChange,
                    placeholder = "显示名称（可选）",
                )
                state.error?.let {
                    Spacer(Modifier.height(10.dp))
                    Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium)
                }
                Spacer(Modifier.height(18.dp))
                HillsPrimaryAction(
                    label = if (state.loading) "正在连接" else "连接服务器",
                    onClick = onConnect,
                    modifier = Modifier.fillMaxWidth(),
                    enabled = !state.loading,
                )
                Spacer(Modifier.height(12.dp))
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    HillsSecondaryAction(
                        label = "扫码",
                        icon = Icons.Default.QrCodeScanner,
                        onClick = onScanQr,
                        modifier = Modifier.weight(1f),
                    )
                    HillsSecondaryAction(
                        label = if (state.isScanning) "扫描中" else "局域网",
                        icon = if (state.isScanning) Icons.Default.Wifi else Icons.Default.Refresh,
                        onClick = onRefreshDiscovery,
                        enabled = !state.isScanning,
                        modifier = Modifier.weight(1f),
                    )
                }
                if (state.discoveredServers.isNotEmpty()) {
                    Spacer(Modifier.height(20.dp))
                    Text("附近服务器", style = MaterialTheme.typography.titleMedium)
                    Spacer(Modifier.height(8.dp))
                    state.discoveredServers.take(4).forEach { server ->
                        DiscoveredServerRow(
                            server = server,
                            connecting = state.connectingUrl?.let(::normalizedServerKey) == normalizedServerKey(server.url),
                            onClick = { onDiscoveredClick(server) },
                        )
                        Spacer(Modifier.height(8.dp))
                    }
                }
            }
        }
    }
}

@Composable
private fun DiscoveredServerRow(
    server: DiscoveredServer,
    connecting: Boolean,
    onClick: () -> Unit,
) {
    HillsPressable(onClick = onClick, enabled = !connecting, modifier = Modifier.fillMaxWidth()) {
        Surface(
            shape = RoundedCornerShape(12.dp),
            color = MaterialTheme.colorScheme.surfaceVariant,
        ) {
            Row(
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(Icons.Default.Wifi, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
                Spacer(Modifier.width(10.dp))
                Column(Modifier.weight(1f)) {
                    Text(server.name, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Text(server.url, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
                if (connecting) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                else Icon(Icons.Default.Add, contentDescription = "添加")
            }
        }
    }
}

internal fun discoverySourceLabel(source: DiscoverySource): String = when (source) {
    DiscoverySource.MDNS -> "mDNS 自动发现"
    DiscoverySource.HTTP_SWEEP -> "局域网端口探测"
    DiscoverySource.QR -> "二维码"
}

internal fun normalizedServerKey(value: String): String = value.trim().trimEnd('/').lowercase()
