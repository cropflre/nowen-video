package com.nowen.video.v2.feature.main

import android.Manifest
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.weight
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.VideoLibrary
import androidx.compose.material.icons.outlined.Home
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.Search
import androidx.compose.material.icons.outlined.VideoLibrary
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
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
import com.nowen.video.v2.core.data.ServerSessionStore
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
    Library("library", "影视库", Icons.Outlined.VideoLibrary, Icons.Filled.VideoLibrary),
    Search("search", "搜索", Icons.Outlined.Search, Icons.Filled.Search),
    Profile("profile", "我的", Icons.Outlined.Person, Icons.Filled.Person),
}

private const val DETAIL_ROUTE = "detail/{mediaId}"
private const val SERIES_DETAIL_ROUTE = "series/{seriesId}"
private const val PLAYER_ROUTE = "player/{mediaId}"
private const val OFFLINE_PLAYER_ROUTE = "offline/{mediaId}"
private const val DOWNLOADS_ROUTE = "downloads"
private const val FAVORITES_ROUTE = "favorites"
private const val HISTORY_ROUTE = "history"
private const val COLLECTIONS_ROUTE = "collections"
private const val COLLECTION_DETAIL_ROUTE = "collection/{collectionId}"
private const val PERSON_DETAIL_ROUTE = "person/{personId}"
private const val SETTINGS_ROUTE = "settings"

@HiltViewModel
class MainShellViewModel @Inject constructor(
    private val repository: NowenRepository,
    playerPreferencesStore: PlayerPreferencesStore,
    val store: ServerSessionStore,
) : ViewModel() {
    val playerPreferences: StateFlow<PlayerPreferences> = playerPreferencesStore.preferences.stateIn(
        viewModelScope,
        SharingStarted.Eagerly,
        PlayerPreferences(),
    )

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
        MainTab.Home.route -> MainTab.Home
        MainTab.Search.route, PERSON_DETAIL_ROUTE -> MainTab.Search
        MainTab.Profile.route, FAVORITES_ROUTE, HISTORY_ROUTE, DOWNLOADS_ROUTE, SETTINGS_ROUTE -> MainTab.Profile
        MainTab.Library.route, DETAIL_ROUTE, SERIES_DETAIL_ROUTE, COLLECTIONS_ROUTE, COLLECTION_DETAIL_ROUTE -> MainTab.Library
        else -> null
    }
    val showBottomBar = currentRoute != PLAYER_ROUTE && currentRoute != OFFLINE_PLAYER_ROUTE
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

    fun openDetail(mediaId: String) {
        if (mediaId.isNotBlank()) navController.navigate("detail/${Uri.encode(mediaId)}")
    }

    fun openSeries(seriesId: String) {
        if (seriesId.isNotBlank()) navController.navigate("series/${Uri.encode(seriesId)}")
    }

    fun openPlayer(mediaId: String) {
        if (mediaId.isNotBlank()) navController.navigate("player/${Uri.encode(mediaId)}")
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

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        bottomBar = {
            if (showBottomBar) {
                WebMobileBottomBar(
                    selectedTab = selectedTab,
                    onSelect = { item ->
                        navController.navigate(item.route) {
                            popUpTo(MainTab.Home.route) { saveState = true }
                            launchSingleTop = true
                            restoreState = true
                        }
                    },
                )
            }
        },
    ) { padding ->
        NavHost(
            navController = navController,
            startDestination = MainTab.Home.route,
            modifier = Modifier.padding(if (showBottomBar) padding else PaddingValues()),
        ) {
            composable(MainTab.Home.route) {
                HomeScreen(
                    onMediaClick = ::openDetail,
                    onPlay = ::openPlayer,
                    onLibraryClick = {
                        navController.navigate(MainTab.Library.route) {
                            launchSingleTop = true
                            restoreState = true
                        }
                    },
                    onHistoryClick = { navController.navigate(HISTORY_ROUTE) },
                    onFavoritesClick = { navController.navigate(FAVORITES_ROUTE) },
                )
            }
            composable(MainTab.Library.route) {
                LibraryScreen(
                    onMediaClick = ::openDetail,
                    onPlay = ::openPlayer,
                )
            }
            composable(MainTab.Search.route) {
                SearchScreen(
                    onMediaClick = ::openDetail,
                    onPersonClick = ::openPerson,
                    onCollectionClick = ::openCollection,
                )
            }
            composable(MainTab.Profile.route) {
                ProfileScreen(
                    sessionStore = viewModel.store,
                    onFavorites = { navController.navigate(FAVORITES_ROUTE) },
                    onHistory = { navController.navigate(HISTORY_ROUTE) },
                    onCollections = { navController.navigate(COLLECTIONS_ROUTE) },
                    onSettings = { navController.navigate(SETTINGS_ROUTE) },
                    onLogout = viewModel::logout,
                )
            }
            composable(DOWNLOADS_ROUTE) {
                DownloadsScreen(onPlayOffline = ::openOfflinePlayer)
            }
            composable(SETTINGS_ROUTE) {
                MobileSettingsScreen(onBack = { navController.popBackStack() })
            }
            composable(FAVORITES_ROUTE) {
                PagedFavoritesScreen(
                    onBack = { navController.popBackStack() },
                    onMediaClick = ::openDetail,
                )
            }
            composable(HISTORY_ROUTE) {
                PagedHistoryScreen(
                    onBack = { navController.popBackStack() },
                    onMediaClick = ::openDetail,
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
            ) { entry ->
                PersonDetailScreen(
                    personId = entry.arguments?.getString("personId").orEmpty(),
                    onBack = { navController.popBackStack() },
                    onMediaClick = ::openDetail,
                )
            }
            composable(
                route = SERIES_DETAIL_ROUTE,
                arguments = listOf(navArgument("seriesId") { type = NavType.StringType }),
            ) { entry ->
                SeriesDetailScreen(
                    seriesId = entry.arguments?.getString("seriesId").orEmpty(),
                    onBack = { navController.popBackStack() },
                    onEpisodeClick = ::openDetail,
                    onPlayEpisode = ::openPlayer,
                    onPersonClick = ::openPerson,
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
                    onPlay = ::openPlayer,
                    onPersonClick = ::openPerson,
                    onCollectionClick = ::openCollection,
                )
            }
            composable(
                route = PLAYER_ROUTE,
                arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
            ) { entry ->
                val mediaId = entry.arguments?.getString("mediaId").orEmpty()
                PlaybackPictureInPictureBinding(enabled = playerPreferences.pictureInPictureEnabled) { _ ->
                    PlayerScreen(
                        mediaId = mediaId,
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
                PlaybackPictureInPictureBinding(enabled = playerPreferences.pictureInPictureEnabled) {
                    OfflinePlayerScreen(
                        mediaId = mediaId,
                        onBack = { navController.popBackStack() },
                    )
                }
            }
        }
    }
}

/**
 * Web 移动端同款悬浮底部导航：四个等宽入口，选中态是轻量紫色胶囊，
 * 而不是 Material NavigationBar 默认的大面积指示器。
 */
@Composable
private fun WebMobileBottomBar(
    selectedTab: MainTab?,
    onSelect: (MainTab) -> Unit,
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .navigationBarsPadding()
            .padding(start = 12.dp, end = 12.dp, top = 6.dp, bottom = 8.dp),
        shape = RoundedCornerShape(28.dp),
        color = MaterialTheme.colorScheme.surface.copy(alpha = 0.98f),
        shadowElevation = 8.dp,
        tonalElevation = 0.dp,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .height(72.dp)
                .padding(horizontal = 6.dp, vertical = 6.dp),
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
            .height(60.dp)
            .clip(RoundedCornerShape(15.dp))
            .background(
                if (selected) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.66f)
                else Color.Transparent,
            )
            .clickable(onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            Icon(
                if (selected) item.selectedIcon else item.icon,
                contentDescription = item.label,
                tint = contentColor,
                modifier = Modifier.size(23.dp),
            )
            Text(
                item.label,
                color = contentColor,
                fontSize = 12.sp,
                lineHeight = 16.sp,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            )
        }
    }
}

@Composable
private fun PlaybackPictureInPictureBinding(
    enabled: Boolean,
    content: @Composable (Boolean) -> Unit,
) {
    val context = LocalContext.current
    val host = remember(context) { context.findPlaybackPictureInPictureHost() }
    val fallbackMode = remember { MutableStateFlow(false) }
    val inPictureInPictureMode by (host?.pictureInPictureMode ?: fallbackMode).collectAsState()

    DisposableEffect(host, enabled) {
        host?.setPlaybackPictureInPictureActive(enabled)
        onDispose { host?.setPlaybackPictureInPictureActive(false) }
    }

    content(inPictureInPictureMode)
}
