package com.nowen.video.v2.feature.main

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
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
) : ViewModel() {
    val session: StateFlow<SessionSnapshot> = sessionStore.snapshot.stateIn(
        viewModelScope,
        SharingStarted.Eagerly,
        SessionSnapshot(),
    )

    init {
        viewModelScope.launch { sessionStore.bootstrap() }
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
                RootDestination.Server -> ServerSetupScreen()
                RootDestination.Login -> LoginScreen()
                RootDestination.Password -> ForcePasswordScreen()
                RootDestination.Main -> MainShell()
            }
        }
    }
}

private enum class RootDestination { Loading, Server, Login, Password, Main }
