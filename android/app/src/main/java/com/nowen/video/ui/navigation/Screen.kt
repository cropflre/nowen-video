package com.nowen.video.ui.navigation

/**
 * 路由定义 — 所有页面的导航路径
 */
sealed class Screen(val route: String) {
    /** 服务器配置页（首次启动） */
    data object ServerSetup : Screen("server_setup")

    /** 登录/注册页 */
    data object Login : Screen("login")

    /** 首页（底部导航容器） */
    data object Home : Screen("home")

    /** 媒体库列表 */
    data object Libraries : Screen("libraries")

    /** 媒体列表（按媒体库筛选） */
    data object MediaList : Screen("media_list/{libraryId}") {
        fun createRoute(libraryId: String) = "media_list/$libraryId"
    }

    /** 电影详情 */
    data object MediaDetail : Screen("media_detail/{mediaId}") {
        fun createRoute(mediaId: String) = "media_detail/$mediaId"
    }

    /** 剧集详情 */
    data object SeriesDetail : Screen("series_detail/{seriesId}") {
        fun createRoute(seriesId: String) = "series_detail/$seriesId"
    }

    /** 视频播放器 */
    data object Player : Screen("player/{mediaId}") {
        fun createRoute(mediaId: String) = "player/$mediaId"
    }

    /** 搜索 */
    data object Search : Screen("search?q={query}") {
        const val baseRoute = "search"
        fun createRoute(query: String = "") = if (query.isNotBlank()) "search?q=$query" else "search"
    }

    /** 收藏列表 */
    data object Favorites : Screen("favorites")

    /** 观看历史 */
    data object History : Screen("history")

    /** 合集列表 */
    data object Collections : Screen("collections")

    /** 合集详情 */
    data object CollectionDetail : Screen("collection_detail/{collectionId}") {
        fun createRoute(collectionId: String) = "collection_detail/$collectionId"
    }

    /** 设置 */
    data object Settings : Screen("settings")

    /** 播放器设置 */
    data object PlayerSettings : Screen("player_settings")

    /** 服务器管理 */
    data object ServerManage : Screen("server_manage")

    /** 实时通知（后台任务） */
    data object Notifications : Screen("notifications")
}
