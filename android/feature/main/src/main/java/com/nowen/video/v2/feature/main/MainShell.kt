package com.nowen.video.v2.feature.main

import android.Manifest
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Collections
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.outlined.FavoriteBorder
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
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
import com.nowen.video.v2.core.data.PlayerPreferences
import com.nowen.video.v2.core.data.PlayerPreferencesStore
import com.nowen.video.v2.core.data.ProgressRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.model.LibrarySummary
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.designsystem.HillsBottomDock
import com.nowen.video.v2.core.designsystem.NowenMobileMetrics
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch

enum class MainTab(
    val route: String,
    val label: String,
    val icon: ImageVector,
    val selectedIcon: ImageVector,
) {
    Home("home", "首页", Icons.Outlined.Home, Icons.Filled.Home),
    Favorites("favorites", "收藏", Icons.Outlined.FavoriteBorder, Icons.Filled.Favorite),
    Search("search", "搜索", Icons.Outlined.Search, Icons.Filled.Search),
}

private const val LIBRARY_BROWSE_ROUTE = "library/{libraryId}/{libraryName}"
private const val DETAIL_ROUTE = "detail/{mediaId}"
private const val SERIES_DETAIL_ROUTE = "series/{seriesId}"
private const val SEASON_EPISODES_ROUTE = "series/{seriesId}/season/{seasonNumber}"
private const val PLAYER_ROUTE = "player/{mediaId}"
private const val OFFLINE_PLAYER_ROUTE = "offline/{mediaId}"
private const val DOWNLOADS_ROUTE = "downloads"
private const val FAVORITES_ROUTE = "favorites"
private const val HISTORY_ROUTE = "history"
private const val COLLECTIONS_ROUTE = "collections"
private const val COLLECTION_DETAIL_ROUTE = "collection/{collectionId}"
private const val PERSON_DETAIL_ROUTE = "person/{personId}"
private const val SETTINGS_ROUTE = "settings"

private val detailEnterTransition = fadeIn(tween(180)) + slideInHorizontally(tween(180)) { it / 12 }
private val detailExitTransition = fadeOut(tween(140)) + slideOutHorizontally(tween(140)) { -it / 20 }
private val detailPopEnterTransition = fadeIn(tween(160)) + slideInHorizontally(tween(160)) { -it / 20 }
private val detailPopExitTransition = fadeOut(tween(120)) + slideOutHorizontally(tween(120)) { it / 12 }

@HiltViewModel
class MainShellViewModel @Inject constructor(
    private val repository: NowenRepository,
    private val progressRepository: ProgressRepository,
    playerPreferencesStore: PlayerPreferencesStore,
    val store: ServerSessionStore,
) : ViewModel() {
    val playerPreferences: StateFlow<PlayerPreferences> = playerPreferencesStore.preferences.stateIn(
        viewModelScope,
        SharingStarted.Eagerly,
        PlayerPreferences(),
    )

    fun preparePlaybackStart(mediaId: String, positionSeconds: Double) {
        progressRepository.prepareNextPlaybackStart(mediaId, positionSeconds)
    }

    fun logout() {
        viewModelScope.launch { repository.logout() }
    }
}

@Composable
fun MainShell(viewModel: MainShellViewModel = hiltViewModel()) {
    val navController = rememberNavController()
    val backStackEntry by navController.currentBackStackEntryAsState()
    val currentRoute = backStackEntry?.destination?.route
    val selectedTab = when (currentRoute) {
        MainTab.Home.route, LIBRARY_BROWSE_ROUTE, DETAIL_ROUTE, SERIES_DETAIL_ROUTE, SEASON_EPISODES_ROUTE -> MainTab.Home
        MainTab.Favorites.route -> MainTab.Favorites
        MainTab.Search.route, PERSON_DETAIL_ROUTE -> MainTab.Search
        HISTORY_ROUTE, DOWNLOADS_ROUTE, SETTINGS_ROUTE, COLLECTIONS_ROUTE, COLLECTION_DETAIL_ROUTE -> null
        else -> null
    }
    val showBottomBar = currentRoute != PLAYER_ROUTE &&
        currentRoute != OFFLINE_PLAYER_ROUTE &&
        currentRoute != LIBRARY_BROWSE_ROUTE &&
        currentRoute != DETAIL_ROUTE &&
        currentRoute != SERIES_DETAIL_ROUTE &&
        currentRoute != SEASON_EPISODES_ROUTE
    val playerPreferences by viewModel.playerPreferences.collectAsState()
    val context = LocalContext.current
    var askedDownloadNotificationPermission by rememberSaveable { mutableStateOf(false) }
    val notificationPermissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { }

    LaunchedEffect(currentRoute) {
        if (
            currentRoute == DOWNLOADS_ROUTE &&
            !askedDownloadNotificationPermission &&
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            askedDownloadNotificationPermission = true
            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
        }
    }

    fun openLibraryBrowse(library: LibrarySummary) {
        if (library.id.isBlank()) return
        navController.navigate("library/${Uri.encode(library.id)}/${Uri.encode(library.name)}")
    }

    fun openDetail(mediaId: String) {
        if (mediaId.isNotBlank()) navController.navigate("detail/${Uri.encode(mediaId)}")
    }

    fun openSeries(seriesId: String) {
        if (seriesId.isNotBlank()) navController.navigate("series/${Uri.encode(seriesId)}")
    }

    fun openSeason(seriesId: String, seasonNumber: Int) {
        if (seriesId.isNotBlank()) {
            navController.navigate("series/${Uri.encode(seriesId)}/season/$seasonNumber")
        }
    }

    fun openCatalogDetail(card: MediaCard) {
        val id = card.resolvedId
        if (id.isBlank()) return
        if (card.isSeries) openSeries(id) else openDetail(id)
    }

    fun openPlayer(mediaId: String) {
        if (mediaId.isNotBlank()) navController.navigate("player/${Uri.encode(mediaId)}")
    }

    fun openPlayerAt(mediaId: String, positionSeconds: Double) {
        if (mediaId.isBlank()) return
        viewModel.preparePlaybackStart(mediaId, positionSeconds)
        openPlayer(mediaId)
    }

    fun openOfflinePlayer(mediaId: String) {
        if (mediaId.isNotBlank()) navController.navigate("offline/${Uri.encode(mediaId)}")
    }

    fun openCollection(collectionId: String) {
        if (collectionId.isNotBlank()) navController.navigate("collection/${Uri.encode(collectionId)}")
    }

    fun openPerson(personId: String) {
        if (personId.isNotBlank()) navController.navigate("person/${Uri.encode(personId)}")
    }

    fun openSearch(query: String) {
        val normalized = query.trim()
        if (normalized.isBlank()) return
        val searchEntry = runCatching {
            navController.getBackStackEntry(MainTab.Search.route)
        }.getOrNull()
        searchEntry?.savedStateHandle?.set("search_query", normalized)
        navController.navigate(MainTab.Search.route) {
            launchSingleTop = true
        }
        navController.currentBackStackEntry?.savedStateHandle?.set("search_query", normalized)
    }

    var showWorkspace by rememberSaveable { mutableStateOf(false) }

    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        NavHost(
            navController = navController,
            startDestination = MainTab.Home.route,
            modifier = Modifier.fillMaxSize().padding(bottom = if (showBottomBar) 84.dp else 0.dp),
        ) {
            composable(MainTab.Home.route) {
                LibraryScreen(
                    onMediaClick = ::openCatalogDetail,
                    onPlay = ::openPlayer,
                    onBrowseLibrary = ::openLibraryBrowse,
                    onOpenSettings = { navController.navigate(SETTINGS_ROUTE) },
                    onOpenWorkspace = { showWorkspace = true },
                )
            }
            composable(
                route = LIBRARY_BROWSE_ROUTE,
                arguments = listOf(
                    navArgument("libraryId") { type = NavType.StringType },
                    navArgument("libraryName") { type = NavType.StringType },
                ),
                enterTransition = { detailEnterTransition },
                exitTransition = { detailExitTransition },
                popEnterTransition = { detailPopEnterTransition },
                popExitTransition = { detailPopExitTransition },
            ) { entry ->
                LibraryBrowseScreen(
                    libraryId = entry.arguments?.getString("libraryId").orEmpty(),
                    libraryName = entry.arguments?.getString("libraryName").orEmpty(),
                    onBack = { navController.popBackStack() },
                    onMediaClick = ::openCatalogDetail,
                )
            }
            composable(MainTab.Search.route) { entry ->
                val initialQuery by entry.savedStateHandle
                    .getStateFlow("search_query", "")
                    .collectAsState()
                SearchScreen(
                    initialQuery = initialQuery,
                    onMediaClick = ::openCatalogDetail,
                    onPersonClick = ::openPerson,
                    onCollectionClick = ::openCollection,
                )
            }
            composable(DOWNLOADS_ROUTE) {
                DownloadsScreen(
                    onBack = { navController.popBackStack() },
                    onPlayOffline = ::openOfflinePlayer,
                )
            }
            composable(SETTINGS_ROUTE) {
                MobileSettingsScreen(onBack = { navController.popBackStack() })
            }
            composable(MainTab.Favorites.route) {
                PagedFavoritesScreen(
                    primaryDestination = true,
                    onMediaClick = ::openCatalogDetail,
                )
            }
            composable(HISTORY_ROUTE) {
                PagedHistoryScreen(
                    onBack = { navController.popBackStack() },
                    onMediaClick = ::openCatalogDetail,
                    onPlay = ::openPlayer,
                )
            }
            composable(COLLECTIONS_ROUTE) {
                CollectionsScreen(
                    onBack = { navController.popBackStack() },
                    onCollectionClick = ::openCollection,
                )
            }
            composable(
                route = COLLECTION_DETAIL_ROUTE,
                arguments = listOf(navArgument("collectionId") { type = NavType.StringType }),
                enterTransition = { detailEnterTransition },
                exitTransition = { detailExitTransition },
                popEnterTransition = { detailPopEnterTransition },
                popExitTransition = { detailPopExitTransition },
            ) { entry ->
                CollectionDetailScreen(
                    collectionId = entry.arguments?.getString("collectionId").orEmpty(),
                    onBack = { navController.popBackStack() },
                    onMediaClick = ::openDetail,
                )
            }
            composable(
                route = PERSON_DETAIL_ROUTE,
                arguments = listOf(navArgument("personId") { type = NavType.StringType }),
                enterTransition = { detailEnterTransition },
                exitTransition = { detailExitTransition },
                popEnterTransition = { detailPopEnterTransition },
                popExitTransition = { detailPopExitTransition },
            ) { entry ->
                PersonDetailScreen(
                    personId = entry.arguments?.getString("personId").orEmpty(),
                    onBack = { navController.popBackStack() },
                    onMediaClick = ::openDetail,
                    onSeriesClick = ::openSeries,
                )
            }
            composable(
                route = SERIES_DETAIL_ROUTE,
                arguments = listOf(navArgument("seriesId") { type = NavType.StringType }),
                enterTransition = { detailEnterTransition },
                exitTransition = { detailExitTransition },
                popEnterTransition = { detailPopEnterTransition },
                popExitTransition = { detailPopExitTransition },
            ) { entry ->
                SeriesDetailScreen(
                    seriesId = entry.arguments?.getString("seriesId").orEmpty(),
                    onBack = { navController.popBackStack() },
                    onEpisodeClick = ::openDetail,
                    onPlayEpisode = ::openPlayer,
                    onGenreClick = ::openSearch,
                    onPersonClick = ::openPerson,
                    onSeasonClick = { season -> openSeason(entry.arguments?.getString("seriesId").orEmpty(), season) },
                )
            }
            composable(
                route = SEASON_EPISODES_ROUTE,
                arguments = listOf(
                    navArgument("seriesId") { type = NavType.StringType },
                    navArgument("seasonNumber") { type = NavType.IntType },
                ),
                enterTransition = { detailEnterTransition },
                exitTransition = { detailExitTransition },
                popEnterTransition = { detailPopEnterTransition },
                popExitTransition = { detailPopExitTransition },
            ) { entry ->
                SeasonEpisodesScreen(
                    seriesId = entry.arguments?.getString("seriesId").orEmpty(),
                    seasonNumber = entry.arguments?.getInt("seasonNumber") ?: 1,
                    onBack = { navController.popBackStack() },
                    onEpisodeClick = ::openDetail,
                    onPlayEpisode = ::openPlayer,
                )
            }
            composable(
                route = DETAIL_ROUTE,
                arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
                enterTransition = { detailEnterTransition },
                exitTransition = { detailExitTransition },
                popEnterTransition = { detailPopEnterTransition },
                popExitTransition = { detailPopExitTransition },
            ) { entry ->
                val mediaId = entry.arguments?.getString("mediaId").orEmpty()
                MediaDetailScreen(
                    mediaId = mediaId,
                    onBack = { navController.popBackStack() },
                    onPlay = ::openPlayer,
                    onHighlightPlay = ::openPlayerAt,
                    onPersonClick = ::openPerson,
                    onCollectionClick = ::openCollection,
                    onMediaClick = ::openDetail,
                    onRecommendationClick = ::openCatalogDetail,
                    onSeriesClick = ::openSeries,
                )
            }
            composable(
                route = PLAYER_ROUTE,
                arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
            ) { entry ->
                val mediaId = entry.arguments?.getString("mediaId").orEmpty()
                PlaybackPictureInPictureBinding(enabled = playerPreferences.pictureInPictureEnabled) { _, pipAvailable ->
                    PlayerScreen(
                        mediaId = mediaId,
                        pictureInPictureAvailable = pipAvailable,
                        onBack = { navController.popBackStack() },
                        onPlayNext = { nextId ->
                            navController.navigate("player/${Uri.encode(nextId)}") {
                                popUpTo(entry.destination.id) { inclusive = true }
                                launchSingleTop = true
                            }
                        },
                    )
                }
            }
            composable(
                route = OFFLINE_PLAYER_ROUTE,
                arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
            ) { entry ->
                val mediaId = entry.arguments?.getString("mediaId").orEmpty()
                PlaybackPictureInPictureBinding(enabled = playerPreferences.pictureInPictureEnabled) { _, pipAvailable ->
                    OfflinePlayerScreen(
                        mediaId = mediaId,
                        pictureInPictureAvailable = pipAvailable,
                        onBack = { navController.popBackStack() },
                    )
                }
            }
        }
        if (showWorkspace) {
            WorkspaceMenu(
                onDismiss = { showWorkspace = false },
                onHistory = {
                    showWorkspace = false
                    navController.navigate(HISTORY_ROUTE)
                },
                onDownloads = {
                    showWorkspace = false
                    navController.navigate(DOWNLOADS_ROUTE)
                },
                onCollections = {
                    showWorkspace = false
                    navController.navigate(COLLECTIONS_ROUTE)
                },
                onSettings = {
                    showWorkspace = false
                    navController.navigate(SETTINGS_ROUTE)
                },
            )
        }
        if (showBottomBar) {
            Box(Modifier.align(Alignment.BottomCenter)) {
            HillsBottomDock {
                MainTab.entries.forEach { item ->
                    val selected = selectedTab == item
                    WebMobileBottomBarItem(
                        item = item,
                        selected = selected,
                        onClick = {
                            navController.navigate(item.route) {
                                popUpTo(MainTab.Home.route) { saveState = true }
                                launchSingleTop = true
                                restoreState = true
                            }
                        },
                        modifier = Modifier.weight(1f),
                    )
                }
            }
            }
        }
    }
}

@Composable
private fun WorkspaceMenu(
    onDismiss: () -> Unit,
    onHistory: () -> Unit,
    onDownloads: () -> Unit,
    onCollections: () -> Unit,
    onSettings: () -> Unit,
) {
    androidx.compose.material3.AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("工作区") },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                WorkspaceMenuRow(Icons.Default.History, "观看历史", "继续播放或管理记录", onHistory)
                WorkspaceMenuRow(Icons.Default.CloudDownload, "离线下载", "管理本机媒体与空间", onDownloads)
                WorkspaceMenuRow(Icons.Default.Collections, "系列合集", "按合集浏览影片", onCollections)
                WorkspaceMenuRow(Icons.Default.Settings, "设置", "播放、下载和账号", onSettings)
            }
        },
        confirmButton = { androidx.compose.material3.TextButton(onClick = onDismiss) { Text("关闭") } },
    )
}

@Composable
private fun WorkspaceMenuRow(
    icon: ImageVector,
    title: String,
    subtitle: String,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .clickable(onClick = onClick)
            .padding(vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
        Column(Modifier.padding(start = 14.dp).weight(1f)) {
            Text(title, style = MaterialTheme.typography.titleMedium)
            Text(subtitle, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun WebMobileBottomBar(
    selectedTab: MainTab?,
    onSelect: (MainTab) -> Unit,
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .navigationBarsPadding()
            .padding(horizontal = 20.dp, vertical = 10.dp),
        shape = RoundedCornerShape(32.dp),
        color = MaterialTheme.colorScheme.surface.copy(alpha = 0.96f),
        tonalElevation = 0.dp,
        shadowElevation = 8.dp,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .height(NowenMobileMetrics.BottomBarHeight)
                .padding(horizontal = 8.dp, vertical = 7.dp),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MainTab.entries.forEach { item ->
                val selected = selectedTab == item
                WebMobileBottomBarItem(
                    item = item,
                    selected = selected,
                    onClick = { onSelect(item) },
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

@Composable
private fun WebMobileBottomBarItem(
    item: MainTab,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val contentColor = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant
    Box(
        modifier = modifier
            .padding(horizontal = 3.dp)
            .height(NowenMobileMetrics.TouchTarget)
            .clip(RoundedCornerShape(NowenMobileMetrics.ControlRadius))
            .background(
                if (selected) MaterialTheme.colorScheme.primaryContainer else Color.Transparent,
            )
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(6.dp, Alignment.CenterHorizontally),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                if (selected) item.selectedIcon else item.icon,
                contentDescription = item.label,
                tint = contentColor,
                modifier = Modifier.size(20.dp),
            )
            if (selected) {
                Text(
                    item.label,
                    color = contentColor,
                    fontSize = 12.sp,
                    lineHeight = 16.sp,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                )
            }
        }
    }
}

@Composable
private fun PlaybackPictureInPictureBinding(
    enabled: Boolean,
    content: @Composable (Boolean, Boolean) -> Unit,
) {
    val context = LocalContext.current
    val host = remember(context) { context.findPlaybackPictureInPictureHost() }
    val fallbackMode = remember { MutableStateFlow(false) }
    val inPictureInPictureMode by (host?.pictureInPictureMode ?: fallbackMode).collectAsState()

    val available = enabled && (host?.playbackPictureInPictureSupported == true)
    DisposableEffect(host, available) {
        host?.setPlaybackPictureInPictureActive(available)
        onDispose {
            host?.setPlaybackPictureInPictureActive(false)
        }
    }

    content(inPictureInPictureMode, available)
}
