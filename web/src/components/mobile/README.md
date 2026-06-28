# 移动端组件库

Hills Pro 风格的移动端 UI 组件库，用于 nowen-video 的移动端重构。

## 组件列表

### 基础组件

| 组件 | 说明 | 用法 |
|------|------|------|
| `MobileShell` | 移动端外壳，处理背景和安全区 | `<MobileShell>...</MobileShell>` |
| `FloatingTabBar` | 悬浮毛玻璃底部导航 | `<FloatingTabBar items={...} />` |
| `MobilePageHeader` | 页面标题 | `<MobilePageHeader title="..." />` |
| `SegmentedTabs` | 分段式 Tab 切换 | `<SegmentedTabs tabs={...} />` |

### 媒体组件

| 组件 | 说明 | 用法 |
|------|------|------|
| `MediaPosterCard` | 媒体海报卡片 | `<MediaPosterCard title="..." />` |
| `MediaRail` | 横向滚动轨道 | `<MediaRail title="...">...</MediaRail>` |
| `MobileHeroCarousel` | Hero 轮播 | `<MobileHeroCarousel items={...} />` |

### 服务器组件

| 组件 | 说明 | 用法 |
|------|------|------|
| `ServerCard` | 服务器卡片 | `<ServerCard name="..." />` |
| `FloatingActionButton` | 悬浮操作按钮 | `<FloatingActionButton icon={...} />` |

### 设置组件

| 组件 | 说明 | 用法 |
|------|------|------|
| `MobileSettingGroup` | 设置分组 | `<MobileSettingGroup title="...">` |
| `MobileSettingItem` | 设置项 | `<MobileSettingItem title="..." />` |

### 搜索组件

| 组件 | 说明 | 用法 |
|------|------|------|
| `MobileSearchBar` | 搜索框 | `<MobileSearchBar />` |

## 设计 Token

```typescript
import { mobileTokens } from '@/styles/mobile-tokens'

// 背景色
mobileTokens.bg        // #F8F5FB
mobileTokens.bgAlt     // #F4F1F8

// 文字色
mobileTokens.text      // #171820
mobileTokens.textMuted // #6B6E7A

// 主题色
mobileTokens.primary      // #142060
mobileTokens.primarySoft  // #DFE2FF

// 圆角
mobileTokens.radius.lg   // 20px
mobileTokens.radius.xl   // 24px
mobileTokens.radius.full // 999px

// 间距
mobileTokens.spacing.xl  // 32px
```

## 钩子函数

```typescript
import { useIsMobile, useBreakpoint } from '@/hooks/useMobile'

// 检测是否为移动端
const isMobile = useIsMobile()

// 获取当前断点
const breakpoint = useBreakpoint() // 'mobile' | 'tablet' | 'desktop'
```

## 使用示例

### 服务器页面

```tsx
import { MobileShell, MobilePageHeader, ServerCard } from '@/components/mobile'

export default function ServersPage() {
  return (
    <MobileShell>
      <MobilePageHeader title="服务器" />
      <ServerCard name="emby" lastAccess="27 天前" />
    </MobileShell>
  )
}
```

### 设置页面

```tsx
import { MobileShell, MobileSettingGroup, MobileSettingItem } from '@/components/mobile'
import { Globe, Palette } from 'lucide-react'

export default function SettingsPage() {
  return (
    <MobileShell>
      <MobilePageHeader title="设置" />
      <MobileSettingGroup title="通用">
        <MobileSettingItem icon={<Globe />} title="语言" value="Auto" />
        <MobileSettingItem icon={<Palette />} title="主题" />
      </MobileSettingGroup>
    </MobileShell>
  )
}
```

### 媒体网格

```tsx
import { MediaPosterCard } from '@/components/mobile'

export default function MediaGrid() {
  return (
    <div className="mobile-media-grid">
      {items.map(item => (
        <MediaPosterCard
          key={item.id}
          title={item.title}
          year={item.year}
          progress={item.progress}
        />
      ))}
    </div>
  )
}
```

## 样式

### CSS 变量

```css
:root {
  --mobile-bg: #F8F5FB;
  --mobile-text: #171820;
  --mobile-primary: #142060;
  --mobile-card: rgba(255, 255, 255, 0.72);
  --mobile-glass: rgba(246, 244, 252, 0.82);
}
```

### Tailwind 类

```html
<!-- 移动端安全区 -->
<div class="mobile-safe-area">...</div>

<!-- 移动端媒体网格 -->
<div class="mobile-media-grid">...</div>

<!-- 移动端毛玻璃 -->
<div class="mobile-glass">...</div>
```

## 设计规范

### 圆角

- 页面卡片：24px - 30px
- 海报卡片：16px - 20px
- 按钮：18px - 999px
- 底部导航：999px

### 间距

- 页面左右 padding：32px
- 标题下间距：32px
- 分组间距：36px
- 网格 gap：28px - 32px
- 底部预留：112px

### 字号

- 页面标题：32px - 36px
- 分组标题：22px - 26px
- 卡片标题：18px - 22px
- 辅助文字：14px - 16px
- 底部导航文字：14px - 16px

## 注意事项

1. 所有组件都使用 Framer Motion 进行动画
2. 图片使用 lazy loading
3. 支持安全区（刘海屏、手势条）
4. 底部导航使用毛玻璃效果
5. 卡片点击有按压缩放反馈
6. 横向滚动隐藏滚动条
