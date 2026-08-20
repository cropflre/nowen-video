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
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil.compose.AsyncImage

/**
 * Android 与 Web 移动端共用的视觉基准。
 *
 * 色彩、圆角、排版和内容密度直接映射 Web UI 2.0 的移动端规范，避免 Compose
 * 页面继续落回 Material 默认蓝色、大字号和过度圆角的视觉语言。
 */
object NowenColors {
    val DeepSpace = Color(0xFF070A12)
    val DeepSurface = Color(0xFF101522)
    val DeepRaised = Color(0xFF171D2B)
    val Lavender = Color(0xFF7057FF)
    val LavenderPressed = Color(0xFF6047F2)
    val Cyan = Color(0xFF5CCED2)
    val LightBackground = Color(0xFFF7F7FB)
    val LightSurface = Color(0xFFFFFFFF)
    val LightRaised = Color(0xFFF2F2F8)
    val Ink = Color(0xFF171923)
    val Muted = Color(0xFF737887)
    val LightOutline = Color(0xFFE7E8EF)
}

/** Web 移动端与 Android 共用的几何密度。 */
object NowenMobileMetrics {
    val PageHorizontal = 10.dp
    val SectionPadding = 14.dp
    val SectionRadius = 18.dp
    val CardRadius = 10.dp
    val CompactCardRadius = 9.dp
    val RailGap = 10.dp
    val SectionGap = 18.dp
    val ControlRadius = 11.dp
}

private val DarkScheme = darkColorScheme(
    primary = NowenColors.Lavender,
    onPrimary = Color.White,
    primaryContainer = Color(0xFF282044),
    onPrimaryContainer = Color(0xFFEDE9FF),
    secondary = NowenColors.Cyan,
    background = NowenColors.DeepSpace,
    onBackground = Color(0xFFF5F6FA),
    surface = NowenColors.DeepSurface,
    onSurface = Color(0xFFF5F6FA),
    surfaceVariant = NowenColors.DeepRaised,
    onSurfaceVariant = Color(0xFFA9AFBE),
    outline = Color(0xFF303746),
    outlineVariant = Color(0xFF222937),
)

private val LightScheme = lightColorScheme(
    primary = NowenColors.Lavender,
    onPrimary = Color.White,
    primaryContainer = Color(0xFFF0EDFF),
    onPrimaryContainer = Color(0xFF4C38C9),
    secondary = Color(0xFF4E6B79),
    background = NowenColors.LightBackground,
    onBackground = NowenColors.Ink,
    surface = NowenColors.LightSurface,
    onSurface = NowenColors.Ink,
    surfaceVariant = NowenColors.LightRaised,
    onSurfaceVariant = NowenColors.Muted,
    outline = NowenColors.LightOutline,
    outlineVariant = Color(0xFFF0F0F5),
)

private val Shapes = Shapes(
    extraSmall = RoundedCornerShape(6.dp),
    small = RoundedCornerShape(9.dp),
    medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(18.dp),
    extraLarge = RoundedCornerShape(22.dp),
)

private val Type = Typography(
    headlineLarge = TextStyle(fontSize = 28.sp, lineHeight = 33.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.45).sp),
    headlineMedium = TextStyle(fontSize = 22.sp, lineHeight = 28.sp, fontWeight = FontWeight.Bold, letterSpacing = (-0.25).sp),
    titleLarge = TextStyle(fontSize = 18.sp, lineHeight = 23.sp, fontWeight = FontWeight.SemiBold),
    titleMedium = TextStyle(fontSize = 15.sp, lineHeight = 20.sp, fontWeight = FontWeight.SemiBold),
    bodyLarge = TextStyle(fontSize = 15.sp, lineHeight = 23.sp),
    bodyMedium = TextStyle(fontSize = 13.sp, lineHeight = 20.sp),
    labelLarge = TextStyle(fontSize = 13.sp, lineHeight = 17.sp, fontWeight = FontWeight.SemiBold),
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
    contentPadding: PaddingValues = PaddingValues(horizontal = NowenMobileMetrics.PageHorizontal),
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
                .size(if (compact) 36.dp else 46.dp)
                .clip(RoundedCornerShape(if (compact) 11.dp else 14.dp))
                .background(Brush.linearGradient(listOf(Color(0xFF8F7AFF), NowenColors.Lavender))),
            contentAlignment = Alignment.Center,
        ) {
            Text("N", color = Color.White, style = MaterialTheme.typography.titleLarge)
        }
        Spacer(Modifier.width(11.dp))
        Column {
            Text("NOWEN VIDEO", style = MaterialTheme.typography.titleMedium)
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
        shape = RoundedCornerShape(NowenMobileMetrics.SectionRadius),
        tonalElevation = 0.dp,
        shadowElevation = 0.dp,
        color = MaterialTheme.colorScheme.surface,
        border = androidx.compose.foundation.BorderStroke(
            1.dp,
            MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.78f),
        ),
    ) {
        Column(Modifier.padding(NowenMobileMetrics.SectionPadding), content = content)
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
    Column(modifier = modifier.width(112.dp).clickable(onClick = onClick)) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(NowenMobileMetrics.CompactCardRadius))
                .background(MaterialTheme.colorScheme.surfaceVariant),
        ) {
            AsyncImage(
                model = imageUrl,
                contentDescription = title,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize(),
            )
            if (progress > 0f) {
                LinearProgressIndicator(
                    progress = { progress },
                    modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(3.dp),
                    color = MaterialTheme.colorScheme.primary,
                    trackColor = Color.Black.copy(alpha = 0.18f),
                )
            }
        }
        Spacer(Modifier.height(6.dp))
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
            Spacer(Modifier.height(14.dp))
            FilledTonalButton(onClick = onAction, shape = RoundedCornerShape(NowenMobileMetrics.ControlRadius)) {
                Text(actionLabel)
            }
        }
    }
}
