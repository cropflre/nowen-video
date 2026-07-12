package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.BrandMark
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.SectionTitle
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
    val address: String = "",
    val name: String = "",
    val loading: Boolean = false,
    val error: String? = null,
)

@HiltViewModel
class ServerSetupViewModel @Inject constructor(
    private val store: ServerSessionStore,
    private val repository: NowenRepository,
) : ViewModel() {
    private val form = MutableStateFlow(ServerSetupUiState())
    val state: StateFlow<ServerSetupUiState> = combine(store.snapshot, form) { session, local ->
        local.copy(servers = session.servers)
    }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), ServerSetupUiState())

    fun address(value: String) = form.update { it.copy(address = value, error = null) }
    fun name(value: String) = form.update { it.copy(name = value, error = null) }

    fun connect() {
        val current = form.value
        if (current.address.isBlank()) {
            form.update { it.copy(error = "请输入服务器地址") }
            return
        }
        viewModelScope.launch {
            form.update { it.copy(loading = true, error = null) }
            repository.probe(current.address)
                .onSuccess { probe ->
                    store.saveServer(current.name.ifBlank { probe.serverName }, current.address)
                    form.update { it.copy(loading = false, address = "", name = "") }
                }
                .onFailure { error ->
                    form.update { it.copy(loading = false, error = error.message ?: "连接失败") }
                }
        }
    }

    fun activate(serverId: String) {
        viewModelScope.launch { store.activate(serverId) }
    }

    fun remove(serverId: String) {
        viewModelScope.launch { store.remove(serverId) }
    }
}

@Composable
fun ServerSetupScreen(viewModel: ServerSetupViewModel = hiltViewModel()) {
    val state by viewModel.state.collectAsState()

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
                "自动保存多个 Nowen Video 服务器。每台服务器拥有独立、安全的登录会话。",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(Modifier.height(24.dp))
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
                        Icon(Icons.Default.Add, null)
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
            items(state.servers, key = { it.id }) { server ->
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
                        Icon(Icons.Default.Dns, null, tint = MaterialTheme.colorScheme.primary)
                        Spacer(Modifier.width(14.dp))
                        Column(Modifier.weight(1f)) {
                            Text(server.name, style = MaterialTheme.typography.titleMedium)
                            Text(
                                server.baseUrl,
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
}
