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
private const val ANIM_DURATION = 300
private const val ANIM_FAST = 220

/**
 * 柔和进入 — 淡入 + 微缩放（舒适自然）
 */
private fun softEnterTransition(): EnterTransition {
    return fadeIn(
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    ) + scaleIn(
        initialScale = 0.96f,
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    )
}

/**
 * 柔和退出 — 淡出 + 微缩小
 */
private fun softExitTransition(): ExitTransition {
    return fadeOut(
        animationSpec = tween(ANIM_FAST, easing = EaseInOutCubic)
    ) + scaleOut(
        targetScale = 0.96f,
        animationSpec = tween(ANIM_FAST, easing = EaseInOutCubic)
    )
}

/**
 * 返回时的进入动画 — 淡入（无缩放，更平静）
 */
private fun softPopEnterTransition(): EnterTransition {
    return fadeIn(
        animationSpec = tween(ANIM_DURATION, easing = EaseInOutCubic)
    )
}

/**
 * 返回时的退出动画 — 淡出 + 微缩小
 */
private fun softPopExitTransition(): ExitTransition {
    return fadeOut(
        animationSpec = tween(ANIM_FAST, easing = EaseInOutCubic)
    ) + scaleOut(
        targetScale = 0.97f,
        animationSpec = tween(ANIM_FAST, easing = EaseInOutCubic)
    )
}

/**
 * 认证页面进入 — 淡入 + 轻微缩放
 */
private fun authEnterTransition(): EnterTransition {
    return fadeIn(
        animationSpec = tween(400, easing = EaseInOutCubic)
    ) + scaleIn(
        initialScale = 0.94f,
        animationSpec = tween(400, easing = EaseInOutCubic)
    )
}

/**
 * 认证页面退出 — 淡出
 */
private fun authExitTransition(): ExitTransition {
    return fadeOut(
        animationSpec = tween(300, easing = EaseInOutCubic)
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
        enterTransition = { softEnterTransition() },
        exitTransition = { softExitTransition() },
        popEnterTransition = { softPopEnterTransition() },
        popExitTransition = { softPopExitTransition() }
    ) {
        // 服务器配置 — 全息投影式过渡
        composable(
            route = Screen.ServerSetup.route,
            enterTransition = { authEnterTransition() },
            exitTransition = { authExitTransition() }
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
            enterTransition = { authEnterTransition() },
            exitTransition = { authExitTransition() }
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
                fadeIn(animationSpec = tween(350, easing = EaseInOutCubic))
            },
            exitTransition = { softExitTransition() },
            popEnterTransition = { softPopEnterTransition() }
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
                fadeIn(animationSpec = tween(300, easing = EaseInOutCubic)) +
                    scaleIn(
                        initialScale = 0.92f,
                        animationSpec = tween(350, easing = EaseInOutCubic)
                    )
            },
            exitTransition = { fadeOut(animationSpec = tween(200)) },
            popExitTransition = {
                fadeOut(animationSpec = tween(250, easing = EaseInOutCubic)) +
                    scaleOut(
                        targetScale = 0.92f,
                        animationSpec = tween(300, easing = EaseInOutCubic)
                    )
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
