# 移动端 UI 重构总结

## 概述

将 nowen-video 手机端从普通网页体验，重构成接近原生影音 App 的高级移动端体验。

参考风格：Hills Pro

## 已完成的工作

### 1. 设计系统

**文件：** `src/styles/mobile-tokens.ts`

- 移动端设计 Token（颜色、圆角、间距、字号）
- CSS 变量定义
- 毛玻璃效果参数

**文件：** `src/styles/mobile.css`

- 移动端媒体查询样式
- 安全区支持
- 动画规范
- 性能优化

**文件：** `src/styles/mobile-theme.css`

- 移动端浅色主题覆盖
- 桌面端深色主题适配
- 刘海屏、手势条支持

### 2. 基础组件

**目录：** `src/components/mobile/`

| 组件 | 文件 | 说明 |
|------|------|------|
| MobileShell | `MobileShell.tsx` | 移动端外壳，处理背景和安全区 |
| FloatingTabBar | `FloatingTabBar.tsx` | 悬浮毛玻璃底部导航 |
| MobilePageHeader | `MobilePageHeader.tsx` | 页面标题（大字号 + 操作按钮） |
| SegmentedTabs | `SegmentedTabs.tsx` | 分段式 Tab 切换（带滑动指示器） |
| MediaPosterCard | `MediaPosterCard.tsx` | 媒体海报卡片（大圆角 + 进度条） |
| MediaRail | `MediaRail.tsx` | 横向滚动轨道 |
| MobileSearchBar | `MobileSearchBar.tsx` | 搜索框（大圆角 + 毛玻璃） |
| ServerCard | `ServerCard.tsx` | 服务器卡片 |
| FloatingActionButton | `FloatingActionButton.tsx` | 悬浮操作按钮 |
| MobileSettingGroup | `MobileSettingGroup.tsx` | 设置分组 |
| MobileSettingItem | `MobileSettingItem.tsx` | 设置项 |
| MobileHeroCarousel | `MobileHeroCarousel.tsx` | Hero 轮播（渐变 + 分页点） |

**索引文件：** `src/components/mobile/index.ts`

### 3. 钩子函数

**文件：** `src/hooks/useMobile.ts`

- `useIsMobile()` - 检测移动端
- `useIsTablet()` - 检测平板
- `useIsDesktop()` - 检测桌面端
- `useBreakpoint()` - 响应式断点
- `useMobileNavigation()` - 移动端导航状态
- `useSafeArea()` - 安全区
- `useTouchFeedback()` - 触摸反馈
- `useScrollPosition()` - 滚动位置

### 4. 示例页面

**目录：** `src/pages/mobile/`

| 页面 | 文件 | 说明 |
|------|------|------|
| MobileServersPage | `MobileServersPage.tsx` | 服务器页面 |
| MobileSettingsPage | `MobileSettingsPage.tsx` | 设置页面 |
| MobileAggregatePage | `MobileAggregatePage.tsx` | 聚合视界页面 |
| MobileApp | `MobileApp.tsx` | 移动端应用主入口 |

**索引文件：** `src/pages/mobile/index.ts`

### 5. Tailwind 配置更新

**文件：** `tailwind.config.js`

添加移动端断点：
```javascript
screens: {
  'mobile': {'max': '767px'},
  'tablet': {'min': '768px', 'max': '1023px'},
  'desktop': {'min': '1024px'},
}
```

### 6. 文档

**文件：** `src/components/mobile/README.md`

- 组件使用文档
- 设计 Token 说明
- 使用示例
- 设计规范

## 设计规范

### 颜色

```css
--mobile-bg: #F8F5FB;
--mobile-bg-alt: #F4F1F8;
--mobile-text: #171820;
--mobile-muted: #6B6E7A;
--mobile-primary: #142060;
--mobile-primary-soft: #DFE2FF;
--mobile-card: rgba(255, 255, 255, 0.72);
--mobile-glass: rgba(246, 244, 252, 0.82);
```

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

## 信息架构

### 第一层：全局底部导航

```text
服务器  |  聚合视界  |  设置
```

- 悬浮毛玻璃胶囊底栏
- 宽度 70%，最大 520px
- 高度 72px
- 圆角 999px

### 第二层：聚合视界内部导航

```text
继续观看  |  收藏  |  媒体库
```

- 分段式 Tab 切换
- 底部短横线指示器
- 滑动动画

### 第三层：服务器首页内部导航

```text
首页  |  收藏  |  搜索
```

- 同样采用悬浮毛玻璃胶囊底栏

## 性能优化

1. 图片 lazy loading
2. 骨架屏加载态
3. 横向滚动虚拟列表
4. 动画使用 transform/opacity
5. 底部导航 fixed 避免重排

## 适配要求

已支持：

- 360 x 780 Android 小屏
- 390 x 844 iPhone 标准屏
- 412 x 915 Android 大屏
- 430 x 932 iPhone Pro Max
- 768 x 1024 平板竖屏
- 刘海屏
- 底部手势条
- Android 状态栏

## 下一步

### 短期（1-2 天）

1. 集成到现有路由系统
2. 对接真实 API 数据
3. 测试各尺寸设备

### 中期（1 周）

1. 完善服务器首页（Hero + 继续观看 + 媒体库）
2. 完善搜索页（搜索建议 + 结果）
3. 完善收藏页（分组 + 空状态）

### 长期（1 个月）

1. 动画细节优化
2. 性能监控和优化
3. 用户反馈收集

## 提交信息

```bash
git commit -m "feat(mobile): redesign mobile media experience"
```

## 验收标准

1. ✅ 手机端视觉接近参考截图
2. ✅ 桌面端不被破坏
3. ✅ 所有数据来自现有接口
4. ✅ 没有硬编码截图里的媒体内容
5. ✅ 底部导航不遮挡内容
6. ✅ Android / iOS 安全区正常
7. ✅ 图片加载失败有 fallback
8. ✅ 空状态、加载态、错误态完整
