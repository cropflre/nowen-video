package com.nowen.video.ui.screen.mobile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.nowen.video.ui.component.mobile.EmptyState
import com.nowen.video.ui.component.mobile.MobilePageHeader
import com.nowen.video.ui.component.mobile.MobileServerEntryCard
import com.nowen.video.ui.component.mobile.PageHeaderAction
import com.nowen.video.ui.component.mobile.inferServerIconType
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileSpacing
import com.nowen.video.ui.screen.server.ServerManageViewModel

/**
 * 服务器 Root 页面
 * 显示服务器列表网格和管理入口
 */
@Composable
fun ServerRootScreen(
    onEnterServer: () -> Unit,
    onAddServerClick: () -> Unit,
    onServerManageClick: () -> Unit,
    onMediaClick: (String) -> Unit,
    onSeriesClick: (String) -> Unit,
    onLibraryClick: (String) -> Unit,
    onSearchClick: () -> Unit,
    onPlayerClick: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: ServerManageViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()

    // 进入页面时加载服务器列表
    LaunchedEffect(Unit) {
        viewModel.loadServers()
    }

    Box(
        modifier = modifier.fillMaxSize(),
    ) {
        // 主内容区域：标题 + 服务器列表
        Column(
            modifier = Modifier.fillMaxSize(),
        ) {
            // 页面标题
            MobilePageHeader(
                title = "服务器",
                actions = listOf(
                    PageHeaderAction(
                        icon = Icons.Default.MoreVert,
                        contentDescription = "服务器管理",
                        onClick = onServerManageClick,
                    ),
                ),
            )

            // 服务器列表
            if (uiState.servers.isNotEmpty()) {
                LazyVerticalGrid(
                    columns = GridCells.Adaptive(minSize = 180.dp),
                    modifier = Modifier.fillMaxWidth(),
                    contentPadding = PaddingValues(
                        start = MobileSpacing.xl,
                        end = MobileSpacing.xl,
                        top = 20.dp,
                        bottom = 120.dp, // 为底部导航和 FAB 留空间
                    ),
                    horizontalArrangement = Arrangement.spacedBy(16.dp),
                    verticalArrangement = Arrangement.spacedBy(16.dp),
                ) {
                    items(uiState.servers) { server ->
                        val isActive = server.id == uiState.activeServerId
                        MobileServerEntryCard(
                            name = server.name.ifBlank { "默认服务器" },
                            subtitle = if (isActive) "当前服务器" else "点击切换",
                            iconType = inferServerIconType(server.name),
                            isActive = isActive,
                            onClick = {
                                if (!isActive) {
                                    viewModel.switchServer(server.id) {
                                        onEnterServer()
                                    }
                                } else {
                                    onEnterServer()
                                }
                            },
                            onLongClick = {
                                onServerManageClick()
                            },
                        )
                    }
                }
            } else {
                // 空状态
                EmptyState(
                    icon = Icons.Default.Dns,
                    title = "还没有服务器",
                    subtitle = "点击右下角 + 添加你的媒体服务器",
                    modifier = Modifier
                        .fillMaxWidth()
                        .weight(1f),
                )
            }
        }

        // 右下角 FAB - 添加服务器
        FloatingActionButton(
            onClick = onAddServerClick,
            modifier = Modifier
                .align(Alignment.BottomEnd)
                .navigationBarsPadding()
                .padding(
                    end = MobileSpacing.xl,
                    bottom = 112.dp, // 为底部导航留空间
                )
                .size(56.dp),
            containerColor = MobileColors.PrimarySoft,
            contentColor = MobileColors.Primary,
        ) {
            Icon(
                imageVector = Icons.Default.Add,
                contentDescription = "添加服务器",
                modifier = Modifier.size(24.dp),
            )
        }
    }
}
