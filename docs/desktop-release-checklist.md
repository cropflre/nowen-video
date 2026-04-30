# NoWen Video 桌面端 — M1~M7 验收清单

> 路线图最终里程碑：本清单用于 release 前完整回归测试，覆盖 M1~M7 全部承诺功能。
> 每项打 ☐（未测）/ ✅（通过）/ ❌（失败）。

## M1 — Hills 外观

- ☐ 启动后窗口背景为 Mica（Win11）/ Acrylic（Win10）毛玻璃
- ☐ 主窗口自绘标题栏三连按钮（最小化/最大化/关闭）可用
- ☐ 标题栏双击切换最大化
- ☐ 切换深/浅色主题时视觉层级对齐
- ☐ 系统托盘图标可见，右键菜单"打开主界面/退出"有效

## M2 — 资源系统

- ☐ `scripts/fetch-assets.ps1` 一键下载 mpv / libmpv / Anime4K shaders / yt-dlp
- ☐ `desktop/src-tauri/resources/mpv/libmpv-2.dll` 存在，大小 ≈ 115MB
- ☐ `desktop/src-tauri/resources/shaders/*.glsl` ≥ 39 个官方 shader
- ☐ 打包后 NSIS 安装程序内包含 `resources/` 完整目录

## M3 — 嵌入播放器骨架

- ☐ 打开媒体详情页 → 点击"播放"进入 PlayerPage
- ☐ 桌面端自动选择 `mpv-embed` 引擎（右下角徽章 `libmpv · gpu-next`）
- ☐ 嵌入子窗口与 React 容器尺寸/位置精确对齐（滚动/窗口缩放无抖动）
- ☐ 控制条 3s 无操作自动隐藏，鼠标进入再出现
- ☐ 返回键退出播放器回到详情页

## M4 — Anime4K / HDR 深度联调

- ☐ SDR 片源无 HDR 徽章；HDR10 片源显示橙色 `HDR10` 徽章
- ☐ 4K / 2K / 1080p 分辨率徽章与实际源一致
- ☐ Anime4K 面板切换 off → low → medium → high 实时生效，无黑屏
- ☐ Anime4K High 档在 RTX 3060/RX 6600 以上保持 60fps
- ☐ 拖拽进度条 seek 响应 < 500ms
- ☐ 时长显示准确（含 > 1h 片源的 hh:mm:ss 格式）

## M5 — Fluent v9 全站视觉

- ☐ 所有 Menu / Popover / Dialog 具有毛玻璃背景（不再是纯色）
- ☐ Input/Combobox focus 时显示霓虹蓝 ring
- ☐ Primary 按钮为渐变霓虹色，悬浮有发光
- ☐ 全局滚动条细条、hover 时变霓虹色
- ☐ 浅色主题下 Portal 组件也正确使用浅色毛玻璃
- ☐ `@media (prefers-reduced-motion: reduce)` 时动画被弱化

## M6 — 画中画 / 投屏

- ☐ 播放时点击画中画按钮 → 窗口缩小到屏幕右下（480×270），去装饰、始终置顶
- ☐ PiP 模式下播放持续不中断（mpv 不重启）
- ☐ 再次点击退出 PiP，窗口恢复到原尺寸/位置（含最大化状态）
- ☐ 点击 Pin 按钮切换始终置顶，图标高亮
- ☐ 投屏按钮能弹出 CastPanel（若服务器配置了 DLNA）

## M7 — 打包验收

- ☐ `npm run tauri build --bundles nsis` 在 Windows 本地成功
- ☐ NSIS 产物能在干净的 Windows 11 上双击安装
- ☐ 安装后"开始菜单"出现 NoWen Video 快捷方式
- ☐ 首次启动耗时 < 5s（含 sidecar 启动）
- ☐ GitHub Actions `release-desktop.yml` 在 tag push 时自动构建
- ☐ 自动生成的 GitHub Release（draft）附带 .exe / .msi
- ☐ `.jwt_secret` 在 data 目录下持久化，容器重启后旧 token 继续有效
- ☐ docker-compose 健康检查返回 200

## 冒烟脚本（开发本地）

```powershell
# 全量 check
cd d:\UGit\nowen-video
cd desktop\src-tauri ; cargo check --features embed-mpv ; cd ..\..
cd web ; npx tsc -b --noEmit ; npm run build ; cd ..
go build ./...

# 快速运行桌面端
cd desktop ; npm run tauri dev
```

## 已知限制

- libmpv 子窗口在极端 DPI 缩放（175%/200%）下 1px 对不齐 —— 已用 rAF 缓解
- 移动端 H5 打开详情页时嵌入播放器回退到 HLS/WebCodecs
- Anime4K 的 AutoDownscalePre 仅在 VL 链上能看出视觉差异；低档位可能无效果
