# Nowen Video Desktop 2.0 架构契约门禁。
#
# 目的：防止后续改动把正式桌面端悄悄退回旧的双内核 / wid / 第二 WebView / 轮询架构。
# 该脚本只检查稳定的产品级边界，不绑定实现细节到具体函数行号。

$ErrorActionPreference = "Stop"

$ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$DesktopRoot = Split-Path -Parent $ScriptRoot
$ProjectRoot = Split-Path -Parent $DesktopRoot

function Read-RequiredFile([string]$RelativePath) {
    $path = Join-Path $ProjectRoot $RelativePath
    if (-not (Test-Path $path)) {
        throw "Desktop 2.0 架构门禁失败：缺少文件 $RelativePath"
    }
    return Get-Content -Path $path -Raw -Encoding UTF8
}

function Assert-Contains([string]$Content, [string]$Needle, [string]$Message) {
    if (-not $Content.Contains($Needle)) {
        throw "Desktop 2.0 架构门禁失败：$Message"
    }
}

function Assert-NotContains([string]$Content, [string]$Needle, [string]$Message) {
    if ($Content.Contains($Needle)) {
        throw "Desktop 2.0 架构门禁失败：$Message"
    }
}

function Assert-FileMissing([string]$RelativePath, [string]$Message) {
    if (Test-Path (Join-Path $ProjectRoot $RelativePath)) {
        throw "Desktop 2.0 架构门禁失败：$Message"
    }
}

Write-Host "检查 Desktop 2.0 架构契约..." -ForegroundColor Cyan

# 旧桌面架构必须保持删除状态。
Assert-FileMissing "desktop/src-tauri/src/mpv.rs" "旧根目录 mpv.rs 不允许恢复"
Assert-FileMissing "desktop/src-tauri/src/embed_window.rs" "旧第二 WebView embed_window.rs 不允许恢复"
Assert-FileMissing "desktop/src-tauri/src/strategy.rs" "旧播放器策略器 strategy.rs 不允许恢复"

$player = Read-RequiredFile "desktop/src-tauri/src/player/mod.rs"
$surface = Read-RequiredFile "desktop/src-tauri/src/player/surface/windows.rs"
$desktopPlayer = Read-RequiredFile "web/src/desktop/DesktopPlayer.tsx"
$bridge = Read-RequiredFile "web/src/platform/desktop/bridge.ts"

# Windows 正式播放器必须是 libmpv Render API，而不是 wid。
Assert-Contains $player 'init.set_option("vo", "libmpv")' "Windows Player Core 必须在初始化前启用 vo=libmpv"
Assert-Contains $surface "mpv_render_context_create" "Windows Surface 必须创建 libmpv Render Context"
Assert-Contains $surface "mpv_render_context_render" "Windows Surface 必须通过 Render API 绘制视频帧"
Assert-Contains $surface "mpv_render_context_report_swap" "Windows Surface 必须向 libmpv 报告 swap"
Assert-Contains $surface "SwapBuffers" "Windows Surface 必须由宿主交换 framebuffer"
Assert-Contains $surface "WS_POPUP" "Windows Surface 必须使用纯 Win32 原生窗口"
Assert-NotContains $surface "WebviewWindow" "Windows 视频 Surface 不允许恢复第二个 Tauri WebView"
Assert-NotContains $surface 'set_property("wid"' "Windows 视频 Surface 不允许恢复 wid 嵌入"

# Web 产品层只允许使用正式 Tauri 2 bridge，并由事件驱动同步播放器状态。
Assert-Contains $bridge "@tauri-apps/api/core" "Desktop bridge 必须使用官方 Tauri 2 API"
Assert-Contains $bridge "onPlayerState" "Desktop bridge 必须暴露 player-state 事件订阅"
Assert-NotContains $bridge "__TAURI_INTERNALS__" "业务层不允许直接访问 Tauri 内部全局对象"
Assert-NotContains $bridge "window.__TAURI__" "不允许恢复 Tauri v1 全局对象兼容"
Assert-Contains $desktopPlayer "desktop.onPlayerState" "DesktopPlayer 必须消费 player-state 事件"
Assert-NotContains $desktopPlayer "setInterval(" "DesktopPlayer 不允许恢复高频定时轮询"

Write-Host "[OK] Desktop 2.0 架构契约通过" -ForegroundColor Green
