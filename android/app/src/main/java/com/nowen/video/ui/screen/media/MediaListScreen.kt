package com.nowen.video.ui.screen.media

import androidx.compose.animation.*
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Sort
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.MixedItem
import com.nowen.video.data.repository.MediaRepository
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

// ==================== 筛选常量（与 Web 端 BrowsePage 一致） ====================

/** 排序选项 */
data class SortOption(val value: String, val label: String)

val SORT_OPTIONS = listOf(
    SortOption("created_desc", "最近添加"),
    SortOption("rating_desc", "评分最高"),
    SortOption("year_desc", "年份最新"),
    SortOption("year_asc", "年份最早"),
    SortOption("title_asc", "名称 A-Z"),
    SortOption("title_desc", "名称 Z-A"),
)

/** 年份范围 */
data class YearRange(val label: String, val min: Int, val max: Int)

val YEAR_RANGES = listOf(
    YearRange("全部", 0, 0),
    YearRange("2024-2026", 2024, 2026),
    YearRange("2020-2023", 2020, 2023),
    YearRange("2010-2019", 2010, 2019),
    YearRange("2000-2009", 2000, 2009),
    YearRange("更早", 0, 1999),
)

/** 评分选项 */
data class RatingOption(val label: String, val value: Double)

val RATING_OPTIONS = listOf(
    RatingOption("不限", 0.0),
    RatingOption("≥6分", 6.0),
    RatingOption("≥7分", 7.0),
    RatingOption("≥8分", 8.0),
    RatingOption("≥9分", 9.0),
)

// ==================== 主页面 ====================

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MediaListScreen(
    libraryId: String,
    onMediaClick: (String) -> Unit,
    onSeriesClick: (String) -> Unit,
    onBack: () -> Unit,
    viewModel: MediaListViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    val colorScheme = MaterialTheme.colorScheme
    val isDark = isSystemInDarkTheme()

    LaunchedEffect(libraryId) {
        viewModel.loadMedia(libraryId)
    }

    // ===== 筛选状态 =====
    var searchQuery by remember { mutableStateOf("") }
    var mediaType by remember { mutableStateOf("") } // "" | "movie" | "series"
    var selectedGenres by remember { mutableStateOf(setOf<String>()) }
    var selectedCountry by remember { mutableStateOf("") }
    var yearRange by remember { mutableStateOf(YEAR_RANGES[0]) }
    var minRating by remember { mutableStateOf(RATING_OPTIONS[0]) }
    var sortValue by remember { mutableStateOf(SORT_OPTIONS[0]) }
    var showFilters by remember { mutableStateOf(false) }
    var showSortMenu by remember { mutableStateOf(false) }

    // ===== 从数据中提取所有类型标签和地区 =====
    val allGenres = remember(uiState.mixedList) {
        val genres = mutableSetOf<String>()
        uiState.mixedList.forEach { item ->
            val g = if (item.type == "series") item.series?.genres else item.media?.genres
            g?.split(",")?.forEach { s -> s.trim().takeIf { it.isNotBlank() }?.let { genres.add(it) } }
        }
        genres.sorted()
    }

    val allCountries = remember(uiState.mixedList) {
        val countries = mutableSetOf<String>()
        uiState.mixedList.forEach { item ->
            val c = if (item.type == "series") item.series?.country else item.media?.country
            c?.split(",")?.forEach { s -> s.trim().takeIf { it.isNotBlank() }?.let { countries.add(it) } }
        }
        countries.sorted()
    }

    // ===== 前端筛选和排序 =====
    val filteredItems = remember(
        uiState.mixedList, searchQuery, mediaType, selectedGenres,
        selectedCountry, yearRange, minRating, sortValue
    ) {
        var items = uiState.mixedList.toList()

        // 媒体类型
        if (mediaType.isNotBlank()) {
            items = items.filter { it.type == mediaType }
        }

        // 搜索
        if (searchQuery.isNotBlank()) {
            val q = searchQuery.trim().lowercase()
            items = items.filter { item ->
                val title = if (item.type == "series") item.series?.title else item.media?.title
                val origTitle = if (item.type == "series") item.series?.origTitle else item.media?.origTitle
                (title?.lowercase()?.contains(q) == true) ||
                        (origTitle?.lowercase()?.contains(q) == true)
            }
        }

        // 类型标签（多选，全部匹配）
        if (selectedGenres.isNotEmpty()) {
            items = items.filter { item ->
                val genres = if (item.type == "series") item.series?.genres else item.media?.genres
                selectedGenres.all { g -> genres?.contains(g) == true }
            }
        }

        // 地区
        if (selectedCountry.isNotBlank()) {
            items = items.filter { item ->
                val country = if (item.type == "series") item.series?.country else item.media?.country
                country?.contains(selectedCountry) == true
            }
        }

        // 年份范围
        if (yearRange.min > 0 || yearRange.max > 0) {
            items = items.filter { item ->
                val year = if (item.type == "series") item.series?.year ?: 0 else item.media?.year ?: 0
                if (year == 0) return@filter false
                if (yearRange.min > 0 && year < yearRange.min) return@filter false
                if (yearRange.max > 0 && year > yearRange.max) return@filter false
                true
            }
        }

        // 评分
        if (minRating.value > 0) {
            items = items.filter { item ->
                val rating = if (item.type == "series") item.series?.rating ?: 0.0 else item.media?.rating ?: 0.0
                rating >= minRating.value
            }
        }

        // 排序
        val (field, dir) = sortValue.value.split("_")
        items = items.sortedWith(Comparator { a, b ->
            val cmp = when (field) {
                "title" -> {
                    val ta = if (a.type == "series") a.series?.title ?: "" else a.media?.title ?: ""
                    val tb = if (b.type == "series") b.series?.title ?: "" else b.media?.title ?: ""
                    ta.compareTo(tb)
                }
                "year" -> {
                    val ya = if (a.type == "series") a.series?.year ?: 0 else a.media?.year ?: 0
                    val yb = if (b.type == "series") b.series?.year ?: 0 else b.media?.year ?: 0
                    ya.compareTo(yb)
                }
                "rating" -> {
                    val ra = if (a.type == "series") a.series?.rating ?: 0.0 else a.media?.rating ?: 0.0
                    val rb = if (b.type == "series") b.series?.rating ?: 0.0 else b.media?.rating ?: 0.0
                    ra.compareTo(rb)
                }
                else -> { // created
                    val ca = if (a.type == "series") a.series?.createdAt ?: "" else a.media?.createdAt ?: ""
                    val cb = if (b.type == "series") b.series?.createdAt ?: "" else b.media?.createdAt ?: ""
                    ca.compareTo(cb)
                }
            }
            if (dir == "desc") -cmp else cmp
        })

        items
    }

    // 活跃筛选数量
    val activeFilterCount = listOfNotNull(
        if (selectedGenres.isNotEmpty()) true else null,
        if (selectedCountry.isNotBlank()) true else null,
        if (yearRange.min > 0 || yearRange.max > 0) true else null,
        if (minRating.value > 0) true else null,
    ).size

    // 统计
    val stats = remember(uiState.mixedList) {
        val movieCount = uiState.mixedList.count { it.type == "movie" }
        val seriesCount = uiState.mixedList.count { it.type == "series" }
        Triple(uiState.mixedList.size, movieCount, seriesCount)
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .spaceBackground()
    ) {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    title = {
                        Text(
                            uiState.libraryName.ifBlank { "媒体列表" },
                            color = colorScheme.primary,
                            style = MaterialTheme.typography.titleLarge.copy(letterSpacing = 1.sp)
                        )
                    },
                    navigationIcon = {
                        IconButton(onClick = onBack) {
                            Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = colorScheme.primary)
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(
                        containerColor = if (isDark) colorScheme.scrim.copy(alpha = 0.85f)
                        else colorScheme.surface.copy(alpha = 0.95f)
                    )
                )
            }
        ) { padding ->
            if (uiState.loading) {
                Box(
                    modifier = Modifier.fillMaxSize().padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        CircularProgressIndicator(
                            color = colorScheme.primary,
                            trackColor = colorScheme.surfaceContainerHigh
                        )
                        Spacer(modifier = Modifier.height(12.dp))
                        Text(
                            "加载中...",
                            color = colorScheme.primary.copy(alpha = 0.7f),
                            style = MaterialTheme.typography.bodySmall,
                            letterSpacing = 2.sp
                        )
                    }
                }
            } else {
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding)
                ) {
                    // ===== 统计卡片 =====
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp, vertical = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        StatChip(
                            label = "全部",
                            count = stats.first,
                            isSelected = mediaType == "",
                            color = colorScheme.primary,
                            onClick = { mediaType = "" },
                            modifier = Modifier.weight(1f)
                        )
                        StatChip(
                            label = "电影",
                            count = stats.second,
                            isSelected = mediaType == "movie",
                            color = NeonPurple,
                            onClick = { mediaType = if (mediaType == "movie") "" else "movie" },
                            modifier = Modifier.weight(1f)
                        )
                        StatChip(
                            label = "剧集",
                            count = stats.third,
                            isSelected = mediaType == "series",
                            color = MaterialTheme.colorScheme.tertiary,
                            onClick = { mediaType = if (mediaType == "series") "" else "series" },
                            modifier = Modifier.weight(1f)
                        )
                    }

                    // ===== 搜索栏 + 筛选/排序按钮 =====
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp, vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        // 搜索框
                        Box(
                            modifier = Modifier
                                .weight(1f)
                                .height(40.dp)
                                .clip(RoundedCornerShape(12.dp))
                                .background(
                                    if (isDark) colorScheme.surfaceContainerHigh.copy(alpha = 0.8f)
                                    else colorScheme.surfaceContainerLow
                                )
                                .border(
                                    1.dp,
                                    if (isDark) colorScheme.primary.copy(alpha = 0.15f)
                                    else colorScheme.outline.copy(alpha = 0.5f),
                                    RoundedCornerShape(12.dp)
                                )
                                .padding(horizontal = 12.dp),
                            contentAlignment = Alignment.CenterStart
                        ) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Icon(
                                    Icons.Default.Search,
                                    contentDescription = null,
                                    tint = colorScheme.outline,
                                    modifier = Modifier.size(18.dp)
                                )
                                Spacer(modifier = Modifier.width(8.dp))
                                Box(modifier = Modifier.weight(1f)) {
                                    if (searchQuery.isEmpty()) {
                                        Text(
                                            "搜索影视作品...",
                                            style = MaterialTheme.typography.bodySmall,
                                            color = colorScheme.outline
                                        )
                                    }
                                    BasicTextField(
                                        value = searchQuery,
                                        onValueChange = { searchQuery = it },
                                        singleLine = true,
                                        textStyle = TextStyle(
                                            color = colorScheme.onSurface,
                                            fontSize = 14.sp
                                        ),
                                        cursorBrush = SolidColor(colorScheme.primary),
                                        modifier = Modifier.fillMaxWidth()
                                    )
                                }
                                if (searchQuery.isNotEmpty()) {
                                    IconButton(
                                        onClick = { searchQuery = "" },
                                        modifier = Modifier.size(20.dp)
                                    ) {
                                        Icon(
                                            Icons.Default.Close,
                                            contentDescription = "清除",
                                            tint = colorScheme.outline,
                                            modifier = Modifier.size(16.dp)
                                        )
                                    }
                                }
                            }
                        }

                        // 筛选按钮
                        Box {
                            FilterChipButton(
                                label = "筛选",
                                icon = Icons.Default.FilterList,
                                badge = if (activeFilterCount > 0) activeFilterCount else null,
                                isActive = showFilters || activeFilterCount > 0,
                                onClick = { showFilters = !showFilters }
                            )
                        }

                        // 排序按钮
                        Box {
                            FilterChipButton(
                                label = sortValue.label,
                                icon = Icons.AutoMirrored.Filled.Sort,
                                isActive = sortValue != SORT_OPTIONS[0],
                                onClick = { showSortMenu = true }
                            )
                            DropdownMenu(
                                expanded = showSortMenu,
                                onDismissRequest = { showSortMenu = false },
                                containerColor = if (isDark) colorScheme.surfaceContainerHigh
                                else colorScheme.surface,
                                shape = RoundedCornerShape(12.dp)
                            ) {
                                SORT_OPTIONS.forEach { option ->
                                    DropdownMenuItem(
                                        text = {
                                            Text(
                                                option.label,
                                                color = if (sortValue == option) colorScheme.primary
                                                else colorScheme.onSurface,
                                                fontWeight = if (sortValue == option) FontWeight.Bold
                                                else FontWeight.Normal,
                                                fontSize = 13.sp
                                            )
                                        },
                                        onClick = {
                                            sortValue = option
                                            showSortMenu = false
                                        },
                                        leadingIcon = if (sortValue == option) {
                                            {
                                                Icon(
                                                    Icons.Default.Check,
                                                    contentDescription = null,
                                                    tint = colorScheme.primary,
                                                    modifier = Modifier.size(16.dp)
                                                )
                                            }
                                        } else null
                                    )
                                }
                            }
                        }
                    }

                    // ===== 筛选面板（可展开/收起） =====
                    AnimatedVisibility(
                        visible = showFilters,
                        enter = expandVertically() + fadeIn(),
                        exit = shrinkVertically() + fadeOut()
                    ) {
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(horizontal = 12.dp, vertical = 4.dp)
                                .clip(RoundedCornerShape(14.dp))
                                .background(
                                    if (isDark) colorScheme.surfaceContainerHigh.copy(alpha = 0.6f)
                                    else colorScheme.surfaceContainerLow
                                )
                                .border(
                                    1.dp,
                                    if (isDark) colorScheme.primary.copy(alpha = 0.1f)
                                    else colorScheme.outline.copy(alpha = 0.4f),
                                    RoundedCornerShape(14.dp)
                                )
                                .padding(12.dp),
                            verticalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            // 类型标签
                            if (allGenres.isNotEmpty()) {
                                FilterSection(title = "类型标签", icon = Icons.Default.Label) {
                                    FlowChipRow(
                                        items = allGenres,
                                        selectedItems = selectedGenres,
                                        onToggle = { genre ->
                                            selectedGenres = if (genre in selectedGenres)
                                                selectedGenres - genre
                                            else
                                                selectedGenres + genre
                                        }
                                    )
                                }
                            }

                            // 地区
                            if (allCountries.isNotEmpty()) {
                                FilterSection(title = "地区", icon = Icons.Default.Public) {
                                    FlowChipRow(
                                        items = listOf("全部") + allCountries,
                                        selectedItems = if (selectedCountry.isBlank()) setOf("全部")
                                        else setOf(selectedCountry),
                                        onToggle = { country ->
                                            selectedCountry = if (country == "全部" || country == selectedCountry) ""
                                            else country
                                        },
                                        singleSelect = true
                                    )
                                }
                            }

                            // 年份范围
                            FilterSection(title = "年份", icon = Icons.Default.CalendarMonth) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .horizontalScroll(rememberScrollState()),
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    YEAR_RANGES.forEach { yr ->
                                        CyberFilterChip(
                                            label = yr.label,
                                            isSelected = yearRange == yr,
                                            onClick = { yearRange = yr }
                                        )
                                    }
                                }
                            }

                            // 评分
                            FilterSection(title = "最低评分", icon = Icons.Default.Star) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .horizontalScroll(rememberScrollState()),
                                    horizontalArrangement = Arrangement.spacedBy(6.dp)
                                ) {
                                    RATING_OPTIONS.forEach { opt ->
                                        CyberFilterChip(
                                            label = opt.label,
                                            isSelected = minRating == opt,
                                            onClick = { minRating = opt }
                                        )
                                    }
                                }
                            }

                            // 清除筛选
                            if (activeFilterCount > 0) {
                                HorizontalDivider(
                                    color = if (isDark) colorScheme.primary.copy(alpha = 0.1f)
                                    else colorScheme.outline.copy(alpha = 0.3f),
                                    thickness = 1.dp
                                )
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Text(
                                        "已选 $activeFilterCount 个筛选条件",
                                        style = MaterialTheme.typography.labelSmall,
                                        color = colorScheme.outline
                                    )
                                    TextButton(
                                        onClick = {
                                            selectedGenres = emptySet()
                                            selectedCountry = ""
                                            yearRange = YEAR_RANGES[0]
                                            minRating = RATING_OPTIONS[0]
                                        }
                                    ) {
                                        Icon(
                                            Icons.Default.Close,
                                            contentDescription = null,
                                            modifier = Modifier.size(14.dp),
                                            tint = MaterialTheme.colorScheme.error
                                        )
                                        Spacer(modifier = Modifier.width(4.dp))
                                        Text(
                                            "清除所有",
                                            fontSize = 12.sp,
                                            color = MaterialTheme.colorScheme.error
                                        )
                                    }
                                }
                            }
                        }
                    }

                    // 结果提示
                    if (searchQuery.isNotBlank() || activeFilterCount > 0) {
                        Row(
                            modifier = Modifier.padding(horizontal = 14.dp, vertical = 4.dp),
                            verticalAlignment = Alignment.CenterVertically
                        ) {
                            Text(
                                "找到 ",
                                style = MaterialTheme.typography.labelSmall,
                                color = colorScheme.outline
                            )
                            Text(
                                "${filteredItems.size}",
                                style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Bold),
                                color = colorScheme.primary
                            )
                            Text(
                                " 个结果",
                                style = MaterialTheme.typography.labelSmall,
                                color = colorScheme.outline
                            )
                        }
                    }

                    // ===== 媒体网格 =====
                    if (filteredItems.isEmpty() && !uiState.loading) {
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .padding(64.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Icon(
                                    Icons.Default.SearchOff,
                                    contentDescription = null,
                                    modifier = Modifier.size(48.dp),
                                    tint = colorScheme.primary.copy(alpha = 0.4f)
                                )
                                Spacer(modifier = Modifier.height(12.dp))
                                Text(
                                    "没有找到匹配的内容",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = colorScheme.onSurfaceVariant
                                )
                                Text(
                                    "试试调整筛选条件",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = colorScheme.outline
                                )
                            }
                        }
                    } else {
                        LazyVerticalGrid(
                            columns = GridCells.Adaptive(minSize = 130.dp),
                            modifier = Modifier.fillMaxSize(),
                            contentPadding = PaddingValues(12.dp),
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                            verticalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            items(filteredItems) { item ->
                                CyberMixedGridItem(
                                    item = item,
                                    serverUrl = uiState.serverUrl,
                                    token = uiState.token,
                                    onClick = {
                                        if (item.type == "series" && item.series != null) {
                                            onSeriesClick(item.series.id)
                                        } else if (item.media != null) {
                                            onMediaClick(item.media.id)
                                        }
                                    }
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

// ==================== 筛选子组件 ====================

/** 统计芯片 */
@Composable
private fun StatChip(
    label: String,
    count: Int,
    isSelected: Boolean,
    color: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val colorScheme = MaterialTheme.colorScheme
    val isDark = isSystemInDarkTheme()
    Surface(
        onClick = onClick,
        modifier = modifier,
        shape = RoundedCornerShape(12.dp),
        color = if (isSelected) color.copy(alpha = if (isDark) 0.12f else 0.1f)
        else if (isDark) colorScheme.surfaceContainerHigh.copy(alpha = 0.7f)
        else colorScheme.surfaceContainerLow,
        border = androidx.compose.foundation.BorderStroke(
            1.dp,
            if (isSelected) color.copy(alpha = if (isDark) 0.3f else 0.5f)
            else if (isDark) colorScheme.primary.copy(alpha = 0.08f)
            else colorScheme.outline.copy(alpha = 0.3f)
        )
    ) {
        Column(
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 8.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = "$count",
                style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.Bold),
                color = if (isSelected) color else colorScheme.onSurface
            )
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = if (isSelected) color.copy(alpha = 0.8f) else colorScheme.outline
            )
        }
    }
}

/** 筛选/排序按钮 */
@Composable
private fun FilterChipButton(
    label: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    badge: Int? = null,
    isActive: Boolean = false,
    onClick: () -> Unit
) {
    val colorScheme = MaterialTheme.colorScheme
    val isDark = isSystemInDarkTheme()
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(10.dp),
        color = if (isActive) colorScheme.primary.copy(alpha = if (isDark) 0.1f else 0.08f)
        else if (isDark) colorScheme.surfaceContainerHigh.copy(alpha = 0.6f)
        else colorScheme.surfaceContainerLow,
        border = androidx.compose.foundation.BorderStroke(
            1.dp,
            if (isActive) colorScheme.primary.copy(alpha = if (isDark) 0.3f else 0.5f)
            else if (isDark) colorScheme.primary.copy(alpha = 0.08f)
            else colorScheme.outline.copy(alpha = 0.3f)
        )
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            Icon(
                icon,
                contentDescription = null,
                modifier = Modifier.size(14.dp),
                tint = if (isActive) colorScheme.primary else colorScheme.outline
            )
            Text(
                label,
                fontSize = 12.sp,
                color = if (isActive) colorScheme.primary else colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
            if (badge != null && badge > 0) {
                Box(
                    modifier = Modifier
                        .size(16.dp)
                        .background(
                            brush = Brush.linearGradient(
                                colors = listOf(colorScheme.primary, NeonPurple)
                            ),
                            shape = RoundedCornerShape(8.dp)
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        "$badge",
                        fontSize = 9.sp,
                        fontWeight = FontWeight.Bold,
                        color = Color.White
                    )
                }
            }
        }
    }
}

/** 筛选面板小节标题 */
@Composable
private fun FilterSection(
    title: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    content: @Composable () -> Unit
) {
    val colorScheme = MaterialTheme.colorScheme
    Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            Icon(
                icon,
                contentDescription = null,
                modifier = Modifier.size(14.dp),
                tint = colorScheme.outline
            )
            Text(
                title,
                style = MaterialTheme.typography.labelMedium,
                color = colorScheme.onSurfaceVariant,
                fontWeight = FontWeight.Medium
            )
        }
        content()
    }
}

/** 可换行的标签行（使用 FlowRow 效果） */
@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun FlowChipRow(
    items: List<String>,
    selectedItems: Set<String>,
    onToggle: (String) -> Unit,
    singleSelect: Boolean = false
) {
    FlowRow(
        horizontalArrangement = Arrangement.spacedBy(6.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp)
    ) {
        items.forEach { item ->
            CyberFilterChip(
                label = item,
                isSelected = item in selectedItems,
                onClick = { onToggle(item) }
            )
        }
    }
}

/** 赛博朋克风格筛选 Chip */
@Composable
private fun CyberFilterChip(
    label: String,
    isSelected: Boolean,
    onClick: () -> Unit
) {
    val colorScheme = MaterialTheme.colorScheme
    val isDark = isSystemInDarkTheme()
    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(8.dp),
        color = if (isSelected) colorScheme.primary.copy(alpha = if (isDark) 0.15f else 0.1f)
        else if (isDark) Color.Transparent
        else colorScheme.surfaceContainerLowest,
        border = androidx.compose.foundation.BorderStroke(
            1.dp,
            if (isSelected) colorScheme.primary.copy(alpha = if (isDark) 0.35f else 0.6f)
            else if (isDark) colorScheme.primary.copy(alpha = 0.1f)
            else colorScheme.outline.copy(alpha = 0.35f)
        )
    ) {
        Text(
            text = label,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp),
            style = MaterialTheme.typography.labelSmall,
            color = if (isSelected) colorScheme.primary
            else if (isDark) colorScheme.outline
            else colorScheme.onSurfaceVariant,
            fontWeight = if (isSelected) FontWeight.Medium else FontWeight.Normal
        )
    }
}

// ==================== 媒体卡片 ====================

@Composable
private fun CyberMixedGridItem(
    item: MixedItem,
    serverUrl: String,
    token: String,
    onClick: () -> Unit
) {
    val title: String
    val year: Int
    val rating: Double
    val posterUrl: String
    val resolution: String
    val badgeText: String?

    if (item.type == "series" && item.series != null) {
        val series = item.series
        title = series.title
        year = series.year
        rating = series.rating
        posterUrl = "$serverUrl/api/series/${series.id}/poster?token=$token"
        resolution = ""
        badgeText = if (series.episodeCount > 0) "${series.episodeCount} 集" else null
    } else if (item.media != null) {
        val media = item.media
        title = media.title
        year = media.year
        rating = media.rating
        posterUrl = "$serverUrl/api/media/${media.id}/poster?token=$token"
        resolution = media.resolution
        badgeText = null
    } else {
        return
    }

    Box(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(14.dp))
            .cyberCard(cornerRadius = 14.dp)
            .clickable(onClick = onClick)
    ) {
        Column {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .aspectRatio(2f / 3f)
                    .clip(RoundedCornerShape(topStart = 14.dp, topEnd = 14.dp))
            ) {
                AsyncImage(
                    model = posterUrl,
                    contentDescription = title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop
                )

                // 底部渐变遮罩
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(50.dp)
                        .align(Alignment.BottomCenter)
                        .gradientScrim()
                )

                // 评分角标
                if (rating > 0) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.TopEnd)
                            .padding(6.dp),
                        shape = RoundedCornerShape(6.dp),
                        color = MaterialTheme.colorScheme.scrim.copy(alpha = 0.7f)
                    ) {
                        Text(
                            text = String.format("%.1f", rating),
                            style = MaterialTheme.typography.labelSmall,
                            color = AmberGold,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }

                // 分辨率标签
                if (resolution.isNotBlank()) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.BottomStart)
                            .padding(6.dp),
                        shape = RoundedCornerShape(4.dp),
                        color = MaterialTheme.colorScheme.scrim.copy(alpha = 0.7f)
                    ) {
                        Text(
                            text = resolution,
                            style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Medium),
                            color = MaterialTheme.colorScheme.tertiary,
                            modifier = Modifier.padding(horizontal = 5.dp, vertical = 1.dp)
                        )
                    }
                }

                // 集数角标
                if (badgeText != null) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.BottomEnd)
                            .padding(6.dp),
                        shape = RoundedCornerShape(4.dp),
                        color = MaterialTheme.colorScheme.secondary.copy(alpha = 0.85f)
                    ) {
                        Text(
                            text = badgeText,
                            style = MaterialTheme.typography.labelSmall,
                            color = Color.White,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }
            }

            Column(modifier = Modifier.padding(8.dp)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (year > 0) {
                    Text(
                        text = "$year",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.outline
                    )
                }
            }
        }
    }
}

// ==================== ViewModel ====================

data class MediaListUiState(
    val loading: Boolean = true,
    val libraryName: String = "",
    val mixedList: List<MixedItem> = emptyList(),
    val serverUrl: String = "",
    val token: String = "",
    val error: String? = null
)

@HiltViewModel
class MediaListViewModel @Inject constructor(
    private val mediaRepository: MediaRepository,
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(MediaListUiState())
    val uiState = _uiState.asStateFlow()

    fun loadMedia(libraryId: String) {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true)

            val serverUrl = tokenManager.getServerUrl() ?: ""
            val token = tokenManager.getToken() ?: ""
            _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)

            // 加载媒体库名称
            launch {
                mediaRepository.getLibraries().onSuccess { libraries ->
                    val lib = libraries.find { it.id == libraryId }
                    if (lib != null) {
                        _uiState.value = _uiState.value.copy(libraryName = lib.name)
                    }
                }
            }

            // 加载更多数据以支持筛选（提高 limit）
            mediaRepository.getMediaMixed(libraryId = libraryId, limit = 500).onSuccess { response ->
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    mixedList = response.data
                )
            }.onFailure { e ->
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    error = e.message
                )
            }
        }
    }
}
