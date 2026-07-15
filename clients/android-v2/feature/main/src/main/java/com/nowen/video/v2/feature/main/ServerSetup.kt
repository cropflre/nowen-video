package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.weight
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.QrCodeScanner
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Wifi
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerDiscoveryManager
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.BrandMark
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.SectionTitle
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
                it.copy(
                    error = "无法识别服务器二维码。支持服务器 URL、Nowen JSON 或 nowen-video://server 链接。",
                )
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
fun ServerSetupScreen(viewModel: ServerSetupViewModel = hiltViewModel()) {
    val state by viewModel.state.collectAsState()
    var showScanner by rememberSaveable { mutableStateOf(false) }

    LaunchedEffect(Unit) { viewModel.startDiscovery() }
    DisposableEffect(Unit) {
        onDispose { viewModel.stopDiscovery() }
    }

    LazyColumn(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .windowInsetsPadding(WindowInsets.safeDrawing),
        contentPadding = PaddingValues(horizontal = 20.dp, vertical = 28.dp),
    ) {
        item {
            BrandMark()
            Spacer(Modifier.height(34.dp))
            Text("连接你的媒体空间", style = MaterialTheme.typography.headlineLarge)
            Spacer(Modifier.height(8.dp))
            Text(
                "自动发现同一局域网内的 Nowen Video，也可以扫码或手动添加。每台服务器拥有独立、安全的登录会话。",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(22.dp))
            DiscoveryActions(
                isScanning = state.isScanning,
                onScanQr = { showScanner = true },
                onRefresh = viewModel::startDiscovery,
            )
        }

        if (state.isScanning || state.discoveredServers.isNotEmpty() || state.discoveryError != null) {
            item {
                Spacer(Modifier.height(24.dp))
                SectionTitle(
                    title = "附近的服务器",
                    subtitle = if (state.isScanning) "正在通过 mDNS 和局域网探测查找" else "点击即可检测并添加",
                )
                if (state.isScanning) {
                    Spacer(Modifier.height(10.dp))
                    LinearProgressIndicator(Modifier.fillMaxWidth())
                }
                state.discoveryError?.let { error ->
                    Spacer(Modifier.height(8.dp))
                    Text(error, color = MaterialTheme.colorScheme.error)
                }
                Spacer(Modifier.height(12.dp))
            }
            if (state.discoveredServers.isEmpty() && !state.isScanning) {
                item {
                    ElevatedPanel(Modifier.fillMaxWidth()) {
                        Text("没有发现服务器", style = MaterialTheme.typography.titleMedium)
                        Spacer(Modifier.height(6.dp))
                        Text(
                            "确认手机和服务器位于同一局域网，或使用二维码和手动地址连接。",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            } else {
                items(state.discoveredServers, key = DiscoveredServer::uniqueKey) { server ->
                    val saved = state.servers.firstOrNull {
                        normalizedServerKey(it.baseUrl) == normalizedServerKey(server.url)
                    }
                    DiscoveredServerCard(
                        server = server,
                        saved = saved != null,
                        connecting = state.connectingUrl?.let(::normalizedServerKey) == normalizedServerKey(server.url),
                        onClick = {
                            if (saved != null) viewModel.activate(saved.id) else viewModel.addDiscovered(server)
                        },
                    )
                    Spacer(Modifier.height(10.dp))
                }
            }
        }

        item {
            Spacer(Modifier.height(26.dp))
            SectionTitle("手动添加", "支持 HTTP、HTTPS、域名、IPv4 和 IPv6 地址")
            Spacer(Modifier.height(12.dp))
            ElevatedPanel(Modifier.fillMaxWidth()) {
                OutlinedTextField(
                    value = state.name,
                    onValueChange = viewModel::name,
                    modifier = Modifier.fillMaxWidth(),
                    label = { Text("服务器名称（可选）") },
                    singleLine = true,
                )
                Spacer(Modifier.height(12.dp))
                OutlinedTextField(
                    value = state.address,
                    onValueChange = viewModel::address,
                    modifier = Modifier.fillMaxWidth(),
                    label = { Text("服务器地址") },
                    placeholder = { Text("http://192.168.1.10:8080") },
                    singleLine = true,
                )
                state.error?.let {
                    Spacer(Modifier.height(10.dp))
                    Text(it, color = MaterialTheme.colorScheme.error)
                }
                Spacer(Modifier.height(16.dp))
                Button(
                    onClick = viewModel::connect,
                    enabled = !state.loading,
                    modifier = Modifier.fillMaxWidth(),
                ) {
                    if (state.loading) {
                        CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                        Spacer(Modifier.width(10.dp))
                        Text("检测中")
                    } else {
                        Icon(Icons.Default.Add, contentDescription = null)
                        Spacer(Modifier.width(8.dp))
                        Text("检测并添加")
                    }
                }
            }
        }

        if (state.servers.isNotEmpty()) {
            item {
                Spacer(Modifier.height(28.dp))
                SectionTitle("已保存的服务器", "点击即可切换并登录")
                Spacer(Modifier.height(12.dp))
            }
            items(state.servers, key = ServerProfile::id) { server ->
                Surface(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(bottom = 10.dp)
                        .clickable { viewModel.activate(server.id) },
                    shape = MaterialTheme.shapes.medium,
                    color = MaterialTheme.colorScheme.surface,
                    tonalElevation = 1.dp,
                ) {
                    Row(Modifier.padding(16.dp), verticalAlignment = Alignment.CenterVertically) {
                        Icon(Icons.Default.Dns, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
                        Spacer(Modifier.width(14.dp))
                        Column(Modifier.weight(1f)) {
                            Text(server.name, style = MaterialTheme.typography.titleMedium)
                            Text(
                                server.baseUrl,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis,
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                        TextButton(onClick = { viewModel.remove(server.id) }) { Text("删除") }
                    }
                }
            }
        }
        item { Spacer(Modifier.height(24.dp)) }
    }

    if (showScanner) {
        QrScannerDialog(
            onDismiss = { showScanner = false },
            onResult = { rawValue ->
                showScanner = false
                viewModel.addFromQr(rawValue)
            },
        )
    }
}

@Composable
private fun DiscoveryActions(
    isScanning: Boolean,
    onScanQr: () -> Unit,
    onRefresh: () -> Unit,
) {
    ElevatedPanel(Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            OutlinedButton(
                onClick = onScanQr,
                modifier = Modifier.weight(1f),
            ) {
                Icon(Icons.Default.QrCodeScanner, contentDescription = null)
                Spacer(Modifier.width(8.dp))
                Text("扫描二维码")
            }
            FilledTonalButton(
                onClick = onRefresh,
                enabled = !isScanning,
                modifier = Modifier.weight(1f),
            ) {
                Icon(if (isScanning) Icons.Default.Wifi else Icons.Default.Refresh, contentDescription = null)
                Spacer(Modifier.width(8.dp))
                Text(if (isScanning) "扫描中" else "刷新局域网")
            }
        }
        Spacer(Modifier.height(10.dp))
        Text(
            "优先使用服务器的 mDNS 广播；未收到广播时会自动扫描当前私有网段的常用端口。",
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            style = MaterialTheme.typography.bodyMedium,
        )
    }
}

@Composable
private fun DiscoveredServerCard(
    server: DiscoveredServer,
    saved: Boolean,
    connecting: Boolean,
    onClick: () -> Unit,
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(enabled = !connecting, onClick = onClick),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surface,
        tonalElevation = 1.dp,
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                modifier = Modifier
                    .size(42.dp)
                    .background(MaterialTheme.colorScheme.surfaceVariant, MaterialTheme.shapes.medium),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    if (saved) Icons.Default.CheckCircle else Icons.Default.Wifi,
                    contentDescription = null,
                    tint = if (saved) MaterialTheme.colorScheme.secondary else MaterialTheme.colorScheme.primary,
                )
            }
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f)) {
                Text(server.name, style = MaterialTheme.typography.titleMedium)
                Text(
                    server.url,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodyMedium,
                )
                Text(
                    buildString {
                        append(discoverySourceLabel(server.source))
                        if (server.version.isNotBlank()) append(" · v${server.version}")
                    },
                    color = MaterialTheme.colorScheme.primary,
                    style = MaterialTheme.typography.bodyMedium,
                )
            }
            if (connecting) {
                CircularProgressIndicator(Modifier.size(22.dp), strokeWidth = 2.dp)
            } else {
                Text(if (saved) "打开" else "添加", color = MaterialTheme.colorScheme.primary)
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
