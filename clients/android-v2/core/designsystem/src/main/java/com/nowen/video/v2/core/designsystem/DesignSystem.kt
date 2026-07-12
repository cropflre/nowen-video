package com.nowen.video.v2.core.designsystem

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil.compose.AsyncImage

object NowenColors {
    val DeepSpace = Color(0xFF090C12)
    val DeepSurface = Color(0xFF111621)
    val DeepRaised = Color(0xFF171D2A)
    val Lavender = Color(0xFF6578FF)
    val Cyan = Color(0xFF2ED3D0)
    val LightBackground = Color(0xFFF6F7FB)
    val Ink = Color(0xFF171A24)
    val Muted = Color(0xFF9BA3B4)
}

private val DarkScheme = darkColorScheme(
    primary = NowenColors.Lavender,
    onPrimary = Color.White,
    secondary = NowenColors.Cyan,
    background = NowenColors.DeepSpace,
    onBackground = Color(0xFFF4F6FA),
    surface = NowenColors.DeepSurface,
    onSurface = Color(0xFFF4F6FA),
    surfaceVariant = NowenColors.DeepRaised,
    onSurfaceVariant = NowenColors.Muted,
    outline = Color(0xFF303747),
)

private val LightScheme = lightColorScheme(
    primary = Color(0xFF5063E7),
    onPrimary = Color.White,
    secondary = Color(0xFF087F7B),
    background = NowenColors.LightBackground,
    onBackground = NowenColors.Ink,
    surface = Color.White,
    onSurface = NowenColors.Ink,
    surfaceVariant = Color(0xFFEEF0F8),
    onSurfaceVariant = Color(0xFF646B7A),
    outline = Color(0xFFD8DCE8),
)

private val Shapes = Shapes(
    extraSmall = RoundedCornerShape(10.dp),
    small = RoundedCornerShape(14.dp),
    medium = RoundedCornerShape(18.dp),
    large = RoundedCornerShape(24.dp),
    extraLarge = RoundedCornerShape(28.dp),
)

private val Type = Typography(
    headlineLarge = TextStyle(fontSize = 32.sp, lineHeight = 38.sp, fontWeight = FontWeight.Bold),
    headlineMedium = TextStyle(fontSize = 26.sp, lineHeight = 32.sp, fontWeight = FontWeight.Bold),
    titleLarge = TextStyle(fontSize = 21.sp, lineHeight = 27.sp, fontWeight = FontWeight.SemiBold),
    titleMedium = TextStyle(fontSize = 16.sp, lineHeight = 22.sp, fontWeight = FontWeight.SemiBold),
    bodyLarge = TextStyle(fontSize = 16.sp, lineHeight = 24.sp),
    bodyMedium = TextStyle(fontSize = 14.sp, lineHeight = 21.sp),
    labelLarge = TextStyle(fontSize = 14.sp, lineHeight = 18.sp, fontWeight = FontWeight.SemiBold),
)

@Composable
fun NowenTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit,
) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkScheme else LightScheme,
        typography = Type,
        shapes = Shapes,
        content = content,
    )
}

@Composable
fun NowenPage(
    modifier: Modifier = Modifier,
    contentPadding: PaddingValues = PaddingValues(horizontal = 20.dp),
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background)
            .windowInsetsPadding(WindowInsets.safeDrawing)
            .padding(contentPadding),
        content = content,
    )
}

@Composable
fun BrandMark(modifier: Modifier = Modifier, compact: Boolean = false) {
    Row(modifier = modifier, verticalAlignment = Alignment.CenterVertically) {
        Box(
            modifier = Modifier
                .size(if (compact) 36.dp else 48.dp)
                .clip(RoundedCornerShape(if (compact) 12.dp else 16.dp))
                .background(Brush.linearGradient(listOf(NowenColors.Cyan, NowenColors.Lavender))),
            contentAlignment = Alignment.Center,
        ) {
            Text("N", color = Color.White, style = MaterialTheme.typography.titleLarge)
        }
        Spacer(Modifier.width(12.dp))
        Column {
            Text("NOWEN", style = MaterialTheme.typography.titleLarge)
            if (!compact) {
                Text(
                    "你的私人媒体空间",
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

@Composable
fun ElevatedPanel(
    modifier: Modifier = Modifier,
    content: @Composable ColumnScope.() -> Unit,
) {
    Surface(
        modifier = modifier,
        shape = MaterialTheme.shapes.large,
        tonalElevation = 2.dp,
        shadowElevation = 1.dp,
        color = MaterialTheme.colorScheme.surface,
    ) {
        Column(Modifier.padding(20.dp), content = content)
    }
}

@Composable
fun SectionTitle(title: String, subtitle: String? = null, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxWidth()) {
        Text(title, style = MaterialTheme.typography.titleLarge)
        if (!subtitle.isNullOrBlank()) {
            Spacer(Modifier.height(2.dp))
            Text(subtitle, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
fun MediaPosterCard(
    title: String,
    subtitle: String?,
    imageUrl: String?,
    progress: Float,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier = modifier.width(142.dp).clickable(onClick = onClick)) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(MaterialTheme.shapes.medium)
                .background(MaterialTheme.colorScheme.surfaceVariant),
        ) {
            AsyncImage(model = imageUrl, contentDescription = title, modifier = Modifier.fillMaxSize())
            if (progress > 0f) {
                LinearProgressIndicator(
                    progress = { progress },
                    modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(4.dp),
                    color = MaterialTheme.colorScheme.secondary,
                    trackColor = Color.Black.copy(alpha = 0.25f),
                )
            }
        }
        Spacer(Modifier.height(9.dp))
        Text(title, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.titleMedium)
        if (!subtitle.isNullOrBlank()) {
            Text(
                subtitle,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
fun MessagePanel(
    title: String,
    message: String,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
    modifier: Modifier = Modifier,
) {
    ElevatedPanel(modifier.fillMaxWidth()) {
        Text(title, style = MaterialTheme.typography.titleLarge)
        Spacer(Modifier.height(8.dp))
        Text(message, color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (actionLabel != null && onAction != null) {
            Spacer(Modifier.height(16.dp))
            FilledTonalButton(onClick = onAction) { Text(actionLabel) }
        }
    }
}
