
# 🖥️ nowen-video Desktop（PC 端）

基于 **Tauri 2.0 + libmpv** 的跨平台桌面客户端，为 Windows / macOS / Linux 提供**原画原音**的极致观影体验。

## ✨ 特性

### 🎬 播放能力
- **双内核播放**：Web `<video>` + 原生 mpv，策略引擎自动选择
- **libmpv 嵌入**（可选）：mpv 渲染直接嵌入 Tauri 窗口，零闪烁、前端 UI 可控
- **全格式支持**：4K / HEVC / AV1 / HDR10+ / 杜比视界 / DTS / TrueHD / Atmos 零转码
- **字幕完美**：libass 渲染 ASS/PGS 字幕

### 🪶 系统集成
- ✅ **系统托盘**：最小化后驻留，右键菜单快速访问
- ✅ **原生菜单栏**：文件 / 播放 / 工具 / 帮助 四级菜单
- ✅ **文件关联**：双击 `.mkv`/`.mp4` 等自动打开
- ✅ **Deep Link**：`nowen-video://play?media_id=123` URL 协议
- ✅ **全局快捷键**：`Ctrl+Shift+N` 从任意位置呼出
- ✅ **单实例**：重复启动自动复用已打开窗口
- ✅ **自动更新**：启动静默检查，一键下载安装

### 🔒 隐私与性能
- **本地优先**：内置 Go 后端 sidecar，断网也能刷本地媒体库
- **极致轻量**：打包产物 ~80MB（Infuse 200MB+、VidHub 150MB+）
- **秒启动**：Tauri WebView 共享系统内核，冷启动 <500ms
- **低内存**：120-180MB（Electron 同类 400MB+）

## 🏗️ 架构

```
┌─────────────────────────────────────────────────────┐
│         Tauri 2.0 桌面壳（Rust + WebView2）         │
├─────────────────────────────────────────────────────┤
│                                                     │
│  前端（复用 web/ 构建产物）                          │
│   ├─ 海报墙 / 详情页 / 设置                          │
│   ├─ DesktopEventBinder（菜单/文件/DeepLink 桥接）   │
│   ├─ UpdateBanner（更新提示）                        │
│   ├─ DesktopPlayerBadge（播放内核切换）              │
│   └─ MpvEmbedPlayer（M4 嵌入式播放器）               │
│             ↓ Tauri invoke / event                  │
│                                                     │
│  Rust 壳层（src-tauri/src/）                        │
│   ├─ main.rs          应用入口                      │
│   ├─ sidecar.rs       Go sidecar 生命周期           │
│   ├─ mpv.rs           mpv 双模式（进程+FFI）        │
│   ├─ embed_window.rs  M4 嵌入窗口                   │
│   ├─ strategy.rs      播放决策引擎                  │
│   ├─ tray.rs          M7 菜单/托盘                  │
│   ├─ updater.rs       M5 自动更新                   │
│   ├─ file_assoc.rs    M6 文件关联/DeepLink          │
│   ├─ settings.rs      持久化设置                    │
│   └─ commands.rs      IPC 命令                      │
│             ↓ HTTP localhost:8080                   │
│                                                     │
│  Go sidecar（复用 cmd/server）                      │
│   ├─ 元数据 / 刮削 / 推荐                            │
│   ├─ 媒体库 / 搜索 / 字幕                            │
│   └─ 按需转码（仅边界场景）                          │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## 🚀 快速开始

### 前置依赖

- **Rust** ≥ 1.77（[安装](https://www.rust-lang.org/tools/install)）
- **Node.js** ≥ 18
- **Go** ≥ 1.22
- **Tauri CLI**：`cargo install tauri-cli --version "^2.0" --locked`
- **mpv**（外部进程模式必需）
  - Windows: `winget install mpv`
  - macOS: `brew install mpv`
  - Linux: `apt install mpv`
- **libmpv**（可选，M4 嵌入模式必需）
  - Windows: 下载 [mpv-lib](https://sourceforge.net/projects/mpv-player-windows/files/libmpv/) 解压至 `%PATH%`
  - macOS: `brew install mpv`（自带 libmpv）
  - Linux: `apt install libmpv-dev`

### 开发模式（一键启动）

```powershell
# Windows
pwsh desktop/scripts/dev.ps1

# macOS / Linux
bash desktop/scripts/dev.sh
```

该脚本会自动：
1. 构建 Go sidecar 到 `desktop/bin/`
2. 启动 Vite dev server（:3000）
3. 启动 Tauri dev 模式（调试模式热重载）

### 生产打包

```powershell
# Windows
pwsh desktop/scripts/build-sidecar.ps1 -Production
cd desktop/src-tauri
cargo tauri build

# 启用 libmpv 嵌入模式
cargo tauri build -- --features embed-mpv
```

产物：
- Windows: `desktop/src-tauri/target/release/bundle/nsis/*.exe`
- macOS: `desktop/src-tauri/target/release/bundle/dmg/*.dmg`
- Linux: `desktop/src-tauri/target/release/bundle/appimage/*.AppImage`

## 🎬 播放策略

应用启动时自动决策播放内核：

| 场景 | 内核 | 说明 |
|---|---|---|
| MP4 + H.264 + AAC | Web | 浏览器原生，启动快 |
| MKV / HEVC / AV1 / VC-1 | **mpv** | 浏览器不支持，必须 mpv |
| DTS / TrueHD / E-AC3 / Atmos | **mpv** | 浏览器音频黑洞 |
| HDR10 / HDR10+ / Dolby Vision | **mpv** | 色彩空间正确处理 |
| ASS / PGS 字幕 | **mpv** | libass 完美渲染 |

用户也可在设置中全局强制：**`自动` / `总是 mpv` / `总是 Web`**。

## 🎮 快捷键

| 快捷键 | 功能 |
|---|---|
| `Ctrl+Shift+N` | 全局：从任意位置唤出窗口 |
| `Ctrl+O` | 打开文件 |
| `Ctrl+Shift+O` | 打开文件夹 |
| `Ctrl+,` | 打开设置 |
| `Ctrl+Q` | 退出 |
| `F11` | 切换全屏 |
| `Space` | 播放/暂停 |

## 🔗 Deep Link 协议

其他应用/网页可以通过以下 URL 直接调用：

```
nowen-video://play?media_id=12345    # 跳转播放
nowen-video://settings                # 打开设置
```

## 🔄 自动更新

- 启动后 3 秒静默检查更新
- 有新版本时右下角弹出横幅
- 点击"下载并安装"自动完成所有流程
- 基于 Tauri Updater 签名校验，防篡改

### 配置发布端点

编辑 [`tauri.conf.json`](./src-tauri/tauri.conf.json) 中的 `plugins.updater.endpoints`：

```json
{
  "plugins": {
    "updater": {
      "endpoints": [
        "https://github.com/YOUR_ORG/nowen-video/releases/latest/download/latest.json"
      ],
      "pubkey": "<你的公钥>"
    }
  }
}
```

生成签名密钥：

```bash
cargo tauri signer generate -w ~/.tauri/myapp.key
```

## 📁 文件关联

打包后应用会自动注册以下扩展名（NSIS/DMG/DEB 安装器一并处理）：

`mp4` `mkv` `avi` `mov` `wmv` `flv` `webm` `ts` `m2ts` `rmvb` `m4v`

用户双击任一视频文件，会自动唤起本应用并加载播放。

## ⚙️ 配置文件

应用配置持久化在：

- Windows: `%APPDATA%\nowen-video\settings.json`
- macOS: `~/Library/Application Support/nowen-video/settings.json`
- Linux: `~/.config/nowen-video/settings.json`

完整字段：

```json
{
  "server": {
    "mode": "embedded",
    "remote_url": "",
    "sidecar_port": 8080
  },
  "player": {
    "engine": "auto",
    "mpv_path": "",
    "mpv_args": ["--hwdec=auto-safe", "--keep-open=yes"],
    "hardware_accel": true
  },
  "window": {
    "width": 1400,
    "height": 900,
    "remember_size": true,
    "minimize_to_tray": false
  }
}
```

## 🗺️ 路线图

- [x] **M0** Tauri 骨架 + 加载现有前端
- [x] **M1** Go sidecar 自动启动/退出
- [x] **M2** mpv 外部进程播放内核
- [x] **M3** 策略决策引擎 + 前端 desktopBridge
- [x] **M4** libmpv FFI 嵌入原生窗口（`--features embed-mpv`）
- [x] **M5** 自动更新（Tauri Updater + GitHub Releases）
- [x] **M6** 文件关联 + Deep Link 协议（nowen-video://）
- [x] **M7** 原生菜单 + 托盘 + 全局快捷键 + 单实例

## 🔧 故障排查

### "未找到 mpv 可执行文件"
- 安装 mpv，或在设置中指定完整路径
- Windows 注意避免 `C:\Users\X\AppData\Local\Microsoft\WindowsApps\` 下的别名

### "嵌入式 mpv 启动失败"
- 确保编译时加了 `--features embed-mpv`
- 确保系统装了 libmpv（非 mpv 二进制，是开发库）

### 自动更新不生效
- 检查 `tauri.conf.json` 的 endpoints 是否正确
- 签名密钥必须与发布包匹配
- 开发模式下不检查更新

### Windows 双击文件没反应
- 用管理员权限运行一次安装包
- 或手动在"默认应用"中指定 nowen-video

## 📝 许可

与主项目一致。
