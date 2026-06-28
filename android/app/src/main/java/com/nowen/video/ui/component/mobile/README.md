# 移动端 Compose 组件库

Hills Pro 风格的移动端 UI 组件库，用于 nowen-video Android 原生端重构。

## 组件列表

| 组件 | 说明 |
|------|------|
| `AppScaffold` | 移动端应用脚手架，处理背景和安全区 |
| `FloatingGlassBottomBar` | 悬浮毛玻璃底部导航栏 |
| `MobilePageHeader` | 页面标题（大字号 + 操作按钮） |
| `MediaPosterCard` | 媒体海报卡片（大圆角 + 进度条） |
| `EmptyState` | 空状态组件 |

## 设计 Token

### 颜色 (MobileColors)

```kotlin
MobileColors.Bg         // #F8F5FB - 背景色
MobileColors.BgAlt      // #F4F1F8 - 备用背景
MobileColors.Text       // #171820 - 文字色
MobileColors.Muted      // #6B6E7A - 次要文字
MobileColors.Primary    // #142060 - 主题色
MobileColors.PrimarySoft// #DFE2FF - 浅主题色
MobileColors.Card       // rgba(255,255,255,0.72) - 卡片背景
MobileColors.Glass      // rgba(246,244,252,0.82) - 毛玻璃
```

### 圆角 (MobileRadius)

```kotlin
MobileRadius.xs   // 8.dp
MobileRadius.sm   // 12.dp
MobileRadius.md   // 16.dp
MobileRadius.lg   // 20.dp
MobileRadius.xl   // 24.dp
MobileRadius.full // 999.dp
```

### 间距 (MobileSpacing)

```kotlin
MobileSpacing.xs  // 8.dp
MobileSpacing.sm  // 12.dp
MobileSpacing.md  // 16.dp
MobileSpacing.lg  // 24.dp
MobileSpacing.xl  // 32.dp
```

### 字号 (MobileFontSize)

```kotlin
MobileFontSize.xs   // 12.sp
MobileFontSize.sm   // 14.sp
MobileFontSize.md   // 16.sp
MobileFontSize.lg   // 18.sp
MobileFontSize.xl   // 22.sp
MobileFontSize.xxxl // 32.sp
```

## 使用示例

### AppScaffold

```kotlin
AppScaffold(
    showBottomBar = true,
    bottomBar = {
        FloatingGlassBottomBar(
            items = navItems,
            selectedKey = selectedTab,
            onItemClick = { selectedTab = it },
        )
    },
) { paddingValues ->
    // 内容
}
```

### MediaPosterCard

```kotlin
MediaPosterCard(
    title = "电影名称",
    year = 2024,
    imageUrl = "https://...",
    progress = 0.45f,
    onClick = { /* 点击事件 */ },
)
```

### EmptyState

```kotlin
EmptyState(
    icon = Icons.Default.FavoriteBorder,
    title = "还没有收藏",
    subtitle = "点亮喜欢的影片后会出现在这里",
)
```
