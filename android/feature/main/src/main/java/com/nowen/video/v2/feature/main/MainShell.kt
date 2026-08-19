package com.nowen.video.v2.feature.main

import android.Manifest
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
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
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
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
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
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
                    onLibraryClick = { navController.navigate(MainTab.Library.route) },
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
                PlaybackPictureInPictureBinding(enabled = playerPreferences.pictureInPictureEnabled) { inPip ->
                    PlayerScreen(
                        mediaId = mediaId,
                        pictureInPictureMode = inPip,
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

@Composable
private fun WebMobileBottomBar(
    selectedTab: MainTab?,
    onSelect: (MainTab) -> Unit,
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 12.dp, vertical = 8.dp),
        shape = RoundedCornerShape(24.dp),
        color = MaterialTheme.colorScheme.surface,
        shadowElevation = 7.dp,
        tonalElevation = 0.dp,
        border = androidx.compose.foundation.BorderStroke(
            1.dp,
            MaterialTheme.colorScheme.outlineVariant,
        ),
    ) {
        NavigationBar(
            containerColor = MaterialTheme.colorScheme.surface,
            tonalElevation = 0.dp,
        ) {
            MainTab.entries.forEach { item ->
                val selected = selectedTab == item
                NavigationBarItem(
                    selected = selected,
                    onClick = { onSelect(item) },
                    icon = {
                        Icon(
                            if (selected) item.selectedIcon else item.icon,
                            contentDescription = item.label,
                        )
                    },
                    label = { Text(item.label) },
                    colors = NavigationBarItemDefaults.colors(
                        selectedIconColor = MaterialTheme.colorScheme.primary,
                        selectedTextColor = MaterialTheme.colorScheme.primary,
                        indicatorColor = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.78f),
                        unselectedIconColor = MaterialTheme.colorScheme.onSurfaceVariant,
                        unselectedTextColor = MaterialTheme.colorScheme.onSurfaceVariant,
                    ),
                )
            }
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
