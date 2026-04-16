package com.nowen.video.ui.navigation

import androidx.compose.animation.*
import androidx.compose.animation.core.tween
import androidx.compose.runtime.Composable
import androidx.navigation.NavHostController
import androidx.navigation.NavType
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.navArgument
import com.nowen.video.ui.screen.home.HomeScreen
import com.nowen.video.ui.screen.auth.LoginScreen
import com.nowen.video.ui.screen.auth.ServerSetupScreen
import com.nowen.video.ui.screen.collection.CollectionDetailScreen
import com.nowen.video.ui.screen.collection.CollectionListScreen
import com.nowen.video.ui.screen.favorites.FavoritesScreen
import com.nowen.video.ui.screen.history.HistoryScreen
import com.nowen.video.ui.screen.media.MediaDetailScreen
import com.nowen.video.ui.screen.media.MediaListScreen
import com.nowen.video.ui.screen.player.PlayerScreen
import com.nowen.video.ui.screen.search.SearchScreen
import com.nowen.video.ui.screen.series.SeriesDetailScreen
import com.nowen.video.ui.screen.settings.SettingsScreen
import com.nowen.video.ui.screen.settings.PlayerSettingsScreen
import com.nowen.video.ui.screen.server.ServerManageScreen
import com.nowen.video.ui.screen.notification.NotificationScreen

/**
 * 页面过渡动画时长
 */
private const val ANIM_DURATION = 300

/**
 * 通用进入动画 — 从右侧滑入 + 淡入
 */
private fun defaultEnterTransition(): EnterTransition {
    return slideInHorizontally(
        initialOffsetX = { fullWidth -> fullWidth / 4 },
        animationSpec = tween(ANIM_DURATION)
    ) + fadeIn(animationSpec = tween(ANIM_DURATION))
}

/**
 * 通用退出动画 — 向左侧滑出 + 淡出
 */
private fun defaultExitTransition(): ExitTransition {
    return slideOutHorizontally(
        targetOffsetX = { fullWidth -> -fullWidth / 4 },
        animationSpec = tween(ANIM_DURATION)
    ) + fadeOut(animationSpec = tween(ANIM_DURATION))
}

/**
 * 返回时的进入动画 — 从左侧滑入 + 淡入
 */
private fun defaultPopEnterTransition(): EnterTransition {
    return slideInHorizontally(
        initialOffsetX = { fullWidth -> -fullWidth / 4 },
        animationSpec = tween(ANIM_DURATION)
    ) + fadeIn(animationSpec = tween(ANIM_DURATION))
}

/**
 * 返回时的退出动画 — 向右侧滑出 + 淡出
 */
private fun defaultPopExitTransition(): ExitTransition {
    return slideOutHorizontally(
        targetOffsetX = { fullWidth -> fullWidth / 4 },
        animationSpec = tween(ANIM_DURATION)
    ) + fadeOut(animationSpec = tween(ANIM_DURATION))
}

/**
 * 应用导航图 — Phase 4 增强版（带页面过渡动画）
 */
@Composable
fun NowenNavGraph(
    navController: NavHostController,
    startDestination: String
) {
    NavHost(
        navController = navController,
        startDestination = startDestination,
        enterTransition = { defaultEnterTransition() },
        exitTransition = { defaultExitTransition() },
        popEnterTransition = { defaultPopEnterTransition() },
        popExitTransition = { defaultPopExitTransition() }
    ) {
        // 服务器配置 — 使用淡入淡出（无滑动）
        composable(
            route = Screen.ServerSetup.route,
            enterTransition = { fadeIn(animationSpec = tween(400)) },
            exitTransition = { fadeOut(animationSpec = tween(400)) }
        ) {
            ServerSetupScreen(
                onServerConfigured = {
                    navController.navigate(Screen.Login.route) {
                        popUpTo(Screen.ServerSetup.route) { inclusive = true }
                    }
                }
            )
        }

        // 登录 — 使用淡入淡出
        composable(
            route = Screen.Login.route,
            enterTransition = { fadeIn(animationSpec = tween(400)) },
            exitTransition = { fadeOut(animationSpec = tween(400)) }
        ) {
            LoginScreen(
                onLoginSuccess = {
                    navController.navigate(Screen.Home.route) {
                        popUpTo(Screen.Login.route) { inclusive = true }
                    }
                },
                onChangeServer = {
                    navController.navigate(Screen.ServerSetup.route) {
                        popUpTo(Screen.Login.route) { inclusive = true }
                    }
                }
            )
        }

        // 首页 — 使用淡入淡出（作为根页面）
        composable(
            route = Screen.Home.route,
            enterTransition = { fadeIn(animationSpec = tween(300)) },
            exitTransition = { defaultExitTransition() },
            popEnterTransition = { defaultPopEnterTransition() }
        ) {
            HomeScreen(
                onMediaClick = { mediaId ->
                    navController.navigate(Screen.MediaDetail.createRoute(mediaId))
                },
                onSeriesClick = { seriesId ->
                    navController.navigate(Screen.SeriesDetail.createRoute(seriesId))
                },
                onSearchClick = {
                    navController.navigate(Screen.Search.createRoute())
                },
                onSettingsClick = {
                    navController.navigate(Screen.Settings.route)
                },
                onLibraryClick = { libraryId ->
                    navController.navigate(Screen.MediaList.createRoute(libraryId))
                },
                onFavoritesClick = {
                    navController.navigate(Screen.Favorites.route)
                },
                onHistoryClick = {
                    navController.navigate(Screen.History.route)
                },
                onCollectionsClick = {
                    navController.navigate(Screen.Collections.route)
                }
            )
        }

        // 媒体列表
        composable(
            route = Screen.MediaList.route,
            arguments = listOf(navArgument("libraryId") { type = NavType.StringType })
        ) { backStackEntry ->
            val libraryId = backStackEntry.arguments?.getString("libraryId") ?: return@composable
            MediaListScreen(
                libraryId = libraryId,
                onMediaClick = { mediaId ->
                    navController.navigate(Screen.MediaDetail.createRoute(mediaId))
                },
                onSeriesClick = { seriesId ->
                    navController.navigate(Screen.SeriesDetail.createRoute(seriesId))
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 电影详情
        composable(
            route = Screen.MediaDetail.route,
            arguments = listOf(navArgument("mediaId") { type = NavType.StringType })
        ) { backStackEntry ->
            val mediaId = backStackEntry.arguments?.getString("mediaId") ?: return@composable
            MediaDetailScreen(
                mediaId = mediaId,
                onPlayClick = { id ->
                    navController.navigate(Screen.Player.createRoute(id))
                },
                onCollectionClick = { collectionId ->
                    navController.navigate(Screen.CollectionDetail.createRoute(collectionId))
                },
                onSearchClick = { query ->
                    navController.navigate(Screen.Search.createRoute(query))
                },
                onMediaNavigate = { targetMediaId ->
                    navController.navigate(Screen.MediaDetail.createRoute(targetMediaId))
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 剧集详情
        composable(
            route = Screen.SeriesDetail.route,
            arguments = listOf(navArgument("seriesId") { type = NavType.StringType })
        ) { backStackEntry ->
            val seriesId = backStackEntry.arguments?.getString("seriesId") ?: return@composable
            SeriesDetailScreen(
                seriesId = seriesId,
                onEpisodeClick = { mediaId ->
                    navController.navigate(Screen.Player.createRoute(mediaId))
                },
                onSearchClick = { query ->
                    navController.navigate(Screen.Search.createRoute(query))
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 播放器 — 使用垂直滑入动画（从底部弹出）
        composable(
            route = Screen.Player.route,
            arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
            enterTransition = {
                slideInVertically(
                    initialOffsetY = { it },
                    animationSpec = tween(350)
                ) + fadeIn(animationSpec = tween(350))
            },
            exitTransition = { fadeOut(animationSpec = tween(200)) },
            popExitTransition = {
                slideOutVertically(
                    targetOffsetY = { it },
                    animationSpec = tween(350)
                ) + fadeOut(animationSpec = tween(350))
            }
        ) { backStackEntry ->
            val mediaId = backStackEntry.arguments?.getString("mediaId") ?: return@composable
            PlayerScreen(
                mediaId = mediaId,
                onBack = { navController.popBackStack() }
            )
        }

        // 搜索（支持可选的初始搜索关键词）
        composable(
            route = "search?q={query}",
            arguments = listOf(navArgument("query") {
                type = NavType.StringType
                defaultValue = ""
                nullable = true
            })
        ) { backStackEntry ->
            val initialQuery = backStackEntry.arguments?.getString("query") ?: ""
            SearchScreen(
                initialQuery = initialQuery,
                onMediaClick = { mediaId ->
                    navController.navigate(Screen.MediaDetail.createRoute(mediaId))
                },
                onSeriesClick = { seriesId ->
                    navController.navigate(Screen.SeriesDetail.createRoute(seriesId))
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 收藏列表
        composable(Screen.Favorites.route) {
            FavoritesScreen(
                onMediaClick = { mediaId ->
                    navController.navigate(Screen.MediaDetail.createRoute(mediaId))
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 观看历史
        composable(Screen.History.route) {
            HistoryScreen(
                onMediaClick = { mediaId ->
                    navController.navigate(Screen.MediaDetail.createRoute(mediaId))
                },
                onSeriesClick = { seriesId ->
                    navController.navigate(Screen.SeriesDetail.createRoute(seriesId))
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 合集列表
        composable(Screen.Collections.route) {
            CollectionListScreen(
                onCollectionClick = { collectionId ->
                    navController.navigate(Screen.CollectionDetail.createRoute(collectionId))
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 合集详情
        composable(
            route = Screen.CollectionDetail.route,
            arguments = listOf(navArgument("collectionId") { type = NavType.StringType })
        ) { backStackEntry ->
            val collectionId = backStackEntry.arguments?.getString("collectionId") ?: return@composable
            CollectionDetailScreen(
                collectionId = collectionId,
                onMediaClick = { mediaId ->
                    navController.navigate(Screen.MediaDetail.createRoute(mediaId))
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 设置
        composable(Screen.Settings.route) {
            SettingsScreen(
                onLogout = {
                    navController.navigate(Screen.Login.route) {
                        popUpTo(0) { inclusive = true }
                    }
                },
                onPlayerSettings = {
                    navController.navigate(Screen.PlayerSettings.route)
                },
                onServerManage = {
                    navController.navigate(Screen.ServerManage.route)
                },
                onNotifications = {
                    navController.navigate(Screen.Notifications.route)
                },
                onBack = { navController.popBackStack() }
            )
        }

        // 播放器设置
        composable(Screen.PlayerSettings.route) {
            PlayerSettingsScreen(
                onBack = { navController.popBackStack() }
            )
        }

        // 服务器管理
        composable(Screen.ServerManage.route) {
            ServerManageScreen(
                onBack = { navController.popBackStack() },
                onServerSwitch = {
                    navController.navigate(Screen.Home.route) {
                        popUpTo(0) { inclusive = true }
                    }
                },
                onAddServer = {
                    navController.navigate(Screen.ServerSetup.route)
                }
            )
        }

        // 实时通知
        composable(Screen.Notifications.route) {
            NotificationScreen(
                onBack = { navController.popBackStack() }
            )
        }
    }
}
