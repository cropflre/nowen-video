package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
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
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.FilterList
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Sort
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedButton
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.input.KeyboardType
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
import com.nowen.video.v2.core.designsystem.MediaPosterCard
import com.nowen.video.v2.core.designsystem.MessagePanel
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
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

private const val EXPANDED_LIBRARY_WIDTH_DP = 840

data class LibraryUiState(
    val libraries: List<LibrarySummary> = emptyList(),
    val librariesLoading: Boolean = true,
    val librariesError: String? = null,
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
                        )
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
    onMediaClick: (String) -> Unit,
    onPlay: (String) -> Unit,
    viewModel: LibraryViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    val media = viewModel.media.collectAsLazyPagingItems()
    var showFilters by rememberSaveable { mutableStateOf(false) }

    BoxWithConstraints(modifier = modifier.fillMaxSize()) {
        val expanded = isExpandedLibraryLayout(maxWidth.value.toInt())
        LaunchedEffect(expanded, media.itemCount, state.selectedMediaId) {
            if (expanded && state.selectedMediaId == null && media.itemCount > 0) {
                media[0]?.resolvedId?.let(viewModel::selectMedia)
            }
        }

        if (expanded) {
            Row(Modifier.fillMaxSize()) {
                LibraryCatalogPane(
                    media = media,
                    state = state,
                    baseUrl = session.activeServer?.baseUrl,
                    onFilterClick = { showFilters = true },
                    onRefresh = {
                        viewModel.refreshLibraries()
                        media.refresh()
                    },
                    onFilterChange = viewModel::applyFilter,
                    onMediaClick = viewModel::selectMedia,
                    modifier = Modifier
                        .weight(0.58f)
                        .fillMaxHeight(),
                )
                VerticalDivider(Modifier.fillMaxHeight())
                LibraryDetailPane(
                    state = state,
                    baseUrl = session.activeServer?.baseUrl,
                    onRetry = viewModel::selectMedia,
                    onOpenDetail = onMediaClick,
                    onPlay = onPlay,
                    modifier = Modifier
                        .weight(0.42f)
                        .fillMaxHeight(),
                )
            }
        } else {
            LibraryCatalogPane(
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
                modifier = Modifier.fillMaxSize(),
            )
        }
    }

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

@Composable
private fun LibraryCatalogPane(
    media: LazyPagingItems<MediaCard>,
    state: LibraryUiState,
    baseUrl: String?,
    onFilterClick: () -> Unit,
    onRefresh: () -> Unit,
    onFilterChange: (LibraryFilter) -> Unit,
    onMediaClick: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .background(MaterialTheme.colorScheme.background)
            .windowInsetsPadding(WindowInsets.statusBars),
    ) {
        LibraryHeader(
            filter = state.filter,
            refreshing = media.loadState.refresh is LoadState.Loading,
            onFilterClick = onFilterClick,
            onRefresh = onRefresh,
        )
        LibraryQuickFilters(filter = state.filter, onChange = onFilterChange)

        Box(Modifier.weight(1f)) {
            when {
                media.loadState.refresh is LoadState.Loading && media.itemCount == 0 -> {
                    CircularProgressIndicator(Modifier.align(Alignment.Center))
                }

                media.loadState.refresh is LoadState.Error && media.itemCount == 0 -> {
                    val error = (media.loadState.refresh as LoadState.Error).error
                    MessagePanel(
                        title = "媒体库加载失败",
                        message = error.message ?: "无法连接服务器",
                        actionLabel = "重试",
                        onAction = media::retry,
                        modifier = Modifier
                            .align(Alignment.Center)
                            .padding(20.dp),
                    )
                }

                media.itemCount == 0 -> {
                    MessagePanel(
                        title = if (state.filter.activeFilterCount > 0) {
                            "没有符合条件的内容"
                        } else {
                            "媒体库还是空的"
                        },
                        message = if (state.filter.activeFilterCount > 0) {
                            "尝试清除部分筛选条件或切换媒体库。"
                        } else {
                            "请先在服务器端添加媒体库并完成扫描。"
                        },
                        modifier = Modifier
                            .align(Alignment.Center)
                            .padding(20.dp),
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
    onMediaClick: (String) -> Unit,
) {
    LazyVerticalGrid(
        columns = GridCells.Adaptive(minSize = 132.dp),
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = 20.dp, vertical = 18.dp),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
        verticalArrangement = Arrangement.spacedBy(18.dp),
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
                val selected = item.resolvedId == state.selectedMediaId
                Surface(
                    shape = MaterialTheme.shapes.extraLarge,
                    color = if (selected) {
                        MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.48f)
                    } else {
                        MaterialTheme.colorScheme.surface.copy(alpha = 0f)
                    },
                    tonalElevation = if (selected) 4.dp else 0.dp,
                ) {
                    MediaPosterCard(
                        title = item.displayTitle,
                        subtitle = item.year?.toString(),
                        imageUrl = resolveImage(baseUrl, item.resolvedPoster),
                        progress = item.normalizedProgress,
                        onClick = { onMediaClick(item.resolvedId) },
                        modifier = Modifier.padding(3.dp),
                    )
                }
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
                    TextButton(onClick = media::retry) { Text("重试") }
                }
            }

            else -> Unit
        }
    }
}

@Composable
private fun LibraryHeader(
    filter: LibraryFilter,
    refreshing: Boolean,
    onFilterClick: () -> Unit,
    onRefresh: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(start = 20.dp, end = 10.dp, top = 12.dp, bottom = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text("媒体库", style = MaterialTheme.typography.headlineMedium)
            Text(
                libraryFilterSummary(filter),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        IconButton(onClick = onFilterClick) {
            BadgedBox(
                badge = {
                    if (filter.activeFilterCount > 0) {
                        Badge { Text(filter.activeFilterCount.toString()) }
                    }
                },
            ) {
                Icon(Icons.Default.FilterList, contentDescription = "筛选媒体库")
            }
        }
        IconButton(onClick = onRefresh, enabled = !refreshing) {
            if (refreshing) {
                CircularProgressIndicator(Modifier.width(22.dp), strokeWidth = 2.dp)
            } else {
                Icon(Icons.Default.Refresh, contentDescription = "刷新媒体库")
            }
        }
    }
}

@Composable
private fun LibraryQuickFilters(
    filter: LibraryFilter,
    onChange: (LibraryFilter) -> Unit,
) {
    Column {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .horizontalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 6.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            LibraryContentType.entries.forEach { type ->
                FilterChip(
                    selected = filter.contentType == type,
                    onClick = { onChange(filter.copy(contentType = type)) },
                    label = { Text(contentTypeLabel(type)) },
                )
            }
            Spacer(Modifier.width(4.dp))
            LibrarySort.entries.forEach { sort ->
                FilterChip(
                    selected = filter.sort == sort,
                    onClick = {
                        onChange(
                            filter.copy(
                                sort = sort,
                                order = if (sort == LibrarySort.Title) {
                                    LibraryOrder.Ascending
                                } else {
                                    LibraryOrder.Descending
                                },
                            ),
                        )
                    },
                    leadingIcon = if (filter.sort == sort) {
                        { Icon(Icons.Default.Sort, contentDescription = null) }
                    } else {
                        null
                    },
                    label = { Text(sortLabel(sort)) },
                )
            }
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
            state.selectedMediaId == null -> MessagePanel(
                title = "选择一部影片",
                message = "在左侧媒体库选择内容后，可在这里预览详情并直接播放。",
                modifier = Modifier
                    .align(Alignment.Center)
                    .padding(28.dp),
            )

            state.selectedLoading -> CircularProgressIndicator(Modifier.align(Alignment.Center))

            state.selectedError != null -> MessagePanel(
                title = "详情加载失败",
                message = state.selectedError,
                actionLabel = "重试",
                onAction = { state.selectedMediaId?.let(onRetry) },
                modifier = Modifier
                    .align(Alignment.Center)
                    .padding(28.dp),
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
        Button(
            onClick = { onPlay(media.id) },
            modifier = Modifier.fillMaxWidth(),
        ) {
            Icon(Icons.Default.PlayArrow, contentDescription = null)
            Spacer(Modifier.width(8.dp))
            Text("立即播放")
        }
        OutlinedButton(
            onClick = { onOpenDetail(media.id) },
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 10.dp),
        ) {
            Text("打开完整详情")
        }
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
    onRetryLibraries: () -> Unit,
    onApply: (LibraryFilter) -> Unit,
    onReset: () -> Unit,
    onDismiss: () -> Unit,
) {
    var draft by remember(current) { mutableStateOf(current) }
    var yearFromText by remember(current) { mutableStateOf(current.yearFrom?.toString().orEmpty()) }
    var yearToText by remember(current) { mutableStateOf(current.yearTo?.toString().orEmpty()) }

    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 20.dp, vertical = 8.dp),
        ) {
            Text("筛选媒体库", style = MaterialTheme.typography.headlineSmall)
            Spacer(Modifier.height(18.dp))
            OutlinedTextField(
                value = draft.query,
                onValueChange = { draft = draft.copy(query = it) },
                label = { Text("标题关键字") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = draft.genre,
                onValueChange = { draft = draft.copy(genre = it) },
                label = { Text("类型标签，例如：科幻、喜剧") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )

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

                else -> ChoiceRow {
                    FilterChip(
                        selected = draft.libraryId == null,
                        onClick = { draft = draft.copy(libraryId = null) },
                        label = { Text("全部") },
                    )
                    libraries.forEach { library ->
                        FilterChip(
                            selected = draft.libraryId == library.id,
                            onClick = { draft = draft.copy(libraryId = library.id) },
                            label = { Text(library.name) },
                        )
                    }
                }
            }

            FilterSectionTitle("内容类型")
            ChoiceRow {
                LibraryContentType.entries.forEach { type ->
                    FilterChip(
                        selected = draft.contentType == type,
                        onClick = { draft = draft.copy(contentType = type) },
                        label = { Text(contentTypeLabel(type)) },
                    )
                }
            }

            FilterSectionTitle("年份范围")
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                OutlinedTextField(
                    value = yearFromText,
                    onValueChange = { yearFromText = it.filter(Char::isDigit).take(4) },
                    label = { Text("起始年份") },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    singleLine = true,
                    modifier = Modifier.weight(1f),
                )
                OutlinedTextField(
                    value = yearToText,
                    onValueChange = { yearToText = it.filter(Char::isDigit).take(4) },
                    label = { Text("结束年份") },
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                    singleLine = true,
                    modifier = Modifier.weight(1f),
                )
            }

            FilterSectionTitle("排序")
            ChoiceRow {
                LibrarySort.entries.forEach { sort ->
                    FilterChip(
                        selected = draft.sort == sort,
                        onClick = { draft = draft.copy(sort = sort) },
                        label = { Text(sortLabel(sort)) },
                    )
                }
            }
            ChoiceRow {
                LibraryOrder.entries.forEach { order ->
                    FilterChip(
                        selected = draft.order == order,
                        onClick = { draft = draft.copy(order = order) },
                        label = { Text(if (order == LibraryOrder.Ascending) "升序" else "降序") },
                    )
                }
            }

            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 28.dp, bottom = 24.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                OutlinedButton(onClick = onReset, modifier = Modifier.weight(1f)) {
                    Text("重置")
                }
                Button(
                    onClick = {
                        onApply(
                            draft.copy(
                                yearFrom = yearFromText.toIntOrNull(),
                                yearTo = yearToText.toIntOrNull(),
                            ).normalized(),
                        )
                    },
                    modifier = Modifier.weight(1f),
                ) {
                    Text("应用")
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
