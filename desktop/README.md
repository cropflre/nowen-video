# Nowen Video Desktop 2.0

Nowen Video Desktop 2.0 是桌面端正式架构基线。旧桌面端从未进入正式环境，因此 2.0 不保留 Web/mpv 双内核、外部 mpv 进程、播放器策略器和 Tauri v1 兼容层。

## 技术栈

- **UI**：React 18 + TypeScript + Vite，复用 `web/` 产品界面与设计系统。
- **Desktop Runtime**：Tauri 2 + Rust。
- **Player Core**：libmpv，桌面端唯一正式播放器内核。
- **Media Core**：Go `cmd/server-lite` sidecar，复用正式服务端媒体库、刮削、搜索、字幕、FFmpeg 与 SQLite 能力。
- **Web Player**：只属于浏览器平台，继续使用 Direct / WebCodecs / Remux / HLS，不参与 Desktop 播放内核选择。

## 架构

```text
React / TypeScript
        │
        │ platform/desktop + custom Tauri IPC / events
        ▼
Tauri 2 / Rust Runtime
        │
        ├── Player Core (libmpv)
        │      ├── control handle
        │      ├── independent event client
        │      └── Player Surface
        │
        └── Go Media Core sidecar
                 │
                 ├── SQLite
                 ├── FFmpeg
                 ├── metadata / scraper
                 ├── subtitle / search
                 └── media API
```

职责边界：

- React 只负责产品 UI、交互和状态展示。
- Rust 负责窗口、托盘、Deep Link、文件关联、更新入口、Sidecar 生命周期和原生播放器。
- Go 负责媒体业务，不在 Rust 中复制服务端业务逻辑。
- mpv 是 Rust Player Core 的实现细节，React 业务层只使用 `DesktopPlayer` / `player_*` API。

## 目录

```text
desktop/
├── scripts/
│   ├── build-sidecar.ps1
│   ├── build-sidecar.sh
│   ├── dev.ps1
│   └── dev.sh
└── src-tauri/
    ├── capabilities/
    ├── bin/
    ├── resources/
    └── src/
        ├── app/              # 应用启动与生命周期
        ├── runtime/          # 全局运行时状态
        ├── player/           # Desktop Player Core + Surface + event pump
        ├── player_commands.rs# 低频轨道/章节查询 IPC
        ├── commands.rs       # 通用产品级自定义 IPC
        ├── sidecar.rs        # Go Media Core 生命周期
        ├── settings.rs
        ├── file_assoc.rs
        ├── tray.rs
        ├── updater.rs
        └── vibrancy.rs
```

Web 侧桌面适配入口：

```text
web/src/platform/desktop/bridge.ts
web/src/desktop/DesktopPlayer.tsx
web/src/desktop/DesktopEventBinder.tsx
web/src/desktop/DesktopServerPicker.tsx
```

## Media Core 运行方式

内嵌模式启动时由 Rust：

1. 在 `127.0.0.1` 选择动态空闲端口；
2. 设置 `NOWEN_APP_PORT=<port>`；
3. 设置 `NOWEN_DESKTOP_RUNTIME=1`；
4. 启动 `nowen-video-server-<target-triple>`；
5. Web 平台桥接从 Tauri IPC 获取真实端口；
6. API、海报/背景图和原生播放器媒体 URL 全部使用同一个运行时服务器地址。

Desktop Runtime 下 Go Media Core 只监听 `127.0.0.1`，并关闭 mDNS 服务发现。远程模式仍可连接 NAS / Docker 部署的 Nowen Video Server。

## 播放器规则

Desktop 2.0 普通播放固定使用原生 Player Core，不再存在：

- `auto / web / mpv` 设置；
- `decide_engine`；
- 外部 `mpv.exe` 播放进程；
- `DesktopPlayerBadge`；
- Web 播放失败后切 mpv 或 mpv 失败后回 Web 的双向回退。

如果桌面 Player Core 不可用，客户端会明确提示运行时错误，而不是静默切回浏览器播放器。

### 状态同步

Player Core 已改为事件驱动，不再由 React 每 750ms 轮询：

```text
libmpv observe_property
        │
        ▼
independent mpv client event queue
        │
        ▼
Rust player-state event
        │
        ▼
Desktop bridge
        │
        ▼
DesktopPlayer + Zustand
```

主控制 handle 负责命令和属性设置；独立 libmpv client 只消费 `observe_property` / `mpv_wait_event`，因此两者不会争抢同一个事件队列。前端在订阅建立后只读取一次 bootstrap 快照，之后由事件更新进度、时长、播放/暂停、音量、静音、分辨率、codec 和 HDR 基础状态。

### 音轨、字幕与章节

轨道和章节属于低频结构数据，不塞进高频 `player-state` payload。Player Core 在以下时机发出 `media-info-change`：

- `aid` / `sid` / `chapter` 变化；
- `track-list` / `chapter-list` 变化；
- 文件加载完成或事件队列需要重新同步。

React 收到事件后按需读取 `player_media_info`。播放器 UI 当前支持：

- 明确选择音轨，不再盲目 cycle；
- 字幕开/关和指定字幕轨；
- 显示语言、codec、默认/强制/外挂标记；
- 显示章节标题与时间并跳转章节；
- 当前音轨、字幕和章节选中状态同步。

## Player Surface / Render API

当前 Windows 首发实现把原生渲染 Surface 封装在 `player/surface.rs`，PlayerManager 和 Web IPC 不再接触 `wid` / HWND。

libmpv Render API 不是把视频直接绘制进 React DOM；它要求宿主拥有可供 libmpv 渲染的图形上下文/帧缓冲。因此下一阶段只在 `player/surface.rs` 与 NativePlayer renderer 初始化边界内实现 app-owned native GL surface + libmpv Render Context，不再改 React、IPC 和状态管理。

## 安全基线

- Tauri 已启用 CSP，不再使用 `csp: null`。
- 不再开放全局 `assetProtocol: **`。
- WebView Capability 只保留核心事件监听和标题栏拖拽能力。
- Web 业务不直接访问 `window.__TAURI__` / `window.__TAURI_INTERNALS__`。
- 文件、窗口、系统能力统一通过自定义 Rust IPC 暴露。
- 内嵌 Go Media Core 使用动态端口并仅监听 loopback。

## 开发

Windows：

```powershell
pwsh desktop/scripts/dev.ps1
```

强制重建 Sidecar：

```powershell
pwsh desktop/scripts/dev.ps1 -RebuildSidecar
```

macOS / Linux：

```bash
bash desktop/scripts/dev.sh
```

脚本会统一从 `./cmd/server-lite` 构建正式 Go Media Core，并把 Tauri sidecar 产物放到：

```text
desktop/src-tauri/bin/nowen-video-server-<target-triple>[.exe]
```

## CI

`.github/workflows/desktop2-ci.yml` 对 `refactor/server-lite-v1` 的 Desktop/Web/Media Core 改动执行：

- Web：`npm ci` + `npm run build`
- Go：`go build ./cmd/server-lite`
- Windows Desktop：获取 libmpv 资源后执行 `cargo check --all-targets`

正式发布流水线使用同一个 `cmd/server-lite` 和同一个 `nowen-video-server-<target-triple>` Sidecar 命名。

## 自动更新

Tauri Updater 代码保留，但在正式签名公钥和更新产物配置完成前默认关闭。只有编译时显式设置：

```text
NOWEN_DESKTOP_UPDATER_ENABLED=1
```

才会执行更新检查与安装。不要在公钥为空或未生成签名更新产物时启用。

## Desktop 2.0 首发边界

首发目标优先保证 Windows x64：媒体库、详情/搜索、Local/Remote Server、原生 libmpv 播放、进度/音量/字幕/音轨/章节、硬件解码、HDR 基础链路、全屏、文件关联、Deep Link 和应用生命周期。

Anime4K、高级 Shader、复杂 PiP/色彩调校等不作为 2.0 首发核心能力。

尚未完成且不能标记为正式完成的深层项：

- `player/surface.rs` 从当前原生窗口承载切换到 app-owned graphics surface + libmpv Render API；
- 在不破坏 HLS/媒体子请求的前提下增加完整 Desktop Runtime Token 握手；
- 配置真实 Tauri Updater 公钥、签名产物并开启更新开关。

这些工作已经被隔离在 Player/Runtime/Updater 边界内，不需要恢复旧双内核架构。
