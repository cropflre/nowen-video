package com.nowen.video.v2.core.designsystem

import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil.compose.AsyncImage

/** Semantic colors for the independently branded Android client. */
object NowenColors {
    val NightBackground = Color(0xFF111318)
    val NightSurface = Color(0xFF1A1D24)
    val NightRaised = Color(0xFF252932)
    val NightOutline = Color(0xFF343944)
    val Brand = Color(0xFF5D6F9A)
    val BrandPressed = Color(0xFF4C5F87)
    val BrandContainer = Color(0xFF29334A)
    val Gold = Color(0xFFE8AE4D)
    val LightBackground = Color(0xFFF5F6F8)
    val LightSurface = Color(0xFFFFFFFF)
    val LightRaised = Color(0xFFE9ECF1)
    val Ink = Color(0xFF20242C)
    val Muted = Color(0xFF69707D)
    val LightOutline = Color(0xFFD9DEE6)
}

/** Shared mobile density. Keep media pages compact and touch targets stable. */
object NowenMobileMetrics {
    val PageHorizontal = 20.dp
    val SectionPadding = 16.dp
    val SectionRadius = 16.dp
    val CardRadius = 10.dp
    val CompactCardRadius = 10.dp
    val RailGap = 12.dp
    val SectionGap = 24.dp
    val ControlRadius = 12.dp
    val BottomBarHeight = 64.dp
    val TouchTarget = 48.dp
    val FloatingActionSize = 56.dp
    val ModalRadius = 24.dp
}

object NowenMotion {
    const val PressInMs = 80
    const val PressOutMs = 120
    const val ContentEnterMs = 180
    const val ContentExitMs = 140
    const val HeroEnterMs = 220
    const val HeroExitMs = 180
}

private val DarkScheme = darkColorScheme(
    primary = NowenColors.Brand,
    onPrimary = Color.White,
    primaryContainer = NowenColors.BrandContainer,
    onPrimaryContainer = Color(0xFFDCE5FF),
    secondary = NowenColors.Gold,
    onSecondary = Color(0xFF302000),
    background = NowenColors.NightBackground,
    onBackground = Color(0xFFF3F5F9),
    surface = NowenColors.NightSurface,
    onSurface = Color(0xFFF3F5F9),
    surfaceVariant = NowenColors.NightRaised,
    onSurfaceVariant = Color(0xFFB6BCC8),
    outline = NowenColors.NightOutline,
    outlineVariant = Color(0xFF2A2E37),
)

private val LightScheme = lightColorScheme(
    primary = NowenColors.Brand,
    onPrimary = Color.White,
    primaryContainer = Color(0xFFDCE5FF),
    onPrimaryContainer = Color(0xFF263553),
    secondary = Color(0xFF8B5A00),
    onSecondary = Color.White,
    background = NowenColors.LightBackground,
    onBackground = NowenColors.Ink,
    surface = NowenColors.LightSurface,
    onSurface = NowenColors.Ink,
    surfaceVariant = NowenColors.LightRaised,
    onSurfaceVariant = NowenColors.Muted,
    outline = NowenColors.LightOutline,
    outlineVariant = Color(0xFFE5E8ED),
)

private val Shapes = Shapes(
    extraSmall = RoundedCornerShape(4.dp),
    small = RoundedCornerShape(6.dp),
    medium = RoundedCornerShape(8.dp),
    large = RoundedCornerShape(10.dp),
    extraLarge = RoundedCornerShape(12.dp),
)

private val Type = Typography(
    headlineLarge = TextStyle(fontSize = 28.sp, lineHeight = 34.sp, fontWeight = FontWeight.Bold),
    headlineMedium = TextStyle(fontSize = 22.sp, lineHeight = 28.sp, fontWeight = FontWeight.Bold),
    titleLarge = TextStyle(fontSize = 18.sp, lineHeight = 24.sp, fontWeight = FontWeight.SemiBold),
    titleMedium = TextStyle(fontSize = 15.sp, lineHeight = 21.sp, fontWeight = FontWeight.SemiBold),
    bodyLarge = TextStyle(fontSize = 15.sp, lineHeight = 23.sp),
    bodyMedium = TextStyle(fontSize = 13.sp, lineHeight = 19.sp),
    labelLarge = TextStyle(fontSize = 13.sp, lineHeight = 18.sp, fontWeight = FontWeight.SemiBold),
)

@Composable
fun NowenTheme(
    darkTheme: Boolean = true,
    content: @Composable () -> Unit,
) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkScheme else LightScheme,
        typography = Type,
        shapes = Shapes,
    ) {
        Surface(
            modifier = Modifier.fillMaxSize(),
            color = MaterialTheme.colorScheme.background,
            contentColor = MaterialTheme.colorScheme.onBackground,
            content = content,
        )
    }
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
                .size(if (compact) 36.dp else 44.dp)
                .clip(RoundedCornerShape(if (compact) 8.dp else 10.dp))
                .background(MaterialTheme.colorScheme.primary),
            contentAlignment = Alignment.Center,
        ) {
            Text(
                text = ">",
                color = MaterialTheme.colorScheme.onPrimary,
                style = MaterialTheme.typography.headlineMedium,
            )
        }
        Spacer(Modifier.width(10.dp))
        Column {
            Text(ProductIdentity.wordmark, style = MaterialTheme.typography.titleMedium)
            if (!compact) {
                Text(
                    ProductIdentity.tagline,
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
            MaterialTheme.colorScheme.outlineVariant,
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
                    trackColor = Color.Black.copy(alpha = 0.24f),
                )
            }
        }
        Spacer(Modifier.height(7.dp))
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
fun HillsPressable(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    content: @Composable BoxScope.() -> Unit,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val pressed by interactionSource.collectIsPressedAsState()
    val scale = androidx.compose.animation.core.animateFloatAsState(
        targetValue = if (pressed && enabled) 0.98f else 1f,
        animationSpec = tween(if (pressed) NowenMotion.PressInMs else NowenMotion.PressOutMs),
        label = "hills_press_scale",
    )
    Box(
        modifier = modifier
            .graphicsLayer {
                scaleX = scale.value
                scaleY = scale.value
            }
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                onClick = onClick,
            ),
        content = content,
    )
}

@Composable
fun HillsEmptyState(
    title: String,
    message: String,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier,
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(
            modifier = Modifier
                .size(92.dp)
                .clip(RoundedCornerShape(28.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant),
            contentAlignment = Alignment.Center,
        ) {
            Text("+", style = MaterialTheme.typography.headlineLarge, color = MaterialTheme.colorScheme.primary)
        }
        Text(title, style = MaterialTheme.typography.titleLarge)
        Text(
            message,
            style = MaterialTheme.typography.bodyLarge,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
fun HillsTechnicalRow(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier.fillMaxWidth().padding(vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, style = MaterialTheme.typography.labelLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(12.dp))
        Text(value, modifier = Modifier.weight(1f), style = MaterialTheme.typography.bodyMedium, maxLines = 2)
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
