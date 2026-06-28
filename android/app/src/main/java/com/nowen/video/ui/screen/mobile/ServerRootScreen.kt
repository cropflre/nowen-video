package com.nowen.video.ui.screen.mobile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Settings
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.hilt.navigation.compose.hiltViewModel
import com.nowen.video.ui.component.mobile.EmptyState
import com.nowen.video.ui.component.mobile.MobilePageHeader
import com.nowen.video.ui.component.mobile.PageHeaderAction
import com.nowen.video.ui.component.mobile.ServerCard
import com.nowen.video.ui.theme.MobileSpacing
import com.nowen.video.ui.screen.server.ServerManageViewModel

/**
 * 服务器 Root 页面
 * 显示当前服务器卡片和管理入口
 */
@Composable
fun ServerRootScreen(
    onEnterServer: () -> Unit,
    onMediaClick: (String) -> Unit,
    onSeriesClick: (String) -> Unit,
    onLibraryClick: (String) -> Unit,
    onSearchClick: () -> Unit,
    onPlayerClick: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: ServerManageViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()
    val activeServer = uiState.servers.find { it.id == uiState.activeServerId }

    Column(
        modifier = modifier.fillMaxSize(),
    ) {
        // 页面标题
        MobilePageHeader(
            title = "服务器",
            actions = listOf(
                PageHeaderAction(
                    icon = Icons.Default.Settings,
                    contentDescription = "服务器设置",
                    onClick = { /* TODO: 导航到服务器设置 */ },
                ),
            ),
        )

        // 服务器卡片
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = MobileSpacing.xl),
        ) {
            if (activeServer != null) {
                // 已配置服务器
                ServerCard(
                    serverName = activeServer.name.ifBlank { "Nowen Video" },
                    serverUrl = activeServer.url,
                    isConnected = true, // TODO: 真实健康检查
                    onClick = onEnterServer,
                    onLongClick = {
                        // TODO: 显示服务器操作菜单
                    },
                )
            } else if (uiState.servers.isNotEmpty()) {
                // 有服务器但没有激活的
                ServerCard(
                    serverName = uiState.servers.first().name.ifBlank { "Nowen Video" },
                    serverUrl = uiState.servers.first().url,
                    isConnected = false,
                    onClick = {
                        // TODO: 切换到这个服务器
                    },
                    onLongClick = {
                        // TODO: 显示服务器操作菜单
                    },
                )
            } else {
                // 未配置服务器
                EmptyState(
                    icon = Icons.Default.Dns,
                    title = "还没有服务器",
                    subtitle = "点击右下角 + 添加你的媒体服务器",
                )
            }
        }
    }
}
