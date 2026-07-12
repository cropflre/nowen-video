package com.nowen.video.v2.feature.main

import android.net.Uri
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Download
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.VideoLibrary
import androidx.compose.material.icons.outlined.Download
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import androidx.navigation.navArgument
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.launch

enum class MainTab(
    val route: String,
    val label: String,
    val icon: ImageVector,
    val selectedIcon: ImageVector,
) {
    Home("home", "首页", Icons.Outlined.Home, Icons.Filled.Home),
    Library("library", "媒体库", Icons.Outlined.VideoLibrary, Icons.Filled.VideoLibrary),
    Search("search", "搜索", Icons.Outlined.Search, Icons.Filled.Search),
    Downloads("downloads", "下载", Icons.Outlined.Download, Icons.Filled.Download),
    Profile("profile", "我的", Icons.Outlined.Person, Icons.Filled.Person),
}

private const val DETAIL_ROUTE = "detail/{mediaId}"
private const val PLAYER_ROUTE = "player/{mediaId}"

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
    val navController = rememberNavController()
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = backStackEntry?.destination?.route
    val currentTab = MainTab.entries.firstOrNull { it.route == currentRoute }
    val showBottomBar = currentTab != null

    fun openDetail(mediaId: String) {
        if (mediaId.isNotBlank()) navController.navigate("detail/${Uri.encode(mediaId)}")
    }

    Scaffold(
        bottomBar = {
            if (showBottomBar) {
                NavigationBar {
                    MainTab.entries.forEach { item ->
                        val selected = currentTab == item
                        NavigationBarItem(
                            selected = selected,
                            onClick = {
                                navController.navigate(item.route) {
                                    popUpTo(MainTab.Home.route) { saveState = true }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            },
                            icon = {
                                Icon(
                                    if (selected) item.selectedIcon else item.icon,
                                    contentDescription = item.label,
                                )
                            },
                            label = { Text(item.label) },
                        )
                    }
                }
            }
        },
    ) { padding ->
        NavHost(
            navController = navController,
            startDestination = MainTab.Home.route,
            modifier = Modifier.padding(if (showBottomBar) padding else androidx.compose.foundation.layout.PaddingValues()),
        ) {
            composable(MainTab.Home.route) {
                HomeScreen(
                    onMediaClick = ::openDetail,
                    onLibraryClick = { navController.navigate(MainTab.Library.route) },
                )
            }
            composable(MainTab.Library.route) {
                LibraryScreen(onMediaClick = ::openDetail)
            }
            composable(MainTab.Search.route) {
                SearchScreen(onMediaClick = ::openDetail)
            }
            composable(MainTab.Downloads.route) {
                DownloadsScreen()
            }
            composable(MainTab.Profile.route) {
                ProfileScreen(
                    sessionStore = viewModel.store,
                    onLogout = viewModel::logout,
                )
            }
            composable(
                route = DETAIL_ROUTE,
                arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
            ) { entry ->
                val mediaId = entry.arguments?.getString("mediaId").orEmpty()
                MediaDetailScreen(
                    mediaId = mediaId,
                    onBack = { navController.popBackStack() },
                    onPlay = { id -> navController.navigate("player/${Uri.encode(id)}") },
                )
            }
            composable(
                route = PLAYER_ROUTE,
                arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
            ) { entry ->
                val mediaId = entry.arguments?.getString("mediaId").orEmpty()
                PlayerScreen(
                    mediaId = mediaId,
                    onBack = { navController.popBackStack() },
                )
            }
        }
    }
}
