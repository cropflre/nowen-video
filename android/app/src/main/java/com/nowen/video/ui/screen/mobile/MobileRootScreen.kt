package com.nowen.video.ui.screen.mobile

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.LibraryBooks
import androidx.compose.material.icons.automirrored.outlined.LibraryBooks
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.Home
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
 * Root 主导航 Tab
 */
enum class RootTab {
    Servers,
    Aggregate,
    Settings,
}

/**
 * 移动端 Root 主页面
 * 底部导航：服务器 / 聚合视界 / 设置
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
    var selectedTab by remember { mutableStateOf(RootTab.Servers) }

    val navItems = listOf(
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

    AppScaffold(
        modifier = modifier.fillMaxSize(),
        showBottomBar = true,
        bottomBar = {
            FloatingGlassBottomBar(
                items = navItems,
                selectedKey = selectedTab.name,
                onItemClick = { key ->
                    selectedTab = RootTab.valueOf(key)
                },
            )
        },
    ) { _ ->
        AnimatedContent(
            targetState = selectedTab,
            transitionSpec = { fadeIn() togetherWith fadeOut() },
            label = "RootTabTransition",
        ) { tab ->
            when (tab) {
                RootTab.Servers -> ServerRootScreen(
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
                )
                RootTab.Settings -> MobileSettingsScreen(
                    onSettingsClick = onSettingsClick,
                )
            }
        }
    }
}
