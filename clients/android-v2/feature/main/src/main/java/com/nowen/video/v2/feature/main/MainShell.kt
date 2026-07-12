package com.nowen.video.v2.feature.main

import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material.icons.outlined.*
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.launch

enum class MainTab(val label: String, val icon: ImageVector, val selectedIcon: ImageVector) {
    Home("首页", Icons.Outlined.Home, Icons.Filled.Home),
    Library("媒体库", Icons.Outlined.VideoLibrary, Icons.Filled.VideoLibrary),
    Search("搜索", Icons.Outlined.Search, Icons.Filled.Search),
    Downloads("下载", Icons.Outlined.Download, Icons.Filled.Download),
    Profile("我的", Icons.Outlined.Person, Icons.Filled.Person),
}

@HiltViewModel
class MainShellViewModel @Inject constructor(
    private val repository: NowenRepository,
    val store: ServerSessionStore,
) : ViewModel() {
    fun logout() {
        viewModelScope.launch { repository.logout() }
    }
}

@Composable
fun MainShell(viewModel: MainShellViewModel = hiltViewModel()) {
    var tab by rememberSaveable { mutableStateOf(MainTab.Home) }

    Scaffold(
        bottomBar = {
            NavigationBar {
                MainTab.entries.forEach { item ->
                    NavigationBarItem(
                        selected = tab == item,
                        onClick = { tab = item },
                        icon = {
                            Icon(
                                if (tab == item) item.selectedIcon else item.icon,
                                contentDescription = item.label,
                            )
                        },
                        label = { Text(item.label) },
                    )
                }
            }
        },
    ) { padding ->
        when (tab) {
            MainTab.Home -> HomeScreen(Modifier.padding(padding))
            MainTab.Library -> LibraryScreen(Modifier.padding(padding))
            MainTab.Search -> SearchScreen(Modifier.padding(padding))
            MainTab.Downloads -> DownloadsScreen(Modifier.padding(padding))
            MainTab.Profile -> ProfileScreen(
                modifier = Modifier.padding(padding),
                sessionStore = viewModel.store,
                onLogout = viewModel::logout,
            )
        }
    }
}
