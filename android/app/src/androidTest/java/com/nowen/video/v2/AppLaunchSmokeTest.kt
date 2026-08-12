package com.nowen.video.v2

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class AppLaunchSmokeTest {
    @get:Rule
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Test
    fun freshInstallShowsServerSetupWithoutMigrationNotice() {
        composeRule.waitUntil(timeoutMillis = 20_000) {
            composeRule
                .onAllNodesWithText("连接你的媒体空间")
                .fetchSemanticsNodes()
                .isNotEmpty()
        }

        composeRule.onNodeWithText("连接你的媒体空间").assertIsDisplayed()
        composeRule.onNodeWithText("扫描二维码").assertIsDisplayed()
        composeRule.onNodeWithText("手动添加").assertIsDisplayed()

        check(
            composeRule
                .onAllNodesWithText("已迁移旧版服务器")
                .fetchSemanticsNodes()
                .isEmpty(),
        ) { "fresh install must not show a V1 migration notice" }
    }
}
