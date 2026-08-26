package com.nowen.video.v2.feature.main

import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Key
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.BrandMark
import com.nowen.video.v2.core.designsystem.HillsPrimaryAction
import com.nowen.video.v2.core.designsystem.HillsSecondaryAction
import com.nowen.video.v2.core.designsystem.HillsScreen
import com.nowen.video.v2.core.designsystem.HillsTextField
import com.nowen.video.v2.core.designsystem.HillsTopBar
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class LoginUiState(
    val username: String = "",
    val password: String = "",
    val loading: Boolean = false,
    val error: String? = null,
)

@HiltViewModel
class LoginViewModel @Inject constructor(
    private val repository: NowenRepository,
    private val store: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(LoginUiState())
    val state: StateFlow<LoginUiState> = _state
    val serverName: String get() = store.snapshot.value.activeServer?.name ?: "Nowen 服务器"

    fun username(value: String) = _state.update { it.copy(username = value, error = null) }
    fun password(value: String) = _state.update { it.copy(password = value, error = null) }

    fun login() {
        val current = _state.value
        if (current.username.isBlank() || current.password.isBlank()) {
            _state.update { it.copy(error = "请输入用户名和密码") }
            return
        }
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null) }
            repository.login(current.username, current.password)
                .onFailure { error -> _state.update { it.copy(error = error.message ?: "登录失败") } }
            _state.update { it.copy(loading = false) }
        }
    }

    fun changeServer() {
        viewModelScope.launch { store.deactivate() }
    }
}

@Composable
fun LoginScreen(viewModel: LoginViewModel = hiltViewModel()) {
    val state by viewModel.state.collectAsState()
    Dialog(
        onDismissRequest = viewModel::changeServer,
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .padding(22.dp),
            shape = MaterialTheme.shapes.extraLarge,
            color = MaterialTheme.colorScheme.surface,
            border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
        ) {
            Column(Modifier.padding(24.dp)) {
                Text("登录服务器", style = MaterialTheme.typography.headlineMedium)
                Spacer(Modifier.height(6.dp))
                Text(
                    viewModel.serverName,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodyLarge,
                )
                Spacer(Modifier.height(22.dp))
                HillsTextField(
                    value = state.username,
                    onValueChange = viewModel::username,
                    placeholder = "用户名",
                )
                Spacer(Modifier.height(12.dp))
                HillsTextField(
                    value = state.password,
                    onValueChange = viewModel::password,
                    placeholder = "密码",
                    visualTransformation = PasswordVisualTransformation(),
                )
                state.error?.let {
                    Spacer(Modifier.height(10.dp))
                    Text(it, color = MaterialTheme.colorScheme.error)
                }
                Spacer(Modifier.height(20.dp))
                HillsPrimaryAction(
                    label = if (state.loading) "正在登录" else "登录",
                    onClick = viewModel::login,
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(Modifier.height(6.dp))
                HillsSecondaryAction(
                    label = "更换服务器",
                    icon = Icons.Default.ArrowBack,
                    onClick = viewModel::changeServer,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
    }
}

data class PasswordUiState(
    val current: String = "",
    val next: String = "",
    val confirm: String = "",
    val loading: Boolean = false,
    val error: String? = null,
)

@HiltViewModel
class PasswordViewModel @Inject constructor(
    private val repository: NowenRepository,
) : ViewModel() {
    private val _state = MutableStateFlow(PasswordUiState())
    val state: StateFlow<PasswordUiState> = _state

    fun current(value: String) = _state.update { it.copy(current = value, error = null) }
    fun next(value: String) = _state.update { it.copy(next = value, error = null) }
    fun confirm(value: String) = _state.update { it.copy(confirm = value, error = null) }

    fun submit() {
        val value = _state.value
        val error = when {
            value.current.length < 6 -> "当前密码至少 6 位"
            value.next.length < 6 -> "新密码至少 6 位"
            value.next != value.confirm -> "两次输入的新密码不一致"
            value.current == value.next -> "新密码不能与当前密码相同"
            else -> null
        }
        if (error != null) {
            _state.update { it.copy(error = error) }
            return
        }
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null) }
            repository.changePassword(value.current, value.next)
                .onFailure { failure -> _state.update { it.copy(error = failure.message ?: "修改密码失败") } }
            _state.update { it.copy(loading = false) }
        }
    }
}

@Composable
fun ForcePasswordScreen(viewModel: PasswordViewModel = hiltViewModel()) {
    val state by viewModel.state.collectAsState()
    HillsScreen(top = { HillsTopBar(title = "保护账号") }) { topPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(topPadding)
                .padding(horizontal = 20.dp),
        ) {
            Spacer(Modifier.height(28.dp))
            BrandMark(compact = true)
            Spacer(Modifier.height(28.dp))
            Text("修改初始密码", style = MaterialTheme.typography.headlineLarge)
            Spacer(Modifier.height(8.dp))
            Text("首次登录需要修改初始密码，完成后会自动更新当前会话。", color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(24.dp))
            PasswordField("当前密码", state.current, viewModel::current)
            Spacer(Modifier.height(12.dp))
            PasswordField("新密码", state.next, viewModel::next)
            Spacer(Modifier.height(12.dp))
            PasswordField("确认新密码", state.confirm, viewModel::confirm)
            state.error?.let {
                Spacer(Modifier.height(10.dp))
                Text(it, color = MaterialTheme.colorScheme.error)
            }
            Spacer(Modifier.height(20.dp))
            HillsPrimaryAction(
                label = if (state.loading) "正在修改" else "修改密码并继续",
                onClick = viewModel::submit,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

@Composable
private fun PasswordField(label: String, value: String, onChange: (String) -> Unit) {
    HillsTextField(
        value = value,
        onValueChange = onChange,
        placeholder = label,
        visualTransformation = PasswordVisualTransformation(),
    )
}
