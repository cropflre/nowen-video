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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
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

    init {
        viewModelScope.launch {
            legacyV1Migration.migrateIfNeeded()
            sessionStore.bootstrap()
        }
    }
}

@Composable
fun NowenV2App(viewModel: AppViewModel = hiltViewModel()) {
    val session by viewModel.session.collectAsState()

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
                RootDestination.Server -> ServerSetupWithUpgradeNotice()
                RootDestination.Login -> LoginScreen()
                RootDestination.Password -> ForcePasswordScreen()
                RootDestination.Main -> MainShell()
            }
        }
    }
}

@Composable
private fun ServerSetupWithUpgradeNotice() {
    var showUpgradeNotice by rememberSaveable { mutableStateOf(true) }

    ServerSetupScreen()
    if (showUpgradeNotice) {
        AlertDialog(
            onDismissRequest = { showUpgradeNotice = false },
            title = { Text("Android 客户端已升级") },
            text = {
                Text(
                    "新版 Android 客户端已经正式替换旧版。升级安装时会尽量自动导入旧版服务器地址，" +
                        "但出于安全考虑不会迁移旧版明文 Token 或密码；如未保持登录状态，请重新登录。",
                )
            },
            confirmButton = {
                TextButton(onClick = { showUpgradeNotice = false }) {
                    Text("我知道了")
                }
            },
        )
    }
}

private enum class RootDestination { Loading, Server, Login, Password, Main }
