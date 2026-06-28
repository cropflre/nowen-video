package com.nowen.video.ui.component.mobile

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalSoftwareKeyboardController
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 大圆角搜索框
 * Hills Pro 风格：大圆角 + 半透明背景 + 键盘搜索按钮
 */
@Composable
fun SearchBarLarge(
    query: String,
    onQueryChange: (String) -> Unit,
    onSearch: (String) -> Unit,
    modifier: Modifier = Modifier,
    placeholder: String = "输入搜索内容",
) {
    val keyboardController = LocalSoftwareKeyboardController.current

    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(56.dp)
            .clip(RoundedCornerShape(MobileRadius.full))
            .background(MobileColors.Card)
            .border(
                width = 1.dp,
                color = MobileColors.CardBorder,
                shape = RoundedCornerShape(MobileRadius.full),
            )
            .padding(horizontal = MobileSpacing.lg),
        contentAlignment = Alignment.CenterStart,
    ) {
        // 搜索图标
        Icon(
            imageVector = Icons.Default.Search,
            contentDescription = "搜索",
            tint = MobileColors.Muted,
            modifier = Modifier.size(22.dp),
        )

        // 输入框
        BasicTextField(
            value = query,
            onValueChange = onQueryChange,
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 36.dp),
            textStyle = TextStyle(
                color = MobileColors.Text,
                fontSize = MobileFontSize.md,
            ),
            cursorBrush = SolidColor(MobileColors.Primary),
            singleLine = true,
            keyboardOptions = KeyboardOptions(
                imeAction = ImeAction.Search,
            ),
            keyboardActions = KeyboardActions(
                onSearch = {
                    onSearch(query)
                    keyboardController?.hide()
                },
            ),
            decorationBox = { innerTextField ->
                if (query.isBlank()) {
                    Text(
                        text = placeholder,
                        color = MobileColors.Muted,
                        fontSize = MobileFontSize.md,
                    )
                }
                innerTextField()
            },
        )
    }
}
