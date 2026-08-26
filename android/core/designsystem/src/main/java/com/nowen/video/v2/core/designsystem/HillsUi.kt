package com.nowen.video.v2.core.designsystem

import androidx.compose.foundation.background
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import androidx.compose.ui.layout.ContentScale

object HillsMetrics {
    val gutter = 20.dp
    val sectionGap = 28.dp
    val dockHeight = 64.dp
    val dockBottomGap = 10.dp
    val imageRadius = 10.dp
    val panelRadius = 12.dp
    val controlHeight = 48.dp
}

@Composable
fun HillsScreen(
    modifier: Modifier = Modifier,
    top: @Composable (() -> Unit)? = null,
    content: @Composable (PaddingValues) -> Unit,
) {
    Box(modifier.fillMaxSize().background(NowenColors.NightBackground)) {
        content(PaddingValues(top = if (top == null) 0.dp else 76.dp))
        top?.let {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .statusBarsPadding()
                    .background(NowenColors.NightBackground.copy(alpha = 0.96f)),
            ) { it() }
        }
    }
}

@Composable
fun HillsTopBar(
    title: String,
    subtitle: String? = null,
    navigationIcon: ImageVector? = null,
    onNavigate: (() -> Unit)? = null,
    actions: @Composable RowScope.() -> Unit = {},
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(64.dp)
            .padding(horizontal = HillsMetrics.gutter),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (navigationIcon != null && onNavigate != null) {
            HillsPressable(onClick = onNavigate, modifier = Modifier.size(44.dp)) {
                Icon(navigationIcon, contentDescription = "返回", tint = Color(0xFFF2F4FA), modifier = Modifier.align(Alignment.Center))
            }
            Spacer(Modifier.width(10.dp))
        }
        Column(Modifier.weight(1f)) {
            Text(title, color = Color(0xFFF2F4FA), fontWeight = FontWeight.Bold, style = androidx.compose.material3.MaterialTheme.typography.titleLarge)
            if (!subtitle.isNullOrBlank()) {
                Text(subtitle, color = Color(0xFFB6BCC8), style = androidx.compose.material3.MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        actions()
    }
}

@Composable
fun HillsState(
    title: String,
    message: String,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.padding(horizontal = HillsMetrics.gutter),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(title, color = Color(0xFFF2F4FA), style = androidx.compose.material3.MaterialTheme.typography.titleLarge)
        Text(message, color = Color(0xFFB6BCC8), style = androidx.compose.material3.MaterialTheme.typography.bodyLarge)
        if (actionLabel != null && onAction != null) {
            HillsPrimaryAction(actionLabel, onClick = onAction)
        }
    }
}

@Composable
fun HillsPrimaryAction(label: String, onClick: () -> Unit, modifier: Modifier = Modifier, icon: ImageVector? = null) {
    HillsPressable(onClick = onClick, modifier = modifier.height(HillsMetrics.controlHeight).clip(RoundedCornerShape(18.dp)).background(NowenColors.Brand)) {
        Row(Modifier.align(Alignment.Center), verticalAlignment = Alignment.CenterVertically) {
            if (icon != null) { Icon(icon, null, tint = Color.White); Spacer(Modifier.width(8.dp)) }
            Text(label, color = Color.White, fontWeight = FontWeight.SemiBold)
        }
    }
}

@Composable
fun HillsSecondaryAction(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    icon: ImageVector? = null,
    enabled: Boolean = true,
) {
    HillsPressable(
        onClick = onClick,
        enabled = enabled,
        modifier = modifier
            .height(HillsMetrics.controlHeight)
            .clip(RoundedCornerShape(16.dp))
            .background(NowenColors.NightRaised),
    ) {
        Row(Modifier.align(Alignment.Center), verticalAlignment = Alignment.CenterVertically) {
            if (icon != null) {
                Icon(icon, contentDescription = null, tint = Color(0xFFB6BCC8))
                Spacer(Modifier.width(8.dp))
            }
            Text(
                label,
                color = if (enabled) Color(0xFFD6DBE6) else Color(0xFF737987),
                fontWeight = FontWeight.Medium,
            )
        }
    }
}

@Composable
fun HillsToggle(
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    val trackColor = when {
        !enabled -> MaterialTheme.colorScheme.onSurface.copy(alpha = 0.12f)
        checked -> NowenColors.Brand
        else -> MaterialTheme.colorScheme.outline.copy(alpha = 0.72f)
    }
    HillsPressable(
        onClick = { onCheckedChange(!checked) },
        enabled = enabled,
        modifier = modifier
            .width(48.dp)
            .height(28.dp)
            .clip(RoundedCornerShape(14.dp))
            .background(trackColor),
    ) {
        Box(
            modifier = Modifier
                .align(if (checked) Alignment.CenterEnd else Alignment.CenterStart)
                .padding(horizontal = 3.dp)
                .size(22.dp)
                .clip(RoundedCornerShape(11.dp))
                .background(
                    if (enabled) MaterialTheme.colorScheme.onSurface else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.38f),
                ),
        )
    }
}

@Composable
fun HillsChoiceRail(
    options: List<Pair<String, String>>,
    selected: String,
    onSelect: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.horizontalScroll(rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(20.dp),
    ) {
        options.forEach { (key, label) ->
            val active = key == selected
            HillsPressable(onClick = { onSelect(key) }) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Text(label, color = if (active) Color(0xFF8397CE) else Color(0xFFB6BCC8), fontWeight = if (active) FontWeight.SemiBold else FontWeight.Normal)
                    Spacer(Modifier.height(7.dp))
                    Box(Modifier.width(if (active) 28.dp else 0.dp).height(3.dp).background(if (active) Color(0xFF8397CE) else Color.Transparent))
                }
            }
        }
    }
}

@Composable
fun HillsTextField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier,
    singleLine: Boolean = true,
    visualTransformation: VisualTransformation = VisualTransformation.None,
    keyboardOptions: KeyboardOptions = KeyboardOptions.Default,
) {
    BasicTextField(
        value = value,
        onValueChange = onValueChange,
        singleLine = singleLine,
        visualTransformation = visualTransformation,
        keyboardOptions = keyboardOptions,
        textStyle = androidx.compose.material3.MaterialTheme.typography.bodyLarge.copy(color = Color(0xFFF2F4FA)),
        modifier = modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(HillsMetrics.panelRadius))
            .background(NowenColors.NightRaised)
            .padding(horizontal = 16.dp, vertical = 14.dp),
        decorationBox = { inner ->
            Box {
                if (value.isBlank()) Text(placeholder, color = Color(0xFF98A0B0))
                inner()
            }
        },
    )
}

@Composable
fun HillsPoster(
    title: String,
    subtitle: String?,
    imageUrl: String?,
    progress: Float = 0f,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier.width(112.dp)) {
        HillsPressable(onClick = onClick, modifier = Modifier.fillMaxWidth()) {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .aspectRatio(2f / 3f)
                    .clip(RoundedCornerShape(HillsMetrics.imageRadius))
                    .background(NowenColors.NightRaised),
            ) {
                AsyncImage(
                    model = imageUrl,
                    contentDescription = title,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                )
                if (progress > 0f) {
                    Box(
                        modifier = Modifier
                            .align(Alignment.BottomStart)
                            .fillMaxWidth(progress.coerceIn(0f, 1f))
                            .height(3.dp)
                            .background(Color(0xFF8397CE)),
                    )
                }
            }
        }
        Spacer(Modifier.height(7.dp))
        Text(title, color = Color(0xFFF2F4FA), maxLines = 1, overflow = TextOverflow.Ellipsis, style = androidx.compose.material3.MaterialTheme.typography.titleMedium)
        if (!subtitle.isNullOrBlank()) Text(subtitle, color = Color(0xFF98A0B0), maxLines = 1, overflow = TextOverflow.Ellipsis, style = androidx.compose.material3.MaterialTheme.typography.bodyMedium)
    }
}

@Composable
fun HillsBottomDock(content: @Composable RowScope.() -> Unit) {
    Box(Modifier.fillMaxWidth().navigationBarsPadding().padding(horizontal = HillsMetrics.gutter, vertical = HillsMetrics.dockBottomGap)) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .height(HillsMetrics.dockHeight)
                .clip(RoundedCornerShape(32.dp))
                .background(NowenColors.NightSurface)
                .padding(7.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceEvenly,
            content = content,
        )
    }
}
