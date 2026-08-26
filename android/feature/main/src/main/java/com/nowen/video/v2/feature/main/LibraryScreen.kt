package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items as lazyItems
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.FilterList
import androidx.compose.material.icons.filled.GridView
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Sort
import androidx.compose.material.icons.filled.SwapVert
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.VerticalDivider
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.paging.LoadState
import androidx.paging.PagingData
import androidx.paging.cachedIn
import androidx.paging.compose.LazyPagingItems
import androidx.paging.compose.collectAsLazyPagingItems
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.CatalogRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.HillsChoiceRail
import com.nowen.video.v2.core.designsystem.HillsPoster
import com.nowen.video.v2.core.designsystem.HillsPrimaryAction
import com.nowen.video.v2.core.designsystem.HillsSecondaryAction
import com.nowen.video.v2.core.designsystem.HillsState
import com.nowen.video.v2.core.designsystem.HillsTextField
import com.nowen.video.v2.core.model.LibraryContentType
import com.nowen.video.v2.core.model.LibraryFilter
import com.nowen.video.v2.core.model.LibraryOrder
import com.nowen.video.v2.core.model.LibrarySort
import com.nowen.video.v2.core.model.LibrarySummary
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MediaDetail
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

private const val EXPANDED_LIBRARY_WIDTH_DP = 840

data class LibraryUiState(
    val libraries: List<LibrarySummary> = emptyList(),
    val librariesLoading: Boolean = true,
    val librariesError: String? = null,
    val libraryPreviews: Map<String, MediaCard?> = emptyMap(),
    val filter: LibraryFilter = LibraryFilter(),
    val selectedMediaId: String? = null,
    val selectedDetail: MediaDetail? = null,
    val selectedLoading: Boolean = false,
    val selectedError: String? = null,
)

@OptIn(ExperimentalCoroutinesApi::class)
@HiltViewModel
class LibraryViewModel @Inject constructor(
    private val repository: CatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val filterFlow = MutableStateFlow(LibraryFilter())
    private val _state = MutableStateFlow(LibraryUiState())
    val state: StateFlow<LibraryUiState> = _state
    val media: Flow<PagingData<MediaCard>> = filterFlow
        .flatMapLatest { repository.pagedMedia(it) }
        .cachedIn(viewModelScope)
    private var selectionJob: Job? = null

    init {
        refreshLibraries()
    }

    fun refreshLibraries() {
        viewModelScope.launch {
            _state.update { it.copy(librariesLoading = true, librariesError = null) }
            repository.libraries()
                .onSuccess { libraries ->
                    _state.update {
                        it.copy(
                            libraries = libraries,
                            librariesLoading = false,
                            librariesError = null,
                            libraryPreviews = emptyMap(),
                        )
                    }
                    val previews = coroutineScope {
                        libraries.map { library ->
                            async {
                                library.id to repository.media(size = 1, libraryId = library.id)
                                    .getOrNull()
                                    ?.data
                                    ?.firstOrNull()
                            }
                        }.awaitAll().toMap()
                    }
                    _state.update { current ->
                        if (current.libraries == libraries) current.copy(libraryPreviews = previews) else current
                    }
                }
                .onFailure { error ->
                    _state.update {
                        it.copy(
                            librariesLoading = false,
                            librariesError = error.message ?: "媒体库列表加载失败",
                        )
                    }
                }
        }
    }

    fun applyFilter(filter: LibraryFilter) {
        val normalized = filter.normalized()
        if (normalized == filterFlow.value) return
        filterFlow.value = normalized
        selectionJob?.cancel()
        _state.update {
            it.copy(
                filter = normalized,
                selectedMediaId = null,
                selectedDetail = null,
                selectedLoading = false,
                selectedError = null,
            )
        }
    }

    fun resetFilter() = applyFilter(LibraryFilter())

    fun selectMedia(id: String) {
        if (id.isBlank()) return
        if (_state.value.selectedMediaId == id && _state.value.selectedDetail != null) return
        selectionJob?.cancel()
        _state.update {
            it.copy(
                selectedMediaId = id,
                selectedDetail = null,
                selectedLoading = true,
                selectedError = null,
            )
        }
        selectionJob = viewModelScope.launch {
            repository.detail(id)
                .onSuccess { detail ->
                    if (_state.value.selectedMediaId == id) {
                        _state.update { it.copy(selectedDetail = detail, selectedLoading = false) }
                    }
                }
                .onFailure { error ->
                    if (_state.value.selectedMediaId == id) {
                        _state.update {
                            it.copy(
                                selectedLoading = false,
                                selectedError = error.message ?: "详情加载失败",
                            )
                        }
                    }
                }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LibraryScreen(
    modifier: Modifier = Modifier,
    onMediaClick: (MediaCard) -> Unit,
    onPlay: (String) -> Unit,
    onBrowseLibrary: (LibrarySummary) -> Unit,
    viewModel: LibraryViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    val media = viewModel.media.collectAsLazyPagingItems()
    var showFilters by rememberSaveable { mutableStateOf(false) }

    LibraryLandingScreen(
        media = media,
        state = state,
        baseUrl = session.activeServer?.baseUrl,
        onFilterClick = { showFilters = true },
        onRefresh = {
            viewModel.refreshLibraries()
            media.refresh()
        },
        onFilterChange = viewModel::applyFilter,
        onMediaClick = onMediaClick,
        onPlay = onPlay,
        onBrowseLibrary = onBrowseLibrary,
        modifier = modifier.fillMaxSize(),
    )

    if (showFilters) {
        LibraryFilterSheet(
            current = state.filter,
            libraries = state.libraries,
            librariesLoading = state.librariesLoading,
            librariesError = state.librariesError,
            onRetryLibraries = viewModel::refreshLibraries,
            onApply = {
                viewModel.applyFilter(it)
                showFilters = false
            },
            onReset = {
                viewModel.resetFilter()
                showFilters = false
            },
            onDismiss = { showFilters = false },
        )
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LibraryBrowseScreen(
    libraryId: String,
    libraryName: String,
    onBack: () -> Unit,
    onMediaClick: (MediaCard) -> Unit,
    viewModel: LibraryViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    val media = viewModel.media.collectAsLazyPagingItems()
    var showFilters by rememberSaveable(libraryId) { mutableStateOf(false) }

    LaunchedEffect(libraryId) {
        viewModel.applyFilter(
            LibraryFilter(
                libraryId = libraryId,
                sort = state.filter.sort,
                order = state.filter.order,
            ),
        )
    }

    LibraryCatalogPane(
        media = media,
        state = state,
        baseUrl = session.activeServer?.baseUrl,
        title = libraryName,
        onBack = onBack,
        onFilterClick = { showFilters = true },
        onFilterChange = viewModel::applyFilter,
        onMediaClick = onMediaClick,
        modifier = Modifier.fillMaxSize(),
    )

    if (showFilters) {
        LibraryFilterSheet(
            current = state.filter,
            libraries = state.libraries,
            librariesLoading = state.librariesLoading,
            librariesError = state.librariesError,
            lockedLibraryId = libraryId,
            onRetryLibraries = viewModel::refreshLibraries,
            onApply = {
                viewModel.applyFilter(it.copy(libraryId = libraryId))
                showFilters = false
            },
            onReset = {
                viewModel.applyFilter(LibraryFilter(libraryId = libraryId))
                showFilters = false
            },
            onDismiss = { showFilters = false },
        )
    }
}

@Composable
private fun LibraryLandingScreen(
    media: LazyPagingItems<MediaCard>,
    state: LibraryUiState,
    baseUrl: String?,
    onFilterClick: () -> Unit,
    onRefresh: () -> Unit,
    onFilterChange: (LibraryFilter) -> Unit,
    onMediaClick: (MediaCard) -> Unit,
    onPlay: (String) -> Unit,
    onBrowseLibrary: (LibrarySummary) -> Unit,
    modifier: Modifier = Modifier,
) {
    val loadedItems = media.itemSnapshotList.items.filter { it.resolvedId.isNotBlank() }
    val heroItems = loadedItems.take(4)
    val hero = heroItems.firstOrNull()
    val continueWatching = loadedItems.filter { it.normalizedProgress > 0f }
    val nextUp = (continueWatching + loadedItems).distinctBy(MediaCard::resolvedId).take(10)

    LazyColumn(
        modifier = modifier.background(Color(0xFF101116)),
        contentPadding = PaddingValues(bottom = 102.dp),
        verticalArrangement = Arrangement.spacedBy(28.dp),
    ) {
        when {
            media.loadState.refresh is LoadState.Loading && hero == null -> {
                item {
                    Box(Modifier.fillMaxWidth().height(520.dp), contentAlignment = Alignment.Center) {
                        CircularProgressIndicator()
                    }
                }
            }

            media.loadState.refresh is LoadState.Error && hero == null -> {
                val error = (media.loadState.refresh as LoadState.Error).error
                item {
                    HillsState(
                        title = "媒体库加载失败",
                        message = error.message ?: "无法连接服务器",
                        actionLabel = "重试",
                        onAction = media::retry,
                        modifier = Modifier.padding(top = 160.dp),
                    )
                }
            }

            hero == null -> {
                item {
                    HillsState(
                        title = "媒体库还是空的",
                        message = "请先在服务器端添加媒体库并完成扫描。",
                        modifier = Modifier.padding(top = 160.dp),
                    )
                }
            }

            else -> {
                item {
                    LibraryLandingHero(
                        items = heroItems,
                        baseUrl = baseUrl,
                        onOpen = onMediaClick,
                        onFilterClick = onFilterClick,
                        onRefresh = onRefresh,
                    )
                }

                // 媒体库分类是首页的一级入口，必须紧随 Hero 展示。
                if (state.libraries.isNotEmpty()) {
                    item {
                        LibraryLandingLibraryShelf(
                            libraries = state.libraries,
                            previews = state.libraryPreviews,
                            baseUrl = baseUrl,
                            onClick = onBrowseLibrary,
                        )
                    }
                }

                if (continueWatching.isNotEmpty()) {
                    item {
                        LibraryLandingShelf(
                            title = "继续观看",
                            items = continueWatching,
                            baseUrl = baseUrl,
                            onClick = onPlay,
                        )
                    }
                }

                item {
                    LibraryLandingShelf(
                        title = "接下来",
                        items = nextUp,
                        baseUrl = baseUrl,
                        onClick = { id ->
                            loadedItems.firstOrNull { it.resolvedId == id }?.let(onMediaClick)
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun LibraryLandingHero(
    items: List<MediaCard>,
    baseUrl: String?,
    onOpen: (MediaCard) -> Unit,
    onFilterClick: () -> Unit,
    onRefresh: () -> Unit,
) {
    var selectedIndex by rememberSaveable(items.map(MediaCard::resolvedId)) { mutableStateOf(0) }
    var dragDistance by remember { mutableStateOf(0f) }
    val safeIndex = selectedIndex.coerceIn(0, items.lastIndex)
    val media = items[safeIndex]
    val usePosterAsBackground = media.resolvedBackdrop.isNullOrBlank()
    val imageUrl = resolveImage(
        baseUrl,
        if (usePosterAsBackground) media.resolvedPoster else media.resolvedBackdrop,
    )

    Box(
        modifier = Modifier
            .fillMaxWidth()
            .height(590.dp)
            .background(MaterialTheme.colorScheme.surfaceVariant)
            .pointerInput(items.map(MediaCard::resolvedId), safeIndex) {
                detectHorizontalDragGestures(
                    onDragStart = { dragDistance = 0f },
                    onHorizontalDrag = { _, amount -> dragDistance += amount },
                    onDragCancel = { dragDistance = 0f },
                    onDragEnd = {
                        if (items.size > 1 && dragDistance > 54f) {
                            selectedIndex = (safeIndex - 1 + items.size) % items.size
                        } else if (items.size > 1 && dragDistance < -54f) {
                            selectedIndex = (safeIndex + 1) % items.size
                        }
                        dragDistance = 0f
                    },
                )
            }
            .clickable(onClick = { onOpen(media) }),
    ) {
        AsyncImage(
            model = imageUrl,
            contentDescription = media.displayTitle,
            contentScale = ContentScale.Crop,
            modifier = Modifier
                .fillMaxSize()
                .then(
                    if (usePosterAsBackground) {
                        Modifier
                            .graphicsLayer { scaleX = 1.12f; scaleY = 1.12f }
                            .blur(18.dp)
                    } else Modifier,
                ),
        )
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(
                    Brush.verticalGradient(
                        0f to Color.Black.copy(alpha = 0.16f),
                        0.44f to Color.Black.copy(alpha = 0.18f),
                        0.76f to MaterialTheme.colorScheme.background.copy(alpha = 0.70f),
                        1f to MaterialTheme.colorScheme.background,
                    ),
                ),
        )
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(horizontal = 20.dp, vertical = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                "媒体库",
                color = Color.White,
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold,
                modifier = Modifier.weight(1f),
            )
            IconButton(
                onClick = onFilterClick,
                modifier = Modifier
                    .clip(RoundedCornerShape(999.dp))
                    .background(Color.Black.copy(alpha = 0.34f))
                    .border(1.dp, Color.White.copy(alpha = 0.12f), RoundedCornerShape(999.dp)),
            ) {
                Icon(Icons.Default.FilterList, contentDescription = "筛选媒体库", tint = Color.White)
            }
            IconButton(
                onClick = onRefresh,
                modifier = Modifier
                    .clip(RoundedCornerShape(999.dp))
                    .background(Color.Black.copy(alpha = 0.34f))
                    .border(1.dp, Color.White.copy(alpha = 0.12f), RoundedCornerShape(999.dp)),
            ) {
                Icon(Icons.Default.Refresh, contentDescription = "刷新媒体库", tint = Color.White)
            }
        }
        Column(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 24.dp),
        ) {
            Text(
                media.displayTitle,
                color = Color.White,
                style = MaterialTheme.typography.headlineLarge,
                fontWeight = FontWeight.Bold,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            val metadata = listOfNotNull(
                media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
                media.year?.toString(),
                media.genres.takeIf { it.isNotBlank() },
            ).joinToString("  ")
            if (metadata.isNotBlank()) {
                Spacer(Modifier.height(8.dp))
                Text(
                    metadata,
                    color = Color.White.copy(alpha = 0.86f),
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            media.overview.takeIf { it.isNotBlank() }?.let { overview ->
                Spacer(Modifier.height(10.dp))
                Text(
                    overview,
                    color = Color.White.copy(alpha = 0.82f),
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Spacer(Modifier.height(16.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.Center,
            ) {
                items.indices.forEach { index ->
                    Box(
                        modifier = Modifier
                            .padding(horizontal = 4.dp)
                            .width(if (index == safeIndex) 12.dp else 6.dp)
                            .height(6.dp)
                            .clip(RoundedCornerShape(999.dp))
                            .background(if (index == safeIndex) Color.White else Color.White.copy(alpha = 0.35f)),
                    )
                }
            }
        }
    }
}

@Composable
private fun LibraryLandingShelf(
    title: String,
    items: List<MediaCard>,
    baseUrl: String?,
    onClick: (String) -> Unit,
) {
    if (items.isEmpty()) return
    Column(Modifier.fillMaxWidth()) {
        Text(
            title,
            color = Color.White,
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(horizontal = 20.dp),
        )
        Spacer(Modifier.height(11.dp))
        LazyRow(
            contentPadding = PaddingValues(horizontal = 20.dp),
            horizontalArrangement = Arrangement.spacedBy(9.dp),
        ) {
            lazyItems(items, key = MediaCard::resolvedId) { item ->
                LibraryLandingTile(
                    media = item,
                    imageUrl = resolveImage(baseUrl, item.resolvedBackdrop ?: item.resolvedPoster),
                    onClick = { onClick(item.resolvedId) },
                )
            }
        }
    }
}

@Composable
private fun LibraryLandingTile(
    media: MediaCard,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Column(
        modifier = Modifier
            .width(172.dp)
            .clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(16f / 9f)
                .clip(RoundedCornerShape(10.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant)
                .border(1.dp, MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.82f), RoundedCornerShape(10.dp)),
        ) {
            AsyncImage(
                model = imageUrl,
                contentDescription = media.displayTitle,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize(),
            )
            if (media.normalizedProgress > 0f) {
                Box(
                    modifier = Modifier
                        .align(Alignment.BottomStart)
                        .fillMaxWidth(media.normalizedProgress)
                        .height(4.dp)
                        .background(MaterialTheme.colorScheme.primary),
                )
            }
        }
        Text(
            media.displayTitle,
            color = Color.White,
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 8.dp).fillMaxWidth(),
            textAlign = androidx.compose.ui.text.style.TextAlign.Center,
        )
        Text(
            listOfNotNull(media.year?.toString(), media.episodeTitle.takeIf { it.isNotBlank() }).joinToString(" · "),
            color = Color(0xFFB6BCC8),
            style = MaterialTheme.typography.bodySmall,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 2.dp, bottom = 2.dp).fillMaxWidth(),
            textAlign = androidx.compose.ui.text.style.TextAlign.Center,
        )
    }
}

@Composable
private fun LibraryLandingLibraryShelf(
    libraries: List<LibrarySummary>,
    previews: Map<String, MediaCard?>,
    baseUrl: String?,
    onClick: (LibrarySummary) -> Unit,
) {
    Column(Modifier.fillMaxWidth()) {
        Text(
            "媒体库",
            color = Color.White,
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.Bold,
            modifier = Modifier.padding(horizontal = 20.dp),
        )
        Spacer(Modifier.height(11.dp))
        LazyRow(
            contentPadding = PaddingValues(horizontal = 20.dp),
            horizontalArrangement = Arrangement.spacedBy(9.dp),
        ) {
            lazyItems(libraries, key = LibrarySummary::id) { library ->
                LibraryLandingLibraryTile(
                    library = library,
                    preview = previews[library.id],
                    imageUrl = previews[library.id]?.let { preview ->
                        resolveImage(baseUrl, preview.resolvedBackdrop ?: preview.resolvedPoster)
                    },
                    onClick = { onClick(library) },
                )
            }
        }
    }
}

@Composable
private fun LibraryLandingLibraryTile(
    library: LibrarySummary,
    preview: MediaCard?,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Column(
        modifier = Modifier
            .width(172.dp)
            .clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(16f / 9f)
                .clip(RoundedCornerShape(10.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant)
                .border(1.dp, MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.82f), RoundedCornerShape(10.dp)),
        ) {
            if (imageUrl != null) {
                AsyncImage(
                    model = imageUrl,
                    contentDescription = library.name,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                )
            }
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.42f)))),
            )
        }
        Text(
            library.name,
            color = Color.White,
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 8.dp).fillMaxWidth(),
            textAlign = androidx.compose.ui.text.style.TextAlign.Center,
        )
        Text(
            libraryTypeLabel(library.type, preview),
            color = Color(0xFFB6BCC8),
            style = MaterialTheme.typography.bodySmall,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 2.dp, bottom = 2.dp).fillMaxWidth(),
            textAlign = androidx.compose.ui.text.style.TextAlign.Center,
        )
    }
}

private fun libraryTypeLabel(type: String, preview: MediaCard?): String = when {
    type.contains("series", ignoreCase = true) -> "剧集媒体库"
    type.contains("movie", ignoreCase = true) -> "电影媒体库"
    preview?.isSeries == true -> "剧集媒体库"
    preview != null -> "影片媒体库"
    else -> "媒体库"
}

@Composable
private fun LibraryCatalogPane(
    media: LazyPagingItems<MediaCard>,
    state: LibraryUiState,
    baseUrl: String?,
    title: String = "媒体库",
    onBack: (() -> Unit)? = null,
    onFilterClick: () -> Unit,
    onFilterChange: (LibraryFilter) -> Unit,
    onMediaClick: (MediaCard) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .background(MaterialTheme.colorScheme.background)
            .windowInsetsPadding(WindowInsets.statusBars),
    ) {
        LibraryHeader(
            title = title,
            onBack = onBack,
        )
        LibraryQuickFilters(
            filter = state.filter,
            loadedCount = media.itemCount,
            onFilterClick = onFilterClick,
            onChange = onFilterChange,
        )

        Box(Modifier.weight(1f)) {
            when {
                media.loadState.refresh is LoadState.Loading && media.itemCount == 0 -> {
                    CircularProgressIndicator(Modifier.align(Alignment.Center))
                }

                media.loadState.refresh is LoadState.Error && media.itemCount == 0 -> {
                    val error = (media.loadState.refresh as LoadState.Error).error
                    HillsState(
                        title = "媒体库加载失败",
                        message = error.message ?: "无法连接服务器",
                        actionLabel = "重试",
                        onAction = media::retry,
                        modifier = Modifier.align(Alignment.Center),
                    )
                }

                media.itemCount == 0 -> {
                    HillsState(
                        title = if (state.filter.activeFilterCount > 0) "没有符合条件的内容" else "媒体库还是空的",
                        message = if (state.filter.activeFilterCount > 0) "尝试清除部分筛选条件或切换媒体库。" else "请先在服务器端添加媒体库并完成扫描。",
                        modifier = Modifier.align(Alignment.Center),
                    )
                }

                else -> LibraryGrid(
                    media = media,
                    state = state,
                    baseUrl = baseUrl,
                    onMediaClick = onMediaClick,
                )
            }
        }
    }
}

@Composable
private fun LibraryGrid(
    media: LazyPagingItems<MediaCard>,
    state: LibraryUiState,
    baseUrl: String?,
    onMediaClick: (MediaCard) -> Unit,
) {
    LazyVerticalGrid(
        columns = GridCells.Fixed(3),
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 16.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
        verticalArrangement = Arrangement.spacedBy(22.dp),
    ) {
        items(count = media.itemCount) { index ->
            val item = media[index]
            if (item == null) {
                Box(
                    Modifier
                        .fillMaxWidth()
                        .aspectRatio(2f / 3f)
                        .clip(MaterialTheme.shapes.large)
                        .background(MaterialTheme.colorScheme.surfaceVariant),
                )
            } else {
                LibraryGridPoster(
                    media = item,
                    imageUrl = resolveImage(baseUrl, item.resolvedPoster),
                    selected = item.resolvedId == state.selectedMediaId,
                    onClick = { onMediaClick(item) },
                )
            }
        }

        when (val append = media.loadState.append) {
            is LoadState.Loading -> item(span = { GridItemSpan(maxLineSpan) }) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(24.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator()
                }
            }

            is LoadState.Error -> item(span = { GridItemSpan(maxLineSpan) }) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(20.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Text(
                        append.error.message ?: "下一页加载失败",
                        color = MaterialTheme.colorScheme.error,
                    )
                    HillsSecondaryAction(
                        label = "重试",
                        onClick = media::retry,
                        modifier = Modifier.width(96.dp),
                    )
                }
            }

            else -> Unit
        }
    }
}

@Composable
private fun LibraryGridPoster(
    media: MediaCard,
    imageUrl: String?,
    selected: Boolean,
    onClick: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(10.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant)
                .border(
                    1.dp,
                    if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.82f),
                    RoundedCornerShape(10.dp),
                ),
        ) {
            AsyncImage(
                model = imageUrl,
                contentDescription = media.displayTitle,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize(),
            )
            media.rating.takeIf { it > 0 }?.let { rating ->
                Text(
                    "%.1f".format(rating),
                    color = Color.White,
                    style = MaterialTheme.typography.labelSmall,
                    fontWeight = FontWeight.SemiBold,
                    modifier = Modifier
                        .align(Alignment.BottomEnd)
                        .padding(5.dp)
                        .clip(RoundedCornerShape(7.dp))
                        .background(Color.Black.copy(alpha = 0.62f))
                        .border(1.dp, Color.White.copy(alpha = 0.10f), RoundedCornerShape(7.dp))
                        .padding(horizontal = 5.dp, vertical = 2.dp),
                )
            }
            if (media.normalizedProgress > 0f) {
                Box(
                    modifier = Modifier
                        .align(Alignment.BottomStart)
                        .fillMaxWidth(media.normalizedProgress)
                        .height(3.dp)
                        .background(MaterialTheme.colorScheme.primary),
                )
            }
        }
        Text(
            media.displayTitle,
            color = Color.White,
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.SemiBold,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
            textAlign = androidx.compose.ui.text.style.TextAlign.Center,
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 7.dp),
        )
        media.year?.let { year ->
            Text(
                year.toString(),
                color = Color(0xFFAEB5C4),
                style = MaterialTheme.typography.bodySmall,
                textAlign = androidx.compose.ui.text.style.TextAlign.Center,
                modifier = Modifier.fillMaxWidth().padding(top = 1.dp),
            )
        }
    }
}

@Composable
private fun LibraryHeader(
    title: String,
    onBack: (() -> Unit)?,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = 20.dp, end = 10.dp, top = 12.dp, bottom = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (onBack != null) {
            IconButton(onClick = onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回媒体库首页")
            }
            Spacer(Modifier.width(4.dp))
        }
        Text(
            title,
            style = MaterialTheme.typography.headlineMedium,
            modifier = Modifier.weight(1f),
        )
    }
}

@Composable
private fun LibraryQuickFilters(
    filter: LibraryFilter,
    loadedCount: Int,
    onFilterClick: () -> Unit,
    onChange: (LibraryFilter) -> Unit,
) {
    val nextSort = LibrarySort.entries[(LibrarySort.entries.indexOf(filter.sort) + 1) % LibrarySort.entries.size]
    Column {
        val tabs = listOf("全部", "继续观看", "收藏", "类型", "标签")
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .horizontalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 8.dp),
            horizontalArrangement = Arrangement.spacedBy(36.dp),
        ) {
            tabs.forEachIndexed { index, label ->
                val selected = index == 0
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(
                        label,
                        color = if (selected) Color(0xFF8298D1) else Color(0xFFD0D4DE),
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
                        modifier = Modifier.padding(vertical = 8.dp),
                    )
                    Box(
                        modifier = Modifier
                            .width(if (selected) 34.dp else 0.dp)
                            .height(3.dp)
                            .background(if (selected) Color(0xFF8298D1) else Color.Transparent),
                    )
                }
            }
        }
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(Icons.Default.Sort, contentDescription = null, tint = Color(0xFF8298D1))
            Spacer(Modifier.width(12.dp))
            TextButton(
                onClick = {
                    onChange(
                        filter.copy(
                            sort = nextSort,
                            order = if (nextSort == LibrarySort.Title) LibraryOrder.Ascending else LibraryOrder.Descending,
                        ),
                    )
                },
                contentPadding = PaddingValues(0.dp),
            ) {
                Text(sortLabel(filter.sort), style = MaterialTheme.typography.titleMedium)
                Icon(Icons.Default.KeyboardArrowDown, contentDescription = "切换排序")
            }
            Spacer(Modifier.weight(1f))
            Icon(Icons.Default.GridView, contentDescription = "三列网格", tint = Color(0xFFF2F4FA))
            Spacer(Modifier.width(22.dp))
            IconButton(onClick = onFilterClick) {
                Icon(Icons.Default.FilterList, contentDescription = "筛选媒体库", tint = Color(0xFFF2F4FA))
            }
            IconButton(
                onClick = {
                    val nextOrder = if (filter.order == LibraryOrder.Descending) LibraryOrder.Ascending else LibraryOrder.Descending
                    onChange(filter.copy(order = nextOrder))
                },
            ) {
                Icon(Icons.Default.SwapVert, contentDescription = "切换排序方向", tint = Color(0xFFF2F4FA))
            }
            Text(
                "$loadedCount 项目",
                style = MaterialTheme.typography.titleMedium,
                color = Color(0xFFF2F4FA),
            )
        }
        HorizontalDivider()
    }
}

@Composable
private fun LibraryDetailPane(
    state: LibraryUiState,
    baseUrl: String?,
    onRetry: (String) -> Unit,
    onOpenDetail: (String) -> Unit,
    onPlay: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Box(modifier.background(MaterialTheme.colorScheme.surface)) {
        when {
            state.selectedMediaId == null -> HillsState(
                title = "选择一部影片",
                message = "在左侧媒体库选择内容后，可在这里预览详情并直接播放。",
                modifier = Modifier.align(Alignment.Center),
            )

            state.selectedLoading -> CircularProgressIndicator(Modifier.align(Alignment.Center))

            state.selectedError != null -> HillsState(
                title = "详情加载失败",
                message = state.selectedError,
                actionLabel = "重试",
                onAction = { state.selectedMediaId?.let(onRetry) },
                modifier = Modifier.align(Alignment.Center),
            )

            state.selectedDetail != null -> LibraryDetailContent(
                media = requireNotNull(state.selectedDetail),
                baseUrl = baseUrl,
                onOpenDetail = onOpenDetail,
                onPlay = onPlay,
            )
        }
    }
}

@Composable
private fun LibraryDetailContent(
    media: MediaDetail,
    baseUrl: String?,
    onOpenDetail: (String) -> Unit,
    onPlay: (String) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(24.dp),
    ) {
        AsyncImage(
            model = resolveImage(baseUrl, media.backdropPath.ifBlank { media.posterPath }),
            contentDescription = media.displayTitle,
            contentScale = ContentScale.Crop,
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(16f / 9f)
                .clip(MaterialTheme.shapes.extraLarge)
                .background(MaterialTheme.colorScheme.surfaceVariant),
        )
        Spacer(Modifier.height(20.dp))
        Row {
            AsyncImage(
                model = resolveImage(baseUrl, media.posterPath),
                contentDescription = media.displayTitle,
                contentScale = ContentScale.Crop,
                modifier = Modifier
                    .width(108.dp)
                    .aspectRatio(2f / 3f)
                    .clip(MaterialTheme.shapes.large)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
            )
            Spacer(Modifier.width(18.dp))
            Column(Modifier.weight(1f)) {
                Text(media.displayTitle, style = MaterialTheme.typography.headlineSmall)
                Spacer(Modifier.height(8.dp))
                Text(
                    listOfNotNull(
                        media.year.takeIf { it > 0 }?.toString(),
                        media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
                        media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
                    ).joinToString(" · "),
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                if (media.genres.isNotBlank()) {
                    Spacer(Modifier.height(10.dp))
                    Text(
                        media.genres,
                        color = MaterialTheme.colorScheme.primary,
                        style = MaterialTheme.typography.bodyMedium,
                    )
                }
            }
        }
        Spacer(Modifier.height(20.dp))
        HillsPrimaryAction(
            label = "立即播放",
            icon = Icons.Default.PlayArrow,
            onClick = { onPlay(media.id) },
            modifier = Modifier.fillMaxWidth(),
        )
        HillsSecondaryAction(
            label = "打开完整详情",
            onClick = { onOpenDetail(media.id) },
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 10.dp),
        )
        Spacer(Modifier.height(24.dp))
        Text("简介", style = MaterialTheme.typography.titleLarge)
        Spacer(Modifier.height(8.dp))
        Text(
            media.overview.ifBlank { "暂无简介" },
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            style = MaterialTheme.typography.bodyLarge,
        )
        Spacer(Modifier.height(36.dp))
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun LibraryFilterSheet(
    current: LibraryFilter,
    libraries: List<LibrarySummary>,
    librariesLoading: Boolean,
    librariesError: String?,
    lockedLibraryId: String? = null,
    onRetryLibraries: () -> Unit,
    onApply: (LibraryFilter) -> Unit,
    onReset: () -> Unit,
    onDismiss: () -> Unit,
) {
    var draft by remember(current) { mutableStateOf(current) }
    var yearFromText by remember(current) { mutableStateOf(current.yearFrom?.toString().orEmpty()) }
    var yearToText by remember(current) { mutableStateOf(current.yearTo?.toString().orEmpty()) }

    androidx.compose.ui.window.Dialog(onDismissRequest = onDismiss) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .padding(20.dp),
            shape = RoundedCornerShape(24.dp),
            color = MaterialTheme.colorScheme.surface,
            border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.86f)),
            shadowElevation = 3.dp,
        ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 20.dp),
        ) {
            Text("筛选媒体库", style = MaterialTheme.typography.headlineSmall)
            Spacer(Modifier.height(18.dp))
            HillsTextField(
                value = draft.query,
                onValueChange = { draft = draft.copy(query = it) },
                placeholder = "标题关键字",
            )
            Spacer(Modifier.height(12.dp))
            HillsTextField(
                value = draft.genre,
                onValueChange = { draft = draft.copy(genre = it) },
                placeholder = "类型标签，例如：科幻、喜剧",
            )

            if (lockedLibraryId == null) {
                FilterSectionTitle("媒体库")
                when {
                    librariesLoading -> CircularProgressIndicator(Modifier.padding(8.dp))
                    librariesError != null -> Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            librariesError,
                            color = MaterialTheme.colorScheme.error,
                            modifier = Modifier.weight(1f),
                        )
                        TextButton(onClick = onRetryLibraries) { Text("重试") }
                    }

                    else -> HillsChoiceRail(
                        options = listOf("" to "全部") + libraries.map { it.id to it.name },
                        selected = draft.libraryId.orEmpty(),
                        onSelect = { selected -> draft = draft.copy(libraryId = selected.ifBlank { null }) },
                    )
                }
            }

            FilterSectionTitle("内容类型")
            HillsChoiceRail(
                options = LibraryContentType.entries.map { it.name to contentTypeLabel(it) },
                selected = draft.contentType.name,
                onSelect = { key -> draft = draft.copy(contentType = LibraryContentType.valueOf(key)) },
            )

            FilterSectionTitle("年份范围")
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                HillsTextField(
                    value = yearFromText,
                    onValueChange = { yearFromText = it.filter(Char::isDigit).take(4) },
                    placeholder = "起始年份",
                    modifier = Modifier.weight(1f),
                )
                HillsTextField(
                    value = yearToText,
                    onValueChange = { yearToText = it.filter(Char::isDigit).take(4) },
                    placeholder = "结束年份",
                    modifier = Modifier.weight(1f),
                )
            }

            FilterSectionTitle("排序")
            HillsChoiceRail(
                options = LibrarySort.entries.map { it.name to sortLabel(it) },
                selected = draft.sort.name,
                onSelect = { key -> draft = draft.copy(sort = LibrarySort.valueOf(key)) },
            )
            HillsChoiceRail(
                options = LibraryOrder.entries.map { it.name to if (it == LibraryOrder.Ascending) "升序" else "降序" },
                selected = draft.order.name,
                onSelect = { key -> draft = draft.copy(order = LibraryOrder.valueOf(key)) },
            )

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 28.dp, bottom = 24.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                HillsPrimaryAction(label = "重置", onClick = onReset, modifier = Modifier.weight(1f))
                HillsPrimaryAction(
                    label = "应用",
                    onClick = {
                        onApply(
                            draft.copy(
                                yearFrom = yearFromText.toIntOrNull(),
                                yearTo = yearToText.toIntOrNull(),
                            ).normalized(),
                        )
                    },
                    modifier = Modifier.weight(1f),
                )
            }
        }
        }
    }
}

@Composable
private fun FilterSectionTitle(title: String) {
    Text(
        title,
        style = MaterialTheme.typography.titleMedium,
        modifier = Modifier.padding(top = 20.dp, bottom = 8.dp),
    )
}

@Composable
private fun ChoiceRow(content: @Composable RowScope.() -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        content = content,
    )
}

internal fun isExpandedLibraryLayout(widthDp: Int): Boolean = widthDp >= EXPANDED_LIBRARY_WIDTH_DP

internal fun libraryFilterSummary(filter: LibraryFilter): String = buildList {
    add(contentTypeLabel(filter.contentType))
    add(sortLabel(filter.sort) + if (filter.order == LibraryOrder.Ascending) "升序" else "降序")
    filter.yearFrom?.let { start ->
        add(if (filter.yearTo != null) "$start–${filter.yearTo}" else "$start 年后")
    } ?: filter.yearTo?.let { add("${it} 年前") }
    filter.genre.takeIf(String::isNotBlank)?.let(::add)
}.joinToString(" · ")

internal fun contentTypeLabel(type: LibraryContentType): String = when (type) {
    LibraryContentType.All -> "全部"
    LibraryContentType.Movies -> "电影"
    LibraryContentType.Series -> "剧集"
}

internal fun sortLabel(sort: LibrarySort): String = when (sort) {
    LibrarySort.Added -> "最近添加"
    LibrarySort.Title -> "标题"
    LibrarySort.Year -> "年份"
    LibrarySort.Rating -> "评分"
}
