package com.nowen.video.v2.feature.main

import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material.icons.filled.Key
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.BrandMark
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.NowenPage
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
    val serverName: String get() = store.snapshot.value.activeServer?.name ?: "Nowen Video"

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
    NowenPage {
        Spacer(Modifier.height(30.dp))
        BrandMark()
        Spacer(Modifier.height(42.dp))
        Text("欢迎回来", style = MaterialTheme.typography.headlineLarge)
        Spacer(Modifier.height(6.dp))
        Text("登录 ${viewModel.serverName}", color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.height(24.dp))
        ElevatedPanel(Modifier.fillMaxWidth()) {
            OutlinedTextField(
                value = state.username,
                onValueChange = viewModel::username,
                label = { Text("用户名") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
            )
            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = state.password,
                onValueChange = viewModel::password,
                label = { Text("密码") },
                modifier = Modifier.fillMaxWidth(),
                singleLine = true,
                visualTransformation = PasswordVisualTransformation(),
            )
            state.error?.let {
                Spacer(Modifier.height(10.dp))
                Text(it, color = MaterialTheme.colorScheme.error)
            }
            Spacer(Modifier.height(18.dp))
            Button(
                onClick = viewModel::login,
                enabled = !state.loading,
                modifier = Modifier.fillMaxWidth(),
            ) {
                if (state.loading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                else Text("进入媒体空间")
            }
            Spacer(Modifier.height(8.dp))
            TextButton(onClick = viewModel::changeServer, modifier = Modifier.fillMaxWidth()) {
                Icon(Icons.Default.ArrowBack, null)
                Spacer(Modifier.width(6.dp))
                Text("更换服务器")
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
    NowenPage {
        Spacer(Modifier.height(36.dp))
        BrandMark(compact = true)
        Spacer(Modifier.height(36.dp))
        Icon(Icons.Default.Key, null, tint = MaterialTheme.colorScheme.primary)
        Spacer(Modifier.height(12.dp))
        Text("保护你的账号", style = MaterialTheme.typography.headlineLarge)
        Spacer(Modifier.height(8.dp))
        Text(
            "首次登录需要修改初始密码，完成后会自动更新当前会话。",
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(24.dp))
        ElevatedPanel(Modifier.fillMaxWidth()) {
            PasswordField("当前密码", state.current, viewModel::current)
            Spacer(Modifier.height(12.dp))
            PasswordField("新密码", state.next, viewModel::next)
            Spacer(Modifier.height(12.dp))
            PasswordField("确认新密码", state.confirm, viewModel::confirm)
            state.error?.let {
                Spacer(Modifier.height(10.dp))
                Text(it, color = MaterialTheme.colorScheme.error)
            }
            Spacer(Modifier.height(18.dp))
            Button(
                onClick = viewModel::submit,
                enabled = !state.loading,
                modifier = Modifier.fillMaxWidth(),
            ) {
                if (state.loading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                else Text("修改密码并继续")
            }
        }
    }
}

@Composable
private fun PasswordField(label: String, value: String, onChange: (String) -> Unit) {
    OutlinedTextField(
        value = value,
        onValueChange = onChange,
        label = { Text(label) },
        modifier = Modifier.fillMaxWidth(),
        singleLine = true,
        visualTransformation = PasswordVisualTransformation(),
    )
}
