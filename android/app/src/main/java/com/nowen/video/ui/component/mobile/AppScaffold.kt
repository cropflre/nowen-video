package com.nowen.video.ui.component.mobile

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 移动端应用脚手架
 * 负责：背景、安全区、底部导航预留空间
 */
@Composable
fun AppScaffold(
    modifier: Modifier = Modifier,
    showBottomBar: Boolean = true,
    bottomBar: @Composable () -> Unit = {},
    content: @Composable (PaddingValues) -> Unit,
) {
    Box(
        modifier = modifier
            .fillMaxSize()
            .background(MobileColors.Bg),
    ) {
        content(
            PaddingValues(
                bottom = if (showBottomBar) 96.dp + MobileSpacing.xl else 0.dp,
            ),
        )

        if (showBottomBar) {
            Box(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .navigationBarsPadding()
                    .padding(bottom = MobileSpacing.lg),
            ) {
                bottomBar()
            }
        }
    }
}
