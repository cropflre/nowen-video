package com.nowen.video.v2.feature.main

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.DanmakuRepository
import com.nowen.video.v2.core.model.DanmakuAnime
import com.nowen.video.v2.core.model.DanmakuCue
import com.nowen.video.v2.core.model.DanmakuEpisode
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.launch

internal data class DanmakuSearchState(
    val keyword: String = "",
    val selectedAnime: DanmakuAnime? = null,
    val animes: List<DanmakuAnime> = emptyList(),
    val episodes: List<DanmakuEpisode> = emptyList(),
    val loading: Boolean = false,
    val error: String? = null,
)

@HiltViewModel
internal class DanmakuSearchViewModel @Inject constructor(
    private val repository: DanmakuRepository,
) : ViewModel() {
    var state by mutableStateOf(DanmakuSearchState())
        private set

    fun setKeyword(value: String) {
        state = state.copy(keyword = value, error = null)
    }

    fun search() {
        val keyword = state.keyword.trim()
        if (keyword.isBlank()) return
        viewModelScope.launch {
            state = state.copy(loading = true, error = null, selectedAnime = null, episodes = emptyList())
            repository.searchAnime(keyword)
                .onSuccess { state = state.copy(loading = false, animes = it) }
                .onFailure { state = state.copy(loading = false, error = it.message?.take(160) ?: "搜索失败") }
        }
    }

    fun selectAnime(anime: DanmakuAnime) {
        viewModelScope.launch {
            state = state.copy(selectedAnime = anime, loading = true, error = null, episodes = emptyList())
            repository.searchEpisodes(anime.animeTitle)
                .onSuccess { response ->
                    state = state.copy(
                        loading = false,
                        episodes = response.animes.flatMap { it.episodes },
                    )
                }
                .onFailure { state = state.copy(loading = false, error = it.message?.take(160) ?: "剧集搜索失败") }
        }
    }

    suspend fun loadEpisode(episode: DanmakuEpisode): Result<List<DanmakuCue>> = when {
        episode.episodeId > 0L -> repository.commentsForId(episode.episodeId)
        episode.url.isNotBlank() -> repository.commentsForUrl(episode.url)
        else -> Result.failure(IllegalArgumentException("该剧集没有可用弹幕地址"))
    }
}

@Composable
internal fun DanmakuSearchDialog(
    initialKeyword: String,
    onDismiss: () -> Unit,
    onLoaded: (List<DanmakuCue>) -> Unit,
    viewModel: DanmakuSearchViewModel = hiltViewModel(),
) {
    val state = viewModel.state
    var loadingEpisodeId by remember { mutableStateOf<Long?>(null) }
    var loadError by remember { mutableStateOf<String?>(null) }
    val scope = rememberCoroutineScope()
    LaunchedEffect(initialKeyword) {
        if (viewModel.state.keyword.isBlank()) viewModel.setKeyword(initialKeyword)
    }
    Dialog(onDismissRequest = onDismiss) {
        Surface(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(max = 620.dp),
            shape = MaterialTheme.shapes.extraLarge,
            color = MaterialTheme.colorScheme.surface,
        ) {
            Column(Modifier.padding(20.dp)) {
                Text("手动搜索弹幕", style = MaterialTheme.typography.titleLarge)
                Row(
                    modifier = Modifier.fillMaxWidth().padding(top = 14.dp),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    OutlinedTextField(
                        value = if (state.keyword.isBlank()) initialKeyword else state.keyword,
                        onValueChange = viewModel::setKeyword,
                        modifier = Modifier.weight(1f),
                        singleLine = true,
                        label = { Text("作品名称") },
                    )
                    TextButton(onClick = viewModel::search) { Text("搜索") }
                }
                state.selectedAnime?.let { anime ->
                    Text(
                        "剧集 · ${anime.animeTitle}",
                        style = MaterialTheme.typography.titleMedium,
                        modifier = Modifier.padding(top = 16.dp),
                    )
                }
                if (state.loading) {
                    CircularProgressIndicator(modifier = Modifier.padding(24.dp))
                }
                (state.error ?: loadError)?.let {
                    Text(
                        it,
                        color = MaterialTheme.colorScheme.error,
                        style = MaterialTheme.typography.bodySmall,
                        modifier = Modifier.padding(top = 12.dp),
                    )
                }
                LazyColumn(modifier = Modifier.padding(top = 8.dp)) {
                    if (state.selectedAnime == null) {
                        items(state.animes, key = { "anime-${it.animeId}-${it.animeTitle}" }) { anime ->
                            SearchResultRow(
                                title = anime.animeTitle.ifBlank { "未命名作品" },
                                subtitle = anime.typeDescription.ifBlank { "选择以查看剧集" },
                                onClick = { viewModel.selectAnime(anime) },
                            )
                        }
                    } else {
                        items(state.episodes, key = { "episode-${it.episodeId}-${it.episodeTitle}" }) { episode ->
                            SearchResultRow(
                                title = episode.episodeTitle.ifBlank { "剧集 ${episode.episodeId}" },
                                subtitle = if (loadingEpisodeId == episode.episodeId) "正在加载…" else "加载此剧集弹幕",
                                onClick = {
                                    if (loadingEpisodeId != null) return@SearchResultRow
                                    loadError = null
                                    loadingEpisodeId = episode.episodeId
                                    scope.launch {
                                        viewModel.loadEpisode(episode)
                                            .onSuccess { onLoaded(it); onDismiss() }
                                            .onFailure { loadError = it.message?.take(160) ?: "弹幕加载失败" }
                                        loadingEpisodeId = null
                                    }
                                },
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SearchResultRow(title: String, subtitle: String, onClick: () -> Unit) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(vertical = 12.dp),
    ) {
        Text(title, style = MaterialTheme.typography.bodyLarge)
        Text(subtitle, color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodySmall)
    }
}
