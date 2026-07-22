package com.nowen.video.v2.feature.main

import android.net.Uri
import androidx.compose.foundation.layout.PaddingValues
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
private const val SERIES_DETAIL_ROUTE = "series/{seriesId}"
private const val PLAYER_ROUTE = "player/{mediaId}"
private const val OFFLINE_PLAYER_ROUTE = "offline/{mediaId}"
private const val FAVORITES_ROUTE = "favorites"
private const val HISTORY_ROUTE = "history"
private const val COLLECTIONS_ROUTE = "collections"
private const val COLLECTION_DETAIL_ROUTE = "collection/{collectionId}"
private const val PERSON_DETAIL_ROUTE = "person/{personId}"

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
            modifier = Modifier.padding(if (showBottomBar) padding else PaddingValues()),
        ) {
            composable(MainTab.Home.route) {
                HomeScreen(
                    onMediaClick = ::openDetail,
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
            composable(MainTab.Downloads.route) {
                DownloadsScreen(onPlayOffline = ::openOfflinePlayer)
            }
            composable(MainTab.Profile.route) {
                ProfileScreen(
                    sessionStore = viewModel.store,
                    onFavorites = { navController.navigate(FAVORITES_ROUTE) },
                    onHistory = { navController.navigate(HISTORY_ROUTE) },
                    onCollections = { navController.navigate(COLLECTIONS_ROUTE) },
                    onLogout = viewModel::logout,
                )
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
            composable(
                route = OFFLINE_PLAYER_ROUTE,
                arguments = listOf(navArgument("mediaId") { type = NavType.StringType }),
            ) { entry ->
                val mediaId = entry.arguments?.getString("mediaId").orEmpty()
                OfflinePlayerScreen(
                    mediaId = mediaId,
                    onBack = { navController.popBackStack() },
                )
            }
        }
    }
}
