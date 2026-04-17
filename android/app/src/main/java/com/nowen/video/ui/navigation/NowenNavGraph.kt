package com.nowen.video.ui.navigation

import androidx.compose.animation.*
import androidx.compose.animation.core.EaseInOutCubic
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
private const val ANIM_DURATION = 350
private const val ANIM_FAST = 250

/**
 * 赛博朋克页面进入 — 从右侧滑入 + 缩放 + 淡入
 */
private fun cyberEnterTransition(): EnterTransition {
    return slideInHorizontally(
        initialOffsetX = { fullWidth -> fullWidth / 3 },
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    ) + fadeIn(
        animationSpec = tween(ANIM_DURATION)
    ) + scaleIn(
        initialScale = 0.92f,
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    )
}

/**
 * 赛博朋克页面退出 — 向左侧滑出 + 缩小 + 淡出
 */
private fun cyberExitTransition(): ExitTransition {
    return slideOutHorizontally(
        targetOffsetX = { fullWidth -> -fullWidth / 4 },
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    ) + fadeOut(
        animationSpec = tween(ANIM_FAST)
    ) + scaleOut(
        targetScale = 0.95f,
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    )
}

/**
 * 返回时的进入动画 — 从左侧滑入 + 缩放回复
 */
private fun cyberPopEnterTransition(): EnterTransition {
    return slideInHorizontally(
        initialOffsetX = { fullWidth -> -fullWidth / 4 },
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    ) + fadeIn(
        animationSpec = tween(ANIM_DURATION)
    ) + scaleIn(
        initialScale = 0.95f,
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    )
}

/**
 * 返回时的退出动画 — 向右侧滑出 + 缩小
 */
private fun cyberPopExitTransition(): ExitTransition {
    return slideOutHorizontally(
        targetOffsetX = { fullWidth -> fullWidth / 3 },
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    ) + fadeOut(
        animationSpec = tween(ANIM_FAST)
    ) + scaleOut(
        targetScale = 0.92f,
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    )
}

/**
 * 全息投影式进入 — 缩放 + 淡入（用于认证页面）
 */
private fun holoEnterTransition(): EnterTransition {
    return scaleIn(
        initialScale = 0.85f,
        animationSpec = tween(450, easing = EaseInOutCubic)
    ) + fadeIn(
        animationSpec = tween(450)
    )
}

/**
 * 全息投影式退出 — 缩放 + 淡出
 */
private fun holoExitTransition(): ExitTransition {
    return scaleOut(
        targetScale = 1.1f,
        animationSpec = tween(350, easing = EaseInOutCubic)
    ) + fadeOut(
        animationSpec = tween(350)
    )
}

/**
 * 应用导航图 — 赛博朋克增强版（带科幻页面过渡动画）
 */
@Composable
fun NowenNavGraph(
    navController: NavHostController,
    startDestination: String
) {
    NavHost(
        navController = navController,
        startDestination = startDestination,
        enterTransition = { cyberEnterTransition() },
        exitTransition = { cyberExitTransition() },
        popEnterTransition = { cyberPopEnterTransition() },
        popExitTransition = { cyberPopExitTransition() }
    ) {
        // 服务器配置 — 全息投影式过渡
        composable(
            route = Screen.ServerSetup.route,
            enterTransition = { holoEnterTransition() },
            exitTransition = { holoExitTransition() }
        ) {
            ServerSetupScreen(
                onServerConfigured = {
                    navController.navigate(Screen.Login.route) {
                        popUpTo(Screen.ServerSetup.route) { inclusive = true }
                    }
                }
            )
        }

        // 登录 — 全息投影式过渡
        composable(
            route = Screen.Login.route,
            enterTransition = { holoEnterTransition() },
            exitTransition = { holoExitTransition() }
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

        // 首页 — 淡入 + 微缩放（作为根页面）
        composable(
            route = Screen.Home.route,
            enterTransition = {
                fadeIn(animationSpec = tween(400)) +
                        scaleIn(initialScale = 0.96f, animationSpec = tween(400, easing = EaseInOutCubic))
            },
            exitTransition = { cyberExitTransition() },
            popEnterTransition = { cyberPopEnterTransition() }
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

        // 播放器 — 从底部弹出 + 缩放
        composable(
            route = Screen.Player.route,
            arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
            enterTransition = {
                slideInVertically(
                    initialOffsetY = { it },
                    animationSpec = tween(400, easing = EaseInOutCubic)
                ) + fadeIn(animationSpec = tween(300))
            },
            exitTransition = { fadeOut(animationSpec = tween(200)) },
            popExitTransition = {
                slideOutVertically(
                    targetOffsetY = { it },
                    animationSpec = tween(400, easing = EaseInOutCubic)
                ) + fadeOut(animationSpec = tween(300))
            }
        ) { backStackEntry ->
            val mediaId = backStackEntry.arguments?.getString("mediaId") ?: return@composable
            PlayerScreen(
                mediaId = mediaId,
                onBack = { navController.popBackStack() }
            )
        }

        // 搜索
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
