package com.nowen.video.v2

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class AppLaunchSmokeTest {
    @get:Rule
    val composeRule = createAndroidComposeRule<MainActivity>()

    @Test
    fun freshInstallExplainsMigrationAndShowsServerSetup() {
        composeRule.waitUntil(timeoutMillis = 20_000) {
            composeRule
                .onAllNodesWithText("从旧版迁移到 Android V2")
                .fetchSemanticsNodes()
                .isNotEmpty()
        }

        composeRule.onNodeWithText("从旧版迁移到 Android V2").assertIsDisplayed()
        composeRule.onNodeWithText("我知道了").assertIsDisplayed().performClick()

        composeRule.onNodeWithText("连接你的媒体空间").assertIsDisplayed()
        composeRule.onNodeWithText("扫描二维码").assertIsDisplayed()
        composeRule.onNodeWithText("手动添加").assertIsDisplayed()
    }
}
