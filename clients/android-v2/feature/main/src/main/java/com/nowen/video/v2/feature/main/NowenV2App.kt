package com.nowen.video.v2.feature.main

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.LegacyV1Migration
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.NowenTheme
import com.nowen.video.v2.core.model.SessionSnapshot
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch

@HiltViewModel
class AppViewModel @Inject constructor(
    private val sessionStore: ServerSessionStore,
    private val legacyV1Migration: LegacyV1Migration,
) : ViewModel() {
    val session: StateFlow<SessionSnapshot> = sessionStore.snapshot.stateIn(
        viewModelScope,
        SharingStarted.Eagerly,
        SessionSnapshot(),
    )

    private val _showMigrationNotice = MutableStateFlow(false)
    val showMigrationNotice: StateFlow<Boolean> = _showMigrationNotice

    init {
        viewModelScope.launch {
            val migration = legacyV1Migration.migrateIfNeeded()
            // Fresh installs have no V1 state and should enter setup without an
            // irrelevant upgrade dialog. Only users for whom we actually
            // imported legacy server configuration need the migration notice.
            _showMigrationNotice.value = migration.requiresLogin
            sessionStore.bootstrap()
        }
    }

    fun dismissMigrationNotice() {
        _showMigrationNotice.value = false
    }
}

@Composable
fun NowenV2App(viewModel: AppViewModel = hiltViewModel()) {
    val session by viewModel.session.collectAsState()
    val showMigrationNotice by viewModel.showMigrationNotice.collectAsState()

    NowenTheme {
        AnimatedContent(
            targetState = when {
                !session.initialized -> RootDestination.Loading
                session.activeServer == null -> RootDestination.Server
                !session.isAuthenticated -> RootDestination.Login
                session.user?.mustChangePassword == true -> RootDestination.Password
                else -> RootDestination.Main
            },
            transitionSpec = { fadeIn() togetherWith fadeOut() },
            label = "root_destination",
        ) { destination ->
            when (destination) {
                RootDestination.Loading -> Box(
                    Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) { CircularProgressIndicator() }
                RootDestination.Server -> ServerSetupScreen()
                RootDestination.Login -> LoginScreen()
                RootDestination.Password -> ForcePasswordScreen()
                RootDestination.Main -> MainShell()
            }
        }

        if (showMigrationNotice) {
            AlertDialog(
                onDismissRequest = viewModel::dismissMigrationNotice,
                title = { Text("已迁移旧版服务器") },
                text = {
                    Text(
                        "新版 Android 客户端已经正式替换旧版，并已导入你的旧版服务器地址。" +
                            "出于安全考虑，旧版明文 Token 和密码不会迁移，请重新登录一次。",
                    )
                },
                confirmButton = {
                    TextButton(onClick = viewModel::dismissMigrationNotice) {
                        Text("我知道了")
                    }
                },
            )
        }
    }
}

private enum class RootDestination { Loading, Server, Login, Password, Main }
