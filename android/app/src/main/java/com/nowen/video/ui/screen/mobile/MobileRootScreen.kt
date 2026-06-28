package com.nowen.video.ui.screen.mobile

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.LibraryBooks
import androidx.compose.material.icons.automirrored.outlined.LibraryBooks
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.FavoriteBorder
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import com.nowen.video.ui.component.mobile.AppScaffold
import com.nowen.video.ui.component.mobile.BottomNavItem
import com.nowen.video.ui.component.mobile.FloatingGlassBottomBar

/**
 * 移动端模式
 */
enum class MobileMode {
    Root,
    Server,
}

/**
 * Root 主导航 Tab
 */
enum class RootTab {
    Servers,
    Aggregate,
    Settings,
}

/**
 * 服务器内导航 Tab
 */
enum class ServerTab {
    Home,
    Favorites,
    Search,
}

/**
 * 移动端 Root 主页面
 * 支持 root/server 双模式
 */
@Composable
fun MobileRootScreen(
    onMediaClick: (String) -> Unit,
    onSeriesClick: (String) -> Unit,
    onLibraryClick: (String) -> Unit,
    onSearchClick: () -> Unit,
    onSettingsClick: () -> Unit,
    onPlayerClick: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    var mode by remember { mutableStateOf(MobileMode.Root) }
    var rootTab by remember { mutableStateOf(RootTab.Servers) }
    var serverTab by remember { mutableStateOf(ServerTab.Home) }

    // Root 底部导航项
    val rootNavItems = listOf(
        BottomNavItem(
            key = RootTab.Servers.name,
            label = "服务器",
            icon = Icons.Outlined.Home,
            selectedIcon = Icons.Filled.Home,
        ),
        BottomNavItem(
            key = RootTab.Aggregate.name,
            label = "聚合视界",
            icon = Icons.AutoMirrored.Outlined.LibraryBooks,
            selectedIcon = Icons.AutoMirrored.Filled.LibraryBooks,
        ),
        BottomNavItem(
            key = RootTab.Settings.name,
            label = "设置",
            icon = Icons.Outlined.Settings,
            selectedIcon = Icons.Filled.Settings,
        ),
    )

    // Server 底部导航项
    val serverNavItems = listOf(
        BottomNavItem(
            key = ServerTab.Home.name,
            label = "首页",
            icon = Icons.Outlined.Home,
            selectedIcon = Icons.Filled.Home,
        ),
        BottomNavItem(
            key = ServerTab.Favorites.name,
            label = "收藏",
            icon = Icons.Outlined.FavoriteBorder,
            selectedIcon = Icons.Filled.Favorite,
        ),
        BottomNavItem(
            key = ServerTab.Search.name,
            label = "搜索",
            icon = Icons.Outlined.Search,
            selectedIcon = Icons.Filled.Search,
        ),
    )

    // 进入服务器
    val enterServer: () -> Unit = {
        mode = MobileMode.Server
        serverTab = ServerTab.Home
    }

    // 返回 Root
    val exitServer: () -> Unit = {
        mode = MobileMode.Root
        rootTab = RootTab.Servers
    }

    AppScaffold(
        modifier = modifier.fillMaxSize(),
        showBottomBar = true,
        bottomBar = {
            FloatingGlassBottomBar(
                items = if (mode == MobileMode.Root) rootNavItems else serverNavItems,
                selectedKey = if (mode == MobileMode.Root) rootTab.name else serverTab.name,
                onItemClick = { key ->
                    if (mode == MobileMode.Root) {
                        rootTab = RootTab.valueOf(key)
                    } else {
                        serverTab = ServerTab.valueOf(key)
                    }
                },
            )
        },
    ) { innerPadding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding),
        ) {
            AnimatedContent(
                targetState = mode to (if (mode == MobileMode.Root) rootTab.name else serverTab.name),
                transitionSpec = { fadeIn() togetherWith fadeOut() },
                label = "TabTransition",
            ) { (currentMode, currentTab) ->
                when (currentMode) {
                    MobileMode.Root -> {
                        when (RootTab.valueOf(currentTab)) {
                            RootTab.Servers -> ServerRootScreen(
                                onEnterServer = enterServer,
                                onMediaClick = onMediaClick,
                                onSeriesClick = onSeriesClick,
                                onLibraryClick = onLibraryClick,
                                onSearchClick = onSearchClick,
                                onPlayerClick = onPlayerClick,
                            )
                            RootTab.Aggregate -> AggregateScreen(
                                onMediaClick = onMediaClick,
                                onPlayerClick = onPlayerClick,
                                onLibraryClick = onLibraryClick,
                                onSearchClick = {
                                    // 切换到 Server 搜索模式
                                    mode = MobileMode.Server
                                    serverTab = ServerTab.Search
                                },
                            )
                            RootTab.Settings -> MobileSettingsScreen(
                                onSettingsClick = onSettingsClick,
                                onPlayerSettingsClick = onSettingsClick,
                                onServerManageClick = onSettingsClick,
                            )
                        }
                    }
                    MobileMode.Server -> {
                        when (ServerTab.valueOf(currentTab)) {
                            ServerTab.Home -> ServerHomeScreen(
                                onBack = exitServer,
                                onMediaClick = onMediaClick,
                                onSeriesClick = onSeriesClick,
                                onLibraryClick = onLibraryClick,
                                onPlayerClick = onPlayerClick,
                                onSearchClick = { serverTab = ServerTab.Search },
                            )
                            ServerTab.Favorites -> ServerFavoritesScreen(
                                onBack = exitServer,
                                onMediaClick = onMediaClick,
                            )
                            ServerTab.Search -> ServerSearchScreen(
                                onBack = exitServer,
                                onMediaClick = onMediaClick,
                            )
                        }
                    }
                }
            }
        }
    }
}
