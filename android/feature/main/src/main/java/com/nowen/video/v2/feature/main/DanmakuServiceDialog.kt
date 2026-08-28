package com.nowen.video.v2.feature.main

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.nowen.video.v2.core.designsystem.HillsSecondaryAction
import com.nowen.video.v2.core.designsystem.HillsTextField

@Composable
internal fun DanmakuServiceDialog(
    apiBaseUrl: String,
    apiToken: String,
    testState: String?,
    onApiBaseUrlChange: (String) -> Unit,
    onApiTokenChange: (String) -> Unit,
    onTestConnection: () -> Unit,
    onDismiss: () -> Unit,
) {
    var url by remember(apiBaseUrl) { mutableStateOf(apiBaseUrl) }
    var token by remember(apiToken) { mutableStateOf(apiToken) }
    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(usePlatformDefaultWidth = false),
    ) {
        androidx.compose.material3.Surface(
            modifier = Modifier
                .fillMaxWidth(0.88f)
                .heightIn(max = 520.dp),
            shape = MaterialTheme.shapes.extraLarge,
            color = MaterialTheme.colorScheme.surface,
            tonalElevation = 8.dp,
            shadowElevation = 16.dp,
        ) {
            Column(Modifier.padding(horizontal = 20.dp, vertical = 20.dp)) {
                Text("弹幕服务", style = MaterialTheme.typography.headlineSmall)
                Text(
                    "外部服务",
                    color = MaterialTheme.colorScheme.primary,
                    style = MaterialTheme.typography.titleMedium,
                    modifier = Modifier.padding(top = 18.dp, bottom = 8.dp),
                )
                HillsTextField(
                    value = url,
                    onValueChange = {
                        url = it
                        onApiBaseUrlChange(it)
                    },
                    placeholder = "弹幕 API 地址",
                )
                HillsTextField(
                    value = token,
                    onValueChange = {
                        token = it
                        onApiTokenChange(it)
                    },
                    placeholder = "API Token（可选）",
                    visualTransformation = PasswordVisualTransformation(),
                    modifier = Modifier.padding(top = 10.dp),
                )
                HillsSecondaryAction(
                    label = "测试连接",
                    onClick = onTestConnection,
                    enabled = url.isNotBlank(),
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 12.dp),
                )
                testState?.let {
                    Text(
                        it,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        style = MaterialTheme.typography.bodySmall,
                        modifier = Modifier.padding(top = 8.dp),
                    )
                }
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(top = 16.dp),
                    horizontalArrangement = Arrangement.End,
                ) {
                    HillsSecondaryAction(label = "取消", onClick = onDismiss)
                }
            }
        }
    }
}
